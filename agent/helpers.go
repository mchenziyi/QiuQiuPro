package agent

import (
	"fmt"
	"time"

	"agentdemo/event"
)

// 消息日志的攒/裁/存档逻辑已迁入 Session（见 session.go），这里只保留事件与杂项工具。

// recordEvent 记录事件到日志
func (a *Agent) recordEvent(eventType, content, toolName string) {
	e := event.Event{
		ID:        fmt.Sprintf("%s_%d", a.session.ID, time.Now().UnixNano()),
		Type:      eventType, Content: content, ToolName: toolName,
		Timestamp: time.Now(),
	}
	a.store.Append(a.session.ID, e)
	a.lastEventID = e.ID
}

// truncate 截断字符串用于日志显示
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n { return s }
	return string(runes[:n]) + "..."
}
