// Package skill 定义 Agent 的专业能力包
// Skill = SystemPrompt + ToolWhitelist + Rules
// 不是插件（MCP 才是），是 Agent 的"行为模式配置"
package skill

// Rule 定义一条行为规则，约束 Agent 在特定 Skill 下的行为
type Rule struct {
	Name        string // 规则名，如"必须有 ADR"
	Description string // 规则说明，如"每次架构决策必须记录原因和备选方案"
}

// Skill 定义 Agent 的一种专业行为模式
// 切换 Skill 不重启进程，不改代码，只换一套 SystemPrompt 和工具白名单
type Skill struct {
	Name         string   // 技能名，用户通过 use <name> 切换
	Description  string   // 一句话说明，提示用户这个模式是做什么的
	SystemPrompt string   // 专业提示词，LLM 的行为核心（决定了 Agent 的"人格"）
	ToolWhitelist []string // 该 Skill 能用的工具名列表（空 = 全部可用）
	Rules        []Rule   // 行为规则列表
}

// ========== 内置 Skill ==========

// Architect 架构师模式：注重分析、设计文档、方案对比
// 适用场景：系统设计、技术选型、架构评审
func Architect() Skill {
	return Skill{
		Name:        "architect",
		Description: "资深架构师模式 — 分析系统、对比方案、输出架构决策",
		SystemPrompt: `你是一个资深软件架构师。
在写任何代码之前，你必须：
1. 分析现有系统结构和代码组织
2. 提出至少 2 种方案并比较优劣
3. 输出架构决策记录（ADR）
4. 确认方案后再开始实施`,
		ToolWhitelist: []string{"read_file", "list_directory", "count_file_chars"}, // 只读，不能写
		Rules: []Rule{
			{Name: "必须有 ADR", Description: "每次架构决策必须记录原因和备选方案"},
			{Name: "方案比较", Description: "至少提出 2 种方案并列出优劣"},
		},
	}
}

// CodeReview 代码审查模式：注重安全、性能、规范
// 适用场景：代码审查、安全审计、质量检查
func CodeReview() Skill {
	return Skill{
		Name:        "code_review",
		Description: "代码审查专家模式 — 检查安全、性能、代码规范",
		SystemPrompt: `你是一个代码审查专家。
审查代码时你必须：
1. 列出每个问题的严重级别（Critical / Major / Minor）
2. 对每个问题给出修改建议
3. 先分析影响范围，再给出修改方案`,
		ToolWhitelist: []string{"read_file", "list_directory", "run_shell", "edit_file_block"},
		Rules: []Rule{
			{Name: "严重级别", Description: "每个问题必须标记 Critical/Major/Minor"},
			{Name: "影响分析", Description: "改之前必须分析影响范围"},
		},
	}
}

// Frontend 前端设计模式：注重组件拆分、可访问性、响应式
// 适用场景：UI 设计、组件库开发、前端架构
func Frontend() Skill {
	return Skill{
		Name:        "frontend_design",
		Description: "前端架构师模式 — 组件设计、交互、可访问性",
		SystemPrompt: `你是一个前端架构师。
设计 UI 时你必须：
1. 考虑组件拆分和复用
2. 考虑可访问性（a11y）
3. 考虑响应式布局
4. 输出组件树和状态管理方案`,
		ToolWhitelist: []string{"read_file", "list_directory", "write_file"},
		Rules: []Rule{
			{Name: "可访问性", Description: "每个组件必须考虑 aria 标签和键盘导航"},
			{Name: "响应式", Description: "设计必须适配移动端和桌面端"},
		},
	}
}

// AllBuiltInSkills 返回所有内置 Skill
func AllBuiltInSkills() []Skill {
	return []Skill{
		Architect(),
		CodeReview(),
		Frontend(),
	}
}
