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

// NeedsCompaction 判断历史是否已超过上限、需要压缩（与 Trim 同阈值）。
func (s *Session) NeedsCompaction() bool {
	return len(s.messages) > s.maxMessages
}

// SplitForCompaction 把历史切成「待摘要的旧消息 old」与「保留的近消息 recent」两段。
// 保留段取末尾约 maxMessages/2 条；并跳过其开头的孤立 tool（配对感知），确保 recent
// 自身合法、可直接续在摘要消息之后——否则孤立 tool 会与其 tool_call 失联、接口 400。
func (s *Session) SplitForCompaction() (old, recent []openai.ChatCompletionMessage) {
	keep := s.maxMessages / 2
	n := len(s.messages)
	if n <= keep {
		return nil, append([]openai.ChatCompletionMessage(nil), s.messages...)
	}
	cut := n - keep
	for cut < n && s.messages[cut].Role == "tool" {
		cut++
	}
	old = append([]openai.ChatCompletionMessage(nil), s.messages[:cut]...)
	recent = append([]openai.ChatCompletionMessage(nil), s.messages[cut:]...)
	return old, recent
}

// ApplyCompaction 用「摘要消息 + 近消息」替换历史。
// summary 非空时，前置一条 user 角色的摘要消息（无 tool_calls，不影响配对）；
// summary 为空则退化为只保留近消息（等价一次裁剪）。
func (s *Session) ApplyCompaction(summary string, recent []openai.ChatCompletionMessage) {
	msgs := make([]openai.ChatCompletionMessage, 0, len(recent)+1)
	if summary != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "user",
			Content: "（以下是早前对话的摘要，供你延续上下文）\n" + summary,
		})
	}
	s.messages = append(msgs, recent...)
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
