// Package event 提供 Agent 操作的事件存储与重放
package event

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Event 表示 Agent 的一步操作，不可变，追加写入
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`       // user / assistant / tool_call / tool_result / error
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Store 事件存储：每行一个 JSON 的日志文件（JSON Lines）
type Store struct {
	dir string
}

// NewStore 创建事件存储，目录不存在会自动创建
func NewStore(dir string) *Store {
	os.MkdirAll(dir, 0755)
	return &Store{dir: dir}
}

// Append 追加一条事件到日志文件
func (s *Store) Append(sessionID string, e Event) error {
	path := fmt.Sprintf("%s/%s.jsonl", s.dir, sessionID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, _ := json.Marshal(e)
	f.WriteString(string(data) + "\n")
	return nil
}

// Load 读取某个 session 的全部事件
func (s *Store) Load(sessionID string) ([]Event, error) {
	path := fmt.Sprintf("%s/%s.jsonl", s.dir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, err
	}
	var events []Event
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e Event
		json.Unmarshal([]byte(line), &e)
		events = append(events, e)
	}
	return events, nil
}

// Replay 格式化成可读的对话记录
func Replay(sessionID string, events []Event) string {
	if len(events) == 0 {
		return "没有事件记录"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📋 Session %s（共 %d 条事件）：\n", sessionID, len(events))
	for i, e := range events {
		icon := ""
		switch e.Type {
		case "user":
			icon = "🧑"
		case "assistant":
			icon = "🤖"
		case "tool_call":
			icon = "🔧"
		case "tool_result":
			icon = "📦"
		case "error":
			icon = "❌"
		default:
			icon = "•"
		}
		content := e.Content
		if len([]rune(content)) > 80 {
			content = string([]rune(content)[:80]) + "..."
		}
		if e.ToolName != "" {
			fmt.Fprintf(&b, "%d. %s [%s] %s: %s\n", i+1, icon, e.Type, e.ToolName, content)
		} else {
			fmt.Fprintf(&b, "%d. %s [%s] %s\n", i+1, icon, e.Type, content)
		}
	}
	fmt.Fprintf(&b, "\n✅ 重放完成\n")
	return b.String()
}
