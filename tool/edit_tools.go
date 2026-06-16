package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewEditFileBlockTool 精确编辑文件：找到一段旧代码，替换成新代码
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
			// 必须带 json tag：schema 用 snake_case（old_block/new_block），
			// 而 Go 的大小写不敏感匹配不会忽略下划线，无 tag 会绑不上、导致永远「出现多次」。
			var p struct {
				Path     string `json:"path"`
				OldBlock string `json:"old_block"`
				NewBlock string `json:"new_block"`
			}
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

