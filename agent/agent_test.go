package agent

import "testing"

func TestIsHighRiskTool(t *testing.T) {
	high := []string{"write_file", "edit_file_block", "bash", "run_powershell"}
	for _, name := range high {
		if !IsHighRiskTool(name) {
			t.Errorf("%s 应被判为高危", name)
		}
	}
	safe := []string{"read_file", "ls", "glob", "grep", "search_files", "count_file_chars", "git_commit", "unknown_tool", ""}
	for _, name := range safe {
		if IsHighRiskTool(name) {
			t.Errorf("%s 不应被判为高危", name)
		}
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello", 10, "hello"},      // 短于上限：原样
		{"hello", 5, "hello"},       // 恰好等于上限：原样
		{"hello", 3, "hel..."},      // 超出：截断 + ...
		{"", 3, ""},                 // 空串
		{"你好世界啊", 2, "你好..."}, // 按 rune（而非字节）截断中文
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q,%d)=%q，期望 %q", c.in, c.n, got, c.want)
		}
	}
}

func TestStripCodeFence(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"```json\n[1,2]\n```", "[1,2]"}, // ```json 围栏
		{"```\n[1,2]\n```", "[1,2]"},     // 普通 ``` 围栏
		{"  [1,2]  ", "[1,2]"},           // 无围栏，仅 trim
		{"```json[1]```", "[1]"},         // 同行围栏
		{"[1]", "[1]"},                   // 纯 JSON 原样
	}
	for _, c := range cases {
		if got := stripCodeFence(c.in); got != c.want {
			t.Errorf("stripCodeFence(%q)=%q，期望 %q", c.in, got, c.want)
		}
	}
}

