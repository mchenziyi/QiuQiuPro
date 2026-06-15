package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	runShellTimeout    = 5 * time.Minute // 单条命令最长执行时间，防止卡死整个 Agent
	runShellCaptureMax = 1 << 20         // 最多捕获 1MB 输出（内存上限；控制台仍全量流式）
	runShellMaxOutput  = 16000           // 回灌给 LLM 的字符上限，超出截断
)

// NewRunShellTool 执行 Shell 命令（自动检测系统）
// Windows → cmd，macOS/Linux → sh。实时流式输出，并按退出码判定成败
func NewRunShellTool() Tool {
	shell, arg := detectShell()
	return Tool{
		Name: "run_shell", Description: "执行一条 Shell 命令。自动适配当前系统，实时显示输出，并按退出码判断是否成功",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 Shell 命令"},
			}, "required": []string{"command"},
		},
		Execute: func(args string) string {
			var p struct {
				Command string `json:"command"`
			}
			json.Unmarshal([]byte(args), &p)
			if strings.TrimSpace(p.Command) == "" {
				return "命令为空"
			}
			return runCommandStreaming(runShellTimeout, shell, arg, p.Command)
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
			var p struct {
				Command string `json:"command"`
			}
			json.Unmarshal([]byte(args), &p)
			if strings.TrimSpace(p.Command) == "" {
				return "命令为空"
			}
			return runCommandStreaming(runShellTimeout, "powershell", "-NoProfile", "-Command", p.Command)
		},
	}
}

// runCommandStreaming 执行命令并整理结果：
//   - 输出实时流式打到控制台（耗时命令也能边跑边看），同时捕获（有上限）回灌给 LLM；
//   - 带超时保护，超时强制终止；
//   - 把退出码整理成「成功 / 失败 / 超时」结论，便于模型直接判断成败。
func runCommandStreaming(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	// Stdout 与 Stderr 指向同一个 writer：exec 会把两路合并到一条管道、单 goroutine 写入，
	// 天然串行化、无竞态。MultiWriter 一路实时输出到控制台，一路写进带上限的缓冲。
	w := io.MultiWriter(os.Stdout, &cappedBuffer{buf: &buf, max: runShellCaptureMax})
	cmd.Stdout = w
	cmd.Stderr = w

	err := cmd.Run()
	output := formatShellOutput(buf.String())

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return fmt.Sprintf("❌ 命令超时（超过 %s 被强制终止）\n输出：\n%s", timeout, output)
	case err == nil:
		return fmt.Sprintf("✅ 命令成功（退出码 0）\n输出：\n%s", output)
	default:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return fmt.Sprintf("❌ 命令失败（退出码 %d）\n输出：\n%s", ee.ExitCode(), output)
		}
		// 没能启动（命令不存在、权限不足等）
		return fmt.Sprintf("❌ 命令无法执行：%v\n输出：\n%s", err, output)
	}
}

// formatShellOutput 处理捕获的输出：空则提示，超长按 rune 截断（避免污染上下文）。
func formatShellOutput(s string) string {
	if s == "" {
		return "（无输出）"
	}
	if r := []rune(s); len(r) > runShellMaxOutput {
		return string(r[:runShellMaxOutput]) +
			fmt.Sprintf("\n…（输出过长已截断，仅显示前 %d 个字符）", runShellMaxOutput)
	}
	return s
}

// cappedBuffer 是带容量上限的写入器：超过 max 后丢弃多余内容，
// 但始终声明「全量写入」，以满足 io.MultiWriter 的约定（否则会被判 short write 报错）。
type cappedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if remain := c.max - c.buf.Len(); remain > 0 {
		if len(p) <= remain {
			c.buf.Write(p)
		} else {
			c.buf.Write(p[:remain])
		}
	}
	return len(p), nil
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
