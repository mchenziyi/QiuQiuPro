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
func TestTrimMessages_NoopUnderLimit(t *testing.T) {
	a := &Agent{}
	for i := 0; i < 10; i++ {
		a.messages = append(a.messages,
			openai.ChatCompletionMessage{Role: "user", Content: "hi"},
			openai.ChatCompletionMessage{Role: "assistant", Content: "yo"},
		)
	}
	before := len(a.messages)
	a.trimMessages()
	if len(a.messages) != before {
		t.Fatalf("未超过上限不应裁剪：before=%d after=%d", before, len(a.messages))
	}
}

// 超过上限裁剪后，工具调用/结果的配对必须保持完整，且窗口不能以孤立 tool 开头。
func TestTrimMessages_PreservesToolPairing(t *testing.T) {
	a := &Agent{}
	a.messages = append(a.messages, openai.ChatCompletionMessage{Role: "user", Content: "start"})
	// 1 条 user + 100 组 [assistant(tool_call) + tool] = 201 条，必然超过 maxMessages=100。
	for k := 0; k < 100; k++ {
		id := fmt.Sprintf("t%d", k)
		a.messages = append(a.messages,
			openai.ChatCompletionMessage{
				Role: "assistant",
				ToolCalls: []openai.ToolCall{{
					ID: id, Type: "function",
					Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"},
				}},
			},
			openai.ChatCompletionMessage{Role: "tool", ToolCallID: id, Name: "read_file", Content: "data"},
		)
	}
	if len(a.messages) <= maxMessages {
		t.Fatalf("测试前置不满足：messages=%d 应 > maxMessages=%d", len(a.messages), maxMessages)
	}

	a.trimMessages()

	if len(a.messages) > maxMessages {
		t.Fatalf("裁剪后仍超过上限：%d > %d", len(a.messages), maxMessages)
	}
	if len(a.messages) > 0 && a.messages[0].Role == "tool" {
		t.Fatalf("窗口不应以孤立的 tool 消息开头")
	}
	assertValidToolPairing(t, a.messages)
}

// 有 system 提示词时，请求应把 system 前置，且不修改 a.messages 本身。
func TestBuildRequestMessages_PrependsSystem(t *testing.T) {
	a := &Agent{sysPrompt: "SYS"}
	a.messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "yo"},
	}
	req := a.buildRequestMessages()
	if len(req) != len(a.messages)+1 {
		t.Fatalf("应在最前面加 system：len=%d", len(req))
	}
	if req[0].Role != "system" || req[0].Content != "SYS" {
		t.Fatalf("第一条应为 system，实际 %+v", req[0])
	}
	if len(a.messages) != 2 || a.messages[0].Role != "user" {
		t.Fatalf("buildRequestMessages 不应修改 a.messages")
	}
}

// 无 system 提示词时，不应前置 system 消息。
func TestBuildRequestMessages_NoSystemWhenEmpty(t *testing.T) {
	a := &Agent{sysPrompt: ""}
	a.messages = []openai.ChatCompletionMessage{{Role: "user", Content: "hi"}}
	req := a.buildRequestMessages()
	if len(req) != 1 || req[0].Role != "user" {
		t.Fatalf("空 system 不应前置 system 消息：%+v", req)
	}
}
