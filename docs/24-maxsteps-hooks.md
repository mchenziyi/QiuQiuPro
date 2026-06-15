# 24 — MaxSteps + 协作式暂停恢复 + Tool Hooks

## 为什么要做

长任务如果一次性跑到底，容易失控：步骤太多、成本不可预期、出错后不好接着跑。项目已有
Session checkpoint，但它只保存消息历史，不保存「计划执行到第几步」。因此 #15 补了执行层状态：
达到步数上限或用户请求暂停后，保存 Plan 状态，后续 `/resume` 从下一步继续。

#16 则把工具执行前后抽成 Hook 扩展点，后续可以接审计、脱敏、统计、策略限制等能力，而不再把这些逻辑
塞进 `executeToolCall`。

## #15 做了什么

### 1. 执行状态快照

新增 `ExecutionState`，保存：

- `Goal`：原始目标
- `Steps`：计划步骤与状态
- `NextStepIndex`：恢复时从哪一步继续
- `Status`：`running` / `paused`
- `PauseReason`：`user` / `maxSteps`
- `UpdatedAt`：更新时间

持久化使用 `.reasonix/sessions/<session>.exec.json` sidecar 文件。`event.Store` 只负责读写字节，具体结构留在
`agent` 包，避免包循环。Session checkpoint 仍只保存消息历史。

### 2. maxSteps

`Agent.maxSteps` 控制一次连续执行最多完成多少个 Plan step：

- `DEEPSEEK_MAX_STEPS=3`：启动时默认限制 3 步
- `/maxsteps 3`：运行中设置
- `/maxsteps 0`：关闭限制

达到上限时，当前 step 已完成，Agent 保存 paused 状态并返回 `ErrPlanPaused`。主循环把它视为正常暂停，
不会当作失败打印。

### 3. 协作式暂停恢复

`/pause` 调用 `RequestPause()`，语义是**协作式暂停**：不强杀正在执行的 LLM / 工具调用，而是在当前 step
完成后停下并保存状态。

`/resume` 调用 `ResumePlan(ctx)`：

1. 读取 paused 状态
2. 重建 `Plan`
3. 从 `NextStepIndex` 继续执行
4. 全部完成后清理执行状态

无可恢复状态时返回友好错误：`没有可恢复的暂停计划`。

## #16 做了什么

新增 `ToolHook`：

```go
type ToolHook interface {
	BeforeToolCall(ctx ToolHookContext) (ToolHookDecision, string)
	AfterToolCall(ctx ToolHookContext, result string) string
}
```

行为：

- `BeforeToolCall` 在 Gate 之前执行，可放行或拒绝。
- 拒绝时不执行真实工具，但会返回一段合法 tool result 给模型，保持 tool_call/tool_result 配对。
- `AfterToolCall` 在真实工具执行后执行，可观察或改写结果。
- 默认没有 hook，行为不变。
- 子 Agent 继承父级 hook 链。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/execution_state.go` | 新增执行状态模型、保存/读取/清理、`SetMaxSteps`、`RequestPause`、`ResumePlan` |
| `event/store.go` | 新增执行状态 sidecar 文件读写 |
| `agent/plan.go` | `ExecutePlan` 拆成可续跑的 `executePlanFrom`，接入 maxSteps 与暂停检查 |
| `agent/hooks.go` | 新增 ToolHook 接口与 hook 链执行 |
| `agent/run.go` | `executeToolCall` 接入 Before/After hooks |
| `agent/agent.go` | 新增 maxSteps / pauseRequested / toolHooks 字段，子 Agent 继承 maxSteps 与 hooks |
| `main.go` | 新增 `DEEPSEEK_MAX_STEPS`、`/maxsteps`、`/pause`、`/resume` |
| `agent/execution_state_test.go` | 执行状态、maxSteps、pause、resume 测试 |
| `agent/hooks_test.go` | Before/After hook、拒绝、Gate 顺序测试 |

## 测试

- `TestExecutionState_RoundTrip`：执行状态保存、读取、清理
- `TestExecutePlan_PausesAtMaxStepsAndResumeContinues`：达到 maxSteps 暂停，`/resume` 续跑剩余步骤
- `TestExecutePlan_CooperativePauseAfterCurrentStep`：请求暂停后完成当前 step 再停
- `TestResumePlan_NoPausedState`：无 paused 状态时友好错误
- `TestToolHook_BeforeAndAfterWrapExecution`：Before/After 包裹真实工具执行，After 可改写结果
- `TestToolHook_BeforeCanDenyAndPreserveToolResult`：Before 拒绝时不执行工具，仍回灌合法结果
- `TestToolHook_RunsBeforeGate`：Hook 先于 Gate 执行，便于自定义策略优先生效

## 边界

- 本次不做强中断：不会强杀正在运行的 LLM 请求、shell 命令或工具调用。
- `maxSteps` 限制的是 Plan step 数，不是 `Run()` 内部的 tool-call 循环；`Run()` 内部仍保留原先的
  `maxLoops := 15`。
- Hook 范围先覆盖工具前后，不扩展到 LLM/Plan/Run 全生命周期。
