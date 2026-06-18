package tool

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// toSlash 将路径中的反斜杠转为正斜杠，使路径可安全嵌入 JSON 字符串。
// Go 的文件 API 在 Windows 上也接受正斜杠，因此这是安全的。
func toSlash(p string) string { return filepath.ToSlash(p) }

func TestEditFileTool_UniqueReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	_, err := NewEditFileTool().Execute(context.Background(), json.RawMessage(
		`{"path":"`+toSlash(path)+`","old_string":"world","new_string":"qiuqiu"}`,
	))
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.TrimSpace(string(data)); got != "hello qiuqiu" {
		t.Fatalf("content = %q, want hello qiuqiu", got)
	}
}

func TestMultiEditTool_BatchReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	os.WriteFile(path, []byte("one two three"), 0644)

	_, err := NewMultiEditTool().Execute(context.Background(), json.RawMessage(
		`{"path":"`+toSlash(path)+`","edits":[{"old_string":"one","new_string":"1"},{"old_string":"two","new_string":"2"},{"old_string":"three","new_string":"3"}]}`,
	))
	if err != nil {
		t.Fatalf("multi_edit: %v", err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.TrimSpace(string(data)); got != "1 2 3" {
		t.Fatalf("content = %q, want 1 2 3", got)
	}
}

func TestMultiEditTool_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi.txt")
	os.WriteFile(path, []byte("aa bb aa"), 0644)

	_, err := NewMultiEditTool().Execute(context.Background(), json.RawMessage(
		`{"path":"`+toSlash(path)+`","edits":[{"old_string":"aa","new_string":"x","replace_all":true}]}`,
	))
	if err != nil {
		t.Fatalf("multi_edit replace_all: %v", err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.TrimSpace(string(data)); got != "x bb x" {
		t.Fatalf("content = %q, want x bb x", got)
	}
}

func TestDeleteRangeTool_Inclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "del.txt")
	os.WriteFile(path, []byte("line1\nstart\nmid\nend\nline5"), 0644)

	_, err := NewDeleteRangeTool().Execute(context.Background(), json.RawMessage(
		`{"path":"`+toSlash(path)+`","start_anchor":"start","end_anchor":"end","inclusive":true}`,
	))
	if err != nil {
		t.Fatalf("delete_range inclusive: %v", err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.TrimSpace(string(data)); got != "line1\nline5" {
		t.Fatalf("content = %q, want line1\\nline5", strings.ReplaceAll(string(data), "\n", `\n`))
	}
}

func TestDeleteRangeTool_Exclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "del.txt")
	os.WriteFile(path, []byte("line1\nstart\nmid\nend\nline5"), 0644)

	_, err := NewDeleteRangeTool().Execute(context.Background(), json.RawMessage(
		`{"path":"`+toSlash(path)+`","start_anchor":"start","end_anchor":"end","inclusive":false}`,
	))
	if err != nil {
		t.Fatalf("delete_range exclusive: %v", err)
	}
	data, _ := os.ReadFile(path)
	if got := strings.TrimSpace(string(data)); got != "line1\nstart\nend\nline5" {
		t.Fatalf("content = %q, want line1\\nstart\\nend\\nline5", strings.ReplaceAll(string(data), "\n", `\n`))
	}
}

func TestGrepTool_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "visible.go"), []byte("func SetPlanMode() {}"), 0644)
	hiddenDir := filepath.Join(dir, ".reasonix", "sessions")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, "hidden.txt"), []byte("func SetPlanMode() {}"), 0644)

	out, err := NewGrepTool().Execute(context.Background(), json.RawMessage(
		`{"pattern":"SetPlanMode","path":"`+toSlash(dir)+`"}`,
	))
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if strings.Contains(out, ".reasonix") {
		t.Fatalf("grep scanned hidden dir: %s", out)
	}
	if !strings.Contains(out, "visible.go") {
		t.Fatalf("grep missed visible file: %s", out)
	}
}

func TestGitCommitTool_NoChanges(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	out, err := NewGitCommitTool().Execute(context.Background(), json.RawMessage(`{"message":"noop"}`))
	if err == nil {
		t.Fatal("无变更提交应失败")
	}
	if !strings.Contains(out, "nothing to commit") && !strings.Contains(out, "working tree clean") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("f.txt", "x")
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	run("git", "init")
	run("git", "add", "f.txt")
	run("git", "commit", "-m", "init")
	// git_commit runs in process cwd; test runs from package dir — use chdir only for this test.
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

func TestAllBuiltInTools_HaveNonEmptyName(t *testing.T) {
	tools := AllBuiltInTools()
	for _, tl := range tools {
		if tl.Name == "" {
			t.Errorf("工具名不能为空：%+v", tl)
		}
	}
}

func TestAllBuiltInTools_HaveNonEmptyDescription(t *testing.T) {
	tools := AllBuiltInTools()
	for _, tl := range tools {
		if tl.Description == "" {
			t.Errorf("%s 的描述不能为空", tl.Name)
		}
	}
}

func TestAllBuiltInTools_HaveExecute(t *testing.T) {
	tools := AllBuiltInTools()
	for _, tl := range tools {
		if tl.Execute == nil {
			t.Errorf("%s 的 Execute 不能为 nil", tl.Name)
		}
	}
}

func TestAllBuiltInTools_ReadOnlyFlags(t *testing.T) {
	readOnlyTools := map[string]bool{
		"read_file": true, "ls": true, "glob": true, "grep": true,
		"search_files": true, "code_search": true, "web_fetch": true,
	}
	tools := AllBuiltInTools()
	for _, tl := range tools {
		expectReadOnly, known := readOnlyTools[tl.Name]
		if known && tl.ReadOnly != expectReadOnly {
			t.Errorf("%s ReadOnly=%v，期望 %v", tl.Name, tl.ReadOnly, expectReadOnly)
		}
	}
}

func TestReadFileTool_ValidatesPath(t *testing.T) {
	tl := NewReadFileTool()
	_, err := tl.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("空参数应报错")
	}
}

func TestWriteFileTool_ValidatesArgs(t *testing.T) {
	tl := NewWriteFileTool()
	_, err := tl.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("空参数应报错")
	}
}

func TestWriteFileTool_ReadOnly(t *testing.T) {
	tl := NewWriteFileTool()
	if tl.ReadOnly {
		t.Fatal("write_file.ReadOnly 应为 false")
	}
}

func TestReadFileTool_ReadOnly(t *testing.T) {
	tl := NewReadFileTool()
	if !tl.ReadOnly {
		t.Fatal("read_file.ReadOnly 应为 true")
	}
}

func TestWebFetchTool_ReadOnly(t *testing.T) {
	tl := NewWebFetchTool()
	if !tl.ReadOnly {
		t.Fatal("web_fetch.ReadOnly 应为 true")
	}
}

func TestRunShellTool_NotReadOnly(t *testing.T) {
	tl := NewRunShellTool()
	if tl.ReadOnly {
		t.Fatal("bash.ReadOnly 应为 false")
	}
}

func TestEditFileTool_HasParameters(t *testing.T) {
	tl := NewEditFileTool()
	if tl.Parameters == nil {
		t.Fatal("edit_file 应有 parameters schema")
	}
}

func TestGlobTool_ValidatesPattern(t *testing.T) {
	tl := NewGlobTool()
	_, err := tl.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("glob 空参数应报错")
	}
}

func TestGrepTool_ValidatesPattern(t *testing.T) {
	tl := NewGrepTool()
	_, err := tl.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("grep 空参数应报错")
	}
}

func TestToolCount_MatchesAllBuiltIn(t *testing.T) {
	tools := AllBuiltInTools()
	if len(tools) != 14 {
		t.Fatalf("内置工具数量应为 14，当前 %d：%v", len(tools), toolNames(tools))
	}
}

func TestToolNames_Unique(t *testing.T) {
	tools := AllBuiltInTools()
	seen := make(map[string]bool)
	for _, tl := range tools {
		if seen[tl.Name] {
			t.Errorf("重复的工具名：%s", tl.Name)
		}
		seen[tl.Name] = true
	}
}

func toolNames(tools []Tool) []string {
	names := make([]string, len(tools))
	for i, tl := range tools {
		names[i] = tl.Name
	}
	return names
}
