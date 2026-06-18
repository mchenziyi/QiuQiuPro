package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// addMsgs 往会话塞 n 条交替 user/assistant、内容定长的消息，便于按字符数推算 token。
func addMsgs(s *Session, n, contentLen int) {
	pad := strings.Repeat("x", contentLen)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		s.Add(openai.ChatCompletionMessage{Role: role, Content: pad})
	}
}

func TestSession_CharCount(t *testing.T) {
	s := NewSession("t")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "abc"}) // 3
	s.Add(openai.ChatCompletionMessage{Role: "assistant", ToolCalls: []openai.ToolCall{{
		Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"}}}}) // 9 + 2
	if got := s.CharCount(); got != 14 {
		t.Fatalf("CharCount=%d，期望 14", got)
	}
}

// 切分时保留段不能以孤立 tool 开头，且 old+recent 必须能还原原历史。
func TestSession_SplitForCompaction_PairAware(t *testing.T) {
	s := NewSession("t")
	mk := func(role, content string, tcs ...string) openai.ChatCompletionMessage {
		m := openai.ChatCompletionMessage{Role: role, Content: content}
		for _, id := range tcs {
			m.ToolCalls = append(m.ToolCalls, openai.ToolCall{ID: id, Type: "function",
				Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"}})
		}
		return m
	}
	// 按 tokPerChar=1.0、tailBudget=13 累加，cut 恰好落在孤立 tool C(index 5) 上，应被向前跳过。
	s.messages = []openai.ChatCompletionMessage{
		mk("user", "0"),
		mk("assistant", "", "A"),
		{Role: "tool", ToolCallID: "A", Name: "read_file", Content: "a"},
		mk("assistant", "", "B", "C"),
		{Role: "tool", ToolCallID: "B", Name: "read_file", Content: "b"},
		{Role: "tool", ToolCallID: "C", Name: "read_file", Content: "c"}, // index 5
		mk("assistant", "", "D"),
		{Role: "tool", ToolCallID: "D", Name: "read_file", Content: "d"},
	}

	old, recent := s.SplitForCompaction(13, 1.0)
	if len(recent) == 0 || recent[0].Role == "tool" {
		t.Fatalf("保留段不应以孤立 tool 开头，实际 recent[0]=%+v", recent[0])
	}
	if len(old)+len(recent) != len(s.messages) {
		t.Fatalf("old+recent 应还原原历史：%d+%d≠%d", len(old), len(recent), len(s.messages))
	}
	assertValidToolPairing(t, recent)
}

func TestSession_ApplyCompaction(t *testing.T) {
	recent := []openai.ChatCompletionMessage{{Role: "assistant", Content: "近况"}}

	s := NewSession("t")
	s.ApplyCompaction("摘要内容", recent)
	got := s.Messages()
	if len(got) != 2 || got[0].Role != "user" || !strings.Contains(got[0].Content, "摘要内容") {
		t.Fatalf("应前置一条含摘要的 user 消息，实际 %+v", got)
	}
	if got[1].Content != "近况" {
		t.Fatalf("摘要后应接上近消息，实际 %+v", got[1])
	}

	// 空摘要 → 仅保留近消息（等价裁剪）。
	s2 := NewSession("t")
	s2.ApplyCompaction("", recent)
	if s2.Len() != 1 || s2.Messages()[0].Content != "近况" {
		t.Fatalf("空摘要应只保留近消息，实际 %+v", s2.Messages())
	}
}

func TestRenderForSummary(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: "user", Content: "帮我看下 a.go"},
		{Role: "assistant", Content: "好的", ToolCalls: []openai.ToolCall{{
			Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`}}}},
		{Role: "tool", Name: "read_file", Content: "package main"},
	}
	out := renderForSummary(msgs)
	for _, want := range []string{"【用户】", "【助手】", "【助手·调用工具】read_file", "【工具结果·read_file】"} {
		if !strings.Contains(out, want) {
			t.Fatalf("摘要输入缺少 %q：\n%s", want, out)
		}
	}
}

// tokPerChar 应由真实用量推导；无用量或比值离谱时兜底。
func TestTokPerChar(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	if r := a.tokPerChar(); r != fallbackTokPerChar {
		t.Fatalf("无用量应兜底 %v，实际 %v", fallbackTokPerChar, r)
	}
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: strings.Repeat("x", 600)})
	a.lastPromptTokens = 300 // 300 / 600 = 0.5
	if r := a.tokPerChar(); r != 0.5 {
		t.Fatalf("用量推导应为 0.5，实际 %v", r)
	}
	a.lastPromptTokens = 100000 // 比值 >2，离谱 → 兜底
	if r := a.tokPerChar(); r != fallbackTokPerChar {
		t.Fatalf("离谱比值应兜底，实际 %v", r)
	}
}

// 真实 prompt_tokens 越过触发线（窗口 * compactRatio）时应摘要旧消息、清零遥测。
func TestMaybeCompact_SummarizesOverTrigger(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	addMsgs(a.session, 6, 100) // CharCount=600
	a.lastPromptTokens = 900   // ≥ high(800) → 触发；tokPerChar=900/600=1.5
	called := false
	a.summarizer = func(_ context.Context, msgs []openai.ChatCompletionMessage) (string, error) {
		called = true
		if len(msgs) == 0 {
			t.Fatal("待摘要的旧消息不应为空")
		}
		return "FAKE_SUMMARY", nil
	}

	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 900})

	if !called {
		t.Fatal("提示越过触发线应调用摘要器")
	}
	msgs := a.session.Messages()
	if msgs[0].Role != "user" || !strings.Contains(msgs[0].Content, "FAKE_SUMMARY") {
		t.Fatalf("首条应为含摘要的消息，实际 %+v", msgs[0])
	}
	if a.session.Len() >= 6 {
		t.Fatalf("压缩后条数应下降，实际 %d", a.session.Len())
	}
	if a.lastPromptTokens != 0 {
		t.Fatalf("压缩后应清零用量遥测，实际 %d", a.lastPromptTokens)
	}
}

// 软线到触发线之间：只提醒一次、绝不压缩（避免白白打掉前缀缓存）。
func TestMaybeCompact_SoftNoticeNoCompaction(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	cs := &captureSink{}
	a.SetSink(cs)
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	addMsgs(a.session, 4, 50)
	a.lastPromptTokens = 600 // soft(500) ≤ 600 < high(800)
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		t.Fatal("软线区间不应压缩")
		return "", nil
	}

	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 600})
	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 600}) // 再次：提醒只应出现一次

	if a.session.Len() != 4 {
		t.Fatalf("软线区间历史应不变，实际 %d", a.session.Len())
	}
	notices := 0
	for _, ev := range cs.events {
		if ev.Kind == EventNotice && strings.Contains(ev.Text, "窗口") {
			notices++
		}
	}
	if notices != 1 {
		t.Fatalf("软线提醒应只出现一次，实际 %d", notices)
	}
}

func TestRunCompactsAfterFinalAssistantMessage(t *testing.T) {
	mock := &mockDeepSeek{t: t}
	srv := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer srv.Close()

	a := newCacheHitAgent(t, srv.URL, false, 100)
	a.SetSink(&captureSink{})

	answer, err := a.Run(context.Background(), strings.Repeat("x", 1000))
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if answer != "Done." {
		t.Fatalf("answer=%q, want Done.", answer)
	}
	msgs := a.session.Messages()
	if len(msgs) == 0 || !strings.Contains(msgs[0].Content, "summary") {
		t.Fatalf("Run 应在最终 assistant 入历史后触发压缩，实际消息：%+v", msgs)
	}
}

func TestMaybeCompact_ForceCompactsHugeRecentAssistant(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	a.lastPromptTokens = 1500 // force: >= compactForceRatio
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "请详细解释"})
	a.session.Add(openai.ChatCompletionMessage{Role: "assistant", Content: strings.Repeat("长回答", 3000)})
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "你好"})
	a.summarizer = func(_ context.Context, msgs []openai.ChatCompletionMessage) (string, error) {
		if len(msgs) < 2 {
			t.Fatalf("强制压缩应折叠超长 assistant，实际待摘要消息数 %d", len(msgs))
		}
		return "SUMMARY_WITH_LONG_ASSISTANT", nil
	}

	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 1500})

	msgs := a.session.Messages()
	if len(msgs) != 2 {
		t.Fatalf("强制压缩后应只保留摘要 + 当前用户消息，实际 %d: %+v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0].Content, "SUMMARY_WITH_LONG_ASSISTANT") {
		t.Fatalf("首条应为摘要，实际 %+v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "你好" {
		t.Fatalf("应仅保留最近用户消息，实际 %+v", msgs[1])
	}
}

// 摘要失败应退化为裁剪：仍兜住体积，且不留摘要消息。
func TestMaybeCompact_FallsBackToTrimOnError(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	a.session.maxMessages = 4
	addMsgs(a.session, 6, 100)
	a.lastPromptTokens = 900
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		return "", errors.New("boom")
	}

	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 900})

	if a.session.Len() > a.session.maxMessages {
		t.Fatalf("退化裁剪后不应超过上限，实际 %d", a.session.Len())
	}
	for _, m := range a.session.Messages() {
		if strings.Contains(m.Content, "摘要") {
			t.Fatalf("摘要失败时不应留下摘要消息，实际 %+v", m)
		}
	}
}

// 未达软线不应触发压缩，历史原样不动。
func TestMaybeCompact_NoopUnderTrigger(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		t.Fatal("未达软线不应压缩")
		return "", nil
	}
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
	a.lastPromptTokens = 100 // < soft(500)

	a.maybeCompact(context.Background(), openai.Usage{PromptTokens: 100})

	if a.session.Len() != 1 {
		t.Fatalf("未达阈值历史应不变，实际 %d", a.session.Len())
	}
}

// 没有用量遥测（首轮 / provider 未回传）时不压缩，避免误判。
func TestMaybeCompact_NoUsageNoop(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		t.Fatal("无用量遥测时不应压缩")
		return "", nil
	}
	addMsgs(a.session, 50, 100) // 条数很多，但没有 token 遥测

	a.maybeCompact(context.Background(), openai.Usage{})

	if a.session.Len() != 50 {
		t.Fatalf("无用量遥测应不压缩，实际 %d", a.session.Len())
	}
}

// 手动 /compact 应无视比例阈值，强制压缩一次。
func TestCompact_ManualForces(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.contextWindow, a.compactRatio, a.softCompactRatio = 1000, 0.8, 0.5
	addMsgs(a.session, 6, 2000) // 内容够长，即便按兜底系数也能切出旧消息
	// lastPromptTokens 默认 0：自动压缩不会触发，手动应强制。
	called := false
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		called = true
		return "MANUAL_SUMMARY", nil
	}

	a.Compact(context.Background())

	if !called {
		t.Fatal("手动 /compact 应无视阈值强制摘要")
	}
	if !strings.Contains(a.session.Messages()[0].Content, "MANUAL_SUMMARY") {
		t.Fatalf("手动压缩后首条应为摘要，实际 %+v", a.session.Messages()[0])
	}
}
