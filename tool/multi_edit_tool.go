package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NewMultiEditTool 返回 multi_edit 工具：在单个文件上原子性地应用多条编辑。
// 参照 Reasonix builtin/multiedit.go。
func NewMultiEditTool() Tool {
	return Tool{
		Name:        "multi_edit",
		Description: "Apply a list of edits to a single file atomically: each edit runs against the result of the previous one, all in memory; the file is rewritten only if every edit succeeds. Cheaper and safer than chaining edit_file calls — a failure in step 3 leaves the file untouched instead of half-edited.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "File path"},
				"edits": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "Ordered edits. Each step sees the file as left by the previous step.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string":  map[string]any{"type": "string", "description": "Exact text to find. Without replace_all, must match exactly once."},
							"new_string":  map[string]any{"type": "string", "description": "Replacement text (empty deletes)."},
							"replace_all": map[string]any{"type": "boolean", "description": "Replace every occurrence instead of requiring uniqueness."},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
			},
			"required": []string{"path", "edits"},
		},
		Execute: func(args string) string {
			var p struct {
				Path  string `json:"path"`
				Edits []struct {
					OldString  string `json:"old_string"`
					NewString  string `json:"new_string"`
					ReplaceAll bool   `json:"replace_all,omitempty"`
				} `json:"edits"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				return "path 不能为空"
			}
			if len(p.Edits) == 0 {
				return "edits 不能为空"
			}

			b, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取 %s 失败：%v", p.Path, err)
			}
			content := string(b)
			applied := 0

			for i, step := range p.Edits {
				if step.OldString == "" {
					return fmt.Sprintf("edit %d: old_string 不能为空", i+1)
				}
				if step.ReplaceAll {
					count := strings.Count(content, step.OldString)
					if count == 0 {
						return fmt.Sprintf("edit %d: old_string 未找到", i+1)
					}
					content = strings.ReplaceAll(content, step.OldString, step.NewString)
					applied += count
					continue
				}
				count := strings.Count(content, step.OldString)
				switch count {
				case 0:
					return fmt.Sprintf("edit %d: old_string 未找到", i+1)
				case 1:
					content = strings.Replace(content, step.OldString, step.NewString, 1)
					applied++
				default:
					return fmt.Sprintf("edit %d: old_string 不唯一（出现 %d 次），请加更多上下文或设置 replace_all", i+1, count)
				}
			}

			if err := os.WriteFile(p.Path, []byte(content), 0644); err != nil {
				return fmt.Sprintf("写入 %s 失败：%v", p.Path, err)
			}
			return fmt.Sprintf("multi_edit %s: %d 条编辑全部应用（%d 处替换）", p.Path, len(p.Edits), applied)
		},
	}
}
