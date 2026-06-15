package agent

import "fmt"

// 事件驱动输出（TODO #10）：Agent 不再到处 fmt.Print / debugf 直接打控制台，而是把
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
	default:
		// EventToken / EventPrompt / EventNotice：原样输出，换行由调用方决定。
		fmt.Print(ev.Text)
	}
}
