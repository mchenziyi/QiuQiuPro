package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

func TestBuildRequest_EmptyToolContentHasContentField(t *testing.T) {
	s := NewSession("t")
	s.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "c1", Name: "bash", Content: ""})

	if s.Messages()[0].Content != "" {
		t.Fatal("stored history should keep empty tool content unchanged")
	}

	req := s.BuildRequest("")
	if len(req) != 1 {
		t.Fatalf("expected 1 message, got %d", len(req))
	}
	if req[0].Content != apiEmptyContentPlaceholder {
		t.Fatalf("BuildRequest content=%q want %q", req[0].Content, apiEmptyContentPlaceholder)
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"content":"ok"`) {
		t.Fatalf("serialized request missing content field: %s", data)
	}
}

func TestBuildRequest_EmptyAssistantWithToolCallsHasContentField(t *testing.T) {
	s := NewSession("t")
	s.Add(openai.ChatCompletionMessage{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{
			{ID: "c1", Type: "function", Function: openai.FunctionCall{Name: "bash", Arguments: "{}"}},
		},
	})

	req := s.BuildRequest("")
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"content":"ok"`) {
		t.Fatalf("serialized request missing content field: %s", data)
	}
}

func TestDispatchToolCalls_EmptyResultBuildsValidRequest(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.allTools["bash"] = tool.Tool{
		Name: "bash",
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", nil
		},
	}

	a.dispatchToolCalls([]openai.ToolCall{tcOf("c0", "bash")})

	req := a.session.BuildRequest("sys")
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"content":"ok"`) {
		t.Fatalf("empty bash result must not omit content: %s", data)
	}
}
