package agent

import (
	"encoding/json"
	"testing"
)

func TestEmitToolResultWithDiffIfJSON(t *testing.T) {
	// 模拟 edit_file 返回的 JSON 结果
	result := `{"text":"已编辑 test.go","diff":{"path":"test.go","hunks":[{"old_start":3,"new_start":3,"lines":[{"op":"del","text":"old"},{"op":"add","text":"new"}]}]}}`
	var wrapped struct {
		Text string                 `json:"text"`
		Diff map[string]interface{} `json:"diff"`
	}
	if err := json.Unmarshal([]byte(result), &wrapped); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if wrapped.Text == "" {
		t.Fatal("Text is empty")
	}
	if wrapped.Diff == nil {
		t.Fatal("Diff is nil")
	}
	t.Logf("Text: %s", wrapped.Text)
	t.Logf("Diff: %+v", wrapped.Diff)

	// Verify diff path
	path, _ := wrapped.Diff["path"].(string)
	if path != "test.go" {
		t.Fatalf("expected path=test.go, got %s", path)
	}

	// Verify diff has hunks
	hunks, ok := wrapped.Diff["hunks"].([]interface{})
	if !ok || len(hunks) == 0 {
		t.Fatal("expected hunks array")
	}
	hunk := hunks[0].(map[string]interface{})
	lines, _ := hunk["lines"].([]interface{})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	first := lines[0].(map[string]interface{})
	if first["op"] != "del" {
		t.Fatalf("expected op=del, got %v", first["op"])
	}
}
