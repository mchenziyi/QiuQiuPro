package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// --------------- 文件读写 ---------------

func NewReadFileTool() Tool {
	return Tool{
		Name: "read_file", Description: "读取指定文件的内容", ReadOnly: true,
		Parameters: objParams(
			prop("path", "string", "文件路径"),
		).Required("path"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取 %s 失败", p.Path)
			}
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s", p.Path, len(data), string(data)), nil
		},
	}
}

func NewWriteFileTool() Tool {
	return Tool{
		Name: "write_file", Description: "创建或覆盖文件", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", "文件路径"),
			prop("content", "string", "内容"),
		).Required("path", "content"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path, Content string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if err := os.WriteFile(p.Path, []byte(p.Content), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf("已写入 %s", p.Path), nil
		},
	}
}

func NewListDirectoryTool() Tool {
	return Tool{
		Name: "ls", Description: "列出目录下的文件和子目录", ReadOnly: true,
		Parameters: objParams(
			prop("path", "string", "目录路径"),
		).Build(),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ Path string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				p.Path = "."
			}
			entries, err := os.ReadDir(p.Path)
			if err != nil {
				return "", fmt.Errorf("读目录失败: %v", err)
			}
			var dirs, files []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, e.Name()+"/")
				} else {
					files = append(files, e.Name())
				}
			}
			var b strings.Builder
			if len(dirs) > 0 {
				b.WriteString("子目录：" + strings.Join(dirs, "、") + "\n")
			}
			if len(files) > 0 {
				b.WriteString("文件：\n" + strings.Join(files, "\n"))
			}
			if b.Len() == 0 {
				b.WriteString("（空目录）")
			}
			return fmt.Sprintf("目录 %s：\n%s", p.Path, strings.TrimSpace(b.String())), nil
		},
	}
}

// --------------- 编辑 ---------------

func NewEditFileTool() Tool {
	return Tool{
		Name: "edit_file", Description: "精确替换文件中的一段文本", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", ""),
			prop("old_string", "string", ""),
			prop("new_string", "string", ""),
		).Required("path", "old_string", "new_string"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path      string `json:"path"`
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			content := string(b)
			n := strings.Count(content, p.OldString)
			if n == 0 {
				return "", fmt.Errorf("未找到 old_string")
			}
			if n > 1 {
				return "", fmt.Errorf("old_string 出现 %d 次", n)
			}
			// 计算 diff
			before := content
			after := strings.Replace(content, p.OldString, p.NewString, 1)
			diff := ComputeLineDiff(before, after, p.Path, 3)
			diffJSON, _ := json.Marshal(diff)
			// 路径反斜杠 JSON 转义，防止接收端解析失败
			safePath := strings.ReplaceAll(p.Path, "\\", "\\\\")

			if err := os.WriteFile(p.Path, []byte(after), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf(`{"text":"已编辑 %s","diff":%s}`, safePath, string(diffJSON)), nil
		},
	}
}

func NewMultiEditTool() Tool {
	return Tool{
		Name: "multi_edit", Description: "批量编辑文件，原子性", ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
				"edits": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"old_string":  map[string]any{"type": "string"},
							"new_string":  map[string]any{"type": "string"},
							"replace_all": map[string]any{"type": "boolean"},
						},
						"required": []string{"old_string", "new_string"},
					},
				},
			},
			"required": []string{"path", "edits"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path  string `json:"path"`
				Edits []struct {
					OldString  string `json:"old_string"`
					NewString  string `json:"new_string"`
					ReplaceAll bool   `json:"replace_all"`
				} `json:"edits"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			content := string(b)
			for i, step := range p.Edits {
				if step.ReplaceAll {
					content = strings.ReplaceAll(content, step.OldString, step.NewString)
					continue
				}
				n := strings.Count(content, step.OldString)
				if n == 0 {
					return "", fmt.Errorf("edit %d 未找到", i+1)
				}
				if n > 1 {
					return "", fmt.Errorf("edit %d 不唯一", i+1)
				}
				content = strings.Replace(content, step.OldString, step.NewString, 1)
			}
			if err := os.WriteFile(p.Path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf("已编辑 %s（%d 条）", p.Path, len(p.Edits)), nil
		},
	}
}

func NewDeleteRangeTool() Tool {
	return Tool{
		Name: "delete_range", Description: "按行锚点删除连续区域", ReadOnly: false,
		Parameters: objParams(
			prop("path", "string", ""),
			prop("start_anchor", "string", ""),
			prop("end_anchor", "string", ""),
			prop("inclusive", "boolean", ""),
		).Required("path", "start_anchor", "end_anchor"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Path        string `json:"path"`
				StartAnchor string `json:"start_anchor"`
				EndAnchor   string `json:"end_anchor"`
				Inclusive   *bool  `json:"inclusive"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			inc := true
			if p.Inclusive != nil {
				inc = *p.Inclusive
			}
			b, err := os.ReadFile(p.Path)
			if err != nil {
				return "", fmt.Errorf("读取失败: %v", err)
			}
			orig := string(b)
			lines := strings.Split(strings.ReplaceAll(orig, "\r", ""), "\n")
			s := findLine(lines, p.StartAnchor)
			if s < 0 {
				return "", fmt.Errorf("未找到 start_anchor")
			}
			e := findLine(lines, p.EndAnchor)
			if e < 0 {
				return "", fmt.Errorf("未找到 end_anchor")
			}
			if s > e {
				return "", fmt.Errorf("anchor 顺序颠倒")
			}
			var keep []string
			if inc {
				keep = append(keep, lines[:s]...)
				keep = append(keep, lines[e+1:]...)
			} else {
				keep = append(keep, lines[:s+1]...)
				keep = append(keep, lines[e:]...)
			}
			newContent := strings.Join(keep, "\n")
			if strings.HasSuffix(orig, "\n") && !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			if err := os.WriteFile(p.Path, []byte(newContent), 0644); err != nil {
				return "", fmt.Errorf("写入失败: %v", err)
			}
			return fmt.Sprintf("已删除第 %d-%d 行", s+1, e+1), nil
		},
	}
}

func findLine(lines []string, target string) int {
	for i, l := range lines {
		if l == target {
			return i
		}
	}
	return -1
}

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
// 把 pattern 按 ** 切分为前缀目录和后缀 glob，然后 Walk 目录逐层匹配。
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

// --------------- 任务管理 ---------------

func NewTodoWriteTool() Tool {
	return Tool{
		Name: "todo_write", Description: "记录任务清单", ReadOnly: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content":    map[string]any{"type": "string"},
							"status":     map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
							"activeForm": map[string]any{"type": "string"},
							"level":      map[string]any{"type": "integer", "enum": []any{0, 1}},
						},
					},
				},
			},
			"required": []string{"todos"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Todos []struct {
					Content, Status, ActiveForm string
					Level                       int
				}
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			var done, active, pending int
			for _, t := range p.Todos {
				switch t.Status {
				case "completed":
					done++
				case "in_progress":
					active++
				default:
					pending++
				}
			}
			return fmt.Sprintf("Todos: %d done, %d active, %d pending", done, active, pending), nil
		},
	}
}

// --------------- 网络 ---------------

func stripHTML(s string) string {
	s = regexp.MustCompile(`(?is)<(?:script|style)[^>]*>.*?</(?:script|style)>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<!--.*?-->`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, "")
	repl := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ")
	s = repl.Replace(s)
	return regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`).ReplaceAllString(s, "\n\n")
}

func NewWebFetchTool() Tool {
	return Tool{
		Name: "web_fetch", Description: "HTTP GET 抓取 URL", ReadOnly: true,
		Parameters: objParams(
			prop("url", "string", ""),
		).Required("url"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ URL string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.URL == "" {
				return "", fmt.Errorf("url required")
			}
			client := &http.Client{Timeout: 15 * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
			if err != nil {
				return "", fmt.Errorf("request: %v", err)
			}
			req.Header.Set("User-Agent", "QiuQiuPro/1.0")
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("fetch: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if err != nil {
				return "", fmt.Errorf("read: %v", err)
			}
			out := string(body)
			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			if strings.Contains(ct, "text/html") || strings.Contains(out, "<!doctype") || strings.Contains(out, "<html") {
				out = stripHTML(out)
			}
			if len(out) > 16000 {
				out = safeTruncate(out, 16000)
			}
			return fmt.Sprintf("HTTP %s\n%s", resp.Status, strings.TrimSpace(out)), nil
		},
	}
}

// --------------- Git / Shell ---------------

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

// --------------- 参数构建辅助 ---------------

type paramBuilder struct {
	props    map[string]any
	required []string
}

func objParams(props ...map[string]any) *paramBuilder {
	merged := map[string]any{}
	for _, p := range props {
		for k, v := range p {
			merged[k] = v
		}
	}
	return &paramBuilder{props: merged}
}

func (b *paramBuilder) Required(names ...string) map[string]any {
	b.required = names
	return b.Build()
}

func (b *paramBuilder) Build() map[string]any {
	m := map[string]any{"type": "object", "properties": b.props}
	if len(b.required) > 0 {
		m["required"] = b.required
	}
	return m
}

func prop(name, typ, desc string) map[string]any {
	p := map[string]any{"type": typ}
	if desc != "" {
		p["description"] = desc
	}
	return map[string]any{name: p}
}

// safeTruncate 按字节截断字符串，同时保证不破坏 UTF-8 多字节字符边界。
// 从 maxBytes 位置向前回退到有效的 rune 起始位置，确保返回的字符串合法。
func safeTruncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	for maxBytes > 0 && !utf8.RuneStart(b[maxBytes]) {
		maxBytes--
	}
	return string(b[:maxBytes]) + "…(截断)"
}
