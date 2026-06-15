package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// edit_file_block 的参数 schema 用 snake_case（old_block / new_block），
// 测试必须按 LLM 实际调用方式（snake_case key）构造参数。
func editArgs(t *testing.T, path, oldBlock, newBlock string) string {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"path":      path,
		"old_block": oldBlock,
		"new_block": newBlock,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEditFileBlock_ReplacesUniqueBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	result := NewEditFileBlockTool().Execute(editArgs(t, path, `println("hi")`, `println("hello")`))

	if !strings.Contains(result, "已修改") {
		t.Fatalf("应修改成功，实际返回：%s", result)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `println("hello")`) {
		t.Fatalf("文件未被正确替换：\n%s", got)
	}
	if strings.Contains(string(got), `println("hi")`) {
		t.Fatalf("旧代码应被替换掉：\n%s", got)
	}
}

func TestEditFileBlock_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	result := NewEditFileBlockTool().Execute(editArgs(t, path, "NOPE", "x"))
	if !strings.Contains(result, "找不到指定的旧代码") {
		t.Fatalf("应提示找不到旧代码，实际：%s", result)
	}
}

func TestEditFileBlock_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("x\nx\n"), 0644)

	result := NewEditFileBlockTool().Execute(editArgs(t, path, "x", "y"))
	if !strings.Contains(result, "出现多次") {
		t.Fatalf("应提示出现多次，实际：%s", result)
	}
}

func TestEditFileBlock_FileMissing(t *testing.T) {
	result := NewEditFileBlockTool().Execute(editArgs(t, "/no/such/file_xyz_123", "a", "b"))
	if !strings.Contains(result, "读文件失败") {
		t.Fatalf("应提示读文件失败，实际：%s", result)
	}
}
