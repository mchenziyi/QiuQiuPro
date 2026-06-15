# 19 — 事件驱动输出（Event / Sink）

## 为什么要做

Agent 的「输出」过去是直接写死的 `fmt.Print` / `fmt.Printf` / `debugf`，散落在 `run.go` /
`agent.go` / `plan.go` 十几处。问题：

- **耦合渲染**：流式 token、工具调用、状态提示全都硬绑标准输出，想接到上层 UI / 写成 JSON /
  在测试里断言「输出了什么」都无从下手——只能去截 `os.Stdout`；
- **格式分散**：`🔧` / `📦` 等前缀和换行重复散在各调用点，改一处样式要翻全仓；
- **`Quiet` 逻辑零散**：只有 `debugf` 受安静模式控制，其余 `fmt.Printf` 一律照打，规则不统一。

把「发生了什么」（事件）与「怎么呈现」（渲染）拆开，是 UI 化、结构化输出、可测性的前提。

（TODO-reasonix.md 功能清单 #10，第二梯队、★★★☆☆。）

## 做了什么

### 1. 定义 Event / Sink（`agent/sink.go`）

```go
type EventKind int
const (
	EventToken      // assistant 流式增量（逐字、不换行）
	EventToolCall   // 工具开始执行
	EventToolResult // 工具返回结果
	EventNotice     // 流程 / 状态提示（自带 emoji 与换行）
	EventPrompt     // 需要用户输入的提示（不换行）
)

type Event struct { Kind EventKind; Name, Text string; Verbose bool }

type Sink interface { Emit(ev Event) }
```

`ConsoleSink` 是默认实现，等价于改造前的控制台输出：工具调用 / 结果在这里**统一**加 `🔧` /
`📦` 前缀与换行，其余原样输出。换一个 Sink 就能把同一批事件渲染到别处。

### 2. Agent 只 Emit，不再直接打印（`agent/agent.go`）

新增 `sink Sink` 字段（`New` 默认 `ConsoleSink{}`、`SpawnSubAgent` 继承父级、`SetSink` 可注入），
并提供一组语义化发射器：

| 方法 | 事件 | 安静模式 |
|------|------|----------|
| `emitToken(s)` | 流式 token | 照常 |
| `emitToolCall(name,args)` / `emitToolResult(name,result)` | 工具调用 / 结果 | **隐藏**（Verbose）|
| `emitPrompt(s)` | 确认提示（不换行）| 照常 |
| `noticef(...)` | 常驻状态提示 | 照常 |
| `debugf(...)` | 细节日志（保留旧名）| **隐藏**（Verbose）|

`Quiet` 的过滤集中到唯一入口 `emit()`：`Verbose && Quiet` 即丢弃。规则从此统一——
原来「`debugf` 受 `Quiet`、`fmt.Printf` 不受」的差异被显式建模成每条事件的 `Verbose` 标志。

### 3. 全部打印点改走 Sink

- `run.go`：工具调用 / 结果 → `emitToolCall/Result`；拒绝 / 取消 → `noticef`；确认提示 →
  `emitPrompt`；流式增量与收尾换行 → `emitToken`。
- `agent.go`：切换 Skill / 模式、快照恢复 → `noticef`。
- `plan.go`：审查通过 / 解析失败 / 优化、步骤失败、反思 → `noticef`（细节进度仍走 `debugf`）。

改造后 `agent` 包内**唯一**的 `fmt.Print*` 只剩 `ConsoleSink`——渲染真正单点化。

## 行为是否改变

不变。`ConsoleSink` 逐条对齐了原输出（同样的 emoji、同样的换行、同样的安静模式可见性）。
唯一显式收敛：所有「细节日志 vs 常驻提示」的安静模式行为，现在由 `Verbose` 标志统一决定，
与改造前各调用点的实际表现一致。

## 测试（`agent/sink_test.go`）

| 用例 | 验证 |
|------|------|
| `DispatchEmitsToolEvents` | 工具调用走结构化事件（带工具名 + 结果），而非直接打印 |
| `QuietSuppressesVerbose` | 安静模式丢弃 Verbose 事件、放行 Notice |
| `TokensRouteToSink` | 流式 token 原样转交自定义 Sink，可被上层接管 |
| `RendersToolCall` | `ConsoleSink` 把工具调用渲染成带 🔧 的整行（截 `os.Stdout` 断言）|

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/sink.go` | 新增：Event / EventKind / Sink / ConsoleSink |
| `agent/agent.go` | 新增 `sink` 字段 + `SetSink` + `emit/noticef/emitToken/emitToolCall/emitToolResult/emitPrompt`；`debugf` 改走 Sink；切 Skill/模式/恢复改 `noticef`；子 Agent 继承 Sink |
| `agent/run.go` | 工具调用 / 结果 / 拒绝 / 确认 / 流式 token 全改走 Sink |
| `agent/plan.go` | 审查 / 失败 / 反思等提示改 `noticef` |
| `agent/sink_test.go` | 新增：事件流 / 安静过滤 / token 转交 / 控制台渲染 四组测试 |

## 效果

- 输出可插拔：注入自定义 Sink 即可把运行事件接到 UI、写成 JSON、或在测试里捕获断言。
- 渲染单点化：emoji 与换行集中在 `ConsoleSink`，改样式只动一处。
- 安静模式规则统一：由每条事件的 `Verbose` 显式决定，不再各处各表。
- `go build ./...` / `go vet` / `go test ./agent/ -race` 全绿。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#10 事件驱动输出**
> 难度：★★★☆☆
