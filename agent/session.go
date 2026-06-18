package agent

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// apiEmptyContentPlaceholder 发往 LLM 时，空 tool/assistant 结果的占位 content。
// go-openai 对 "" 使用 omitempty，DeepSeek 等提供方仍要求 JSON 里必须有 content 字段。
const apiEmptyContentPlaceholder = "ok"

// Session 持有一轮会话的状态：会话 ID + 对话历史（唯一事实源）+ 大小管理。
//
// 历史只含 user / assistant / tool 三类消息；system 提示词不在此（由 Agent 持有、
// 在 BuildRequest 时前置），这样历史保持为纯对话，便于裁剪与持久化。
// 把这些从 Agent 里拆出来，让「消息日志怎么攒、怎么裁、怎么存档」聚到一处。
type Session struct {
	ID                string
	messages          []openai.ChatCompletionMessage
	maxMessages       int
	logRewriteVersion int // 历史被 prune/compact 改写时递增，供前缀缓存诊断
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
	for _, m := range s.messages {
		req = append(req, ensureAPIContent(m))
	}
	return req
}

func ensureAPIContent(m openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	if m.Content != "" {
		return m
	}
	switch m.Role {
	case "tool", "assistant":
		out := m
		out.Content = apiEmptyContentPlaceholder
		return out
	default:
		return m
	}
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
	s.IncrementRewrite()
}

// minRecentKeep 是压缩后至少保留的近消息条数（即便单条就已超出 token 预算）。
const minRecentKeep = 2

// msgChars 估算一条消息占用的字符数（content + 工具调用的名字与参数），用于 token 估算。
func msgChars(m openai.ChatCompletionMessage) int {
	n := len(m.Content)
	for _, tc := range m.ToolCalls {
		n += len(tc.Function.Name) + len(tc.Function.Arguments)
	}
	return n
}

// CharCount 返回历史全部消息的字符数之和，供调用方按真实用量推导「token/字符」系数。
func (s *Session) CharCount() int {
	n := 0
	for _, m := range s.messages {
		n += msgChars(m)
	}
	return n
}

// SplitForCompaction 把历史切成「待摘要的旧消息 old」与「保留的近消息 recent」两段。
//
// 保留段从末尾倒着累加估算 token，直到触达 tailBudget；至少保留 minRecentKeep 条。
// tokPerChar 是「token/字符」估算系数（由真实用量推导，见 Agent.tokPerChar），让保留量
// 按 token 而非条数对齐，从而压缩后体积可控、与触发判定一致。
// 配对感知：保留段绝不以孤立 tool 开头，否则它与对应 tool_call 失联、接口直接 400。
func (s *Session) SplitForCompaction(tailBudget int, tokPerChar float64) (old, recent []openai.ChatCompletionMessage) {
	n := len(s.messages)
	if n == 0 {
		return nil, nil
	}
	keep, tokens := 0, 0
	for i := n - 1; i >= 0; i-- {
		tokens += int(float64(msgChars(s.messages[i])) * tokPerChar)
		keep++
		if tokens >= tailBudget && keep >= minRecentKeep {
			break
		}
	}
	if keep > n {
		keep = n
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

// Replace 用新历史整体替换（prune / compact 使用）。
func (s *Session) Replace(msgs []openai.ChatCompletionMessage) {
	s.messages = append([]openai.ChatCompletionMessage(nil), msgs...)
}

// RewriteVersion 返回历史改写代数（prune/compact 会递增）。
func (s *Session) RewriteVersion() int { return s.logRewriteVersion }

// IncrementRewrite 在历史被 prune/compact 改写后调用。
func (s *Session) IncrementRewrite() { s.logRewriteVersion++ }
