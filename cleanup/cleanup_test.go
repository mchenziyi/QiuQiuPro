package cleanup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsJunk(t *testing.T) {
	junk := []string{".DS_Store", "Thumbs.db", "desktop.ini", "a.tmp", "b.temp", "c.bak", "d.orig", ".main.go.swp", "e.swo", "backup~"}
	for _, n := range junk {
		if !IsJunk(n) {
			t.Errorf("%q 应判为垃圾文件", n)
		}
	}
	keep := []string{"main.go", "README.md", "data.json", "go.mod", "tmp.go", "notes"}
	for _, n := range keep {
		if IsJunk(n) {
			t.Errorf("%q 不应判为垃圾文件", n)
		}
	}
}

func TestScan_FindsJunkSkipsGit(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "main.go"), "package main")
	mustWrite(t, filepath.Join(dir, ".DS_Store"), "x")
	mustWrite(t, filepath.Join(dir, "sub", "foo.tmp"), "x")
	// .git 下的垃圾文件必须被跳过，绝不能删。
	mustWrite(t, filepath.Join(dir, ".git", "junk.tmp"), "x")

	files, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("应找到 2 个垃圾文件，实际 %d：%+v", len(files), files)
	}
	for _, f := range files {
		if strings.Contains(f.Path, ".git") {
			t.Fatalf(".git 下的文件不应被扫到：%s", f.Path)
		}
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.tmp")
	p2 := filepath.Join(dir, "b.bak")
	mustWrite(t, p1, "x")
	mustWrite(t, p2, "y")

	deleted, errs := Delete([]JunkFile{{Path: p1}, {Path: p2}})
	if deleted != 2 || len(errs) != 0 {
		t.Fatalf("应删除 2 个且无错误，实际 deleted=%d errs=%v", deleted, errs)
	}
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Errorf("%s 应已被删除", p1)
	}
}

func TestDelete_PartialError(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.tmp")
	mustWrite(t, p1, "x")
	missing := filepath.Join(dir, "not_exist.tmp")

	deleted, errs := Delete([]JunkFile{{Path: p1}, {Path: missing}})
	if deleted != 1 {
		t.Errorf("应只删除 1 个真实文件，实际 %d", deleted)
	}
	if len(errs) != 1 {
		t.Errorf("应有 1 个删除错误，实际 %d", len(errs))
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
	}
	for _, c := range cases {
		if got := HumanSize(c.n); got != c.want {
			t.Errorf("HumanSize(%d)=%q，期望 %q", c.n, got, c.want)
		}
	}
}

func TestFormatList(t *testing.T) {
	out := FormatList([]JunkFile{{Path: "a.tmp", Size: 100}, {Path: "b.bak", Size: 1024}})
	if !strings.Contains(out, "a.tmp") || !strings.Contains(out, "b.bak") {
		t.Errorf("应列出每个文件，实际：%s", out)
	}
	if !strings.Contains(out, "合计 2 个文件") {
		t.Errorf("应有合计行，实际：%s", out)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
