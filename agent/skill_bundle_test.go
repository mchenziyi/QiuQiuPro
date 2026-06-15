package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentdemo/skill"
	"agentdemo/tool"
)

// TestBundledSkillsValid 校验内置 Skill：能加载、必填字段齐全，且 tool_whitelist
// 只引用真实存在的工具——工具名写错会被 ApplySkill 静默过滤，靠这个测试兜底。
func TestBundledSkillsValid(t *testing.T) {
	const dir = "../prompt/skills" // 测试 cwd 为 agent/，故回到仓库根

	validTools := map[string]bool{}
	for _, tl := range tool.AllBuiltInTools() {
		validTools[tl.Name] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取 skills 目录失败：%v", err)
	}

	found := map[string]bool{}
	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		count++
		s, err := skill.LoadFromFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Errorf("%s 加载失败：%v", e.Name(), err)
			continue
		}
		if s.Name == "" || s.Description == "" || s.SystemPrompt == "" {
			t.Errorf("%s 缺少 name / description / system_prompt", e.Name())
		}
		for _, tn := range s.ToolWhitelist {
			if !validTools[tn] {
				t.Errorf("%s 的 tool_whitelist 含未知工具 %q（写错会被静默过滤）", e.Name(), tn)
			}
		}
		found[s.Name] = true
	}

	if count == 0 {
		t.Fatal("没有找到任何 skill JSON")
	}
	for _, name := range []string{"architect", "code_review", "frontend_design", "pm", "backend_dev", "tester", "devops"} {
		if !found[name] {
			t.Errorf("缺少预期 Skill：%s", name)
		}
	}
}
