package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// --------------- 搜索 ---------------

func NewSearchFilesTool() Tool {
	return Tool{
		Name: "search_files", Description: "按文件名或关键词搜索文件", ReadOnly: true,
		Parameters: objParams(
			prop("pattern", "string", "起始路径或目录"),
			prop("term", "string", "文件名关键词"),
		).Build(),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Pattern, Term string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Pattern == "" && p.Term == "" {
				return "", fmt.Errorf("需要 pattern 或 term")
			}
			var matches []string
			root := p.Pattern
			if root == "" {
				root = "."
			}
			filepath.Walk(root, func(fp string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if p.Term != "" && strings.Contains(fi.Name(), p.Term) {
					matches = append(matches, fp)
				}
				return nil
			})
			if len(matches) == 0 {
				return "无匹配", nil
			}
			return fmt.Sprintf("匹配 %d 个文件：\n%s", len(matches), strings.Join(matches, "\n")), nil
		},
	}
}

func NewGlobTool() Tool {
	return Tool{
		Name: "glob", Description: "按文件名模式搜索文件（支持递归 ** 模式）", ReadOnly: true,
		Parameters: objParams(
			prop("pattern", "string", "glob 模式，支持 ** 递归匹配（如 **/*.go）"),
		).Required("pattern"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Pattern string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Pattern == "" {
				return "", fmt.Errorf("pattern required")
			}
			if strings.Contains(p.Pattern, "**") {
				return globRecursive(p.Pattern)
			}
			matches, err := filepath.Glob(p.Pattern)
			if err != nil {
				return "", fmt.Errorf("glob: %v", err)
			}
			if len(matches) == 0 {
				return "无匹配", nil
			}
			return fmt.Sprintf("匹配 %d 个文件：\n%s", len(matches), strings.Join(matches, "\n")), nil
		},
	}
}

// globRecursive 支持 ** 递归模式（如 src/**/*.go）。
func globRecursive(pattern string) (string, error) {
	parts := strings.SplitN(pattern, "**", 2)
	root := strings.TrimRight(parts[0], string(os.PathSeparator))
	if root == "" {
		root = "."
	}
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimLeft(parts[1], string(os.PathSeparator))
	}

	var matches []string
	filepath.Walk(root, func(fp string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		if suffix == "" {
			matches = append(matches, fp)
			return nil
		}
		if ok, _ := filepath.Match(suffix, fi.Name()); ok {
			matches = append(matches, fp)
		}
		return nil
	})
	if len(matches) == 0 {
		return "无匹配", nil
	}
	return fmt.Sprintf("匹配 %d 个文件：\n%s", len(matches), strings.Join(matches, "\n")), nil
}

func NewGrepTool() Tool {
	return Tool{
		Name: "grep", Description: "在文件内容中搜索关键词或正则表达式", ReadOnly: true,
		Parameters: objParams(
			prop("pattern", "string", "搜索模式（支持正则表达式）"),
			prop("path", "string", "搜索起始目录，默认当前目录"),
		).Required("pattern"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Pattern, Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Pattern == "" {
				return "", fmt.Errorf("pattern required")
			}
			re, err := regexp.Compile(p.Pattern)
			if err != nil {
				return "", fmt.Errorf("正则编译失败: %v", err)
			}
			root := p.Path
			if root == "" {
				root = "."
			}
			var matches []string
			filepath.Walk(root, func(fp string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if fi.IsDir() {
					if strings.HasPrefix(fi.Name(), ".") && fi.Name() != "." {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasPrefix(fi.Name(), ".") {
					return nil
				}
				data, err := os.ReadFile(fp)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for i, line := range lines {
					if re.MatchString(line) {
						matches = append(matches, fmt.Sprintf("%s:%d: %s", fp, i+1, strings.TrimSpace(line)))
					}
				}
				return nil
			})
			if len(matches) == 0 {
				return "无匹配", nil
			}
			return fmt.Sprintf("匹配 %d 处：\n%s", len(matches), strings.Join(matches, "\n")), nil
		},
	}
}

func NewCodeSearchTool() Tool {
	return Tool{
		Name: "code_search", Description: "按符号名搜索 Go 代码", ReadOnly: true,
		Parameters: objParams(
			prop("symbol", "string", "符号名"),
			prop("path", "string", "搜索目录"),
		).Required("symbol"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Symbol, Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			searchDir := p.Path
			if searchDir == "" {
				searchDir = "."
			}
			var matches []string
			filepath.Walk(searchDir, func(fp string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() || !strings.HasSuffix(fp, ".go") {
					return nil
				}
				data, err := os.ReadFile(fp)
				if err != nil {
					return nil
				}
				lines := strings.Split(string(data), "\n")
				for i, line := range lines {
					if strings.Contains(line, p.Symbol) {
						matches = append(matches, fmt.Sprintf("%s:%d: %s", fp, i+1, strings.TrimSpace(line)))
					}
				}
				return nil
			})
			if len(matches) == 0 {
				return fmt.Sprintf("未找到符号 %s", p.Symbol), nil
			}
			return fmt.Sprintf("找到 %d 处 %s：\n%s", len(matches), p.Symbol, strings.Join(matches, "\n")), nil
		},
	}
}
