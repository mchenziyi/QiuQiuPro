package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// NewReadFileTool 读取文件内容
func NewReadFileTool() Tool {
	return Tool{
		Name: "read_file", Description: "读取指定文件的内容",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "文件路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s", p.Path, len(data), string(data))
		},
	}
}

// NewWriteFileTool 写入文件
func NewWriteFileTool() Tool {
	return Tool{
		Name: "write_file", Description: "将内容写入指定文件，会覆盖已存在的文件",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "文件路径"},
				"content": map[string]any{"type": "string", "description": "要写入的内容"},
			}, "required": []string{"path", "content"},
		},
		Execute: func(args string) string {
			var p struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			json.Unmarshal([]byte(args), &p)
			err := os.WriteFile(p.Path, []byte(p.Content), 0644)
			if err != nil {
				return fmt.Sprintf("写入失败：%v", err)
			}
			return fmt.Sprintf("文件 %s 已写入（%d 字节）", p.Path, len(p.Content))
		},
	}
}

// NewListDirectoryTool 列出目录内容
func NewListDirectoryTool() Tool {
	return Tool{
		Name: "ls", Description: "列出指定目录下的文件和子目录",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "目录路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			if p.Path == "" {
				p.Path = "."
			}
			entries, err := os.ReadDir(p.Path)
			if err != nil {
				return fmt.Sprintf("列目录失败：找不到 %s", p.Path)
			}
			var files, dirs []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, e.Name())
				} else {
					info, _ := e.Info()
					files = append(files, fmt.Sprintf("%s（%d 字节）", e.Name(), info.Size()))
				}
			}
			var b strings.Builder
			fmt.Fprintf(&b, "目录 %s：\n", p.Path)
			if len(dirs) > 0 {
				fmt.Fprintf(&b, "  子目录：%s\n", strings.Join(dirs, "、"))
			}
			if len(files) > 0 {
				fmt.Fprintf(&b, "  文件：\n    %s\n", strings.Join(files, "\n    "))
			}
			if len(entries) == 0 {
				fmt.Fprint(&b, "  （空目录）\n")
			}
			return b.String()
		},
	}
}

// NewCountFileCharsTool 统计文件字符数
func NewCountFileCharsTool() Tool {
	return Tool{
		Name: "count_file_chars", Description: "统计指定文件的字符数（按实际字符算）",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "文件路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string }
			json.Unmarshal([]byte(args), &p)
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取失败：找不到 %s", p.Path)
			}
			charCount := utf8.RuneCount(data)
			return fmt.Sprintf("文件 %s：字符数 %d，字节数 %d", p.Path, charCount, len(data))
		},
	}
}

