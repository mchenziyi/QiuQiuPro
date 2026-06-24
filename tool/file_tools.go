package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// --------------- 文件读写 ---------------

func NewReadFileTool() Tool {
	return Tool{
		Name: "read_file", Description: "读取指定文件的内容", ReadOnly: true,
		Parameters: objParams(
			prop("path", "string", "文件路径"),
		).Required("path"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取 %s 失败", p.Path)
			}
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s", p.Path, len(data), string(data)), nil
		},
	}
}

func NewWriteFileTool() Tool {
	return Tool{
		Name: "write_file", Description: "创建或覆盖文件", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", "文件路径"),
			prop("content", "string", "内容"),
		).Required("path", "content"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path, Content string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf("已写入 %s", p.Path), nil
		},
	}
}

func NewListDirectoryTool() Tool {
	return Tool{
		Name: "ls", Description: "列出目录下的文件和子目录", ReadOnly: true,
		Parameters: objParams(
			prop("path", "string", "目录路径"),
		).Build(),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				p.Path = "."
			}
			entries, err := os.ReadDir(p.Path)
			if err != nil {
				return "", fmt.Errorf("读目录失败: %v", err)
			}
			var dirs, files []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, e.Name()+"/")
				} else {
					files = append(files, e.Name())
				}
			}
			var b strings.Builder
			if len(dirs) > 0 {
				b.WriteString("子目录：" + strings.Join(dirs, "、") + "\n")
			}
			if len(files) > 0 {
				b.WriteString("文件：\n" + strings.Join(files, "\n"))
			}
			if b.Len() == 0 {
				b.WriteString("（空目录）")
			}
			return fmt.Sprintf("目录 %s：\n%s", p.Path, strings.TrimSpace(b.String())), nil
		},
	}
}

// --------------- 编辑 ---------------

func NewEditFileTool() Tool {
	return Tool{
		Name: "edit_file", Description: "替换文件中的一段文本（可精确一次或全部替换）", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", ""),
			prop("old_string", "string", ""),
			prop("new_string", "string", ""),
			prop("replace_all", "boolean", "设为 true 则替换所有匹配项，缺省只替换第一处"),
		).Required("path", "old_string", "new_string"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path       string `json:"path"`
				OldString  string `json:"old_string"`
				NewString  string `json:"new_string"`
				ReplaceAll bool   `json:"replace_all"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			before := string(b)
			n := strings.Count(before, p.OldString)
			if n == 0 {
				return "", fmt.Errorf("未找到 old_string")
			}
			if !p.ReplaceAll && n > 1 {
				return "", fmt.Errorf("old_string 出现 %d 次，将 replace_all 设为 true 可全部替换", n)
			}
			count := 1
			if p.ReplaceAll {
				count = -1
			}
			after := strings.Replace(before, p.OldString, p.NewString, count)
			diff := ComputeLineDiff(before, after, p.Path, 3)
			diffJSON, _ := json.Marshal(diff)
			safePath := strings.ReplaceAll(p.Path, "\\", "\\\\")

			if err := os.WriteFile(p.Path, []byte(after), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			result := fmt.Sprintf(`{"text":"已编辑 %s（%d 处%s）","diff":%s}`,
				safePath, n, map[bool]string{true: "全部", false: ""}[p.ReplaceAll], string(diffJSON))
			return result, nil
		},
	}
}

func NewMultiEditTool() Tool {
	return Tool{
		Name: "multi_edit", Description: "批量编辑文件（一次性提交多条替换，原子提交）", ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
				"edits": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string":  map[string]any{"type": "string"},
							"new_string":  map[string]any{"type": "string"},
							"replace_all": map[string]any{"type": "boolean"},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
			},
			"required": []string{"path", "edits"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path  string `json:"path"`
				Edits []struct {
					OldString  string `json:"old_string"`
					NewString  string `json:"new_string"`
					ReplaceAll bool   `json:"replace_all"`
				} `json:"edits"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			before := string(b)
			content := before
			for i, step := range p.Edits {
				if step.ReplaceAll {
					content = strings.ReplaceAll(content, step.OldString, step.NewString)
					continue
				}
				n := strings.Count(content, step.OldString)
				if n == 0 {
					return "", fmt.Errorf("edit %d 未找到", i+1)
				}
				if n > 1 {
					return "", fmt.Errorf("edit %d 不唯一", i+1)
				}
				content = strings.Replace(content, step.OldString, step.NewString, 1)
			}
			if err := os.WriteFile(p.Path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			diff := ComputeLineDiff(before, content, p.Path, 3)
			diffJSON, _ := json.Marshal(diff)
			safePath := strings.ReplaceAll(p.Path, "\\", "\\\\")
			return fmt.Sprintf(`{"text":"已编辑 %s（%d 条）","diff":%s}`, safePath, len(p.Edits), string(diffJSON)), nil
		},
	}
}

func NewDeleteRangeTool() Tool {
	return Tool{
		Name: "delete_range", Description: "按行锚点删除连续区域", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", ""),
			prop("start_anchor", "string", ""),
			prop("end_anchor", "string", ""),
			prop("inclusive", "boolean", ""),
		).Required("path", "start_anchor", "end_anchor"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path        string `json:"path"`
				StartAnchor string `json:"start_anchor"`
				EndAnchor   string `json:"end_anchor"`
				Inclusive   *bool  `json:"inclusive"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			inc := true
			if p.Inclusive != nil {
				inc = *p.Inclusive
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			orig := string(b)
			lines := strings.Split(strings.ReplaceAll(orig, "\r", ""), "\n")
			s := findLine(lines, p.StartAnchor)
			if s < 0 {
				return "", fmt.Errorf("未找到 start_anchor")
			}
			e := findLine(lines, p.EndAnchor)
			if e < 0 {
				return "", fmt.Errorf("未找到 end_anchor")
			}
			if s > e {
				return "", fmt.Errorf("anchor 顺序颠倒")
			}
			var keep []string
			if inc {
				keep = append(keep, lines[:s]...)
				keep = append(keep, lines[e+1:]...)
			} else {
				keep = append(keep, lines[:s+1]...)
				keep = append(keep, lines[e:]...)
			}
			newContent := strings.Join(keep, "\n")
			if strings.HasSuffix(orig, "\n") && !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			if err := os.WriteFile(p.Path, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf("已删除第 %d-%d 行", s+1, e+1), nil
		},
	}
}

func findLine(lines []string, target string) int {
	for i, l := range lines {
		if l == target {
			return i
		}
	}
	return -1
}
