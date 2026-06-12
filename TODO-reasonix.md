# QiuQiuPro 优化清单（读 Reasonix 源码对比）

> 读 Reasonix 时看到的好设计，逐一记下来后续优化到 QiuQiuPro。
> 每条注明出处（文件 + 行号）和当前 QiuQiuPro 的做法。

---

## Agent 核心

- [ ] **流式输出** — 替换 CreateChatCompletion 为流式，用户看到逐字输出而不是整段等待
  - Reasonix: internal/agent/agent.go:417 — prov.Stream() 流式输出
  - QiuQiuPro: gent/run.go:27 — client.CreateChatCompletion 阻塞等待

- [ ] **并行工具执行** — 只读工具（read_file、grep）并行跑，写工具保持串行
  - Reasonix: internal/agent/agent.go:507-553 — executeBatch + partitionToolCalls
  - QiuQiuPro: gent/run.go:56 — 全部串行

- [ ] **Session 独立管理** — messages 交给独立 Session 对象管理，而不是在 Run 循环里拼
  - Reasonix: internal/agent/session.go — Session 持久化 + 可恢复
  - QiuQiuPro: gent/run.go:14-23 — 每次在 reqMessages 里拼

- [ ] **可配 maxSteps + 暂停恢复** — maxSteps 到限制时返回"暂停"而非错误，可继续
  - Reasonix: internal/agent/agent.go:363 — maxSteps <= 0 为无限，超限返回 paused
  - QiuQiuPro: gent/run.go:25 — 硬编码 maxLoops=15，到了返回错误

## 输出与事件

- [ ] **事件驱动输出** — 用 Event/Sink 模式替代 debugf 打印，方便前端渲染
  - Reasonix: internal/event/ 包 + internal/agent/textsink.go
  - QiuQiuPro: gent/agent.go:137 — 用 debugf() 打印到终端

## 权限与安全

- [ ] **Plan Mode** — 只读模式下拦截写操作，让 LLM 先出方案再动手
  - Reasonix: internal/agent/agent.go:145 — planMode atomic.Bool
  - QiuQiuPro: 没有

- [ ] **Gate 接口** — 工具执行前可插拔权限检查，取代硬编码的 fmt.Scanln
  - Reasonix: internal/agent/agent.go:76-85 — Gate 接口
  - QiuQiuPro: gent/run.go:66 — mt.Scanln 弹窗确认

- [ ] **Hook 机制** — 工具执行前后可插拔，支持日志/通知等

## Provider 抽象

- [ ] **LLM Provider 接口** — 不绑死 OpenAI，支持多 Provider 切换
  - Reasonix: internal/provider/ 包
  - QiuQiuPro: gent/agent.go:13 — 直接 import openai client

## 上下文管理

- [ ] **上下文压缩** — 自动检测上下文长度，接近限制时压缩历史
  - Reasonix: internal/agent/compact.go — maybeCompact()
  - QiuQiuPro: 没有（messages 一直膨胀）

- [ ] **Token 用量追踪** — 记录每轮 Token 消耗，展示用量
  - Reasonix: internal/agent/agent.go:368 — Usage 事件

## 代码结构

- [ ] **拆分 agent.go** — 按职责分文件：executor/storm/types
  - Reasonix: internal/agent/agent.go 794 行（有合理拆分但不够）
  - QiuQiuPro: gent/agent.go 176 行 + un.go 103 行（结构上已经更好）

---

## 记忆层（讨论日期：2026-06-12）

- [ ] **长期记忆（偏好型）** — 对话结束后让 LLM 分析用户偏好/习惯并持久化存储，下次启动时自动注入 system prompt
  - 来源：读菜鸟教程工作原理页的讨论
  - 用户原话："用户个人喜好和工作偏好...肯定得由Agent去分析"
  - 参考做法：MemGPT/Letta 的记忆管理模块

- [ ] **长期记忆（知识型/RAG）** — 引入向量数据库（如 Chroma），支持文档分块存储、语义检索并注入上下文
  - 适用于：API 文档、公司规范、固定标准知识

## 感知层（讨论日期：2026-06-12）

- [ ] **感知层规范化** — 当前文本输入散落在 run.go 中，未来加图片/结构化输入时需要一个统一的感知入口

## 推理与规划（讨论日期：2026-06-12）

- [ ] **CoT（思维链）** — 在 system prompt 中要求 LLM 先展示推理过程再给出结论
  - 最低成本：在 system prompt 加一句 "Let's think step by step"
  - 适用场景：架构决策、代码审查、问题分析

- [ ] **Reflexion（自我反思）** — 在 ExecutePlan 失败时，先让 LLM 分析失败原因，再带着反思重新规划
  - 接入点：gent/plan.go:155-156 — 当前 RePlan 之前插入 Reflect 步骤
  - 替换当前"失败直接换方案"的做法为"先反思原因再换方案"
  - 需要明确的失败信号（编译报错、测试失败、API 报错）

## Skill 体系（讨论日期：2026-06-12）

- [ ] **Skill 作为人格切换器** — 不只是工具白名单+system prompt，而是让 Agent 在不同阶段扮演不同角色
  - 适用场景：单人全栈项目，通过 /skill 来回切换角色（pm → architect → backend → tester）
  - 相比多 Agent 架构的优势：上下文连续、零编排开销、同一份消息历史
  - 参考做法：当前 skill.go 已有人格切换的基础，但 Skill 种类和切换流畅度待加强
