package skill

import (
	"os"
	"path/filepath"
	"testing"
)

const testSkillJSON = `{
	"name": "test_skill",
	"description": "测试 Skill",
	"system_prompt": "你是测试工程师",
	"tool_whitelist": ["read_file", "bash"],
	"rules": [
		{"name": "规则1", "description": "描述1"}
	]
}`

const testSkillNoName = `{
	"description": "缺少 name",
	"system_prompt": "测试"
}`

const testSkillInvalidJSON = `{not valid json`

func TestLoadFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, []byte(testSkillJSON), 0644)

	s, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile 应成功：%v", err)
	}
	if s.Name != "test_skill" {
		t.Fatalf("Name 应为 test_skill，实际 %q", s.Name)
	}
	if s.Description != "测试 Skill" {
		t.Fatalf("Description 应为 '测试 Skill'，实际 %q", s.Description)
	}
	if len(s.ToolWhitelist) != 2 {
		t.Fatalf("ToolWhitelist 应有 2 个，实际 %d", len(s.ToolWhitelist))
	}
	if len(s.Rules) != 1 {
		t.Fatalf("Rules 应有 1 条，实际 %d", len(s.Rules))
	}
	if s.Rules[0].Name != "规则1" {
		t.Fatalf("规则名应为 '规则1'，实际 %q", s.Rules[0].Name)
	}
}

func TestLoadFromFile_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noname.json")
	os.WriteFile(path, []byte(testSkillNoName), 0644)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("缺少 name 字段的 Skill 应报错")
	}
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(testSkillInvalidJSON), 0644)

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("无效 JSON 应报错")
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/skill.json")
	if err == nil {
		t.Fatal("文件不存在应报错")
	}
}

func TestLoadFromDir_LoadsAllJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(testSkillJSON), 0644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte(testSkillJSON), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a skill"), 0644)

	skills, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir 应成功：%v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("应加载 2 个 skill（跳过 .txt），实际 %d", len(skills))
	}
}

func TestLoadFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	skills, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("空目录应返回空 slice：%v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("空目录应返回 0 个 skill，实际 %d", len(skills))
	}
}

func TestLoadFromDir_NonExistentDir(t *testing.T) {
	skills, err := LoadFromDir("/nonexistent/skills")
	if err != nil {
		t.Fatalf("不存在的目录应返回空 slice：%v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("不存在的目录应返回 0，实际 %d", len(skills))
	}
}

func TestLoadFromFile_SetsFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.json")
	json := `{
		"name": "full_test",
		"description": "完整测试",
		"system_prompt": "你是一个测试 Agent",
		"tool_whitelist": ["read_file", "write_file", "bash"],
		"rules": [
			{"name": "规则A", "description": "A的描述"},
			{"name": "规则B", "description": "B的描述"}
		]
	}`
	os.WriteFile(path, []byte(json), 0644)

	s, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile 应成功：%v", err)
	}
	if s.SystemPrompt != "你是一个测试 Agent" {
		t.Fatalf("SystemPrompt 错误：%q", s.SystemPrompt)
	}
	if len(s.ToolWhitelist) != 3 {
		t.Fatalf("ToolWhitelist 应为 3，实际 %d", len(s.ToolWhitelist))
	}
	if len(s.Rules) != 2 {
		t.Fatalf("Rules 应为 2，实际 %d", len(s.Rules))
	}
}

func TestLoadFromURL_NotImplemented(t *testing.T) {
	_, err := LoadFromURL("http://example.com/skill.json")
	if err == nil {
		t.Fatal("LoadFromURL 应返回错误（未实现）")
	}
}
