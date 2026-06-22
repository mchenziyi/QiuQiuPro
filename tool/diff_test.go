package tool

import (
	"encoding/json"
	"testing"
)

func TestComputeLineDiff_SimpleReplace(t *testing.T) {
	before := "line1\nold_line\nline3"
	after := "line1\nnew_line\nline3"
	result := ComputeLineDiff(before, after, "test.go", 0)
	if len(result.Hunks) == 0 {
		t.Fatal("expected at least 1 hunk")
	}
	h := result.Hunks[0]
	foundDel, foundAdd := false, false
	for _, l := range h.Lines {
		if l.Op == "del" && l.Text == "old_line" {
			foundDel = true
		}
		if l.Op == "add" && l.Text == "new_line" {
			foundAdd = true
		}
	}
	if !foundDel {
		t.Fatal("expected del old_line")
	}
	if !foundAdd {
		t.Fatal("expected add new_line")
	}
}

func TestComputeLineDiff_JSONMarshal(t *testing.T) {
	before := "const (\n\tdefaultCompactRatio = 0.8\n\tdefaultSoftRatio = 0.5\n)"
	after := "const (\n\tdefaultCompactRatio = 0.95\n\tdefaultSoftRatio = 0.85\n)"
	result := ComputeLineDiff(before, after, "agent/compact.go", 1)
	data, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("diff JSON:\n%s", string(data))
	if len(result.Hunks) == 0 {
		t.Fatal("expected at least 1 hunk")
	}
	// Verify JSON output is valid for frontend consumption
	var parsed DiffResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
}

func TestComputeLineDiff_NoChange(t *testing.T) {
	before := "same\ncontent"
	result := ComputeLineDiff(before, before, "same.go", 0)
	if len(result.Hunks) != 0 {
		t.Fatalf("expected 0 hunks for no change, got %d", len(result.Hunks))
	}
}

func TestComputeLineDiff_AddLines(t *testing.T) {
	before := "line1\nline2"
	after := "line1\nnew_line\nline2"
	result := ComputeLineDiff(before, after, "add.go", 0)
	if len(result.Hunks) == 0 {
		t.Fatal("expected at least 1 hunk")
	}
	h := result.Hunks[0]
	foundAdd := false
	for _, l := range h.Lines {
		if l.Op == "add" && l.Text == "new_line" {
			foundAdd = true
		}
	}
	if !foundAdd {
		t.Fatalf("expected add line 'new_line'")
	}
}
