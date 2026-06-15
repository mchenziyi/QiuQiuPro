# 21 — 上下文压缩（超限时让 LLM 总结旧消息）

## 为什么要做

自 P0「全量保留工具链」后，历史是 append-only、永不丢。体积控制只有一个钝器 `Trim()`：
超过 `maxMessages`（100）就**直接砍掉**最早的消息。问题是——砍掉的往往是「用户最初的目标、
早期查明的关键事实、读过哪些文件」，正是后续推理最需要的上下文。一轮 plan 跑十几步、读几个
大文件就可能触顶，一砍就「失忆」。

更好的做法：超限时不丢，而是让 LLM 把旧消息**总结成摘要**，用摘要顶替原文，既压住体积又保住
信息。这正是 P0 文档里留给本条的「compact 摘要」。

（TODO-reasonix.md 功能清单 #13，第三梯队、★★★★☆。）

## 做了什么

### 1. Session 的压缩原语（`agent/session.go`，纯函数、可单测）

| 方法 | 作用 |
|------|------|
| `NeedsCompaction()` | 历史是否超过 `maxMessages`（与 Trim 同阈值）|
| `SplitForCompaction()` | 切成「待摘要的旧消息 old」+「保留的近消息 recent」；recent 取末尾约 `maxMessages/2` 条，**配对感知**地跳过开头的孤立 tool |
| `ApplyCompaction(summary, recent)` | 用 `[摘要 user 消息] + recent` 替换历史；摘要为空则只留 recent（等价裁剪）|

切分的关键是 recent 不能以孤立 `tool` 开头——否则它和对应的 `tool_call` 失联，接口直接 400。
摘要消息是一条无 `tool_calls` 的 `user` 消息，不会引入新的配对问题。

### 2. Agent 的编排（`agent/compact.go`）

```
maybeCompact(ctx):
  未超限 → 直接返回
  切出 old / recent
  summary, err = summarizer(old)        // 默认走 LLM
  摘要失败/为空 → 退化为 Trim()（仍兜住体积，不阻断主流程）
  否则 → ApplyCompaction(summary, recent)
```

- **可注入的摘要器**：Agent 持有 `summarizer summarizeFunc` 字段，默认 `llmSummarize`（非流式
  调一次 LLM）。抽成函数缝是为了**可测**——测试注入桩函数即可验证整条压缩链路，无需联网。
- `renderForSummary` 把消息渲染成带【用户】/【助手】/【工具结果】标记的纯文本喂给摘要 LLM，
  工具结果按 500 字截断防止过长。
- 摘要系统提示词明确要求保留「目标 / 约束 / 关键事实 / 读改过的文件 / 未完成事项」，丢弃寒暄。

### 3. 接入主循环（`agent/run.go`）

在 `Run` 循环顶部、每次组装请求前调用 `a.maybeCompact(ctx)`：一轮里连续工具调用使历史膨胀时，
能在下一次请求前及时压缩，避免超出上下文窗口。子 Agent 各自的 Session 独立压缩。

## 与既有机制的关系

- `Trim()` 保留为兜底：摘要失败时退化使用，行为与改造前一致。
- 触发阈值仍是消息条数 `maxMessages`（沿用既有约定）。按 token / 字符预算触发更精确，但需引入
  分词器，留作后续优化。
- 压缩会重写历史、打断前缀缓存——但只在超限时偶发，是「保住上下文 vs 缓存命中」的合理取舍。

## 测试（`agent/compact_test.go`）

| 用例 | 验证 |
|------|------|
| `NeedsCompaction` | 等于上限不触发、超过才触发 |
| `SplitForCompaction_PairAware` | 构造 cut 落在孤立 tool 的布局，保留段跳过它、配对合法、old+recent 可还原 |
| `ApplyCompaction` | 前置摘要 user 消息 + 近消息；空摘要只留近消息 |
| `RenderForSummary` | 渲染含用户 / 助手 / 工具调用 / 工具结果标记 |
| `MaybeCompact_SummarizesWhenOverLimit` | 注入桩摘要器，超限后历史变为「摘要 + 近消息」、不超上限 |
| `MaybeCompact_FallsBackToTrimOnError` | 摘要失败退化为裁剪，兜住体积且不留摘要消息 |
| `MaybeCompact_NoopUnderLimit` | 未超限不调摘要器、历史不变 |

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/session.go` | 新增 `NeedsCompaction` / `SplitForCompaction` / `ApplyCompaction` |
| `agent/compact.go` | 新增：`maybeCompact` 编排、可注入 `summarizer`、`llmSummarize`、`renderForSummary` |
| `agent/agent.go` | 新增 `summarizer` 字段，`New` 默认接 `llmSummarize` |
| `agent/run.go` | `Run` 循环顶部调用 `maybeCompact` |
| `agent/compact_test.go` | 新增 7 个测试，覆盖切分 / 替换 / 编排 / 退化 |

## 效果

- 超限时保留关键上下文（目标 / 事实 / 文件 / 待办）而非粗暴丢弃，长任务不易「失忆」。
- 摘要器可注入：测试无需联网即可全链路验证；将来也可换更强的摘要策略。
- 摘要失败安全退化为裁剪，不阻断主流程。
- `go build ./...` / `go vet` / `go test ./agent/ -race` 全绿。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#13 上下文压缩**
> 难度：★★★★☆
