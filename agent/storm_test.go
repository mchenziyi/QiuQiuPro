package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

type stormMockDeepSeek struct {
	t        *testing.T
	calls    int
	requests [][]json.RawMessage
}

func (m *stormMockDeepSeek) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	m.requests = append(m.requests, decodeRequestMessages(body))
	m.calls++
	if m.calls > stormThreshold {
		writeCacheSSE(w, m.t,
			cacheRawChunk(`{"choices":[{"index":0,"delta":{"role":"assistant","content":"hello"}}]}`),
			cacheRawChunk(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`),
			cacheUsageChunk(100, 10, 0),
		)
		return
	}
	writeCacheSSE(w, m.t,
		cacheRawChunk(fmt.Sprintf(`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"storm_%d","type":"function","function":{"name":"bad_tool","arguments":"{}"}}]}}]}`, m.calls)),
		cacheRawChunk(`{"choices":[{"index":0,"finish_reason":"tool_calls","delta":{}}]}`),
		cacheUsageChunk(100, 10, 0),
	)
}

func TestRunLoopGuardAppendsBoundaryAndKeepsCachePrefix(t *testing.T) {
	mock := &stormMockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, false, 0)
	a.RegisterTool(tool.Tool{
		Name:     "bad_tool",
		ReadOnly: true,
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "", fmt.Errorf("error: same failure")
		},
	})
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "previous"})
	a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "stable context"})
	start := a.session.Len()

	_, err := a.Run(context.Background(), "trigger repeated failure")
	if err == nil || !strings.Contains(err.Error(), "loop guard") {
		t.Fatalf("expected loop guard error, got %v", err)
	}
	msgs := a.session.Messages()
	if len(msgs) <= start {
		t.Fatalf("loop guard should keep failed turn append-only, len=%d start=%d", len(msgs), start)
	}
	last := msgs[len(msgs)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "新任务") {
		t.Fatalf("loop guard should append assistant boundary, last=%+v", last)
	}

	if answer, err := a.Run(context.Background(), "你好"); err != nil || answer != "hello" {
		t.Fatalf("next turn should run normally, answer=%q err=%v", answer, err)
	}
	if len(mock.requests) < stormThreshold+1 {
		t.Fatalf("expected at least %d requests, got %d", stormThreshold+1, len(mock.requests))
	}
	prev := mock.requests[stormThreshold-1]
	next := mock.requests[stormThreshold]
	if commonPrefixMsgs(prev, next) != len(prev) {
		t.Fatalf("next request should retain previous request as exact prefix for cache hit")
	}
}
