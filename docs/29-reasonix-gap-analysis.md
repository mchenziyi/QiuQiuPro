# 29 — Reasonix vs QiuQiuPro 差距分析

## Date: 2025-06-16

## 代码规模对比

| | Reasonix | QiuQiuPro |
|---|---------|-----------|
| Agent 引擎行数 | 2,296 | 2,206 |
| 测试行数 | 3,256 | 1,540 |
| 非测试文件数 | 10 | 19 |

体量相同但花了不同的钱——Reasonix 花在**基础设施**上，QiuQiuPro 花在**功能特性**上。

## 关键架构差异

| 维度 | Reasonix | QiuQiuPro |
|------|----------|-----------|
| Provider | `provider.Provider` 接口（可换模型） | `openai.Client` 写死 |
| 工具接口 | `Execute(ctx, args) (string, error)` | `func(args string) string` |
| 事件系统 | 11 种事件（Phase, Compaction, Usage...） | 6 种事件 |
| 规划方式 | 隐式（todo_write）或 Coordinator 双模型 | 显式 Plan → Execute → Reflect → RePlan |
| 子 Agent | 独立 Session + 嵌套事件 + 过滤 Registry | 浅拷贝共享 client/tools |
| 并行工具 | 按批次分组 + 信号量上限 8 | 每工具一个 goroutine |
| 记忆 | turn-tail queue（不破坏 cache prefix） | system prompt 直接注入 |

## 差距明细

| # | 差距 | 影响 | 优先级 |
|---|------|------|--------|
| 1 | 提示词质量 | system prompt 是身份声明，不是行为指令 | 🥇 |
| 2 | 工具接口无 ctx/error | 不能超时取消，错误靠字符串 guess | 🥇 |
| 3 | 无输出截断 | read_file 可能灌爆上下文 | 🥇 |
| 4 | 无 Evidence 账本 | 模型可以编造"已完成" | 🥈 |
| 5 | 子 Agent 事件扁平 | 输出混乱 | 🥈 |
| 6 | 无 Compaction 归档 | 旧消息丢了没法追溯 | 🥈 |
| 7 | Hook 扩展点少 | 缺 PreCompact/SubagentStop/PostLLMCall | 🥈 |
| 8 | 缺 finish_reason 处理 | 模型被截断了都不知道 | 🥉 |

## QiuQiuPro 独有优势

- 结构化 Plan/Execute/Reflect/RePlan 流程
- ExecutionState 暂停恢复
- 感知层自动 Ask/Plan 分类
- MemoryStore 长期记忆
- DeepSeek thinking 模式适配
- XML 模板化提示词（可热更新）
