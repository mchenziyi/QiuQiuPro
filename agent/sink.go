package agent

import (
	"encoding/json"
	"fmt"
)

// 事件驱动输出：Agent 不再到处 fmt.Print / debugf 直接打控制台，而是把
// 运行过程中的「发生了什么」抽象成 Event 交给 Sink，由 Sink 决定「怎么呈现」——
// 控制台、JSON、上层 UI、或测试里的捕获。渲染逻辑从此与 Agent 解耦、可替换、可断言。

// EventKind 标识一条运行事件的语义类型。
type EventKind int

const (
	EventToken      EventKind = iota // assistant 流式增量文本（逐字、不换行）
	EventToolCall                    // 工具开始执行
	EventToolResult                  // 工具返回结果
	EventNotice                      // 流程 / 状态提示（自带 emoji 与换行）
	EventPrompt                      // 需要用户输入的提示（不换行）
	EventReasoning                   // 思考模式（thinking）的 reasoning 增量（逐字、不换行、灰显）
	EventConfirmRequest              // 需要用户确认高危操作
)

// Event 是 Agent 运行过程中产生的一条输出事件。
//
// Verbose=true 表示「细节日志」，安静模式（Agent.Quiet）下会被丢弃——等价于原 debugf 的语义。
// Name 仅工具类事件使用；Text 是事件正文（Notice/Prompt 由调用方自带 emoji 与换行，
// 工具类事件则交给 Sink 统一加 emoji 与换行）。
type Event struct {
	Kind    EventKind
	Name    string
	Text    string
	Verbose bool
	Extra   map[string]interface{} // 可选的结构化扩展数据（如 diff）
}

// Sink 接收 Agent 的输出事件并负责呈现。实现需对并发安全持保守态度，
// 但 Agent 仅在串行阶段 Emit（流式 token、工具调用前后），故无需自带锁。
type Sink interface {
	Emit(ev Event)
}

// ConsoleSink 把事件渲染到标准输出，等价于改造前散落各处的 fmt.Print/Printf。
// 工具调用 / 结果在这里统一加 emoji 前缀与换行；其余原样输出。
type ConsoleSink struct{}

func (ConsoleSink) Emit(ev Event) {
	switch ev.Kind {
	case EventToolCall:
		fmt.Printf("  🔧 %s(%s)\n", ev.Name, ev.Text)
	case EventToolResult:
		fmt.Printf("  📦 %s\n", ev.Text)
	case EventReasoning:
		// 思考链灰显，与最终答案在视觉上区分（ANSI dim/gray，逐片包裹）。
		fmt.Print("\033[90m" + ev.Text + "\033[0m")
	default:
		// EventToken / EventPrompt / EventNotice：原样输出，换行由调用方决定。
		fmt.Print(ev.Text)
	}
}

// ----- Agent 侧的事件发射 -----

// SetSink 替换输出去向（默认 ConsoleSink）。供上层 UI 或测试注入自定义渲染。
func (a *Agent) SetSink(s Sink) { a.sink = s }

// emit 把一条事件送往 Sink；细节日志（Verbose）在安静模式下丢弃。
// 不修改 a（无锁），故并发只读阶段调用也安全；Agent 实际仅在串行阶段 Emit。
func (a *Agent) emit(ev Event) {
	if ev.Verbose && a.Quiet {
		return
	}
	s := a.sink
	if s == nil {
		s = ConsoleSink{}
	}
	s.Emit(ev)
}

// debugf 细节日志：等价于原 debugf（安静模式隐藏），现统一走 Sink。
func (a *Agent) debugf(format string, args ...interface{}) {
	a.emit(Event{Kind: EventNotice, Text: fmt.Sprintf(format, args...), Verbose: true})
}

// noticef 常驻提示：始终呈现（不受安静模式影响），等价于原先直接 fmt.Printf 的那些行。
func (a *Agent) noticef(format string, args ...interface{}) {
	a.emit(Event{Kind: EventNotice, Text: fmt.Sprintf(format, args...)})
}

// emitToken 输出 assistant 流式增量（逐字、不换行）。
func (a *Agent) emitToken(text string) { a.emit(Event{Kind: EventToken, Text: text}) }

// emitReasoning 输出思考模式的 reasoning 增量（逐字、不换行）。标记为 Verbose：安静模式下
// 隐藏思考链（仍照常产出，只是不显示），非安静模式下灰显。
func (a *Agent) emitReasoning(text string) {
	a.emit(Event{Kind: EventReasoning, Text: text, Verbose: true})
}

// emitToolCall / emitToolResult 输出工具调用与结果（细节日志，由 Sink 统一加 emoji）。
func (a *Agent) emitToolCall(name, args string) {
	a.emit(Event{Kind: EventToolCall, Name: name, Text: args, Verbose: true})
}
func (a *Agent) emitToolResult(name, result string) {
	a.emit(Event{Kind: EventToolResult, Name: name, Text: result, Verbose: true})
}

// emitToolResultWithDiff 输出带结构化 diff 的工具结果。diffData 为前端可直接消费的 JSON 对象。
func (a *Agent) emitToolResultWithDiff(name, result string, diffData map[string]interface{}) {
	a.emit(Event{Kind: EventToolResult, Name: name, Text: result, Verbose: true, Extra: map[string]interface{}{"diff": diffData}})
}

// emitToolResultWithDiffIfJSON 检查 result 是否为含 diff 的 JSON，若是则拆分发出；否则走普通 emitToolResult。
func (a *Agent) emitToolResultWithDiffIfJSON(name, result string) {
	var wrapped struct {
		Text string                 `json:"text"`
		Diff map[string]interface{} `json:"diff"`
	}
	if err := json.Unmarshal([]byte(result), &wrapped); err == nil && wrapped.Text != "" && wrapped.Diff != nil {
		a.emitToolResultWithDiff(name, truncate(wrapped.Text, 100), wrapped.Diff)
		return
	}
	a.emitToolResult(name, truncate(result, 100))
}

// emitConfirmRequest 输出高危操作确认请求。SSE Sink 转为 confirm_request 事件，
// UI 收到后展示 Approve/Reject 按钮。
func (a *Agent) emitConfirmRequest(name, args, reason string) {
	a.emit(Event{Kind: EventConfirmRequest, Name: name, Text: args, Verbose: false})
	_ = reason
}

// emitPrompt 输出需要用户输入的提示（不换行）。
func (a *Agent) emitPrompt(text string) { a.emit(Event{Kind: EventPrompt, Text: text}) }
