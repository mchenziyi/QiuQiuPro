package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

type recordingToolHook struct {
	before  []string
	after   []string
	deny    bool
	rewrite string
}

func (h *recordingToolHook) BeforeToolCall(ctx ToolHookContext) (ToolHookDecision, string) {
	h.before = append(h.before, ctx.Name)
	if h.deny {
		return ToolHookDeny, "hook denied"
	}
	return ToolHookAllow, ""
}

func (h *recordingToolHook) AfterToolCall(ctx ToolHookContext, result string) string {
	h.after = append(h.after, ctx.Name+":"+result)
	if h.rewrite != "" {
		return h.rewrite
	}
	return result
}

func TestToolHook_BeforeAndAfterWrapExecution(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.allTools["read_file"] = tool.Tool{Name: "read_file", Execute: func(ctx context.Context, args json.RawMessage) (string, error) { return "RAW", nil }}
	hook := &recordingToolHook{rewrite: "REWRITTEN"}
	a.RegisterToolHook(hook)

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`}})

	if got != "REWRITTEN" {
		t.Fatalf("After hook 应能改写结果，实际 %q", got)
	}
	if len(hook.before) != 1 || hook.before[0] != "read_file" {
		t.Fatalf("Before hook 未收到调用上下文：%+v", hook.before)
	}
	if len(hook.after) != 1 || hook.after[0] != "read_file:RAW" {
		t.Fatalf("After hook 应收到原始工具结果，实际 %+v", hook.after)
	}
}

func TestToolHook_BeforeCanDenyAndPreserveToolResult(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	called := false
	a.allTools["read_file"] = tool.Tool{Name: "read_file", Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
		called = true
		return "RAW", nil
	}}
	hook := &recordingToolHook{deny: true, rewrite: "SHOULD_NOT_RUN"}
	a.RegisterToolHook(hook)

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"}})

	if called {
		t.Fatal("Before hook 拒绝时不应执行真实工具")
	}
	if !strings.Contains(got, "hook denied") {
		t.Fatalf("拒绝结果应回灌给模型，实际 %q", got)
	}
	if len(hook.after) != 0 {
		t.Fatalf("Before 拒绝时不应调用 After hook，实际 %+v", hook.after)
	}
}

func TestToolHook_RunsBeforeGate(t *testing.T) {
	a := newDispatchAgent(t, ReadOnlyGate{})
	hook := &recordingToolHook{deny: true}
	a.allTools["write_file"] = tool.Tool{Name: "write_file", Execute: func(ctx context.Context, args json.RawMessage) (string, error) { return "RAW", nil }}
	a.RegisterToolHook(hook)

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{Name: "write_file", Arguments: "{}"}})

	if !strings.Contains(got, "hook denied") {
		t.Fatalf("Hook 应先于 Gate 拒绝，实际 %q", got)
	}
	if len(hook.before) != 1 || hook.before[0] != "write_file" {
		t.Fatalf("Before hook 未执行：%+v", hook.before)
	}
}
