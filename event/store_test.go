package event

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewStore_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	s := NewStore(dir)
	if s == nil {
		t.Fatal("NewStore 应返回非 nil")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("NewStore 应创建目录")
	}
}

func TestAppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	e := Event{
		ID:    "evt-1",
		Type:  "user",
		Content: "你好",
	}
	if err := s.Append("sess-1", e); err != nil {
		t.Fatalf("Append 失败：%v", err)
	}

	events, err := s.Load("sess-1")
	if err != nil {
		t.Fatalf("Load 失败：%v", err)
	}
	if len(events) != 1 {
		t.Fatalf("应有 1 条事件，实际 %d", len(events))
	}
	if events[0].ID != "evt-1" {
		t.Fatalf("事件 ID 应为 evt-1，实际 %q", events[0].ID)
	}
	if events[0].Content != "你好" {
		t.Fatalf("内容应为 '你好'，实际 %q", events[0].Content)
	}
}

func TestLoad_NonExistentSession(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	events, err := s.Load("no-such-session")
	if err != nil {
		t.Fatalf("不存在的 session 应返回空 slice：%v", err)
	}
	if len(events) != 0 {
		t.Fatalf("不存在的 session 应返回 0 条，实际 %d", len(events))
	}
}

func TestAppend_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Append("sess-multi", Event{ID: "1", Type: "user", Content: "hello"})
	s.Append("sess-multi", Event{ID: "2", Type: "assistant", Content: "world"})
	s.Append("sess-multi", Event{ID: "3", Type: "tool_call", ToolName: "read_file", Content: "test.txt"})

	events, err := s.Load("sess-multi")
	if err != nil {
		t.Fatalf("Load 失败：%v", err)
	}
	if len(events) != 3 {
		t.Fatalf("应有 3 条，实际 %d", len(events))
	}
	if events[0].Type != "user" {
		t.Fatalf("第 1 条类型应为 user，实际 %q", events[0].Type)
	}
	if events[2].ToolName != "read_file" {
		t.Fatalf("第 3 条 ToolName 应为 read_file，实际 %q", events[2].ToolName)
	}
}

func TestLoadSince(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.Append("sess-since", Event{ID: "a", Type: "user", Content: "1"})
	s.Append("sess-since", Event{ID: "b", Type: "assistant", Content: "2"})
	s.Append("sess-since", Event{ID: "c", Type: "user", Content: "3"})

	// LoadSince after "a" → should get b, c
	events, err := s.LoadSince("sess-since", "a")
	if err != nil {
		t.Fatalf("LoadSince 失败：%v", err)
	}
	if len(events) != 2 {
		t.Fatalf("LoadSince after 'a' 应返回 2 条，实际 %d", len(events))
	}
	if events[0].ID != "b" {
		t.Fatalf("第 1 条应为 b，实际 %q", events[0].ID)
	}

	// LoadSince empty → all
	events, err = s.LoadSince("sess-since", "")
	if err != nil {
		t.Fatalf("LoadSince('') 失败：%v", err)
	}
	if len(events) != 3 {
		t.Fatalf("LoadSince('') 应返回全部 3 条，实际 %d", len(events))
	}
}

func TestSaveAndLoadCheckpoint(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	err := s.SaveCheckpoint("sess-cp", "evt-5", `[{"role":"user","content":"hi"}]`)
	if err != nil {
		t.Fatalf("SaveCheckpoint 失败：%v", err)
	}

	cp, err := s.LoadCheckpoint("sess-cp")
	if err != nil {
		t.Fatalf("LoadCheckpoint 失败：%v", err)
	}
	if cp == nil {
		t.Fatal("Checkpoint 应存在")
	}
	if cp.LastEventID != "evt-5" {
		t.Fatalf("LastEventID 应为 evt-5，实际 %q", cp.LastEventID)
	}
	if !strings.Contains(cp.MessagesJSON, "hi") {
		t.Fatalf("MessagesJSON 应包含 'hi'，实际 %q", cp.MessagesJSON)
	}
	if cp.SessionID != "sess-cp" {
		t.Fatalf("SessionID 应为 sess-cp，实际 %q", cp.SessionID)
	}
}

func TestLoadCheckpoint_NonExistent(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	cp, err := s.LoadCheckpoint("no-cp")
	if err != nil {
		t.Fatalf("不存在的 checkpoint 应返回 nil：%v", err)
	}
	if cp != nil {
		t.Fatal("不存在的 checkpoint 应为 nil")
	}
}

func TestSaveAndLoadExecutionState(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	data := []byte(`{"step":1,"status":"paused"}`)
	if err := s.SaveExecutionState("sess-exec", data); err != nil {
		t.Fatalf("SaveExecutionState 失败：%v", err)
	}

	loaded, err := s.LoadExecutionState("sess-exec")
	if err != nil {
		t.Fatalf("LoadExecutionState 失败：%v", err)
	}
	if string(loaded) != string(data) {
		t.Fatalf("数据不匹配：期望 %q，实际 %q", string(data), string(loaded))
	}
}

func TestClearExecutionState(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	s.SaveExecutionState("sess-clr", []byte(`{"step":1}`))
	if err := s.ClearExecutionState("sess-clr"); err != nil {
		t.Fatalf("ClearExecutionState 失败：%v", err)
	}

	loaded, _ := s.LoadExecutionState("sess-clr")
	if loaded != nil {
		t.Fatal("清除后执行状态应为 nil")
	}
}

func TestReplay_Empty(t *testing.T) {
	result := Replay("sess", []Event{})
	if !strings.Contains(result, "没有事件记录") {
		t.Fatalf("空事件应显示 '没有事件记录'，实际：%s", result)
	}
}

func TestReplay_FormatsEvents(t *testing.T) {
	events := []Event{
		{ID: "1", Type: "user", Content: "你好"},
		{ID: "2", Type: "assistant", Content: "你好！有什么需要帮忙的吗？"},
		{ID: "3", Type: "tool_call", ToolName: "read_file", Content: "main.go"},
	}
	result := Replay("sess-replay", events)
	if !strings.Contains(result, "🧑") {
		t.Fatal("user 事件应显示 🧑")
	}
	if !strings.Contains(result, "🤖") {
		t.Fatal("assistant 事件应显示 🤖")
	}
	if !strings.Contains(result, "🔧") {
		t.Fatal("tool_call 事件应显示 🔧")
	}
	if !strings.Contains(result, "sess-replay") {
		t.Fatal("应包含 session ID")
	}
}

func TestEvent_HasTimestamp(t *testing.T) {
	// Timestamp 默认 zero，只有通过时间参数设定后才非零
	e := Event{ID: "t1", Type: "user", Content: "test"}
	if !e.Timestamp.IsZero() {
		t.Fatal("未设置 Timestamp 时应为零值")
	}
}
