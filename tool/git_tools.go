package tool

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// NewGitCommitTool 提交所有文件变更到 Git
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
