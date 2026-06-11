# QiuQiuPro 优化清单（读 Reasonix 源码对比）

> 读 Reasonix 时看到的好设计，逐一记下来后续优化到 QiuQiuPro。
> 每条注明"出处"（文件 + 行号）和"当前 QiuQiuPro 的做法"。

---

## Agent 核心

- [ ] **流式输出** — 替换 CreateChatCompletion 为流式，用户看到逐字输出而不是整段等待
  - Reasonix: `internal/agent/agent.go:417` — `prov.Stream()` 流式输出
  - QiuQiuPro: `agent/run.go:27` — `client.CreateChatCompletion` 阻塞等待

- [ ] **并行工具执行** — 只读工具（read_file、grep）并行跑，写工具保持串行
  - Reasonix: `internal/agent/agent.go:507-553` — `executeBatch` + `partitionToolCalls`
  - QiuQiuPro: `agent/run.go:56` — 全部串行

- [ ] **Session 独立管理** — messages 交给独立 Session 对象管理，而不是在 Run 循环里拼
  - Reasonix: `internal/agent/session.go` — Session 持久化 + 可恢复
  - QiuQiuPro: `agent/run.go:14-23` — 每次在 reqMessages 里拼

- [ ] **可配 maxSteps + 暂停恢复** — maxSteps 到限制时返回"暂停"而非错误，可继续
  - Reasonix: `internal/agent/agent.go:363` — `maxSteps <= 0` 为无限，超限返回 paused
  - QiuQiuPro: `agent/run.go:25` — 硬编码 `maxLoops=15`，到了返回错误

## 输出与事件

- [ ] **事件驱动输出** — 用 Event/Sink 模式替代 debugf 打印，方便前端渲染
  - Reasonix: `internal/event/` 包 + `internal/agent/textsink.go`
  - QiuQiuPro: `agent/agent.go:137` — 用 `debugf()` 打印到终端

## 权限与安全

- [ ] **Plan Mode** — 只读模式下拦截写操作，让 LLM 先出方案再动手
  - Reasonix: `internal/agent/agent.go:145` — `planMode atomic.Bool`
  - QiuQiuPro: 没有

- [ ] **Gate 接口** — 工具执行前可插拔权限检查，取代硬编码的 fmt.Scanln
  - Reasonix: `internal/agent/agent.go:76-85` — Gate 接口
  - QiuQiuPro: `agent/run.go:66` — `fmt.Scanln` 弹窗确认

- [ ] **Hook 机制** — 工具执行前后可插拔，支持日志/通知等

## Provider 抽象

- [ ] **LLM Provider 接口** — 不绑死 OpenAI，支持多 Provider 切换
  - Reasonix: `internal/provider/` 包
  - QiuQiuPro: `agent/agent.go:13` — 直接 import openai client

## 上下文管理

- [ ] **上下文压缩** — 自动检测上下文长度，接近限制时压缩历史
  - Reasonix: `internal/agent/compact.go` — `maybeCompact()`
  - QiuQiuPro: 没有（messages 一直膨胀）

- [ ] **Token 用量追踪** — 记录每轮 Token 消耗，展示用量
  - Reasonix: `internal/agent/agent.go:368` — Usage 事件

## 代码结构

- [ ] **拆分 agent.go** — 按职责分文件：executor/storm/types
  - Reasonix: `internal/agent/agent.go` 794 行（有合理拆分但不够）
  - QiuQiuPro: `agent/agent.go` 176 行 + `run.go` 103 行（结构上已经更好）
