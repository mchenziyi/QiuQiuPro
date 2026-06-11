// Package event 提供 Agent 操作的事件存储与重放
// 原理：JSON Lines（.jsonl）——每行一个 JSON，追加写入，不修改已有内容
package event

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Event 表示 Agent 的一步操作
// 一旦写入就不可修改（追加写入，不改已有行）
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`       // 事件类型：user / assistant / tool_call / tool_result / error
	Content   string    `json:"content"`    // 事件内容
	ToolName  string    `json:"tool_name,omitempty"` // 工具名
	Timestamp time.Time `json:"timestamp"`  // 发生时间
}

// Checkpoint 表示 Agent 的状态快照
// 定期保存，崩溃后从最近的 Checkpoint 恢复，不用重放全部事件
type Checkpoint struct {
	SessionID   string `json:"session_id"`
	LastEventID string `json:"last_event_id"`   // 快照对应的最后一条 Event ID
	MessagesJSON string `json:"messages_json"`  // 序列化后的 messages
	CreatedAt   int64  `json:"created_at"`      // 创建时间戳
}

// Store 事件存储
type Store struct {
	dir string // 存储目录，如 ".reasonix/sessions/"
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

// LoadSince 读取某个 session 中从指定 Event ID 之后的事件
// 用于从 Checkpoint 恢复后，只重放快照之后的新事件
func (s *Store) LoadSince(sessionID, afterEventID string) ([]Event, error) {
	all, err := s.Load(sessionID)
	if err != nil {
		return nil, err
	}
	if afterEventID == "" {
		return all, nil
	}
	found := false
	var result []Event
	for _, e := range all {
		if found {
			result = append(result, e)
			continue
		}
		if e.ID == afterEventID {
			found = true
		}
	}
	return result, nil
}

// SaveCheckpoint 保存当前状态快照
// messagesJSON 是序列化后的对话历史
func (s *Store) SaveCheckpoint(sessionID, lastEventID, messagesJSON string) error {
	cp := Checkpoint{
		SessionID:   sessionID,
		LastEventID: lastEventID,
		MessagesJSON: messagesJSON,
		CreatedAt:   time.Now().Unix(),
	}
	path := fmt.Sprintf("%s/%s.ckpt", s.dir, sessionID)
	data, _ := json.Marshal(cp)
	return os.WriteFile(path, data, 0644)
}

// LoadCheckpoint 读取最新的状态快照
// 如果不存在则返回 nil
func (s *Store) LoadCheckpoint(sessionID string) (*Checkpoint, error) {
	path := fmt.Sprintf("%s/%s.ckpt", s.dir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

// Replay 把事件列表格式化成可读的对话记录
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
