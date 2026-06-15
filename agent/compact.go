package agent

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// 上下文压缩（TODO #13）：历史超限时，与其像 Trim 那样直接丢弃最早的消息，不如让 LLM
// 把旧消息总结成摘要，保留「用户目标、关键事实、读改过的文件、未完成事项」等上下文。
// 摘要会破坏前缀缓存，但只在超限时触发，属于可接受的取舍。

// summarizeFunc 产出一段消息历史的摘要。抽成可注入的函数是为了测试——
// 默认实现走真实 LLM（llmSummarize），测试可替换成桩函数，无需联网。
type summarizeFunc func(ctx context.Context, msgs []openai.ChatCompletionMessage) (string, error)

// maybeCompact 在历史超限时用 LLM 摘要替换旧消息；摘要失败则退化为裁剪（仍兜住体积）。
func (a *Agent) maybeCompact(ctx context.Context) {
	if !a.session.NeedsCompaction() {
		return
	}
	old, recent := a.session.SplitForCompaction()
	if len(old) == 0 {
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
		return
	}

	a.session.ApplyCompaction(strings.TrimSpace(summary), recent)
	a.noticef("  🗜️  上下文已压缩：%d 条旧消息折叠为摘要，保留近 %d 条\n", len(old), len(recent))
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
