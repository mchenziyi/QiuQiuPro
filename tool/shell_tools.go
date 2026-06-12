package tool

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

// NewRunShellTool 执行 Shell 命令（自动检测系统）
// Windows → cmd，macOS/Linux → sh
func NewRunShellTool() Tool {
	shell, arg := detectShell()
	return Tool{
		Name: "run_shell", Description: "执行一条 Shell 命令。自动适配当前系统，不用担心中间命令的差异",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 Shell 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			var p struct{ Command string }
			json.Unmarshal([]byte(args), &p)
			out, err := exec.Command(shell, arg, p.Command).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("输出：\n%s", string(out))
		},
	}
}

// NewRunPowerShellTool 执行 PowerShell 命令（仅 Windows 可用）
func NewRunPowerShellTool() Tool {
	return Tool{
		Name: "run_powershell", Description: "执行一条 PowerShell 命令（仅 Windows 可用）。macOS/Linux 用户请用 run_shell",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 PowerShell 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			if runtime.GOOS != "windows" {
				return fmt.Sprintf("PowerShell 仅支持 Windows，当前系统是 %s。请改用 run_shell", runtime.GOOS)
			}
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

// detectShell 根据当前系统返回 (shell, arg)
func detectShell() (string, string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", "/C"
	case "darwin":
		return "sh", "-c"
	default:
		// linux / freebsd 等
		return "sh", "-c"
	}
}
