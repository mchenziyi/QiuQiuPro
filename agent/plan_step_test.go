package agent

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestPlanStepToolFailure_DetectsToolErrorsSinceMark(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "old turn"})
	before := a.session.Len()

	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "请执行：读文件"})
	a.session.Add(openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{
		ID: "c1", Type: "function", Function: openai.FunctionCall{Name: "read_file", Arguments: `{}`},
	}}})
	a.session.Add(openai.ChatCompletionMessage{
		Role: "tool", ToolCallID: "c1", Name: "read_file",
		Content: "读取 /tmp/missing.txt 失败",
	})

	got := a.planStepToolFailure(before)
	if got != "读取 /tmp/missing.txt 失败" {
		t.Fatalf("planStepToolFailure()=%q", got)
	}
}

func TestPlanStepToolFailure_IgnoresErrorsBeforeMark(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.session.Add(openai.ChatCompletionMessage{
		Role: "tool", ToolCallID: "old", Name: "read_file", Content: "读取 /old 失败",
	})
	before := a.session.Len()

	a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "ok"})
	if got := a.planStepToolFailure(before); got != "" {
		t.Fatalf("expected no failure after mark, got %q", got)
	}
}

func TestPlanStepToolFailure_IgnoresBlockedResults(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	before := a.session.Len()
	a.session.Add(openai.ChatCompletionMessage{
		Role: "tool", ToolCallID: "c1", Name: "write_file",
		Content: `blocked: "write_file" is a writer tool and plan mode is read-only.`,
	})
	if got := a.planStepToolFailure(before); got != "" {
		t.Fatalf("blocked result should not fail plan step, got %q", got)
	}
}
