package tool

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// NewDeleteSymbolTool 返回 delete_symbol 工具：从 Go 源文件中按符号名删除函数/方法/类型等。
// 参照 Reasonix builtin/delete_symbol.go。
func NewDeleteSymbolTool() Tool {
	return Tool{
		Name:        "delete_symbol",
		Description: "Delete a named symbol (function, method, type, interface, const, var) from a Go source file using AST parsing. For non-Go files, use delete_range with manual anchors.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "Path to the source file"},
				"name":   map[string]any{"type": "string", "description": "Name of the symbol to delete"},
				"kind":   map[string]any{"type": "string", "description": "Optional kind filter: func, method, type, interface, const, var"},
				"parent": map[string]any{"type": "string", "description": "Optional parent struct name for method disambiguation"},
			},
			"required": []string{"path", "name"},
		},
		Execute: func(args string) string {
			var p struct {
				Path   string `json:"path"`
				Name   string `json:"name"`
				Kind   string `json:"kind"`
				Parent string `json:"parent"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("参数解析失败：%v", err)
			}
			if p.Path == "" {
				return "path 不能为空"
			}
			if p.Name == "" {
				return "name 不能为空"
			}

			ext := strings.ToLower(filepath.Ext(p.Path))
			if ext != ".go" {
				return fmt.Sprintf("delete_symbol 只支持 Go 文件，%s 文件请用 delete_range", ext)
			}

			m, fset, err := findSymbol(p.Path, p.Name, p.Kind, p.Parent)
			if err != nil {
				return err.Error()
			}

			src, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读取 %s 失败：%v", p.Path, err)
			}
			original := string(src)
			newContent := deleteSymbolLines(original, fset, m)

			if err := os.WriteFile(p.Path, []byte(newContent), 0644); err != nil {
				return fmt.Sprintf("写入 %s 失败：%v", p.Path, err)
			}
			return fmt.Sprintf("已从 %s 中删除 %s %s（第 %d 行）", p.Path, m.kind, m.name, m.line)
		},
	}
}

type symbolMatch struct {
	name     string
	kind     string
	parent   string
	start    token.Pos
	docStart token.Pos
	end      token.Pos
	line     int
	siblings []string
}

func findSymbol(path, name, kind, parent string) (symbolMatch, *token.FileSet, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return symbolMatch{}, nil, fmt.Errorf("解析 %s 失败：%v", path, err)
	}

	matches := collectSymbols(fset, f)
	var byName []symbolMatch
	for _, m := range matches {
		if m.name == name {
			byName = append(byName, m)
		}
	}
	if len(byName) == 0 {
		return symbolMatch{}, nil, fmt.Errorf("在 %s 中未找到符号 %q", path, name)
	}

	filtered := byName
	if kind != "" {
		var byKind []symbolMatch
		for _, m := range filtered {
			if m.kind == kind {
				byKind = append(byKind, m)
			}
		}
		if len(byKind) == 0 {
			return symbolMatch{}, nil, fmt.Errorf("未找到 kind=%q 的符号 %q", kind, name)
		}
		filtered = byKind
	}
	if parent != "" {
		var byParent []symbolMatch
		for _, m := range filtered {
			if m.parent == parent {
				byParent = append(byParent, m)
			}
		}
		if len(byParent) == 0 {
			return symbolMatch{}, nil, fmt.Errorf("未找到 parent=%q 的符号 %q", parent, name)
		}
		filtered = byParent
	}

	if len(filtered) > 1 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%q 有多个匹配，请用 kind/parent 区分：\n", name))
		for _, m := range filtered {
			b.WriteString(fmt.Sprintf("  第 %d 行：%s %s", m.line, m.kind, m.name))
			if m.parent != "" {
				b.WriteString(fmt.Sprintf("（在 %s 上）", m.parent))
			}
			b.WriteString("\n")
		}
		return symbolMatch{}, nil, fmt.Errorf("%s", b.String())
	}

	if len(filtered[0].siblings) > 1 {
		return symbolMatch{}, nil, fmt.Errorf("%s %q 在同一个 %s 声明中与 %s 一起定义，delete_symbol 拒绝删除它（会同时删除其他符号）", filtered[0].kind, name, filtered[0].kind, strings.Join(filtered[0].siblings, ", "))
	}

	return filtered[0], fset, nil
}

func collectSymbols(fset *token.FileSet, f *ast.File) []symbolMatch {
	var matches []symbolMatch
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			m := symbolMatch{
				name:  d.Name.Name,
				kind:  "func",
				start: d.Pos(),
				end:   d.End(),
				line:  fset.Position(d.Pos()).Line,
			}
			if d.Doc != nil {
				m.docStart = d.Doc.Pos()
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				m.kind = "method"
				recvType := d.Recv.List[0].Type
				if se, ok := recvType.(*ast.StarExpr); ok {
					if ident, ok := se.X.(*ast.Ident); ok {
						m.parent = ident.Name
					}
				} else if ident, ok := recvType.(*ast.Ident); ok {
					m.parent = ident.Name
				}
			}
			matches = append(matches, m)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					m := symbolMatch{
						name:  s.Name.Name,
						start: s.Pos(),
						end:   s.End(),
						line:  fset.Position(s.Pos()).Line,
					}
					if _, ok := s.Type.(*ast.InterfaceType); ok {
						m.kind = "interface"
					} else {
						m.kind = "type"
					}
					if d.Doc != nil {
						m.docStart = d.Doc.Pos()
					}
					matches = append(matches, m)
				case *ast.ValueSpec:
					k := "var"
					if d.Tok == token.CONST {
						k = "const"
					}
					names := make([]string, 0, len(s.Names))
					for _, ident := range s.Names {
						names = append(names, ident.Name)
					}
					for _, ident := range s.Names {
						matches = append(matches, symbolMatch{
							name:     ident.Name,
							kind:     k,
							start:    ident.Pos(),
							end:      s.End(),
							line:     fset.Position(ident.Pos()).Line,
							siblings: names,
						})
					}
				}
			}
		}
	}
	return matches
}

func deleteSymbolLines(content string, fset *token.FileSet, m symbolMatch) string {
	start := m.start
	if m.docStart.IsValid() {
		start = m.docStart
	}
	startOff := fset.Position(start).Offset
	endOff := fset.Position(m.end).Offset

	lineStart := startOff
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	lineEnd := endOff
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	if lineEnd < len(content) {
		lineEnd++
	}

	return content[:lineStart] + content[lineEnd:]
}
