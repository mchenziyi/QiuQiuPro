package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewSearchFilesTool 搜索文件：按文件名或内容
func NewSearchFilesTool() Tool {
	return Tool{
		Name: "search_files", Description: "搜索文件：按文件名（glob 模式如 *.go）或文件内容关键词搜索",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":        map[string]any{"type": "string", "description": "文件名模式，如 *.go、**/*.md，或内容关键词"},
				"search_content": map[string]any{"type": "boolean", "description": "设为 true 则按文件内容搜索，false 则只按文件名搜索"},
			},
			"required": []string{"pattern"},
		},
		Execute: func(args string) string {
			var p struct {
				Pattern       string `json:"pattern"`
				SearchContent bool   `json:"search_content"`
			}
			json.Unmarshal([]byte(args), &p)
			if p.SearchContent {
				return searchFileContent(".", p.Pattern)
			}
			return searchFileName(".", p.Pattern)
		},
	}
}

// searchFileName 按文件名 glob 模式搜索
func searchFileName(root, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return fmt.Sprintf("搜索失败：%v", err)
	}
	subMatches, _ := filepath.Glob(filepath.Join(root, "**", pattern))
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
		return "没有找到匹配的文件"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "找到 %d 个匹配的文件：\n", len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil {
			fmt.Fprintf(&b, "  %s（%d 字节）\n", m, info.Size())
		} else {
			fmt.Fprintf(&b, "  %s\n", m)
		}
	}
	return b.String()
}

// searchFileContent 按文件内容搜索关键词
func searchFileContent(root, keyword string) string {
	var results []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		ext := filepath.Ext(path)
		textExts := map[string]bool{
			".go": true, ".md": true, ".txt": true, ".json": true,
			".yaml": true, ".yml": true, ".toml": true, ".xml": true,
			".html": true, ".css": true, ".js": true, ".ts": true,
			".py": true, ".rs": true, ".java": true, ".c": true, ".h": true,
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
			if strings.Contains(line, keyword) {
				results = append(results, fmt.Sprintf("  %s:%d  %s", path, i+1, strings.TrimSpace(line)))
				if len(results) >= 20 {
					return filepath.SkipDir
				}
			}
		}
		return nil
	})
	if len(results) == 0 {
		return fmt.Sprintf("在所有文件中未找到关键词：%s", keyword)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "找到 %d 处包含「%s」的内容：\n", len(results), keyword)
	for _, r := range results {
		fmt.Fprintf(&b, "%s\n", r)
	}
	if len(results) >= 20 {
		fmt.Fprint(&b, "（仅显示前 20 条结果）\n")
	}
	return b.String()
}


