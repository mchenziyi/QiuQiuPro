// Package skill 定义 Agent 的专业能力包
package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill 定义 Agent 的一种专业行为模式
type Skill struct {
	Name         string   `json:"name"`          // 技能名
	Description  string   `json:"description"`    // 一句话说明
	SystemPrompt string   `json:"system_prompt"`  // 专业提示词
	ToolWhitelist []string `json:"tool_whitelist"` // 可用工具名列表（空=全部）
	Rules        []Rule   `json:"rules"`          // 行为规则
}

// Rule 定义一条行为规则
type Rule struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ========== 内置 Skill ==========

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
		ToolWhitelist: []string{"read_file", "list_directory", "count_file_chars"},
		Rules: []Rule{
			{Name: "必须有 ADR", Description: "每次架构决策必须记录原因和备选方案"},
			{Name: "方案比较", Description: "至少提出 2 种方案并列出优劣"},
		},
	}
}

func CodeReview() Skill {
	return Skill{
		Name:        "code_review",
		Description: "代码审查专家模式 — 检查安全、性能、代码规范",
		SystemPrompt: `你是一个代码审查专家。
审查代码时你必须：
1. 列出每个问题的严重级别（Critical / Major / Minor）
2. 对每个问题给出修改建议
3. 先分析影响范围，再给出修改方案`,
		ToolWhitelist: []string{"read_file", "list_directory", "run_shell", "run_powershell", "edit_file_block"},
		Rules: []Rule{
			{Name: "严重级别", Description: "每个问题必须标记 Critical/Major/Minor"},
			{Name: "影响分析", Description: "改之前必须分析影响范围"},
		},
	}
}

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

// ========== 外部加载 ==========

// LoadFromFile 从 JSON 文件加载一个 Skill
func LoadFromFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Skill 文件失败：%w", err)
	}
	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析 Skill 文件失败：%w\n路径：%s", err, path)
	}
	if s.Name == "" {
		return nil, fmt.Errorf("Skill 文件缺少 name 字段：%s", path)
	}
	return &s, nil
}

// LoadFromDir 从目录批量加载 Skill（扫描所有 .json 文件）
func LoadFromDir(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Skill{}, nil
		}
		return nil, err
	}

	var skills []Skill
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := LoadFromFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Printf("  ⚠️  加载 Skill 失败 %s：%v\n", e.Name(), err)
			continue
		}
		skills = append(skills, *s)
	}
	return skills, nil
}

// LoadFromURL 从 URL 加载 Skill（预留，暂未实现）
func LoadFromURL(url string) (*Skill, error) {
	return nil, fmt.Errorf("从 URL 加载 Skill 暂未实现，请先下载到 %s", url)
}
