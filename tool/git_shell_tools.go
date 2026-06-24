package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// --------------- Git ---------------

func NewGitCommitTool() Tool {
	return Tool{
		Name: "git_commit", Description: "提交文件变更", ReadOnly: false,
		Parameters: objParams(
			prop("message", "string", ""),
		).Required("message"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Message string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			cmd := exec.CommandContext(ctx, "git", "add", "-A")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Sprintf("git add failed: %s", out), err
			}
			cmd = exec.CommandContext(ctx, "git", "commit", "-m", p.Message)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("git commit failed: %s", out), err
			}
			return strings.TrimSpace(string(out)), nil
		},
	}
}

// --------------- Shell ---------------

func NewRunShellTool() Tool {
	return Tool{
		Name: "bash", Description: "执行 Shell 命令，返回 stdout+stderr。最大输出 32KB，超时 60s。", ReadOnly: false,
		Parameters: objParams(
			prop("command", "string", "要执行的命令"),
		).Required("command"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Command string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Command == "" {
				return "", fmt.Errorf("command required")
			}
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.CommandContext(ctx, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe", "-NoProfile", "-Command", p.Command)
			} else {
				cmd = exec.CommandContext(ctx, "/bin/sh", "-c", p.Command)
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				outStr := strings.TrimSpace(string(out))
				if outStr != "" {
					return outStr, err
				}
				return "", fmt.Errorf("command failed: %v", err)
			}
			output := string(out)
			if len(output) > 32000 {
				output = safeTruncate(output, 32000)
			}
			return strings.TrimSpace(output), nil
		},
	}
}
