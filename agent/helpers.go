package agent

import (
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
)

// trimMessages 截断历史（超过 maxMessages 时丢弃最早的非 system 消息）
func (a *Agent) trimMessages() {
	if len(a.messages) > maxMessages {
		a.messages = append(
			[]openai.ChatCompletionMessage{a.messages[0]},
			a.messages[len(a.messages)-maxMessages+1:]...,
		)
	}
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
