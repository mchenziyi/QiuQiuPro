package agent

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
	"agentdemo/tool"
)

// newDispatchAgent 构造一个能跑 dispatchToolCalls 的最小 Agent：
// 真实的 store（临时目录）+ 空白 Session + 指定权限门，工具由调用方注册。
func newDispatchAgent(t *testing.T, g Gate) *Agent {
	t.Helper()
	return &Agent{
		allTools: map[string]tool.Tool{},
		gate:     g,
		store:    event.NewStore(t.TempDir()),
		session:  NewSession("test"),
		Quiet:    true,
	}
}

func tcOf(id, name string) openai.ToolCall {
	return openai.ToolCall{ID: id, Type: "function", Function: openai.FunctionCall{Name: name, Arguments: "{}"}}
}

// 无论并发与否，结果都必须按原始 tool_call 顺序回灌，且每条都与其 ID 配对。
func TestDispatchToolCalls_PreservesOrder(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	mk := func(name string) tool.Tool {
		return tool.Tool{Name: name, Execute: func(string) string { return "R:" + name }}
	}
	a.allTools["read_file"] = mk("read_file")   // 只读 → 并发
	a.allTools["write_file"] = mk("write_file") // 写 → 串行
	a.allTools["code_search"] = mk("code_search")

	calls := []openai.ToolCall{
		tcOf("c0", "read_file"),
		tcOf("c1", "write_file"),
		tcOf("c2", "code_search"),
		tcOf("c3", "read_file"),
	}
	a.dispatchToolCalls(calls)

	msgs := a.session.Messages()
	if len(msgs) != len(calls) {
		t.Fatalf("应回灌 %d 条 tool 结果，实际 %d", len(calls), len(msgs))
	}
	for i, m := range msgs {
		if m.Role != "tool" {
			t.Fatalf("msg[%d] 角色应为 tool，实际 %s", i, m.Role)
		}
		if m.ToolCallID != calls[i].ID {
			t.Fatalf("msg[%d] 顺序错乱：ToolCallID=%s 期望 %s", i, m.ToolCallID, calls[i].ID)
		}
		if want := "R:" + calls[i].Function.Name; m.Content != want {
			t.Fatalf("msg[%d] 内容=%q 期望 %q", i, m.Content, want)
		}
	}
}

// 只读工具应真正并发：用屏障证明——若串行，只有 1 个会启动，永远收不齐 n 个信号。
func TestDispatchToolCalls_ReadOnlyRunInParallel(t *testing.T) {
	a := newDispatchAgent(t, ConfirmHighRiskGate{})
	const n = 4
	started := make(chan struct{}, n)
	release := make(chan struct{})
	a.allTools["read_file"] = tool.Tool{Name: "read_file", Execute: func(string) string {
		started <- struct{}{} // 报告「我已开始」
		<-release             // 卡住，直到测试放行
		return "ok"
	}}

	calls := make([]openai.ToolCall, n)
	for i := range calls {
		calls[i] = tcOf(fmt.Sprintf("c%d", i), "read_file")
	}

	done := make(chan struct{})
	go func() { a.dispatchToolCalls(calls); close(done) }()

	deadline := time.After(2 * time.Second)
	for i := 0; i < n; i++ {
		select {
		case <-started:
		case <-deadline:
			t.Fatalf("只读工具未并发执行：2 秒内仅 %d/%d 个启动", i, n)
		}
	}
	close(release)
	<-done
}

// 写 / 高危工具必须串行：峰值并发数应恒为 1。
func TestDispatchToolCalls_WritesRunSerially(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{}) // 全放行免 stdin；写工具仍应串行
	var active, maxActive int32
	a.allTools["write_file"] = tool.Tool{Name: "write_file", Execute: func(string) string {
		cur := atomic.AddInt32(&active, 1)
		for {
			m := atomic.LoadInt32(&maxActive)
			if cur <= m || atomic.CompareAndSwapInt32(&maxActive, m, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond) // 拉长窗口：若误并发，必被峰值捕获
		atomic.AddInt32(&active, -1)
		return "w"
	}}

	calls := []openai.ToolCall{tcOf("c0", "write_file"), tcOf("c1", "write_file"), tcOf("c2", "write_file")}
	a.dispatchToolCalls(calls)

	if maxActive != 1 {
		t.Fatalf("写工具应串行（同时最多 1 个），实际峰值并发=%d", maxActive)
	}
}

// canRunParallel：已注册的只读工具 + 权限门放行 才可并发；其余一律串行。
func TestCanRunParallel(t *testing.T) {
	a := newDispatchAgent(t, ConfirmHighRiskGate{})
	for _, name := range []string{"read_file", "code_search", "write_file", "bash", "git_commit"} {
		a.allTools[name] = tool.Tool{Name: name, Execute: func(string) string { return "" }}
	}

	cases := []struct {
		name string
		want bool
	}{
		{"read_file", true},
		{"code_search", true},
		{"write_file", false},   // 高危
		{"bash", false},    // 高危
		{"git_commit", false},   // 改仓库
		{"no_such_tool", false}, // 未注册
	}
	for _, c := range cases {
		if got := a.canRunParallel(tcOf("x", c.name)); got != c.want {
			t.Errorf("canRunParallel(%s)=%v 期望 %v", c.name, got, c.want)
		}
	}

	// 只读门下：读工具仍可并行，写工具被拒、不可并行。
	a.gate = ReadOnlyGate{}
	if !a.canRunParallel(tcOf("x", "read_file")) {
		t.Error("只读门下 read_file 应可并行")
	}
	if a.canRunParallel(tcOf("x", "write_file")) {
		t.Error("只读门下 write_file 应被拒、不可并行")
	}
}



