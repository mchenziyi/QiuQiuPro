package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
)

func TestRestoreFromCheckpoint_ReusesPinnedSession(t *testing.T) {
	dir := t.TempDir()
	store := event.NewStore(dir)

	msgs, _ := (&Session{ID: "sess-old"}).Snapshot()
	if err := store.SaveCheckpoint("sess-old", "evt-1", msgs); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	s := NewSession("sess-old")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "我叫李四"})
	data, _ := s.Snapshot()
	if err := store.SaveCheckpoint("sess-old", "evt-2", data); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	resumedID := store.ResolveSessionID()
	if resumedID != "sess-old" {
		t.Fatalf("ResolveSessionID = %q, want sess-old", resumedID)
	}

	a := &Agent{store: store, session: NewSession(resumedID), sink: ConsoleSink{}}
	if !a.RestoreFromCheckpoint() {
		t.Fatal("RestoreFromCheckpoint 应成功")
	}
	if a.session.Len() != 1 {
		t.Fatalf("恢复后消息数 = %d, want 1", a.session.Len())
	}
	if !strings.Contains(a.session.Messages()[0].Content, "李四") {
		t.Fatalf("恢复内容不对: %q", a.session.Messages()[0].Content)
	}
}
