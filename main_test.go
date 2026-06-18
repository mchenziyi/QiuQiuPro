package main

import (
	"os"
	"path/filepath"
	"testing"

	"agentdemo/agent"
	"agentdemo/command"
	"agentdemo/skill"
)

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
