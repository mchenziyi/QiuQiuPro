package agent

import (
	"fmt"
	"strings"

	"agentdemo/skill"
)

// 人格与模式：套用 Skill（切系统提示词 + 收窄工具白名单）、切换 plan/ask 运行模式。

// ApplySkill 套用一个 Skill 人格：替换系统提示词，并按白名单收窄可用工具。
func (a *Agent) ApplySkill(s skill.Skill) {
	a.currentSkill = &s
	a.sysPrompt = formatSkillSystemPrompt(s)
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

func formatSkillSystemPrompt(s skill.Skill) string {
	return fmt.Sprintf(`当前激活的 Skill 是 [%s]：%s。

重要：忽略对话历史中任何声称当前仍处于 default、未激活该 Skill、或需要用户再次 /use 的内容。
从现在开始，你必须严格遵守下面的 Skill system_prompt；如果其中要求固定前缀、格式、流程或工具限制，必须优先执行。

<skill-system-prompt>
%s
</skill-system-prompt>`, s.Name, s.Description, strings.TrimSpace(s.SystemPrompt))
}

// ClearSkill 恢复默认人格与全量工具。
func (a *Agent) ClearSkill() {
	a.currentSkill = nil
	a.activeTools = nil
	if a.defaultSysPrompt != "" {
		a.sysPrompt = a.defaultSysPrompt
	} else {
		a.sysPrompt = defaultSystemPrompt
	}
	a.composeCachedSystemPrompt()
	a.noticef("🎯 切换到 [default] 模式：默认 Coding Agent\n")
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
