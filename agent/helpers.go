package agent

import (
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
)

// trimMessages 截断历史：只保留最近的 maxMessages 条消息。
//
// 关键约束（配对感知）：现在我们全量保留工具往返，a.messages 里会出现
// assistant(tool_calls) 紧跟若干 tool 结果。裁剪窗口绝不能以孤立的 tool
// 消息开头——否则它会与对应的 tool_call 失联，DeepSeek/OpenAI 接口直接 400。
// 因此裁剪后若开头是 tool，就继续向前丢弃，直到落在 user / assistant 上。
// system 提示词单独存放在 a.sysPrompt（不在 a.messages 里），无需在此特殊保留。
func (a *Agent) trimMessages() {
	if len(a.messages) <= maxMessages {
		return
	}
	start := len(a.messages) - maxMessages
	for start < len(a.messages) && a.messages[start].Role == "tool" {
		start++
	}
	a.messages = append([]openai.ChatCompletionMessage(nil), a.messages[start:]...)
}

// buildRequestMessages 把唯一的消息日志组装成一次 LLM 请求：
// system 提示词单独前置，不进入 a.messages，从而让 a.messages 保持为纯对话历史
// （只含 user / assistant / tool），便于裁剪与持久化。
func (a *Agent) buildRequestMessages() []openai.ChatCompletionMessage {
	req := make([]openai.ChatCompletionMessage, 0, len(a.messages)+1)
	if a.sysPrompt != "" {
		req = append(req, openai.ChatCompletionMessage{Role: "system", Content: a.sysPrompt})
	}
	req = append(req, a.messages...)
	return req
}

// recordEvent 记录事件到日志
func (a *Agent) recordEvent(eventType, content, toolName string) {
	e := event.Event{
		ID:        fmt.Sprintf("%s_%d", a.session, time.Now().UnixNano()),
		Type:      eventType, Content: content, ToolName: toolName,
		Timestamp: time.Now(),
	}
	a.store.Append(a.session, e)
	a.lastEventID = e.ID
}

// truncate 截断字符串用于日志显示
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n { return s }
	return string(runes[:n]) + "..."
}
