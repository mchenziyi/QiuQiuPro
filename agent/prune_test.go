package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestPruneStaleToolResults_ElidesOldToolOutput(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.contextWindow = 1000
	big := strings.Repeat("x", 2000)
	// 足够长的历史，使中间的 tool 结果落在保护尾段之外。
	for i := 0; i < 12; i++ {
		a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("u", 800)})
		a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("a", 800)})
	}
	a.session.Add(openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{
		ID: "1", Type: "function", Function: openai.FunctionCall{Name: "read_file", Arguments: `{}`},
	}}})
	a.session.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "1", Name: "read_file", Content: big})
	// 保护尾段：近几轮对话
	for i := 0; i < 4; i++ {
		a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "recent-" + string(rune('a'+i))})
		a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "ok"})
	}
	a.lastPromptTokens = 900

	st, err := a.PruneStaleToolResults()
	if err != nil {
		t.Fatal(err)
	}
	if st.Results != 1 {
		t.Fatalf("应裁剪 1 条 tool 结果，实际 %d", st.Results)
	}
	for _, m := range a.session.Messages() {
		if m.Role == "tool" && strings.HasPrefix(m.Content, prunedMarker) {
			return
		}
	}
	t.Fatal("未找到被裁剪的 tool 占位符")
}

func TestRememberRule_DoesNotChangeCachedSystemPrompt(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	a := newDispatchAgent(t, AllowAllGate{})
	a.sysPrompt = "BASE"
	a.SetMemoryStore(store)
	a.RegisterTool(a.NewRememberRuleTool())

	before := a.BuildSystemPrompt()
	a.executeToolCall(t.Context(), openai.ToolCall{Function: openai.FunctionCall{
		Name:      memoryToolName,
		Arguments: `{"scope":"global","kind":"preference","content":"以后默认用中文回答","reason":"用户表达了长期偏好"}`,
	}})
	after := a.BuildSystemPrompt()
	if before != after {
		t.Fatalf("remember 不应改动 cached system prompt\nbefore=%q\nafter=%q", before, after)
	}
	if len(a.pendingMemory) != 1 {
		t.Fatalf("应排队 turn-tail 记忆 note，实际 %d", len(a.pendingMemory))
	}
	turn := a.composeUserTurn("下一问")
	if !strings.Contains(turn, "<memory-update>") {
		t.Fatalf("用户轮应含 memory-update：%q", turn)
	}
}
