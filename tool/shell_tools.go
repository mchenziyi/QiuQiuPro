package tool

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// NewRunShellTool 执行 Windows cmd 命令（不推荐，优先用 PowerShell）
func NewRunShellTool() Tool {
	return Tool{
		Name: "run_shell", Description: "【不推荐】执行一条 Windows cmd 命令。优先用 run_powershell，cmd 引号问题多",
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

// NewRunPowerShellTool 执行 PowerShell 命令（Windows 优先推荐）
func NewRunPowerShellTool() Tool {
	return Tool{
		Name: "run_powershell", Description: "执行一条 PowerShell 命令。当前系统是 Windows，优先用这个而不是 run_shell",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 PowerShell 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			var p struct{ Command string }
			json.Unmarshal([]byte(args), &p)
			out, err := exec.Command("powershell", "-NoProfile", "-Command", p.Command).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("输出：\n%s", string(out))
		},
	}
}
