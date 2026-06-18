package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentdemo/agent"
	"agentdemo/command"
	"agentdemo/skill"
)

func TestDefaultSystemPromptMentionsQiuqiuRuleFiles(t *testing.T) {
	prompt, err := agent.LoadRawPrompt("prompt/default/system.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"~/.qiuqiu/QIUQIU.md", "当前项目根目录的 QIUQIU.md", "remember_rule"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("默认 system prompt 应说明 %q：\n%s", want, prompt)
		}
	}
}

func TestUseDefaultRestoresDefaultSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a skill in the install dir so Manager picks it up
	installDir := filepath.Join(home, ".qiuqiu", "skills")
	os.MkdirAll(installDir, 0755)
	os.WriteFile(filepath.Join(installDir, "architect.json"), []byte(`{
		"name": "architect",
		"description": "架构师模式",
		"system_prompt": "arch"
	}`), 0644)

	mgr := skill.NewManager("prompt/skills", installDir)

	a, err := agent.New("test-key", "test-model", false)
	if err != nil {
		t.Fatalf("agent.New failed: %v", err)
	}
	registry := command.NewRegistry()
	registerUseCommand(registry, a, mgr)

	if !registry.Handle("/use architect") {
		t.Fatal("/use architect should be handled")
	}
	if got := a.CurrentSkillName(); got != "architect" {
		t.Fatalf("CurrentSkillName = %q, want architect", got)
	}

	if !registry.Handle("/use default") {
		t.Fatal("/use default should be handled")
	}
	if got := a.CurrentSkillName(); got != "default" {
		t.Fatalf("CurrentSkillName = %q, want default", got)
	}
}
