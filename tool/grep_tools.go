package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewGrepTool 在文件内容中搜索关键词
// LLM 使用场景："找哪个文件里用了 parseConfig"、"搜包含 TODO 的代码"
func NewGrepTool() Tool {
	return Tool{
		Name:        "grep",
		Description: "在文件内容中搜索关键词或正则表达式。不搜索文件名（用 glob），只搜内容",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "要搜索的关键词或正则表达式",
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

			var results []string
			filepath.Walk(p.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					// 跳过隐藏目录
					if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
						return filepath.SkipDir
					}
					return nil
				}
				// 跳过隐藏文件
				if strings.HasPrefix(info.Name(), ".") {
					return nil
				}

				// 只搜常见文本文件
				ext := filepath.Ext(path)
				textExts := map[string]bool{
					".go": true, ".md": true, ".txt": true, ".json": true,
					".yaml": true, ".yml": true, ".toml": true, ".xml": true,
					".html": true, ".css": true, ".js": true, ".ts": true,
					".py": true, ".rs": true, ".java": true, ".c": true, ".h": true,
					".mod": true, ".sum": true,
				}
				if !textExts[ext] {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for i, line := range lines {
					if strings.Contains(line, p.Pattern) {
						results = append(results, fmt.Sprintf("  %s:%d  %s", path, i+1, strings.TrimSpace(line)))
						if len(results) >= 30 {
							return filepath.SkipDir
						}
					}
				}
				return nil
			})

			if len(results) == 0 {
				return fmt.Sprintf("在 %s 中未找到包含「%s」的内容", p.Path, p.Pattern)
			}

			var b strings.Builder
			fmt.Fprintf(&b, "在 %s 中找到 %d 处包含「%s」的内容：\n", p.Path, len(results), p.Pattern)
			for _, r := range results {
				fmt.Fprintf(&b, "%s\n", r)
			}
			if len(results) >= 30 {
				fmt.Fprint(&b, "（仅显示前 30 条结果）\n")
			}
			return b.String()
		},
	}
}
