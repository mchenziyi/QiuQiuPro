package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestSession_NeedsCompaction(t *testing.T) {
	s := NewSession("t")
	s.maxMessages = 4
	for i := 0; i < 4; i++ {
		s.Add(openai.ChatCompletionMessage{Role: "user", Content: "x"})
	}
	if s.NeedsCompaction() {
		t.Fatal("恰好等于上限不应触发压缩")
	}
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "x"})
	if !s.NeedsCompaction() {
		t.Fatal("超过上限应触发压缩")
	}
}

// 切分时保留段不能以孤立 tool 开头，且 old+recent 必须能还原原历史。
func TestSession_SplitForCompaction_PairAware(t *testing.T) {
	s := NewSession("t")
	s.maxMessages = 6 // keep = 3
	mk := func(role, content string, tcs ...string) openai.ChatCompletionMessage {
		m := openai.ChatCompletionMessage{Role: role, Content: content}
		for _, id := range tcs {
			m.ToolCalls = append(m.ToolCalls, openai.ToolCall{ID: id, Type: "function",
				Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"}})
		}
		return m
	}
	// 构造让 cut=n-keep=8-3=5 恰好落在孤立 tool 上：assistant(3) 带 B、C 两个调用 → tool B(4)、tool C(5)。
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

	old, recent := s.SplitForCompaction()
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

// 超限时应调用摘要器、用「摘要 + 近消息」替换历史。
func TestMaybeCompact_SummarizesWhenOverLimit(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.session.maxMessages = 4 // keep = 2
	called := false
	a.summarizer = func(_ context.Context, msgs []openai.ChatCompletionMessage) (string, error) {
		called = true
		return "FAKE_SUMMARY", nil
	}
	for i := 0; i < 6; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		a.session.Add(openai.ChatCompletionMessage{Role: role, Content: fmt.Sprintf("m%d", i)})
	}

	a.maybeCompact(context.Background())

	if !called {
		t.Fatal("超限时应调用摘要器")
	}
	msgs := a.session.Messages()
	if msgs[0].Role != "user" || !strings.Contains(msgs[0].Content, "FAKE_SUMMARY") {
		t.Fatalf("首条应为含摘要的消息，实际 %+v", msgs[0])
	}
	if a.session.Len() > a.session.maxMessages {
		t.Fatalf("压缩后不应超过上限，实际 %d", a.session.Len())
	}
}

// 摘要失败应退化为裁剪：仍兜住体积，且不留摘要消息。
func TestMaybeCompact_FallsBackToTrimOnError(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.session.maxMessages = 4
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		return "", errors.New("boom")
	}
	for i := 0; i < 6; i++ {
		a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: fmt.Sprintf("m%d", i)})
	}

	a.maybeCompact(context.Background())

	if a.session.Len() > a.session.maxMessages {
		t.Fatalf("退化裁剪后不应超过上限，实际 %d", a.session.Len())
	}
	for _, m := range a.session.Messages() {
		if strings.Contains(m.Content, "摘要") {
			t.Fatalf("摘要失败时不应留下摘要消息，实际 %+v", m)
		}
	}
}

// 未超限不应触发压缩，历史原样不动。
func TestMaybeCompact_NoopUnderLimit(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetSink(&captureSink{})
	a.summarizer = func(_ context.Context, _ []openai.ChatCompletionMessage) (string, error) {
		t.Fatal("未超限不应调用摘要器")
		return "", nil
	}
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})

	a.maybeCompact(context.Background())

	if a.session.Len() != 1 {
		t.Fatalf("未超限历史应不变，实际 %d", a.session.Len())
	}
}
