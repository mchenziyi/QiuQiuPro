package agent

import (
	"fmt"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// assertValidToolPairing 校验消息序列对「工具调用 / 工具结果」是配对合法的：
// 每条 tool 结果之前，都必须出现过携带同一 ID tool_call 的 assistant 消息。
// 这正是 DeepSeek/OpenAI 接口的硬性要求——裁剪历史时一旦把二者拆开就会 400。
func assertValidToolPairing(t *testing.T, msgs []openai.ChatCompletionMessage) {
	t.Helper()
	seen := map[string]bool{}
	for i, m := range msgs {
		switch m.Role {
		case "assistant":
			for _, tc := range m.ToolCalls {
				seen[tc.ID] = true
			}
		case "tool":
			if m.ToolCallID == "" || !seen[m.ToolCallID] {
				t.Fatalf("msg[%d] 是孤立的 tool 结果（id=%q），缺少对应的 tool_call", i, m.ToolCallID)
			}
		}
	}
}

// 未超过上限时不应裁剪。
func TestSessionTrim_NoopUnderLimit(t *testing.T) {
	s := NewSession("test")
	for i := 0; i < 10; i++ {
		s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
		s.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "yo"})
	}
	before := s.Len()
	s.Trim()
	if s.Len() != before {
		t.Fatalf("未超过上限不应裁剪：before=%d after=%d", before, s.Len())
	}
}

// 超过上限裁剪后，工具调用/结果的配对必须保持完整，且窗口不能以孤立 tool 开头。
func TestSessionTrim_PreservesToolPairing(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "start"})
	// 1 条 user + 100 组 [assistant(tool_call) + tool] = 201 条，必然超过 maxMessages=100。
	for k := 0; k < 100; k++ {
		id := fmt.Sprintf("t%d", k)
		s.Add(openai.ChatCompletionMessage{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{{
				ID: id, Type: "function",
				Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"},
			}},
		})
		s.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: id, Name: "read_file", Content: "data"})
	}
	if s.Len() <= maxMessages {
		t.Fatalf("测试前置不满足：messages=%d 应 > maxMessages=%d", s.Len(), maxMessages)
	}

	s.Trim()

	if s.Len() > maxMessages {
		t.Fatalf("裁剪后仍超过上限：%d > %d", s.Len(), maxMessages)
	}
	msgs := s.Messages()
	if len(msgs) > 0 && msgs[0].Role == "tool" {
		t.Fatalf("窗口不应以孤立的 tool 消息开头")
	}
	assertValidToolPairing(t, msgs)
}

// 有 system 提示词时，请求应把 system 前置，且不修改历史本身。
func TestSessionBuildRequest_PrependsSystem(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
	s.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "yo"})

	req := s.BuildRequest("SYS")
	if len(req) != s.Len()+1 {
		t.Fatalf("应在最前面加 system：len=%d", len(req))
	}
	if req[0].Role != "system" || req[0].Content != "SYS" {
		t.Fatalf("第一条应为 system，实际 %+v", req[0])
	}
	if s.Len() != 2 || s.Messages()[0].Role != "user" {
		t.Fatalf("BuildRequest 不应修改历史本身")
	}
}

// 无 system 提示词时，不应前置 system 消息。
func TestSessionBuildRequest_NoSystemWhenEmpty(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
	req := s.BuildRequest("")
	if len(req) != 1 || req[0].Role != "user" {
		t.Fatalf("空 system 不应前置 system 消息：%+v", req)
	}
}

// Snapshot 序列化、Restore 反序列化应能完整往返历史。
func TestSessionSnapshotRestore_RoundTrip(t *testing.T) {
	src := NewSession("src")
	src.Add(openai.ChatCompletionMessage{Role: "user", Content: "问题"})
	src.Add(openai.ChatCompletionMessage{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID: "call_1", Type: "function",
			Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`},
		}},
	})
	src.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "call_1", Name: "read_file", Content: "内容"})

	data, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 失败：%v", err)
	}

	dst := NewSession("dst")
	if err := dst.Restore(data); err != nil {
		t.Fatalf("Restore 失败：%v", err)
	}
	if dst.Len() != src.Len() {
		t.Fatalf("恢复后条数不一致：src=%d dst=%d", src.Len(), dst.Len())
	}
	got := dst.Messages()
	if got[0].Content != "问题" || got[2].ToolCallID != "call_1" {
		t.Fatalf("恢复内容不一致：%+v", got)
	}
	assertValidToolPairing(t, got)
}

// Restore 遇到非法 JSON 应返回错误，且不破坏既有历史。
func TestSessionRestore_BadJSON(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "保留我"})
	if err := s.Restore("{not json"); err == nil {
		t.Fatalf("非法 JSON 应返回错误")
	}
	if s.Len() != 1 || s.Messages()[0].Content != "保留我" {
		t.Fatalf("Restore 失败时不应破坏既有历史")
	}
}
