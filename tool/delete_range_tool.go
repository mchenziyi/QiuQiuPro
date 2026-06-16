package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// NewDeleteRangeTool 返回 delete_range 工具：删除文件中由起止锚点界定的连续行。
// 参照 Reasonix builtin/delete_range.go。
func NewDeleteRangeTool() Tool {
	return Tool{
		Name:        "delete_range",
		Description: "Delete a contiguous text range from a file using exact start/end text anchors. Each anchor must match exactly one line. Use for large deletions — smaller changes should use edit_file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":         map[string]any{"type": "string", "description": "File path"},
				"start_anchor": map[string]any{"type": "string", "description": "Exact text of the first line to delete (must be unique in the file)"},
				"end_anchor":   map[string]any{"type": "string", "description": "Exact text of the last line to delete (must be unique in the file)"},
				"inclusive":    map[string]any{"type": "boolean", "description": "Whether to include the anchor lines in the deletion (default true)"},
			},
			"required": []string{"path", "start_anchor", "end_anchor"},
		},
		Execute: func(args string) string {
			var p struct {
				Path        string `json:"path"`
				StartAnchor string `json:"start_anchor"`
				EndAnchor   string `json:"end_anchor"`
				Inclusive   *bool  `json:"inclusive"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				return "path 不能为空"
			}
			if p.StartAnchor == "" {
				return "start_anchor 不能为空"
			}
			if p.EndAnchor == "" {
				return "end_anchor 不能为空"
			}

			inclusive := true
			if p.Inclusive != nil {
				inclusive = *p.Inclusive
			}

			b, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取 %s 失败：%v", p.Path, err)
			}
			original := string(b)

			// 检测行尾风格
			lineSep := "\n"
			if strings.Contains(original, "\r\n") {
				lineSep = "\r\n"
			}

			lines := strings.Split(strings.ReplaceAll(original, "\r", ""), "\n")
			startLine := findUniqueLine(lines, p.StartAnchor)
			if startLine == -2 {
				return fmt.Sprintf("start_anchor 在 %s 中不唯一，请增加更多上下文", p.Path)
			}
			if startLine == -1 {
				return fmt.Sprintf("start_anchor 在 %s 中未找到", p.Path)
			}
			endLine := findUniqueLine(lines, p.EndAnchor)
			if endLine == -2 {
				return fmt.Sprintf("end_anchor 在 %s 中不唯一，请增加更多上下文", p.Path)
			}
			if endLine == -1 {
				return fmt.Sprintf("end_anchor 在 %s 中未找到", p.Path)
			}
			if startLine > endLine {
				return fmt.Sprintf("start_anchor（第 %d 行）出现在 end_anchor（第 %d 行）之后", startLine+1, endLine+1)
			}

			var keep []string
			if inclusive {
				if startLine == endLine && p.Inclusive == nil {
					// 单行删除：默认 inclusive=true，删除该行
				}
				keep = append(keep, lines[:startLine]...)
				keep = append(keep, lines[endLine+1:]...)
				if startLine == endLine && p.Inclusive != nil && !inclusive {
					return fmt.Sprintf("start_anchor 和 end_anchor 指向同一行，且 inclusive=false 时没有可删除的内容")
				}
			} else {
				if startLine == endLine {
					return fmt.Sprintf("start_anchor 和 end_anchor 指向同一行，且 inclusive=false 时没有可删除的内容")
				}
				keep = append(keep, lines[:startLine+1]...)
				keep = append(keep, lines[endLine:]...)
			}
			newContent := strings.Join(keep, lineSep)
			if newContent != "" && strings.HasSuffix(original, lineSep) && !strings.HasSuffix(newContent, lineSep) {
				newContent += lineSep
			}

			if err := os.WriteFile(p.Path, []byte(newContent), 0644); err != nil {
				return fmt.Sprintf("写入 %s 失败：%v", p.Path, err)
			}
			return fmt.Sprintf("已从 %s 中删除第 %d-%d 行", p.Path, startLine+1, endLine+1)
		},
	}
}

func findUniqueLine(lines []string, target string) int {
	idx := -1
	for i, l := range lines {
		if l == target {
			if idx >= 0 {
				return -2 // 不唯一
			}
			idx = i
		}
	}
	return idx
}
