package agent

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// Session 持有一轮会话的状态：会话 ID + 对话历史（唯一事实源）+ 大小管理。
//
// 历史只含 user / assistant / tool 三类消息；system 提示词不在此（由 Agent 持有、
// 在 BuildRequest 时前置），这样历史保持为纯对话，便于裁剪与持久化。
// 把这些从 Agent 里拆出来，让「消息日志怎么攒、怎么裁、怎么存档」聚到一处。
type Session struct {
	ID          string
	messages    []openai.ChatCompletionMessage
	maxMessages int
}

// NewSession 新建一个空会话。
func NewSession(id string) *Session {
	return &Session{ID: id, maxMessages: maxMessages}
}

// Add 追加一条消息（append-only，永不删——体积由 Trim 控制）。
func (s *Session) Add(msg openai.ChatCompletionMessage) {
	s.messages = append(s.messages, msg)
}

// Messages 返回对话历史（只读用途，调用方不应修改返回值）。
func (s *Session) Messages() []openai.ChatCompletionMessage { return s.messages }

// Len 返回历史消息条数。
func (s *Session) Len() int { return len(s.messages) }

// BuildRequest 组装一次 LLM 请求：system 提示词前置（非空时），其后接全量历史。
// 不修改历史本身，因此可反复调用。
func (s *Session) BuildRequest(sysPrompt string) []openai.ChatCompletionMessage {
	req := make([]openai.ChatCompletionMessage, 0, len(s.messages)+1)
	if sysPrompt != "" {
		req = append(req, openai.ChatCompletionMessage{Role: "system", Content: sysPrompt})
	}
	return append(req, s.messages...)
}

// Trim 截断历史到最多 maxMessages 条。
//
// 配对感知：全量保留工具往返后，历史里会出现 assistant(tool_calls) 紧跟若干 tool 结果。
// 裁剪窗口绝不能以孤立的 tool 开头，否则它与对应 tool_call 失联、接口直接 400。
// 因此裁剪后若开头是 tool，就继续向前丢弃，直到落在 user / assistant 上。
func (s *Session) Trim() {
	if len(s.messages) <= s.maxMessages {
		return
	}
	start := len(s.messages) - s.maxMessages
	for start < len(s.messages) && s.messages[start].Role == "tool" {
		start++
	}
	s.messages = append([]openai.ChatCompletionMessage(nil), s.messages[start:]...)
}

// Snapshot 把历史序列化为 JSON，用于存档 checkpoint。
func (s *Session) Snapshot() (string, error) {
	data, err := json.Marshal(s.messages)
	return string(data), err
}

// Restore 用 JSON 覆盖历史，用于从 checkpoint 恢复。
func (s *Session) Restore(messagesJSON string) error {
	var msgs []openai.ChatCompletionMessage
	if err := json.Unmarshal([]byte(messagesJSON), &msgs); err != nil {
		return err
	}
	s.messages = msgs
	return nil
}
