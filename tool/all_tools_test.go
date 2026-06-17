package tool

import (
	"context"
	"encoding/json"
	"testing"
)

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
