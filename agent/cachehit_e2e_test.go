package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
	"agentdemo/tool"
)

// newEchoTool 驱动多步 tool loop。
func newEchoTool() tool.Tool {
	return tool.Tool{
		Name:        "echo",
		Description: "echo back the given text",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []string{"text"},
		},
		ReadOnly: true,
		Execute: func(_ context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(args, &p)
			return "echoed: " + p.Text, nil
		},
	}
}

const cacheTestSystemPrompt = "You are QiuQiuPro, a coding agent. Be concise. " +
	"This system prompt is the cacheable head of every request and must stay stable between turns."

// mockDeepSeek 按相邻请求的 messages 公共前缀推导 cached_tokens，直接测量客户端前缀稳定性。
type mockDeepSeek struct {
	t            *testing.T
	prevMessages []json.RawMessage
	reqChars     []int
	hitChars     []int
	withTools    bool
	toolRounds   int
}

func (m *mockDeepSeek) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	if isSummarizeRequest(body) {
		writeCacheSSE(w, m.t,
			cacheRawChunk(`{"choices":[{"index":0,"delta":{"role":"assistant","content":"summary"}}]}`),
			cacheRawChunk(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`),
			cacheUsageChunk(100, 40, 0),
		)
		return
	}

	msgs := decodeRequestMessages(body)
	common := commonPrefixMsgs(m.prevMessages, msgs)
	hitChars := charsOf(msgs[:common])
	totalChars := charsOf(msgs)
	m.prevMessages = msgs
	m.reqChars = append(m.reqChars, totalChars)
	m.hitChars = append(m.hitChars, hitChars)

	promptTok := max(1, totalChars/4)
	hitTok := hitChars / 4
	if hitTok > promptTok {
		hitTok = promptTok
	}

	emitTool := m.withTools && m.toolRounds > 0
	if emitTool {
		m.toolRounds--
		idx := len(m.reqChars)
		writeCacheSSE(w, m.t,
			cacheRawChunk(fmt.Sprintf(`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_%d","type":"function","function":{"name":"echo","arguments":"{\"text\":\"round-%d\"}"}}]}}]}`, idx, idx)),
			cacheRawChunk(`{"choices":[{"index":0,"finish_reason":"tool_calls","delta":{}}]}`),
			cacheUsageChunk(promptTok, 50, hitTok),
		)
		return
	}
	writeCacheSSE(w, m.t,
		cacheRawChunk(`{"choices":[{"index":0,"delta":{"role":"assistant","content":"Done."}}]}`),
		cacheRawChunk(`{"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}`),
		cacheUsageChunk(promptTok, 50, hitTok),
	)
}

func newCacheHitAgent(t *testing.T, baseURL string, withEcho bool, contextWindow int) *Agent {
	t.Helper()
	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = baseURL
	a := &Agent{
		client:            openai.NewClientWithConfig(cfg),
		model:             "test-model",
		allTools:          map[string]tool.Tool{},
		gate:              AllowAllGate{},
		store:             event.NewStore(t.TempDir()),
		session:           NewSession("cache-e2e"),
		sink:              ConsoleSink{},
		Quiet:             true,
		sysPrompt:         cacheTestSystemPrompt,
		contextWindow:     contextWindow,
		compactRatio:      defaultCompactRatio,
		softCompactRatio:  defaultSoftRatio,
		compactForceRatio: defaultCompactForce,
		summarizer:        func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
			return "summary", nil
		},
	}
	a.composeCachedSystemPrompt()
	if withEcho {
		a.RegisterTool(newEchoTool())
	}
	return a
}

func usageHitRate(u openai.Usage) float64 {
	hit := cacheHitTokens(u)
	miss := cacheMissTokens(u)
	denom := hit + miss
	if denom == 0 {
		if u.PromptTokens == 0 {
			return 0
		}
		return float64(hit) / float64(u.PromptTokens)
	}
	return float64(hit) / float64(denom)
}

func sessionHitRate(a *Agent) float64 {
	hit, miss := a.SessionCacheStats()
	denom := hit + miss
	if denom == 0 {
		return 0
	}
	return float64(hit) / float64(denom)
}

func perRequestHitRates(hitChars, reqChars []int) []float64 {
	out := make([]float64, len(reqChars))
	for i, total := range reqChars {
		if total == 0 {
			continue
		}
		out[i] = float64(hitChars[i]) / float64(total)
	}
	return out
}

func tailAverageRate(rates []float64, n int) float64 {
	if len(rates) == 0 {
		return 0
	}
	if n > len(rates) {
		n = len(rates)
	}
	sum := 0.0
	for _, r := range rates[len(rates)-n:] {
		sum += r
	}
	return sum / float64(n)
}

// TestCacheHitPrefixStable 证明 tool loop 下 append-only 前缀不被破坏。
func TestCacheHitPrefixStable(t *testing.T) {
	mock := &mockDeepSeek{t: t, withTools: true, toolRounds: 2}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, true, 0)
	if _, err := a.Run(context.Background(), "echo a couple things then finish"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(mock.reqChars) < 3 {
		t.Fatalf("tool loop 应至少 3 次 LLM 请求，实际 %d", len(mock.reqChars))
	}
	for i := 1; i < len(mock.reqChars); i++ {
		if mock.hitChars[i] != mock.reqChars[i-1] {
			t.Errorf("req %d: cached prefix %d chars, prior request was %d chars — prefix broken",
				i, mock.hitChars[i], mock.reqChars[i-1])
		}
	}
	rates := perRequestHitRates(mock.hitChars, mock.reqChars)
	t.Logf("prefix stable across %d LLM requests; per-request rates=%v", len(mock.reqChars), rates)
}

// TestCacheHitClimbsWithoutCompaction 多轮对话且关闭压缩，命中率应单调爬升。
func TestCacheHitClimbsWithoutCompaction(t *testing.T) {
	mock := &mockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, false, 0)

	const turns = 14
	for i := 0; i < turns; i++ {
		msg := "Turn " + fmt.Sprint(i) + ": " + strings.Repeat("please keep the prefix stable. ", 8)
		if _, err := a.Run(context.Background(), msg); err != nil {
			t.Fatalf("Run %d: %v", i, err)
		}
	}

	rates := perRequestHitRates(mock.hitChars, mock.reqChars)
	tail := tailAverageRate(rates, 5)
	t.Logf("after %d turns: tail-5 per-request hit rate %.2f%%, session aggregate %.2f%%",
		turns, tail*100, sessionHitRate(a)*100)

	if tail < 0.90 {
		t.Errorf("tail-5 per-request hit rate = %.2f%%, want ≥90%%", tail*100)
	}
	if len(rates) >= 6 && rates[len(rates)-1] <= rates[3] {
		t.Errorf("hit rate should climb: rate[3]=%.2f%% rate[last]=%.2f%%",
			rates[3]*100, rates[len(rates)-1]*100)
	}
}

// TestCacheHitReaches995AfterWarmup 先焐热长前缀、再以极小增量追加，末几轮应达 Reasonix 级 99.5%+。
func TestCacheHitReaches995AfterWarmup(t *testing.T) {
	mock := &mockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, false, 0)

	// 阶段 1：焐热 —— 10 轮长输入
	for i := 0; i < 10; i++ {
		msg := strings.Repeat("warmup context block. ", 50)
		if _, err := a.Run(context.Background(), msg); err != nil {
			t.Fatalf("warmup %d: %v", i, err)
		}
	}
	// 阶段 2：短 tail —— 40 轮极小增量
	for i := 0; i < 40; i++ {
		if _, err := a.Run(context.Background(), "x"); err != nil {
			t.Fatalf("short %d: %v", i, err)
		}
	}

	rates := perRequestHitRates(mock.hitChars, mock.reqChars)
	tail := tailAverageRate(rates, 5)
	t.Logf("warmup+short: tail-5 hit rate %.2f%% (last5=%v)", tail*100, rates[len(rates)-5:])

	if tail < 0.995 {
		t.Errorf("tail-5 per-request hit rate = %.2f%%, want ≥99.5%% after warmup", tail*100)
	}
	for i := 1; i < len(mock.reqChars); i++ {
		if mock.hitChars[i] != mock.reqChars[i-1] {
			t.Fatalf("prefix broken at req %d", i)
		}
	}
}

// TestCacheHitRememberUsesTurnTail 会话内 remember 只改 turn-tail，cached system 前缀保持稳定。
func TestCacheHitRememberUsesTurnTail(t *testing.T) {
	mock := &mockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	store := NewMemoryStore(t.TempDir()+"/g.json", t.TempDir()+"/p.json")
	a := newCacheHitAgent(t, srv.URL, false, 0)
	a.SetMemoryStore(store)
	a.RegisterTool(a.NewRememberRuleTool())

	sysBefore := a.BuildSystemPrompt()
	a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{
		Name:      memoryToolName,
		Arguments: `{"scope":"global","kind":"preference","content":"默认中文","reason":"偏好"}`,
	}})
	if a.BuildSystemPrompt() != sysBefore {
		t.Fatal("remember 不应修改 cached system prompt")
	}

	// remember 后的第一轮含 <memory-update>；末 4 轮短输入应恢复高命中
	for i := 0; i < 10; i++ {
		if _, err := a.Run(context.Background(), strings.Repeat("warm. ", 50)); err != nil {
			t.Fatalf("warm %d: %v", i, err)
		}
	}
	for i := 0; i < 55; i++ {
		if _, err := a.Run(context.Background(), "y"); err != nil {
			t.Fatalf("short %d: %v", i, err)
		}
	}

	if rate := tailAverageRate(perRequestHitRates(mock.hitChars, mock.reqChars), 5); rate < 0.99 {
		t.Errorf("after remember + warmup + short turns, tail-5 hit rate = %.2f%%, want ≥99%% (memory-update 首轮会略拉低)", rate*100)
	}
}

// TestCacheHitPerTurnUsageMatchesPrefix 每轮 streamChat 报告的 cached_tokens 与 mock 前缀一致。
func TestCacheHitPerTurnUsageMatchesPrefix(t *testing.T) {
	mock := &mockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, false, 0)
	// 先焐热再短增量 stream
	for i := 0; i < 10; i++ {
		a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("warm ", 400)})
		a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "ok"})
	}
	for i := 0; i < 10; i++ {
		_, usage, err := a.streamChat(context.Background(), a.session.BuildRequest(a.BuildSystemPrompt()))
		if err != nil {
			t.Fatalf("streamChat %d: %v", i, err)
		}
		if usage.PromptTokens == 0 {
			t.Fatalf("turn %d: missing usage", i)
		}
		if cacheHitTokens(usage)+cacheMissTokens(usage) != usage.PromptTokens {
			t.Fatalf("turn %d: hit+miss != prompt (%d+%d != %d)",
				i, cacheHitTokens(usage), cacheMissTokens(usage), usage.PromptTokens)
		}
		a.accumulateSessionCache(usage)
		a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "ok-" + fmt.Sprint(i)})
		a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "z"})
	}

	if i := len(mock.reqChars); i < 10 {
		t.Fatalf("mock saw %d requests, want 10", i)
	}
	for j := 1; j < len(mock.reqChars); j++ {
		if mock.hitChars[j] != mock.reqChars[j-1] {
			t.Errorf("stream turn %d prefix mismatch", j)
		}
	}
	if rate := tailAverageRate(perRequestHitRates(mock.hitChars, mock.reqChars), 3); rate < 0.995 {
		t.Errorf("stream tail-3 hit rate = %.2f%%, want ≥99.5%%", rate*100)
	}
}

// ---- mock helpers ----

func decodeRequestMessages(body []byte) []json.RawMessage {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Messages
}

func isSummarizeRequest(body []byte) bool {
	msgs := decodeRequestMessages(body)
	if len(msgs) == 0 {
		return false
	}
	var m struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(msgs[0], &m)
	return m.Role == "system" && strings.Contains(m.Content, "对话摘要助手")
}

func commonPrefixMsgs(a, b []json.RawMessage) int {
	n := 0
	for n < len(a) && n < len(b) && bytes.Equal(a[n], b[n]) {
		n++
	}
	return n
}

func charsOf(msgs []json.RawMessage) int {
	total := 0
	for _, m := range msgs {
		total += len(m)
	}
	return total
}

func writeCacheSSE(w http.ResponseWriter, t *testing.T, chunks ...string) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	fl, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("ResponseWriter is not a Flusher")
	}
	for _, c := range chunks {
		fmt.Fprintf(w, "data: %s\n\n", c)
		fl.Flush()
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	fl.Flush()
}

func cacheRawChunk(jsonBody string) string {
	return `{"id":"mock","object":"chat.completion.chunk",` + strings.TrimPrefix(jsonBody, "{")
}

func cacheUsageChunk(prompt, completion, hit int) string {
	return fmt.Sprintf(`{"id":"mock","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d,"prompt_tokens_details":{"cached_tokens":%d}}}`,
		prompt, completion, prompt+completion, hit)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
