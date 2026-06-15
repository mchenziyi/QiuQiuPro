package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSrc = `package sample

type Widget struct {
	Name string
}

const MaxWidgets = 10

var defaultWidget Widget

func MakeWidget(name string) Widget {
	return Widget{Name: name}
}

func (w Widget) Render() string {
	return w.Name
}

func use() {
	w := MakeWidget("a")
	_ = w.Render()
	_ = MaxWidgets
	_ = defaultWidget
	var x Widget
	_ = x
}
`

func TestSearchSymbolInSource_KindsAndCounts(t *testing.T) {
	cases := []struct {
		symbol   string
		defKind  string
		defCount int
		refCount int
	}{
		{"Widget", "type", 1, 5},        // 类型：1 定义 + 5 处用作类型
		{"MakeWidget", "func", 1, 1},    // 函数：1 定义 + 1 处调用
		{"Render", "method", 1, 1},      // 方法：1 定义 + 1 处 .Render() 调用
		{"MaxWidgets", "const", 1, 1},   // 常量
		{"defaultWidget", "var", 1, 1},  // 变量
	}
	for _, c := range cases {
		defs, refs, err := searchSymbolInSource("sample.go", []byte(sampleSrc), c.symbol)
		if err != nil {
			t.Fatalf("%s: 解析失败 %v", c.symbol, err)
		}
		if len(defs) != c.defCount {
			t.Errorf("%s: 定义数=%d，期望 %d（%+v）", c.symbol, len(defs), c.defCount, defs)
		}
		if c.defCount > 0 && defs[0].Kind != c.defKind {
			t.Errorf("%s: 定义 kind=%s，期望 %s", c.symbol, defs[0].Kind, c.defKind)
		}
		if len(refs) != c.refCount {
			t.Errorf("%s: 引用数=%d，期望 %d（%+v）", c.symbol, len(refs), c.refCount, refs)
		}
	}
}

func TestSearchSymbolInSource_ExcludesDefFromRefs(t *testing.T) {
	// MakeWidget 的定义行不应出现在引用里。
	defs, refs, _ := searchSymbolInSource("sample.go", []byte(sampleSrc), "MakeWidget")
	defLine := defs[0].Line
	for _, r := range refs {
		if r.Line == defLine {
			t.Fatalf("定义行 %d 不应被当成引用", defLine)
		}
	}
}

func TestSearchSymbolInSource_ParseError(t *testing.T) {
	if _, _, err := searchSymbolInSource("bad.go", []byte("package x\nfunc ("), "Foo"); err == nil {
		t.Fatal("非法 Go 源码应返回解析错误")
	}
}

func TestSearchSymbol_TempDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(sampleSrc), 0644); err != nil {
		t.Fatal(err)
	}
	result := searchSymbol(dir, "Widget")
	if !strings.Contains(result, "定义（1）") || !strings.Contains(result, "[type]") {
		t.Fatalf("应找到 1 个 type 定义，实际：%s", result)
	}
	if !strings.Contains(result, "引用（5）") {
		t.Fatalf("应找到 5 处引用，实际：%s", result)
	}
}

func TestSearchSymbol_EmptySymbol(t *testing.T) {
	if r := searchSymbol(".", "  "); !strings.Contains(r, "symbol 不能为空") {
		t.Fatalf("空 symbol 应提示，实际：%s", r)
	}
}

func TestSearchSymbol_NotFound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sample.go"), []byte(sampleSrc), 0644)
	result := searchSymbol(dir, "NoSuchSymbolXyz")
	if !strings.Contains(result, "定义（0）：未找到") || !strings.Contains(result, "引用（0）：未找到") {
		t.Fatalf("不存在的符号应两处都未找到，实际：%s", result)
	}
}

func TestCollectGoFiles_SkipsHiddenAndNonGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0755)
	os.WriteFile(filepath.Join(dir, ".hidden", "c.go"), []byte("package c"), 0644)

	files := collectGoFiles(dir)
	if len(files) != 1 || !strings.HasSuffix(files[0], "a.go") {
		t.Fatalf("应只收集到 a.go，实际：%v", files)
	}
}

func TestFormatCodeSearch_CapsRefs(t *testing.T) {
	var refs []codeHit
	for i := 0; i < codeSearchMaxRefs+10; i++ {
		refs = append(refs, codeHit{File: "f.go", Line: i + 1, Kind: "ref", Snippet: "x"})
	}
	out := formatCodeSearch("Foo", nil, refs)
	if !strings.Contains(out, "仅显示前") {
		t.Fatalf("超过上限应截断提示，实际尾部：%s", out[max(0, len(out)-60):])
	}
}
