package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewGlobTool 按文件名模式匹配文件（支持 glob 通配符）
// LLM 使用场景："找所有 .go 文件"、"找 src/ 下的配置文件"
func NewGlobTool() Tool {
	return Tool{
		Name:        "glob",
		Description: "按文件名模式搜索文件，支持 glob 通配符（如 *.go、**/*.md、src/**/*.txt）。不搜索文件内容",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "文件名模式，如 *.go、**/*.md、src/**/*.txt",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "搜索的根目录，默认为当前目录",
				},
			},
			"required": []string{"pattern"},
		},
		Execute: func(args string) string {
			var p struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
			}
			json.Unmarshal([]byte(args), &p)
			if p.Path == "" {
				p.Path = "."
			}

			// 在当前目录匹配
			matches, err := filepath.Glob(filepath.Join(p.Path, p.Pattern))
			if err != nil {
				return fmt.Sprintf("搜索失败：%v", err)
			}

			// 在子目录递归匹配（** 模式）
			subMatches, _ := filepath.Glob(filepath.Join(p.Path, "**", p.Pattern))
			seen := map[string]bool{}
			for _, m := range matches {
				seen[m] = true
			}
			for _, m := range subMatches {
				if !seen[m] {
					matches = append(matches, m)
				}
			}

			if len(matches) == 0 {
				return fmt.Sprintf("在 %s 中没有匹配「%s」的文件", p.Path, p.Pattern)
			}

			var b strings.Builder
			fmt.Fprintf(&b, "在 %s 中找到 %d 个匹配的文件：\n", p.Path, len(matches))
			for _, m := range matches {
				info, err := os.Stat(m)
				if err == nil {
					fmt.Fprintf(&b, "  %s（%d 字节）\n", m, info.Size())
				} else {
					fmt.Fprintf(&b, "  %s\n", m)
				}
			}
			return b.String()
		},
	}
}
