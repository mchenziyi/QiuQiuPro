package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
	"agentdemo/tool"
)

// ---- 纯函数：累加 / 子集 / 命中率 ----

func TestTokenUsage_AddAccumulatesDetails(t *testing.T) {
	var u TokenUsage
	u.Add(openai.Usage{
		PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120,
		PromptTokensDetails:     &openai.PromptTokensDetails{CachedTokens: 80},
		CompletionTokensDetails: &openai.CompletionTokensDetails{ReasoningTokens: 6},
	})
	// 明细为 nil 的一次调用：只累加总量，子项不变。
	u.Add(openai.Usage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14})

	if u.Calls != 2 {
		t.Fatalf("Calls=%d，期望 2", u.Calls)
	}
	if u.PromptTokens != 110 || u.CompletionTokens != 24 || u.TotalTokens != 134 {
		t.Fatalf("总量累加错误：%+v", u)
	}
	if u.CachedTokens != 80 || u.ReasoningTokens != 6 {
		t.Fatalf("明细累加错误：cached=%d reasoning=%d", u.CachedTokens, u.ReasoningTokens)
	}
}

func TestTokenUsage_MissTokensAndHitRate(t *testing.T) {
	u := TokenUsage{PromptTokens: 100, CachedTokens: 80}
	if u.MissTokens() != 20 {
		t.Fatalf("MissTokens=%d，期望 20", u.MissTokens())
	}
	if r := u.HitRate(); r < 0.79 || r > 0.81 {
		t.Fatalf("HitRate=%v，期望 ~0.8", r)
	}
	// 异常：缓存数大于输入（不应发生）→ Miss 兜底为 0，不返回负数。
	bad := TokenUsage{PromptTokens: 10, CachedTokens: 50}
	if bad.MissTokens() != 0 {
		t.Fatalf("缓存>输入时 MissTokens 应兜底 0，实际 %d", bad.MissTokens())
	}
	// 空用量：命中率为 0，不除零。
	if (TokenUsage{}).HitRate() != 0 {
		t.Fatal("空用量命中率应为 0")
	}
}

func TestTokenUsage_AddUsageAndSub(t *testing.T) {
	base := TokenUsage{Calls: 1, PromptTokens: 100, CachedTokens: 80, CompletionTokens: 20, ReasoningTokens: 5, TotalTokens: 120}
	cur := base
	cur.AddUsage(TokenUsage{Calls: 2, PromptTokens: 50, CachedTokens: 10, CompletionTokens: 8, ReasoningTokens: 1, TotalTokens: 58})

	delta := cur.Sub(base)
	if delta.Calls != 2 || delta.PromptTokens != 50 || delta.CachedTokens != 10 ||
		delta.CompletionTokens != 8 || delta.ReasoningTokens != 1 || delta.TotalTokens != 58 {
		t.Fatalf("Sub 增量错误：%+v", delta)
	}
}

// ---- 费用估算 ----

func TestPricing_EnabledAndCost(t *testing.T) {
	if (Pricing{}).Enabled() {
		t.Fatal("零值单价应视为未配置")
	}
	p := Pricing{InputMiss: 2, InputHit: 0.2, Output: 8} // 每 1M token
	if !p.Enabled() {
		t.Fatal("配置了单价应为 enabled")
	}
	// 1M 未命中输入 + 1M 命中输入 + 1M 输出 = 2 + 0.2 + 8 = 10.2
	u := TokenUsage{PromptTokens: 2_000_000, CachedTokens: 1_000_000, CompletionTokens: 1_000_000}
	if got := u.Cost(p); got < 10.19 || got > 10.21 {
		t.Fatalf("Cost=%v，期望 ~10.2", got)
	}
}

// ---- 文案 ----

func TestTokenUsage_Format(t *testing.T) {
	u := TokenUsage{Calls: 3, PromptTokens: 100, CachedTokens: 80, CompletionTokens: 20, ReasoningTokens: 6, TotalTokens: 120}

	compact := u.FormatCompact()
	for _, want := range []string{"输入 100", "缓存 80", "输出 20", "思考 6", "合计 120"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("FormatCompact 缺少 %q：%s", want, compact)
		}
	}

	// 未配置单价：不展示费用。
	noPrice := u.FormatSession(Pricing{})
	for _, want := range []string{"会话 token 用量", "3 次调用", "缓存命中 80", "命中率", "合计 120"} {
		if !strings.Contains(noPrice, want) {
			t.Fatalf("FormatSession 缺少 %q：%s", want, noPrice)
		}
	}
	if strings.Contains(noPrice, "估算费用") {
		t.Fatalf("未配置单价不应展示费用：%s", noPrice)
	}

	// 配置单价：展示费用。
	withPrice := u.FormatSession(Pricing{Output: 8})
	if !strings.Contains(withPrice, "估算费用") {
		t.Fatalf("配置单价应展示费用：%s", withPrice)
	}
}

// ---- 集成：流式响应捕获用量 ----

// streamChunks 构造一段 content + 末尾 usage 的 SSE 分片。
func usageStreamChunks() []string {
	return []string{
		`{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"}}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"世界"}}]}`,
		`{"id":"x","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120,"prompt_tokens_details":{"cached_tokens":80},"completion_tokens_details":{"reasoning_tokens":6}}}`,
	}
}

func newSSEServer(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
			if fl != nil {
				fl.Flush()
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newSSEAgent(t *testing.T, srv *httptest.Server) *Agent {
	t.Helper()
	config := openai.DefaultConfig("test-key")
	config.BaseURL = srv.URL // go-openai 会拼接 /chat/completions
	return &Agent{
		client:   openai.NewClientWithConfig(config),
		model:    "test-model",
		allTools: map[string]tool.Tool{},
		gate:     AllowAllGate{},
		store:    event.NewStore(t.TempDir()),
		session:  NewSession("test"),
		sink:     ConsoleSink{},
	}
}

func TestStreamChat_AccountsUsage(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	a.Quiet = true

	msg, usage, err := a.streamChat(context.Background(), a.session.BuildRequest(""))
	if err != nil {
		t.Fatalf("streamChat 报错：%v", err)
	}
	_ = usage
	if msg.Content != "你好世界" {
		t.Fatalf("内容拼接错误：%q", msg.Content)
	}
	u := a.Usage()
	if u.Calls != 1 || u.PromptTokens != 100 || u.CompletionTokens != 20 ||
		u.CachedTokens != 80 || u.ReasoningTokens != 6 || u.TotalTokens != 120 {
		t.Fatalf("会话用量未正确累计：%+v", u)
	}
	// 仍要为压缩判定保留真实 prompt_tokens。
	if a.lastPromptTokens != 100 {
		t.Fatalf("lastPromptTokens=%d，期望 100", a.lastPromptTokens)
	}
}

// 一轮 Run 结束后应输出「本轮 token」摘要（非安静模式），并把用量计入会话累计。
func TestRun_ReportsTurnUsage(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	cs := &captureSink{}
	a.SetSink(cs)
	a.Quiet = false

	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run 报错：%v", err)
	}

	if a.Usage().TotalTokens != 120 {
		t.Fatalf("会话用量应计入 Run：%+v", a.Usage())
	}
	found := false
	for _, ev := range cs.events {
		if ev.Kind == EventNotice && strings.Contains(ev.Text, "本轮 token") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Run 结束应输出本轮 token 摘要，事件：%+v", cs.events)
	}
}

// 安静模式下不应输出本轮 token 摘要（属细节日志）。
func TestRun_TurnUsageHiddenWhenQuiet(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	cs := &captureSink{}
	a.SetSink(cs)
	a.Quiet = true

	if _, err := a.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("Run 报错：%v", err)
	}
	for _, ev := range cs.events {
		if strings.Contains(ev.Text, "本轮 token") {
			t.Fatalf("安静模式不应输出本轮 token 摘要：%+v", ev)
		}
	}
}

// /usage 走的 ReportUsage：输出会话汇总，配置单价时附带费用。
func TestReportUsage(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.Quiet = false
	cs := &captureSink{}
	a.SetSink(cs)
	a.usage = TokenUsage{Calls: 2, PromptTokens: 100, CachedTokens: 80, CompletionTokens: 20, TotalTokens: 120}
	a.SetPricing(Pricing{Output: 8})

	a.ReportUsage()

	joined := ""
	for _, ev := range cs.events {
		joined += ev.Text
	}
	for _, want := range []string{"会话 token 用量", "合计 120", "估算费用"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ReportUsage 输出缺少 %q：%s", want, joined)
		}
	}
}

// 子 Agent 的用量应在任务结束后并入父级会话总量。
func TestSpawnSubAgent_RollsUpUsage(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	a.Quiet = true

	if _, err := a.SpawnSubAgent(context.Background(), "子任务"); err != nil {
		t.Fatalf("SpawnSubAgent 报错：%v", err)
	}
	if a.Usage().TotalTokens != 120 || a.Usage().Calls != 1 {
		t.Fatalf("子 Agent 用量应并入父级，实际 %+v", a.Usage())
	}
}
