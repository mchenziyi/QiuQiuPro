package agent

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

// captureSink 收集所有事件，供测试断言（带锁，防并发写）。
type captureSink struct {
	mu     sync.Mutex
	events []Event
}

func (c *captureSink) Emit(ev Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

// 工具调用应作为结构化事件送达 Sink（而非直接打印），且带上工具名与结果。
func TestSink_DispatchEmitsToolEvents(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.Quiet = false // 放行细节事件（工具调用 / 结果）
	cs := &captureSink{}
	a.SetSink(cs)
	a.allTools["read_file"] = tool.Tool{Name: "read_file", Execute: func(ctx context.Context, args json.RawMessage) (string, error) { return "DATA", nil }}

	a.dispatchToolCalls([]openai.ToolCall{tcOf("c0", "read_file")})

	var call, result *Event
	for i := range cs.events {
		switch cs.events[i].Kind {
		case EventToolCall:
			call = &cs.events[i]
		case EventToolResult:
			result = &cs.events[i]
		}
	}
	if call == nil || call.Name != "read_file" {
		t.Fatalf("应有 read_file 的 ToolCall 事件，实际 %+v", cs.events)
	}
	if result == nil || result.Name != "read_file" || !strings.Contains(result.Text, "DATA") {
		t.Fatalf("应有含 DATA 的 ToolResult 事件，实际 %+v", cs.events)
	}
}

// 安静模式应丢弃细节事件（Verbose），但常驻提示（Notice）照常送达。
func TestSink_QuietSuppressesVerbose(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{}) // Quiet:true
	cs := &captureSink{}
	a.SetSink(cs)

	a.emitToolCall("read_file", "{}") // Verbose → 丢弃
	a.debugf("verbose\n")             // Verbose → 丢弃
	a.noticef("notice\n")             // 非 Verbose → 保留

	if len(cs.events) != 1 {
		t.Fatalf("安静模式应只放行 1 条非细节事件，实际 %d：%+v", len(cs.events), cs.events)
	}
	if cs.events[0].Kind != EventNotice || cs.events[0].Text != "notice\n" {
		t.Fatalf("放行的应是 notice，实际 %+v", cs.events[0])
	}
}

// 自定义 Sink 注入后，流式 token 应原样转交（不经控制台、可被上层接管）。
func TestSink_TokensRouteToSink(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.Quiet = false
	cs := &captureSink{}
	a.SetSink(cs)

	a.emitToken("Hello, ")
	a.emitToken("世界")

	var got strings.Builder
	for _, ev := range cs.events {
		if ev.Kind == EventToken {
			got.WriteString(ev.Text)
		}
	}
	if got.String() != "Hello, 世界" {
		t.Fatalf("流式 token 应原样拼接，实际 %q", got.String())
	}
}

// ConsoleSink 负责把工具调用渲染成带 🔧 前缀的整行——格式逻辑集中在 Sink。
func TestConsoleSink_RendersToolCall(t *testing.T) {
	out := captureStdout(t, func() {
		ConsoleSink{}.Emit(Event{Kind: EventToolCall, Name: "read_file", Text: `{"path":"a"}`})
	})
	if !strings.Contains(out, "🔧") || !strings.Contains(out, "read_file") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("ConsoleSink 应渲染带 🔧 的工具调用整行，实际 %q", out)
	}
}

// captureStdout 临时把 os.Stdout 重定向到管道，捕获 f 期间的标准输出。
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	data, _ := io.ReadAll(r)
	return string(data)
}