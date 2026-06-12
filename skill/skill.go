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


