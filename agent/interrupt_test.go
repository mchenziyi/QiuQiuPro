package agent

import (
	"errors"
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

func TestCheckInterrupted(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "before"})
	start := a.session.Len()
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: "interrupted turn"})

	if err := a.checkInterrupted(start); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	a.Interrupt()
	if err := a.checkInterrupted(start); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if a.session.Len() != start {
		t.Fatalf("abort should rollback interrupted turn, len=%d want %d", a.session.Len(), start)
	}
	if a.session.Messages()[0].Content != "before" {
		t.Fatalf("rollback kept wrong history: %q", a.session.Messages()[0].Content)
	}
}
