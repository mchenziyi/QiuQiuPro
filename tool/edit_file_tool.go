package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NewEditFileTool 返回 edit_file 工具：精确替换文件中的一段文本。
// 参照 Reasonix builtin/editfile.go。old_string 必须在文件中恰好出现一次。
func NewEditFileTool() Tool {
	return Tool{
		Name:        "edit_file",
		Description: "Replace an exact string in a file with another. old_string must occur exactly once; add surrounding context to disambiguate. Use for targeted edits instead of rewriting the whole file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "File path"},
				"old_string": map[string]any{"type": "string", "description": "Exact text to replace (must be unique in the file)"},
				"new_string": map[string]any{"type": "string", "description": "Replacement text (may be empty to delete)"},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Execute: func(args string) string {
			var p struct {
				Path      string `json:"path"`
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				return "path 不能为空"
			}
			if p.OldString == "" {
				return "old_string 不能为空"
			}

			b, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取 %s 失败：%v", p.Path, err)
			}
			content := string(b)

			count := strings.Count(content, p.OldString)
			switch count {
			case 0:
				return fmt.Sprintf("在 %s 中未找到 old_string", p.Path)
			case 1:
				// ok
			default:
				return fmt.Sprintf("old_string 在 %s 中出现 %d 次（需要恰好一次），请增加更多上下文使其唯一", p.Path, count)
			}

			updated := strings.Replace(content, p.OldString, p.NewString, 1)
			if err := os.WriteFile(p.Path, []byte(updated), 0644); err != nil {
				return fmt.Sprintf("写入 %s 失败：%v", p.Path, err)
			}
			return fmt.Sprintf("已编辑 %s", p.Path)
		},
	}
}
