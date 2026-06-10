// Package tool 定义了 Agent 可调用的工具
package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"
)

// Tool 定义一个 Agent 可调用的工具
type Tool struct {
	Name        string      // 工具名，LLM 通过它调用
	Description string      // 描述，告诉 LLM 什么时候用
	Parameters  any         // 参数定义（JSON Schema）
	Execute     func(string) string // 执行函数，返回 LLM 友好的结果
}

// ========== 内置工具 ==========

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
		Name: "list_directory", Description: "列出指定目录下的文件和子目录",
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

// NewEditFileBlockTool 精确编辑文件
func NewEditFileBlockTool() Tool {
	return Tool{
		Name: "edit_file_block", Description: "精确修改文件：找到一段旧代码，替换成新代码",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path":      map[string]any{"type": "string", "description": "文件路径"},
				"old_block": map[string]any{"type": "string", "description": "要替换的旧代码"},
				"new_block": map[string]any{"type": "string", "description": "替换后的新代码"},
			}, "required": []string{"path", "old_block", "new_block"},
		},
		Execute: func(args string) string {
			var p struct{ Path, OldBlock, NewBlock string }
			json.Unmarshal([]byte(args), &p)
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			text := string(data)
			if !strings.Contains(text, p.OldBlock) {
				return fmt.Sprintf("修改失败：找不到指定的旧代码")
			}
			if strings.Count(text, p.OldBlock) > 1 {
				return fmt.Sprintf("修改失败：旧代码出现多次，请提供更多上下文")
			}
			text = strings.Replace(text, p.OldBlock, p.NewBlock, 1)
			os.WriteFile(p.Path, []byte(text), 0644)
			return fmt.Sprintf("已修改 %s", p.Path)
		},
	}
}

// NewGitCommitTool 提交 Git
func NewGitCommitTool() Tool {
	return Tool{
		Name: "git_commit", Description: "提交所有文件变更到 Git，需要提供提交信息",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "提交信息"},
			}, "required": []string{"message"},
		},
		Execute: func(args string) string {
			var p struct{ Message string }
			json.Unmarshal([]byte(args), &p)
			exec.Command("git", "add", ".").Run()
			_, err := exec.Command("git", "commit", "-m", p.Message).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("提交失败：%v", err)
			}
			return fmt.Sprintf("已提交：%s", p.Message)
		},
	}
}

// NewRunShellTool 执行 shell 命令（兜底工具）
func NewRunShellTool() Tool {
	return Tool{
		Name: "run_shell", Description: "执行一条 Windows cmd 命令。优先用其他专用工具",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 cmd 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			var p struct{ Command string }
			json.Unmarshal([]byte(args), &p)
			out, err := exec.Command("cmd", "/C", p.Command).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("输出：\n%s", string(out))
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

// AllBuiltInTools 返回所有内置工具列表
func AllBuiltInTools() []Tool {
	return []Tool{
		NewReadFileTool(),
		NewWriteFileTool(),
		NewListDirectoryTool(),
		NewEditFileBlockTool(),
		NewGitCommitTool(),
		NewCountFileCharsTool(),
		NewRunShellTool(),
	}
}
