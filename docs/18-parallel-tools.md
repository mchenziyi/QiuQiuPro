# 18 — 并行工具执行（只读并发、写串行）

## 为什么要做

一轮里 LLM 常一次性发起多个工具调用——比如「读 3 个文件 + 搜 1 个符号」。过去 `Run()`
用一个 `for` 把它们**逐个**执行：第二个文件要等第一个读完才开始，纯属串行等待。这些读操作
彼此独立、又都耗在 I/O 上，并发跑能把一轮的总耗时从「N 次之和」压到「最慢的那一次」。

但不能无脑全并发：写文件会互相覆盖、`run_shell` 的实时输出会串台、需要 `stdin` 确认的高危
操作会互相抢输入。所以策略是**按工具性质分流**。

（TODO-reasonix.md 功能清单 #9，第二梯队、★★★☆☆。）

## 做了什么

### 1. 抽出 `dispatchToolCalls`（`agent/run.go`）

把原本写在 `Run` 循环里的「执行所有 tool_call」整段抽成一个方法，分四步：

1. **记录调用事件**（串行、有序）——审计日志保持可读；
2. **并发启动只读工具**——每个 goroutine 只写自己那格 `results[i]`，无共享写；
3. **串行执行写 / 高危 / 需确认 / 未知工具**——与第 2 步的并发读在时间上重叠（读在后台跑，
   主协程同时处理写）；
4. **按原始顺序回灌结果**（串行）——记录结果事件、写入会话历史、按节奏存档。

第 4 步严格按 `toolCalls` 原始顺序 append，保证「assistant(tool_calls) ↔ tool 结果」配对不乱
（乱序会触发接口的配对校验报错）。

### 2. 并发安全判定 `canRunParallel`

一个工具调用**当且仅当**满足三条才并发：

| 条件 | 原因 |
|------|------|
| 已注册（`allTools` 里有）| 未知工具走串行路径回灌「未知工具」错误 |
| `isReadOnlyTool(name)` | 写 / 高危串行，避开文件写竞争与流式输出错乱 |
| 权限门裁决 = `GateAllow` | 不需 `stdin` 确认、也未被拒绝——并发组绝不碰 `stdin` |

第三条很关键：`AllowAllGate` 会放行写工具，但 `isReadOnlyTool` 仍把它挡在并发组外；反过来，
即便某个自定义门对读工具要确认 / 拒绝，也会落到串行路径正确处理。

### 3. 复用「只读」分类，避免漂移（`agent/agent.go` + `gate.go`）

新增谓词 `isReadOnlyTool(name) = !IsHighRiskTool(name) && name != "git_commit"`，并让
`ReadOnlyGate.Check` 也改用它。这样「哪些工具算改动类」只有一处定义——以后往
`highRiskTools` 加新工具，只读门与并发判定会**一起**跟上，不会一个改了另一个忘了。

## 并发安全

- `executeToolCall` 在 `GateAllow` 路径下**不打印、不读 stdin**（确认 / 拒绝的输出都在
  Confirm/Deny 分支，而并发组已被 `canRunParallel` 限定为 Allow），故并发组无终端串台。
- `results` 切片预分配，各 goroutine 写**不同下标**，主协程在 `wg.Wait()` 后才统一读取——
  无数据竞争。
- `a.session` / `a.store` / `stdin` 只在串行阶段（第 1、3、4 步）触碰。
- 测试以 `-race` 运行，无告警。

## 测试（`agent/parallel_test.go`）

| 用例 | 验证 |
|------|------|
| `PreservesOrder` | 混合读写后，结果按原始顺序、按 ID 配对回灌 |
| `ReadOnlyRunInParallel` | 屏障法证明 n 个读工具**同时**启动（串行则收不齐信号 → 超时失败）|
| `WritesRunSerially` | 原子计数证明写工具峰值并发恒为 1 |
| `CanRunParallel` | 读→可并发；写 / git_commit / 未知→串行；只读门下读仍可并发 |

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/run.go` | `Run` 改调 `dispatchToolCalls`；新增 `dispatchToolCalls` + `canRunParallel` |
| `agent/agent.go` | 新增 `isReadOnlyTool` 谓词 |
| `agent/gate.go` | `ReadOnlyGate.Check` 复用 `isReadOnlyTool`（分类单点化）|
| `agent/parallel_test.go` | 新增：顺序 / 并发读 / 串行写 / 判定 四组测试 |

## 效果

- 多个只读工具一轮内并发执行，I/O 等待重叠，明显缩短「批量读取」场景的耗时。
- 写 / 高危 / 确认类操作保持串行，安全性与既有交互（确认提示、流式输出）完全不变。
- 结果回灌顺序与配对零变化，对模型而言行为一致。
- `go build ./...` / `go vet` / `go test ./agent/ -race` 全绿。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#9 并行工具执行**
> 难度：★★★☆☆
