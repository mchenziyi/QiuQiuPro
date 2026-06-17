package agent

import "agentdemo/skill"

// 人格与模式：套用 Skill（切系统提示词 + 收窄工具白名单）、切换 plan/ask 运行模式。

// ApplySkill 套用一个 Skill 人格：替换系统提示词，并按白名单收窄可用工具。
func (a *Agent) ApplySkill(s skill.Skill) {
	a.currentSkill = &s
	a.sysPrompt = s.SystemPrompt
	a.composeCachedSystemPrompt()
	if len(s.ToolWhitelist) > 0 {
		a.activeTools = make([]string, 0)
		for _, name := range s.ToolWhitelist {
			if _, ok := a.allTools[name]; ok {
				a.activeTools = append(a.activeTools, name)
			}
		}
	} else {
		a.activeTools = nil
	}
	a.noticef("🎯 切换到 [%s] 模式：%s\n", s.Name, s.Description)
}

func (a *Agent) CurrentSkillName() string {
	if a.currentSkill != nil {
		return a.currentSkill.Name
	}
	return "default"
}

// SetMode 切换 Agent 运行模式：plan（规划执行）| ask（直接问答）
func (a *Agent) SetMode(mode string) {
	if mode != "ask" && mode != "plan" {
		a.noticef("  ⚠️  未知模式：%s，可选：plan / ask\n", mode)
		return
	}
	a.Mode = mode
	if mode == "plan" {
		a.SetPlanMode(true)
	} else {
		a.SetPlanMode(false)
	}
	a.noticef("  🔄 切换到 [%s] 模式\n", mode)
}

func (a *Agent) CurrentMode() string { return a.Mode }
