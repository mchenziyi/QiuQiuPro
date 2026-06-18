package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

// summarizeFunc 产出一段消息历史的摘要。抽成可注入的函数是为了测试——
// 默认实现走真实 LLM（llmSummarize），测试可替换成桩函数，无需联网。
type summarizeFunc func(ctx context.Context, msgs []openai.ChatCompletionMessage) (string, error)

// 压缩触发参数（对齐 Reasonix 默认值）。
const (
	defaultContextWindow = 1_000_000
	defaultCompactRatio  = 0.8
	defaultSoftRatio     = 0.5
	defaultCompactForce  = 0.9
	defaultCompactTarget = 0.5
	maxTailTokens        = 16384
	minCompactMessages   = 2
	fallbackTokPerChar   = 0.25
	minFoldTokens        = 400
)

const (
	summaryTagOpen  = "<compaction-summary>"
	summaryTagClose = "</compaction-summary>"
)

// cacheColdAfter：会话 idle 超过此时间后恢复时 prune 陈旧 tool 结果（对齐 Reasonix）。
const cacheColdAfter = 24 * time.Hour

// maybeCompact 在 prompt 逼近窗口时才压缩；压缩前先尝试 prune（对齐 Reasonix）。
func (a *Agent) maybeCompact(ctx context.Context, usage openai.Usage) {
	if a.contextWindow <= 0 || usage.PromptTokens == 0 {
		return
	}
	promptTokens := usage.PromptTokens
	a.lastPromptTokens = promptTokens

	high := int(float64(a.contextWindow) * a.compactRatio)
	soft := int(float64(a.contextWindow) * a.softCompactRatio)

	if promptTokens >= soft && promptTokens < high && !a.softCompactNoticed {
		a.softCompactNoticed = true
		a.noticef("  📈 上下文已达窗口 %.0f%%（%d/%d token），到 %.0f%% 才会压缩，期间保持前缀缓存\n",
			float64(promptTokens)/float64(a.contextWindow)*100, promptTokens, a.contextWindow, a.compactRatio*100)
		return
	}
	if promptTokens < high {
		a.consecutiveCompacts = 0
		a.compactStuck = false
		a.softCompactNoticed = false
		return
	}
	if a.compactStuck {
		return
	}
	force := promptTokens >= int(float64(a.contextWindow)*a.compactForceRatio)
	ratio := a.tokPerChar()
	if st, err := a.PruneStaleToolResults(); err == nil && st.Results > 0 {
		saved := int(float64(st.SavedChars) * ratio)
		a.noticef("  ✂️  裁剪 %d 条陈旧 tool 结果（约 %d token），优先保前缀缓存\n", st.Results, saved)
		if !force && promptTokens-saved < high {
			return
		}
	}
	if err := a.compact(ctx, false, force); err != nil {
		a.noticef("  🗜️  压缩跳过：%v\n", err)
		return
	}
	a.consecutiveCompacts++
	if a.consecutiveCompacts >= 2 {
		a.compactStuck = true
		a.noticef("  ⚠️  context_window=%d 过小，系统提示+一轮对话已超过 %.0f%% 窗口；请调大 DEEPSEEK_CONTEXT_WINDOW 或缩小 tool 输出。自动压缩已暂停。\n",
			a.contextWindow, a.compactRatio*100)
	}
}

// Compact 手动触发一次压缩（/compact），无视比例阈值。
func (a *Agent) Compact(ctx context.Context) {
	if a.session.Len() == 0 {
		a.noticef("  🗜️  会话为空，无需压缩\n")
		return
	}
	if err := a.compact(ctx, true, true); err != nil {
		a.noticef("  🗜️  压缩失败：%v\n", err)
	}
}

func (a *Agent) compact(ctx context.Context, manual, force bool) error {
	msgs := a.session.Messages()
	minKeep := minRecentKeep
	if force {
		// 强制压缩时，最近一条用户输入足以延续当前轮；允许折叠刚刚产生的超长 assistant。
		minKeep = 1
	}
	head, start, ok := a.planCompaction(msgs, minCompactMessages, minKeep)
	if !ok {
		head, start, ok = a.planCompaction(msgs, 1, minKeep)
	}
	if !ok {
		if manual {
			a.noticef("  🗜️  历史较短，无需压缩\n")
		}
		return nil
	}
	region := msgs[head:start]
	if !force && !foldEconomics(region) {
		return nil
	}

	summarize := a.summarizer
	if summarize == nil {
		summarize = a.llmSummarize
	}
	summary, err := summarize(ctx, region)
	if err != nil || strings.TrimSpace(summary) == "" {
		a.debugf("  🗜️  摘要失败，退化为裁剪：%v\n", err)
		a.session.Trim()
		a.lastPromptTokens, a.softCompactNoticed = 0, false
		return err
	}
	summary = strings.TrimSpace(summary)

	compacted := make([]openai.ChatCompletionMessage, 0, head+1+len(msgs)-start)
	compacted = append(compacted, msgs[:head]...)
	compacted = append(compacted, openai.ChatCompletionMessage{
		Role: "user",
		Content: summaryTagOpen + "\n" +
			"Summary of earlier conversation (older messages were compacted to save context):\n" +
			summary + "\n" + summaryTagClose,
	})
	compacted = append(compacted, msgs[start:]...)
	a.session.Replace(compacted)
	a.session.IncrementRewrite()
	a.lastPromptTokens, a.softCompactNoticed = 0, false

	tag := "自动"
	if manual {
		tag = "手动"
	}
	a.noticef("  🗜️  上下文已压缩（%s）：%d 条旧消息折叠为摘要，保留近 %d 条\n", tag, len(region), len(msgs)-start)
	return nil
}

func foldEconomics(region []openai.ChatCompletionMessage) bool {
	return estimateMessagesTokens(region) >= minFoldTokens
}

func estimateMessagesTokens(msgs []openai.ChatCompletionMessage) int {
	total := 0
	for _, m := range msgs {
		total += 4
		total += estimateTextTokens(m.Content)
		total += estimateTextTokens(m.Name)
		for _, tc := range m.ToolCalls {
			total += 8
			total += estimateTextTokens(tc.Function.Name)
			total += estimateTextTokens(tc.Function.Arguments)
		}
	}
	return total
}

func estimateTextTokens(s string) int {
	if s == "" {
		return 0
	}
	bytes := len(s)
	runes := utf8.RuneCountInString(s)
	byBytes := (bytes + 3) / 4
	if runes > byBytes {
		return runes
	}
	return byBytes
}

func (a *Agent) planCompaction(msgs []openai.ChatCompletionMessage, min, minKeep int) (head, start int, ok bool) {
	head = 0
	if a.contextWindow > 0 {
		budget := maxTailTokens
		if maxByWin := int(float64(a.contextWindow) * defaultCompactTarget); maxByWin < budget {
			budget = maxByWin
		}
		start = tailStart(msgs, head, budget, a.tokPerChar(), minKeep)
	} else {
		start = len(msgs) - minRecentKeep
		for start > head && msgs[start].Role == "tool" {
			start--
		}
	}
	if start < head {
		start = head
	}
	if start-head < min {
		return head, start, false
	}
	return head, start, true
}

func tailStart(msgs []openai.ChatCompletionMessage, head, budgetTokens int, tokPerChar float64, minKeep int) int {
	start := len(msgs)
	acc := 0
	for i := len(msgs) - 1; i > head; i-- {
		c := int(float64(msgChars(msgs[i])) * tokPerChar)
		if len(msgs)-i > minKeep && acc+c > budgetTokens {
			break
		}
		acc += c
		start = i
	}
	for start > head && start < len(msgs) && msgs[start].Role == "tool" {
		start--
	}
	return start
}

func (a *Agent) SetContextWindow(tokens int) { a.contextWindow = tokens }

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

func renderForSummary(msgs []openai.ChatCompletionMessage) string {
	var b strings.Builder
	for _, m := range msgs {
		switch m.Role {
		case "user":
			b.WriteString("【用户】")
			b.WriteString(m.Content)
			b.WriteString("\n")
		case "assistant":
			if m.Content != "" {
				b.WriteString("【助手】")
				b.WriteString(m.Content)
				b.WriteString("\n")
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

// maybeColdResumePrune 恢复会话后，若 idle 超过 provider 缓存保留期则 prune 陈旧 tool 结果。
func (a *Agent) maybeColdResumePrune(cpCreatedAt int64) {
	if a.contextWindow <= 0 || cpCreatedAt == 0 {
		return
	}
	last := time.Unix(cpCreatedAt, 0)
	if time.Since(last) < cacheColdAfter {
		return
	}
	st, err := a.PruneStaleToolResults()
	if err != nil || st.Results == 0 {
		return
	}
	a.noticef("  ♻️  会话 idle %s，已裁剪 %d 条陈旧 tool 结果以优化冷恢复（前缀缓存已过期）\n",
		time.Since(last).Round(time.Minute), st.Results)
	a.SaveCheckpoint()
}
