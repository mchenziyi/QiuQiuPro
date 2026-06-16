package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func editArgs(t *testing.T, path, oldString, newString string) string {
	t.Helper()
	b, err := json.Marshal(map[string]string{
		"path":       path,
		"old_string": oldString,
		"new_string": newString,
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

	result := NewEditFileTool().Execute(editArgs(t, path, `println("hi")`, `println("hello")`))

	if !strings.Contains(result, "已编辑") {
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

	result := NewEditFileTool().Execute(editArgs(t, path, "NOPE", "x"))
	if !strings.Contains(result, "未找到") {
		t.Fatalf("应提示未找到，实际：%s", result)
	}
}

func TestEditFileBlock_Ambiguous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	os.WriteFile(path, []byte("x\nx\n"), 0644)

	result := NewEditFileTool().Execute(editArgs(t, path, "x", "y"))
	if !strings.Contains(result, "出现") {
		t.Fatalf("应提示出现多次，实际：%s", result)
	}
}

func TestEditFileBlock_FileMissing(t *testing.T) {
	result := NewEditFileTool().Execute(editArgs(t, "/no/such/file_xyz_123", "a", "b"))
	if !strings.Contains(result, "读取") {
		t.Fatalf("应提示读文件失败，实际：%s", result)
	}
}



