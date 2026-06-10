// Package tool 定义了 Agent 可调用的工具
// 每个工具包含：名称、描述、参数定义（JSON Schema）、执行函数
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
	Name        string      // 工具名，LLM 通过这个名字调用
	Description string      // 工具描述，告诉 LLM 什么时候用这个工具
	Parameters  any         // 参数定义（JSON Schema 格式）
	Execute     func(string) string // 执行函数，接收 JSON 参数，返回 LLM 友好的结果
}

// ========== 内置工具 ==========

// NewReadFileTool 创建"读取文件"工具
func NewReadFileTool() Tool {
	return Tool{
		Name: "read_file", Description: "读取指定文件的内容",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "文件路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			// 解析参数：提取 path 字段
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			// 读文件
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			// 返回文件内容，附带路径和字节数（对 LLM 友好）
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s", p.Path, len(data), string(data))
		},
	}
}

// NewWriteFileTool 创建"写入文件"工具
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
			// 解析参数：提取 path 和 content
			var p struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			json.Unmarshal([]byte(args), &p)
			// 写文件（0644 权限）
			err := os.WriteFile(p.Path, []byte(p.Content), 0644)
			if err != nil {
				return fmt.Sprintf("写入失败：%v", err)
			}
			return fmt.Sprintf("文件 %s 已写入（%d 字节）", p.Path, len(p.Content))
		},
	}
}

// NewListDirectoryTool 创建"列出目录"工具
func NewListDirectoryTool() Tool {
	return Tool{
		Name: "list_directory", Description: "列出指定目录下的文件和子目录",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "目录路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			// 解析参数：提取 path
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			// 默认当前目录
			if p.Path == "" {
				p.Path = "."
			}
			// 读取目录下的所有条目
			entries, err := os.ReadDir(p.Path)
			if err != nil {
				return fmt.Sprintf("列目录失败：找不到 %s", p.Path)
			}
			// 分别收集子目录和文件
			var files, dirs []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, e.Name())
				} else {
					info, _ := e.Info()
					files = append(files, fmt.Sprintf("%s（%d 字节）", e.Name(), info.Size()))
				}
			}
			// 格式化输出（LLM 友好的文本）
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

// NewEditFileBlockTool 创建"精确编辑文件"工具
// 核心设计：先找旧代码，找不到或找到多处就拒绝修改——防止改错位置
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
			// 解析参数
			var p struct{ Path, OldBlock, NewBlock string }
			json.Unmarshal([]byte(args), &p)
			// 读文件
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			text := string(data)
			// 安全检查：旧代码必须存在
			if !strings.Contains(text, p.OldBlock) {
				return fmt.Sprintf("修改失败：找不到指定的旧代码")
			}
			// 安全检查：旧代码必须唯一（防止改错位置）
			if strings.Count(text, p.OldBlock) > 1 {
				return fmt.Sprintf("修改失败：旧代码出现多次，请提供更多上下文")
			}
			// 执行替换（只替换第一次出现）
			text = strings.Replace(text, p.OldBlock, p.NewBlock, 1)
			// 写回文件
			os.WriteFile(p.Path, []byte(text), 0644)
			return fmt.Sprintf("已修改 %s", p.Path)
		},
	}
}

// NewGitCommitTool 创建"提交 Git"工具
func NewGitCommitTool() Tool {
	return Tool{
		Name: "git_commit", Description: "提交所有文件变更到 Git，需要提供提交信息",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "提交信息"},
			}, "required": []string{"message"},
		},
		Execute: func(args string) string {
			// 解析参数：提取提交信息
			var p struct{ Message string }
			json.Unmarshal([]byte(args), &p)
			// 先 git add . 暂存所有变更，再提交
			exec.Command("git", "add", ".").Run()
			_, err := exec.Command("git", "commit", "-m", p.Message).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("提交失败：%v", err)
			}
			return fmt.Sprintf("已提交：%s", p.Message)
		},
	}
}

// NewRunShellTool 创建"执行 shell 命令"工具（兜底工具）
// 描述里注明"优先用其他专用工具"，引导 LLM 选更精确的工具
func NewRunShellTool() Tool {
	return Tool{
		Name: "run_shell", Description: "执行一条 Windows cmd 命令。优先用其他专用工具",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 cmd 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			// 解析参数：提取命令
			var p struct{ Command string }
			json.Unmarshal([]byte(args), &p)
			// 执行命令（Windows 用 cmd /C）
			out, err := exec.Command("cmd", "/C", p.Command).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("输出：\n%s", string(out))
		},
	}
}

// NewCountFileCharsTool 创建"统计文件字符数"工具
// 用 utf8.RuneCount 按实际字符算，不是按字节数
func NewCountFileCharsTool() Tool {
	return Tool{
		Name: "count_file_chars", Description: "统计指定文件的字符数（按实际字符算）",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "文件路径"},
			}, "required": []string{"path"},
		},
		Execute: func(args string) string {
			// 解析参数
			var p struct{ Path string }
			json.Unmarshal([]byte(args), &p)
			// 读文件
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取失败：找不到 %s", p.Path)
			}
			// 统计字符数（按实际字符，不是字节）
			charCount := utf8.RuneCount(data)
			return fmt.Sprintf("文件 %s：字符数 %d，字节数 %d", p.Path, charCount, len(data))
		},
	}
}

// AllBuiltInTools 返回所有内置工具列表
// 在 main.go 中调用此函数一次性注册所有工具
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
