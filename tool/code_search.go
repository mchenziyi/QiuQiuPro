package tool

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const codeSearchMaxRefs = 50 // 引用最多展示条数，避免刷屏 / 污染上下文

// NewCodeSearchTool 按符号名做语义级 Go 代码搜索（基于 go/ast，比 grep 更准）
// LLM 使用场景："Foo 定义在哪"、"哪些地方调用了 parseConfig"、"改这个函数会影响谁"
func NewCodeSearchTool() Tool {
	return Tool{
		Name:        "code_search",
		Description: "按符号名搜索 Go 代码：定位函数 / 方法 / 类型 / 变量 / 常量的定义位置，以及所有引用（用法）。基于 go/ast 解析，只匹配真正的标识符，比 grep 准",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{"type": "string", "description": "要搜索的符号名（函数 / 类型 / 变量名等），区分大小写"},
				"path":   map[string]any{"type": "string", "description": "搜索的根目录，默认当前目录"},
			},
			"required": []string{"symbol"},
		},
		Execute: func(args string) string {
			var p struct {
				Symbol string `json:"symbol"`
				Path   string `json:"path"`
			}
			json.Unmarshal([]byte(args), &p)
			return searchSymbol(p.Path, p.Symbol)
		},
	}
}

// codeHit 一处命中（定义或引用）
type codeHit struct {
	File    string
	Line    int
	Kind    string // func / method / type / var / const / ref
	Snippet string // 该行源码（已 trim）
}

// searchSymbol 在 root 下所有 .go 文件中查找 symbol 的定义与引用，返回 LLM 友好的文本。
func searchSymbol(root, symbol string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return "搜索失败：symbol 不能为空"
	}

	var allDefs, allRefs []codeHit
	for _, f := range collectGoFiles(root) {
		src, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		defs, refs, err := searchSymbolInSource(f, src, symbol)
		if err != nil {
			continue // 解析失败的文件跳过
		}
		allDefs = append(allDefs, defs...)
		allRefs = append(allRefs, refs...)
	}
	return formatCodeSearch(symbol, allDefs, allRefs)
}

// collectGoFiles 递归收集 root 下的 .go 文件，跳过隐藏目录 / vendor / node_modules。
func collectGoFiles(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name != "." && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// searchSymbolInSource 解析单个文件源码，分别返回 symbol 的定义与引用。
// 不读文件系统（src 直接传入），便于单测。
func searchSymbolInSource(filename string, src []byte, symbol string) (defs, refs []codeHit, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(string(src), "\n")
	at := func(pos token.Pos, kind string) codeHit {
		p := fset.Position(pos)
		snippet := ""
		if p.Line >= 1 && p.Line <= len(lines) {
			snippet = strings.TrimSpace(lines[p.Line-1])
		}
		return codeHit{File: filename, Line: p.Line, Kind: kind, Snippet: snippet}
	}

	// 第一遍：收集定义，并记下定义名标识符的位置（用于把它从引用中剔除）。
	defPos := map[token.Pos]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == symbol {
				kind := "func"
				if x.Recv != nil {
					kind = "method"
				}
				defs = append(defs, at(x.Name.Pos(), kind))
				defPos[x.Name.Pos()] = true
			}
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == symbol {
						defs = append(defs, at(s.Name.Pos(), "type"))
						defPos[s.Name.Pos()] = true
					}
				case *ast.ValueSpec:
					kind := "var"
					if x.Tok == token.CONST {
						kind = "const"
					}
					for _, name := range s.Names {
						if name.Name == symbol {
							defs = append(defs, at(name.Pos(), kind))
							defPos[name.Pos()] = true
						}
					}
				}
			}
		}
		return true
	})

	// 第二遍：所有同名标识符里，排除定义名本身，即为引用。
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == symbol && !defPos[id.Pos()] {
			refs = append(refs, at(id.Pos(), "ref"))
		}
		return true
	})
	return defs, refs, nil
}

// formatCodeSearch 把定义与引用整理成 LLM 友好的文本（按 文件:行 排序、引用截断）。
func formatCodeSearch(symbol string, defs, refs []codeHit) string {
	sortHits(defs)
	sortHits(refs)

	var b strings.Builder
	fmt.Fprintf(&b, "符号「%s」的搜索结果：\n\n", symbol)

	if len(defs) == 0 {
		fmt.Fprint(&b, "定义（0）：未找到\n")
	} else {
		fmt.Fprintf(&b, "定义（%d）：\n", len(defs))
		for _, h := range defs {
			fmt.Fprintf(&b, "  [%s] %s:%d  %s\n", h.Kind, h.File, h.Line, h.Snippet)
		}
	}

	if len(refs) == 0 {
		fmt.Fprint(&b, "\n引用（0）：未找到\n")
		return b.String()
	}
	fmt.Fprintf(&b, "\n引用（%d）：\n", len(refs))
	shown := refs
	if len(shown) > codeSearchMaxRefs {
		shown = shown[:codeSearchMaxRefs]
	}
	for _, h := range shown {
		fmt.Fprintf(&b, "  %s:%d  %s\n", h.File, h.Line, h.Snippet)
	}
	if len(refs) > codeSearchMaxRefs {
		fmt.Fprintf(&b, "  …（共 %d 处，仅显示前 %d 处）\n", len(refs), codeSearchMaxRefs)
	}
	return b.String()
}

func sortHits(hits []codeHit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
}
