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
	Content   string    `json:"content"`    // 事件内容（用户说的话 / LLM 的回答 / 工具参数 / 工具结果 / 错误信息）
	ToolName  string    `json:"tool_name,omitempty"` // 工具名（只有 tool_call 和 tool_result 时有值）
	Timestamp time.Time `json:"timestamp"`  // 发生时间
}

// Store 事件存储
// 以目录路径初始化，所有 session 的日志文件都放在这个目录下
type Store struct {
	dir string // 存储目录，如 ".reasonix/sessions/"
}

// NewStore 创建事件存储，目录不存在会自动创建
func NewStore(dir string) *Store {
	os.MkdirAll(dir, 0755) // 确保目录存在
	return &Store{dir: dir}
}

// Append 追加一条事件到日志文件
// 以追加模式打开文件，写入一行 JSON，不读不修改已有内容
func (s *Store) Append(sessionID string, e Event) error {
	path := fmt.Sprintf("%s/%s.jsonl", s.dir, sessionID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, _ := json.Marshal(e)                 // 序列化为 JSON
	f.WriteString(string(data) + "\n")          // 每行一条，末尾换行
	return nil
}

// Load 读取某个 session 的全部事件
// 逐行读取 JSON，跳过空行，返回事件列表
func (s *Store) Load(sessionID string) ([]Event, error) {
	path := fmt.Sprintf("%s/%s.jsonl", s.dir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil // 新 session 没有历史，返回空列表
		}
		return nil, err
	}
	var events []Event
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue // 跳过空行（文件末尾的空行）
		}
		var e Event
		json.Unmarshal([]byte(line), &e)
		events = append(events, e)
	}
	return events, nil
}

// Replay 把事件列表格式化成可读的对话记录
// 每行前面加 emoji 图标，方便一眼看出事件类型
func Replay(sessionID string, events []Event) string {
	if len(events) == 0 {
		return "没有事件记录"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📋 Session %s（共 %d 条事件）：\n", sessionID, len(events))
	for i, e := range events {
		// 根据事件类型选择对应的 emoji 图标
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
		// 内容太长时截断，避免刷屏
		content := e.Content
		if len([]rune(content)) > 80 {
			content = string([]rune(content)[:80]) + "..."
		}
		// 有工具名就显示，没有就不显示
		if e.ToolName != "" {
			fmt.Fprintf(&b, "%d. %s [%s] %s: %s\n", i+1, icon, e.Type, e.ToolName, content)
		} else {
			fmt.Fprintf(&b, "%d. %s [%s] %s\n", i+1, icon, e.Type, content)
		}
	}
	fmt.Fprintf(&b, "\n✅ 重放完成\n")
	return b.String()
}
