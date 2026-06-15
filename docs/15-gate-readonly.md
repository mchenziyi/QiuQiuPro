# 15 — Gate 权限门 + 只读模式（#5 + #6 合并）

## 为什么合并做

`#5 Gate 接口` 与 `#6 只读模式` 落在**同一个挂载点**：工具执行前的那道关卡。过去这道关卡
硬编码在 `executeToolCall` 里——「是高危工具就提示确认」，写死、不可配，也无法表达「只读模式
拒绝写操作」。把它抽成可插拔的 `Gate` 接口后，只读模式不过是「换一个拒绝写工具的 Gate」，
一个接口同时兑现两件事。

> 关键前提（已核实）：plan 模式的 `ExecutePlan` 每步都走 `a.Run` → `executeToolCall`，
> 与 ask 模式同一条路径。所以 Gate 放在 `executeToolCall` 这唯一咽喉，对两种模式都生效。

## 做了什么

### 1. Gate 接口与三个实现（`agent/gate.go`）

```go
type Decision int
const ( GateAllow Decision = iota; GateConfirm; GateDeny )

type Gate interface {
	Check(toolName, args string) (Decision, string) // 裁决 + 原因
	Name() string
}
```

| 门 | 行为 |
|----|------|
| `ConfirmHighRiskGate`（默认）| 高危工具 → 确认；其余放行。**等价于改造前的行为** |
| `ReadOnlyGate` | 改动类工具（高危集合 + `git_commit`）→ 拒绝；只读工具放行 |
| `AllowAllGate` | 全放行，适合自动化 / 测试 |

### 2. 接入唯一咽喉（`agent/run.go`）

`executeToolCall` 改为向 Gate 问裁决：

```go
switch decision, reason := gate.Check(name, args); decision {
case GateDeny:    // 回灌拒绝说明，引导模型改用只读手段，而非中断整轮
case GateConfirm: // 走统一输入读取器确认（沿用 P1 的 confirm）
}
```

`GateDeny` 也会**回灌一条结果**（"已拒绝…请改用 read_file / grep / code_search"），让模型据此
自我纠正——和 P0「每个 tool_call 都要有配对结果」的不变量一致。

### 3. Agent 持有可切换的门（`agent/agent.go`）

- 新增 `gate Gate` 字段，`New()` 默认 `ConfirmHighRiskGate{}`；`gate==nil` 时安全回退。
- `SetGate` / `SetReadOnly(bool)` / `IsReadOnly()` / `GateName()`。
- 子 Agent 继承父级的门（只读父 → 只读子）。

### 4. /readonly 命令 + 提示标识（`main.go`）

- `/readonly on|off`（无参显示当前状态）。
- 主循环提示在只读时加 🔒 前缀：`🧑 [🔒PLAN] 你:`。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/gate.go` | 新增：Gate 接口 + 三个实现 |
| `agent/run.go` | `executeToolCall` 改走 Gate 裁决（Allow/Confirm/Deny）|
| `agent/agent.go` | gate 字段 + 默认值 + 4 个方法 + 子 Agent 继承 |
| `main.go` | `/readonly` 命令 + 只读提示标识 |
| `agent/gate_test.go` | 新增 8 个单测 |

## 测试

| 用例 | 验证 |
|------|------|
| `TestGateCheck` | 三门 × 多工具的裁决矩阵（含 git_commit 在默认门放行、只读门拒绝）|
| `TestExecuteToolCall_ReadOnlyDeniesWrite` | 只读下写工具不执行、回灌「已拒绝」|
| `TestExecuteToolCall_ReadOnlyAllowsRead` | 只读下读工具照常执行 |
| `TestExecuteToolCall_AllowAllRunsWrite` | 全放行门直接执行 |
| `TestExecuteToolCall_ConfirmYes / ConfirmNo` | 确认 y 执行 / n 取消并回灌 |
| `TestExecuteToolCall_UnknownTool` | 未知工具提示 |
| `TestReadOnlyToggle` | 开关切换 + GateName + nil 回退 |

## 效果

- 权限判断从硬编码变成可插拔的 Gate，行为默认不变（兼容旧体验）。
- `/readonly on` 一键进入「只看不动」：写文件 / 编辑 / 跑命令 / 提交全部被拒，读类工具照常——
  适合让 Agent 先调研、出方案，再放开权限动手。
- Gate 作用于 ask 与 plan 两种模式的同一咽喉；子 Agent 自动继承。
- `go build` / `go test ./agent/ ./tool/ ./cleanup/` / `go vet` 全绿；agent 包新增 8 个单测。

## 已知边界

- `ReadOnlyGate` 按内置工具名判定改动类操作；**MCP 外部写工具无法自动识别**（需要工具自带
  读 / 写元数据，属更大改动）。如需严格隔离，可在只读模式下配合 Skill 工具白名单收窄可用工具。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#5 Gate 接口** + **#6 Plan Mode（只读模式）**
> 难度：★★★☆☆ / ★★☆☆☆（合并交付）
