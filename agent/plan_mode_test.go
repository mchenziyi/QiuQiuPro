package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"agentdemo/tool"

	openai "github.com/sashabaranov/go-openai"
)

type stubRecorder struct {
	called int
}

func newTestAgent() *Agent {
	return &Agent{
		allTools: make(map[string]tool.Tool),
		planMode: atomic.Bool{},
		gate:     AllowAllGate{},
	}
}

func TestPlanMode_BlocksWriteTools(t *testing.T) {
	a := newTestAgent()
	readRec := &stubRecorder{}
	writeRec := &stubRecorder{}

	a.RegisterTool(tool.Tool{
		Name: "read_file", ReadOnly: true,
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			readRec.called++
			return "stub read", nil
		},
	})
	a.RegisterTool(tool.Tool{
		Name: "write_file", ReadOnly: false,
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			writeRec.called++
			return "stub write", nil
		},
	})

	a.SetPlanMode(true)

	// Read tool should succeed
	result := a.executeToolCall(context.Background(), openai.ToolCall{
		Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"t.txt"}`},
	})
	if readRec.called != 1 {
		t.Fatalf("read tool should have been called (called=%d)", readRec.called)
	}
	if result != "stub read" {
		t.Fatalf("read tool result wrong: %q", result)
	}

	// Write tool should be blocked
	result = a.executeToolCall(context.Background(), openai.ToolCall{
		Function: openai.FunctionCall{Name: "write_file", Arguments: `{"path":"t.txt","content":"x"}`},
	})
	if writeRec.called != 0 {
		t.Fatalf("write tool should NOT have been called (called=%d)", writeRec.called)
	}
	if !strings.Contains(result, "blocked") || !strings.Contains(result, "write_file") {
		t.Fatalf("write tool should be blocked, got: %q", result)
	}
}

func TestPlanMode_Off_AllowsWrites(t *testing.T) {
	a := newTestAgent()
	writeRec := &stubRecorder{}

	a.RegisterTool(tool.Tool{
		Name: "write_file", ReadOnly: false,
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			writeRec.called++
			return "stub write", nil
		},
	})

	a.SetPlanMode(false)

	result := a.executeToolCall(context.Background(), openai.ToolCall{
		Function: openai.FunctionCall{Name: "write_file", Arguments: `{"path":"t.txt","content":"x"}`},
	})
	if writeRec.called != 1 {
		t.Fatalf("write tool should have been called (called=%d)", writeRec.called)
	}
	if result != "stub write" {
		t.Fatalf("write tool result wrong: %q", result)
	}
}

func TestSetPlanMode_Toggles(t *testing.T) {
	a := newTestAgent()
	if a.planMode.Load() {
		t.Fatal("planMode should start false")
	}
	a.SetPlanMode(true)
	if !a.planMode.Load() {
		t.Fatal("planMode should be true after SetPlanMode(true)")
	}
	a.SetPlanMode(false)
	if a.planMode.Load() {
		t.Fatal("planMode should be false after SetPlanMode(false)")
	}
}

func TestSetMode_SetsPlanMode(t *testing.T) {
	a := newTestAgent()
	a.SetMode("plan")
	if !a.planMode.Load() {
		t.Fatal("SetMode('plan') should set planMode=true")
	}
	if a.CurrentMode() != "plan" {
		t.Fatalf("CurrentMode should be 'plan', got %q", a.CurrentMode())
	}

	a.SetMode("ask")
	if a.planMode.Load() {
		t.Fatal("SetMode('ask') should set planMode=false")
	}
	if a.CurrentMode() != "ask" {
		t.Fatalf("CurrentMode should be 'ask', got %q", a.CurrentMode())
	}
}
