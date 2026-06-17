package agent

import (
	"bufio"
	"strings"
	"testing"
)

func newAgentWithInput(s string) *Agent {
	return &Agent{in: bufio.NewReader(strings.NewReader(s))}
}

func TestReadLine_LinesThenEOF(t *testing.T) {
	a := newAgentWithInput("hello\nworld\n")
	if line, ok := a.ReadLine(); !ok || line != "hello" {
		t.Fatalf("第一行应为 hello,true，实际 %q,%v", line, ok)
	}
	if line, ok := a.ReadLine(); !ok || line != "world" {
		t.Fatalf("第二行应为 world,true，实际 %q,%v", line, ok)
	}
	if line, ok := a.ReadLine(); ok || line != "" {
		t.Fatalf("结束应为 \"\",false，实际 %q,%v", line, ok)
	}
}

// 最后一行没有换行符也应能完整读到（bufio.Scanner 也能，但这里确认我们的实现一致）。
func TestReadLine_LastLineNoNewline(t *testing.T) {
	a := newAgentWithInput("partial")
	if line, ok := a.ReadLine(); !ok || line != "partial" {
		t.Fatalf("无换行的最后一行应读到 partial,true，实际 %q,%v", line, ok)
	}
	if line, ok := a.ReadLine(); ok || line != "" {
		t.Fatalf("再次读取应为 \"\",false，实际 %q,%v", line, ok)
	}
}

func TestReadLine_StripsCRLF(t *testing.T) {
	a := newAgentWithInput("win\r\n")
	if line, ok := a.ReadLine(); !ok || line != "win" {
		t.Fatalf("应去掉行尾 \\r\\n，实际 %q,%v", line, ok)
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"\n", true},     // 空行 = 默认 Yes
		{"y\n", true},    //
		{"Y\n", true},    //
		{"yes\n", true},  //
		{"随便\n", true},   // 非 n 都算确认
		{"n\n", false},   //
		{"N\n", false},   //
		{"no\n", false},  //
		{" n \n", false}, // 带空格也能识别
		{"", false},      // EOF = 取消（对高危操作更安全）
	}
	for _, c := range cases {
		a := newAgentWithInput(c.in)
		if got := a.confirm(); got != c.want {
			t.Fatalf("confirm(%q)=%v，期望 %v", c.in, got, c.want)
		}
	}
}
