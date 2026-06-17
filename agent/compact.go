package agent

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// 上下文压缩：历史超限时，与其像 Trim 那样直接丢弃最早的消息，不如让 LLM
// 把旧消息总结成摘要，保留「用户目标、关键事实、读改过的文件、未完成事项」等上下文。
// 摘要会破坏前缀缓存，但只在超限时触发，属于可接受的取舍。

// summarizeFunc 产出一段消息历史的摘要。抽成可注入的函数是为了测试——
// 默认实现走真实 LLM（llmSummarize），测试可替换成桩函数，无需联网。
type summarizeFunc func(ctx context.Context, msgs []openai.ChatCompletionMessage) (string, error)

// 压缩触发参数。压缩按「占模型上下文窗口的比例」触发，而非消息条数——
// 这样窗口越大（DeepSeek V4 已是 1M）平时越压不到，前缀缓存几乎全程命中。
const (
	defaultContextWindow = 1_000_000 // 模型上下文窗口（token）。DeepSeek V4 默认 1M；可经 SetContextWindow 调整
	defaultCompactRatio  = 0.8       // 提示达到窗口该比例时触发压缩（与 Reasonix 默认一致）
	defaultSoftRatio     = 0.5       // 达到该比例时提醒一次（不压缩，保前缀缓存）
	maxTailTokens        = 16384     // 压缩后保留的近消息尾部 token 上限
	fallbackTokPerChar   = 0.25      // 真实用量未知时兜底：约 4 字符/token
)

// maybeCompact 在「上一轮提示的真实 token 数」逼近上下文窗口时才压缩，是 Run 循环每轮调用的钩子。
//
// 时机与缓存的关键取舍：DeepSeek 等按前缀匹配做缓存（命中价仅约未命中的 1/50）。压缩会改写
// 历史前缀、必然打断缓存，因此**只在真接近窗口时压一次**；其余时间维持 append-only，让前缀
// 缓存持续命中。判定基于 provider 回传的真实 prompt_tokens（见 streamChat），比字符估算精确。
func (a *Agent) maybeCompact(ctx context.Context) {
	if a.contextWindow <= 0 || a.lastPromptTokens <= 0 {
		return // 压缩关闭，或还没有真实用量遥测（首轮）
	}
	high := int(float64(a.contextWindow) * a.compactRatio)
	soft := int(float64(a.contextWindow) * a.softCompactRatio)
	switch {
	case a.lastPromptTokens >= high:
		a.compact(ctx, false)
	case a.lastPromptTokens >= soft:
		// 软线到触发线之间：只提醒一次，绝不改写前缀——这里压缩会白白打掉缓存。
		if !a.softCompactNoticed {
			a.softCompactNoticed = true
			a.noticef("  📈 上下文已达窗口 %.0f%%（%d/%d token），到 %.0f%% 才会压缩，期间保持前缀缓存\n",
				float64(a.lastPromptTokens)/float64(a.contextWindow)*100, a.lastPromptTokens, a.contextWindow, a.compactRatio*100)
		}
	default:
		a.softCompactNoticed = false // 回落到软线下，重置一次性提醒
	}
}

// Compact 手动触发一次压缩（供 /compact 命令使用），无视比例阈值。
// 让用户能在前缀自然填满前主动重置一次，把握缓存重建的时机。
func (a *Agent) Compact(ctx context.Context) {
	if a.session.Len() == 0 {
		a.noticef("  🗜️  会话为空，无需压缩\n")
		return
	}
	a.compact(ctx, true)
}

// compact 执行一次压缩：摘要旧消息、保留 token 有界的近消息尾部。
// manual 为手动触发（/compact），仅用于文案区分。摘要失败则退化为裁剪（仍兜住体积）。
func (a *Agent) compact(ctx context.Context, manual bool) {
	tailBudget := a.contextWindow / 4
	if tailBudget <= 0 || tailBudget > maxTailTokens {
		tailBudget = maxTailTokens
	}
	old, recent := a.session.SplitForCompaction(tailBudget, a.tokPerChar())
	if len(old) == 0 {
		if manual {
			a.noticef("  🗜️  历史较短，无需压缩\n")
		}
		return
	}

	summarize := a.summarizer
	if summarize == nil {
		summarize = a.llmSummarize
	}
	summary, err := summarize(ctx, old)
	if err != nil || strings.TrimSpace(summary) == "" {
		a.debugf("  🗜️  摘要失败，退化为裁剪：%v\n", err)
		a.session.Trim()
		a.lastPromptTokens, a.softCompactNoticed = 0, false
		return
	}

	a.session.ApplyCompaction(strings.TrimSpace(summary), recent)
	// 前缀已重写：清零遥测，避免下一轮 maybeCompact 用旧值误判再压一次；下次 streamChat 会刷新。
	a.lastPromptTokens, a.softCompactNoticed = 0, false
	tag := "自动"
	if manual {
		tag = "手动"
	}
	a.noticef("  🗜️  上下文已压缩（%s）：%d 条旧消息折叠为摘要，保留近 %d 条\n", tag, len(old), len(recent))
}

// SetContextWindow 设置模型上下文窗口（token）。<=0 表示关闭自动压缩。
// 切换到更小窗口的模型时务必调小，否则触发线会高于真实窗口、压缩前就先超限报错。
func (a *Agent) SetContextWindow(tokens int) { a.contextWindow = tokens }

// tokPerChar 用「上一轮真实 prompt_tokens / 当前历史字符数」推导每字符的 token 数，
// 让按字符的估算贴合 provider 的分词器，无需本地分词。真实用量未知或比值离谱时兜底为 ~4 字符/token。
func (a *Agent) tokPerChar() float64 {
	if a.lastPromptTokens > 0 {
		if chars := a.session.CharCount(); chars > 0 {
			if r := float64(a.lastPromptTokens) / float64(chars); r > 0.05 && r < 2 {
				return r
			}
		}
	}
	return fallbackTokPerChar
}

const summarizeSystemPrompt = `你是对话摘要助手。请把下面这段 Agent 与用户的历史对话压缩成简洁要点，
务必保留：用户的目标与约束、已查明的关键事实、读过 / 改过的文件与重要结果、尚未完成的事项。
丢弃寒暄与冗余。用中文、分条输出，不要编造未出现的信息。`

// llmSummarize 调用 LLM（非流式）把一段消息历史总结成摘要。
func (a *Agent) llmSummarize(ctx context.Context, msgs []openai.ChatCompletionMessage) (string, error) {
	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: summarizeSystemPrompt},
			{Role: "user", Content: renderForSummary(msgs)},
		},
	})
	if err != nil {
		return "", err
	}
	a.accountUsage(resp.Usage)
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("摘要返回空结果")
	}
	return resp.Choices[0].Message.Content, nil
}

// renderForSummary 把消息历史渲染成纯文本，供摘要 LLM 阅读（工具结果做截断防止过长）。
func renderForSummary(msgs []openai.ChatCompletionMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			b.WriteString("【用户】" + m.Content + "\n")
		case "assistant":
			if m.Content != "" {
				b.WriteString("【助手】" + m.Content + "\n")
			}
			for _, tc := range m.ToolCalls {
				b.WriteString(fmt.Sprintf("【助手·调用工具】%s(%s)\n", tc.Function.Name, tc.Function.Arguments))
			}
		case "tool":
			b.WriteString(fmt.Sprintf("【工具结果·%s】%s\n", m.Name, truncate(m.Content, 500)))
		}
	}
	return b.String()
}
