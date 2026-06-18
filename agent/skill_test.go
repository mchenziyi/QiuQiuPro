package agent

import (
	"strings"
	"testing"

	"agentdemo/skill"
	"agentdemo/tool"
)

func TestClearSkillRestoresDefaultTools(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.RegisterTool(tool.Tool{Name: "read_file"})
	a.RegisterTool(tool.Tool{Name: "write_file"})
	a.RegisterTool(a.NewRememberRuleTool())

	a.ApplySkill(skill.Skill{
		Name:          "architect",
		Description:   "架构师模式",
		SystemPrompt:  "arch",
		ToolWhitelist: []string{"read_file"},
	})
	if got := a.CurrentSkillName(); got != "architect" {
		t.Fatalf("CurrentSkillName = %q, want architect", got)
	}

	a.ClearSkill()

	if got := a.CurrentSkillName(); got != "default" {
		t.Fatalf("CurrentSkillName = %q, want default", got)
	}
	names := map[string]bool{}
	for _, t := range a.availableTools() {
		names[t.Name] = true
	}
	for _, name := range []string{"read_file", "write_file", memoryToolName} {
		if !names[name] {
			t.Fatalf("default skill should expose %q, got %+v", name, names)
		}
	}
}

func TestApplySkillWrapsSystemPromptWithActiveSkillMarker(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.ApplySkill(skill.Skill{
		Name:         "hot_md_test",
		Description:  "Markdown 热安装测试模式",
		SystemPrompt: "你是 Markdown 热安装测试模式。回答必须以 HOT_MD_OK 开头。",
	})

	got := a.BuildSystemPrompt()
	for _, want := range []string{
		"当前激活的 Skill 是 [hot_md_test]",
		"忽略对话历史中任何声称当前仍处于 default",
		"HOT_MD_OK",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildSystemPrompt should contain %q, got:\n%s", want, got)
		}
	}
}
