package agent

import (
	"bufio"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

func TestGateCheck(t *testing.T) {
	cases := []struct {
		gate Gate
		tool string
		want Decision
	}{
		// 默认门：高危确认，其余放行
		{ConfirmHighRiskGate{}, "write_file", GateConfirm},
		{ConfirmHighRiskGate{}, "bash", GateConfirm},
		{ConfirmHighRiskGate{}, "read_file", GateAllow},
		{ConfirmHighRiskGate{}, "git_commit", GateAllow}, // 不改变既有行为：提交仍直接放行
		// 只读门：改动类一律拒绝
		{ReadOnlyGate{}, "write_file", GateDeny},
		{ReadOnlyGate{}, "edit_file", GateDeny},
		{ReadOnlyGate{}, "bash", GateDeny},
		{ReadOnlyGate{}, "git_commit", GateDeny},
		{ReadOnlyGate{}, "read_file", GateAllow},
		{ReadOnlyGate{}, "code_search", GateAllow},
		// 全放行门
		{AllowAllGate{}, "write_file", GateAllow},
		{AllowAllGate{}, "read_file", GateAllow},
	}
	for _, c := range cases {
		if got, _ := c.gate.Check(c.tool, "{}"); got != c.want {
			t.Errorf("%s.Check(%s)=%d，期望 %d", c.gate.Name(), c.tool, got, c.want)
		}
	}
}

// fakeAgent 构造一个只含工具执行所需字段的最小 Agent。
func fakeAgent(g Gate, executed *bool) *Agent {
	a := &Agent{allTools: map[string]tool.Tool{}, gate: g, Quiet: true}
	mk := func(name string) tool.Tool {
		return tool.Tool{Name: name, Execute: func(string) string { *executed = true; return "DID:" + name }}
	}
	a.allTools["write_file"] = mk("write_file") // 高危 / 写
	a.allTools["read_file"] = mk("read_file")   // 只读
	return a
}

func call(a *Agent, name string) string {
	return a.executeToolCall(openai.ToolCall{Function: openai.FunctionCall{Name: name, Arguments: "{}"}})
}

func TestExecuteToolCall_UnknownTool(t *testing.T) {
	executed := false
	a := fakeAgent(ConfirmHighRiskGate{}, &executed)
	if r := call(a, "no_such_tool"); !strings.Contains(r, "未知工具") {
		t.Fatalf("应提示未知工具，实际：%s", r)
	}
}

func TestExecuteToolCall_ReadOnlyDeniesWrite(t *testing.T) {
	executed := false
	a := fakeAgent(ReadOnlyGate{}, &executed)
	r := call(a, "write_file")
	if executed {
		t.Fatal("只读模式下写工具不应被执行")
	}
	if !strings.Contains(r, "已拒绝") {
		t.Fatalf("应回灌拒绝说明，实际：%s", r)
	}
}

func TestExecuteToolCall_ReadOnlyAllowsRead(t *testing.T) {
	executed := false
	a := fakeAgent(ReadOnlyGate{}, &executed)
	r := call(a, "read_file")
	if !executed || !strings.Contains(r, "DID:read_file") {
		t.Fatalf("只读模式应放行读工具，实际 executed=%v r=%s", executed, r)
	}
}

func TestExecuteToolCall_AllowAllRunsWrite(t *testing.T) {
	executed := false
	a := fakeAgent(AllowAllGate{}, &executed)
	if call(a, "write_file"); !executed {
		t.Fatal("全放行门下写工具应直接执行")
	}
}

func TestExecuteToolCall_ConfirmYes(t *testing.T) {
	executed := false
	a := fakeAgent(ConfirmHighRiskGate{}, &executed)
	a.SetInput(bufio.NewReader(strings.NewReader("y\n")))
	if call(a, "write_file"); !executed {
		t.Fatal("确认 y 后写工具应执行")
	}
}

func TestExecuteToolCall_ConfirmNo(t *testing.T) {
	executed := false
	a := fakeAgent(ConfirmHighRiskGate{}, &executed)
	a.SetInput(bufio.NewReader(strings.NewReader("n\n")))
	r := call(a, "write_file")
	if executed {
		t.Fatal("确认 n 后写工具不应执行")
	}
	if !strings.Contains(r, "取消") {
		t.Fatalf("应回灌取消说明，实际：%s", r)
	}
}

func TestReadOnlyToggle(t *testing.T) {
	a := &Agent{gate: ConfirmHighRiskGate{}}
	if a.IsReadOnly() || a.GateName() != "confirm" {
		t.Fatalf("初始应为非只读 confirm，实际 readonly=%v name=%s", a.IsReadOnly(), a.GateName())
	}
	a.SetReadOnly(true)
	if !a.IsReadOnly() || a.GateName() != "read-only" {
		t.Fatalf("开启后应为只读，实际 readonly=%v name=%s", a.IsReadOnly(), a.GateName())
	}
	a.SetReadOnly(false)
	if a.IsReadOnly() || a.GateName() != "confirm" {
		t.Fatalf("关闭后应恢复 confirm，实际 readonly=%v name=%s", a.IsReadOnly(), a.GateName())
	}
	// nil gate 应安全回退为 confirm
	if (&Agent{}).GateName() != "confirm" {
		t.Fatal("nil gate 的 GateName 应回退为 confirm")
	}
}



