# 21 — 上下文压缩（按窗口占比触发，对前缀缓存友好）

## 为什么要做

自 P0「全量保留工具链」后，历史是 append-only、永不丢。体积控制只有一个钝器 `Trim()`：
超过 `maxMessages`（100）就**直接砍掉**最早的消息。砍掉的往往是「用户最初的目标、早期查明的
关键事实、读过哪些文件」，正是后续推理最需要的上下文。一轮 plan 跑十几步、读几个大文件就可能
触顶，一砍就「失忆」。

更好的做法：超限时不丢，而是让 LLM 把旧消息**总结成摘要**，用摘要顶替原文，既压住体积又保住信息。

（TODO-reasonix.md 功能清单 #13，第三梯队、★★★★☆。）

## 压缩时机：为什么用「窗口占比」而非「消息条数」

DeepSeek 等按**前缀匹配**做上下文缓存：请求间共享的前缀算缓存命中，命中价仅约未命中的
**1/50**（V4-Flash：命中 \$0.0028/M vs 未命中 \$0.14/M）。这带来一条硬约束：

- **维持 append-only 时**（第 N+1 次请求 = 第 N 次 + 尾部新增），前缀 `[system + 历史]` 不变 →
  前缀缓存全程命中。
- **压缩那一刻**，最早若干条被改写成一条摘要，前缀整体变化 → 这一次请求**整段 cache miss**。
  之后新前缀固定下来、缓存重新焐热，直到下次压缩。

所以成本 ∝ **压缩触发的频率**。最初实现用「消息条数 > 100」触发：消息一短，100 条可能才几 K
token，离窗口还很远就压，白白打断缓存——这正是「乱压缩」。

**改法（对齐 Reasonix）**：按**占模型上下文窗口的比例**触发，且用 **provider 回传的真实
`prompt_tokens`** 判定（比字符估算精确）。DeepSeek V4 默认窗口已是 **1M**，于是平时几乎压不到、
缓存常热，只有真接近窗口时才兜底压一次。

| 比例（占窗口） | 行为 |
|----------------|------|
| `softCompactRatio` = 0.5 | 提醒一次「上下文在涨，仍保前缀缓存」，**不压缩** |
| `compactRatio` = 0.8 | 触发压缩 |
| `<` 0.5 | 回落，重置一次性提醒 |

## 做了什么

### 1. 真实用量遥测（`agent/run.go`）

`streamChat` 的流式请求加 `StreamOptions{IncludeUsage: true}`，DeepSeek 会在末尾补一个
`choices` 为空、只带 `usage` 的尾包。捕获其中的 `PromptTokens` 写入 `a.lastPromptTokens`，
作为下一轮压缩判定的依据（注意：用量尾包要在 `len(choices)==0 → continue` **之前**读取）。

### 2. 触发与编排（`agent/compact.go`）

```
maybeCompact(ctx):                      // Run 循环每轮调用
  窗口未配置 / 无用量遥测 → 返回
  prompt_tokens ≥ 窗口*0.8 → compact(自动)
  prompt_tokens ≥ 窗口*0.5 → 提醒一次（不压缩，保缓存）
  否则 → 重置一次性提醒

compact(ctx, manual):
  tailBudget = min(窗口/4, 16384) token
  old, recent = SplitForCompaction(tailBudget, tokPerChar)   // 按 token 切尾部
  old 为空 → 返回（没什么可折叠）
  summary, err = summarizer(old)         // 默认走 LLM
  失败/为空 → 退化 Trim()，清零遥测
  否则 → ApplyCompaction(summary, recent)，清零遥测（前缀已变，避免下轮用旧值再压）

Compact(ctx):                           // 手动 /compact：无视比例阈值，强制压一次
```

- **`tokPerChar`**：用「上一轮真实 `prompt_tokens` / 当前历史字符数」推导每字符 token 数，让按
  字符的估算贴合 provider 分词器，无需本地分词；用量未知或比值离谱（>2 或 <0.05）时兜底
  ~4 字符/token。
- **可注入的摘要器**：`summarizer summarizeFunc`，默认 `llmSummarize`（非流式调一次 LLM）。抽成
  函数缝是为了**可测**——测试注入桩函数即可验证整条链路，无需联网。
- `renderForSummary` 把消息渲染成带【用户】/【助手】/【工具结果】标记的纯文本，工具结果按 500
  字截断；系统提示词要求保留「目标 / 约束 / 关键事实 / 读改过的文件 / 未完成事项」，丢弃寒暄。

### 3. Session 的压缩原语（`agent/session.go`，纯函数、可单测）

| 方法 | 作用 |
|------|------|
| `CharCount()` | 历史全部消息的字符数之和，供 `tokPerChar` 推导比值 |
| `SplitForCompaction(tailBudget, tokPerChar)` | 从末尾按 token **倒着累加**到 `tailBudget` 切出保留段（至少 `minRecentKeep`=2 条），其余作 old 待摘要；**配对感知**地跳过开头的孤立 tool |
| `ApplyCompaction(summary, recent)` | 用 `[摘要 user 消息] + recent` 替换历史；摘要为空则只留 recent（等价裁剪）|

保留段不能以孤立 `tool` 开头——否则它和对应 `tool_call` 失联、接口直接 400。摘要消息是一条无
`tool_calls` 的 `user` 消息，不引入新的配对问题。

### 4. 配置入口

- `Agent.SetContextWindow(tokens)`：设窗口（token）；`<=0` 关闭自动压缩。**切到更小窗口的模型时
  务必调小**，否则触发线会高于真实窗口、压缩前就先超限报错。
- 环境变量 `DEEPSEEK_CONTEXT_WINDOW` 可覆盖默认值（`main.go` 读取）。默认 `1_000_000` 贴合
  DeepSeek V4。
- 命令 `/compact`：手动触发一次压缩，在前缀自然填满前主动重置、把握缓存重建时机。

## 与既有机制的关系

- `Trim()` 保留为兜底：摘要失败时退化使用，仍按 `maxMessages` 兜住条数。
- 压缩必然改写前缀、打断缓存，无法避免；策略是**尽量晚压**（贴近窗口才压）+ 压完保持 append-only，
  把缓存命中拉满、把打断次数降到最低。
- 子 Agent 继承窗口与比例配置，长子任务同样能兜底压缩。

## 测试（`agent/compact_test.go`）

| 用例 | 验证 |
|------|------|
| `Session_CharCount` | 字符统计含 content + 工具名/参数 |
| `SplitForCompaction_PairAware` | 按 token 切尾部、cut 落在孤立 tool 时向前跳过、old+recent 可还原 |
| `ApplyCompaction` | 前置摘要 user 消息 + 近消息；空摘要只留近消息 |
| `RenderForSummary` | 渲染含用户 / 助手 / 工具调用 / 工具结果标记 |
| `TokPerChar` | 无用量兜底、有用量推导、比值离谱兜底 |
| `MaybeCompact_SummarizesOverTrigger` | `prompt_tokens` 越过 0.8 触发线 → 摘要替换、清零遥测 |
| `MaybeCompact_SoftNoticeNoCompaction` | 0.5~0.8 之间只提醒一次、不压缩 |
| `MaybeCompact_FallsBackToTrimOnError` | 摘要失败退化为裁剪，兜住体积且不留摘要消息 |
| `MaybeCompact_NoopUnderTrigger` | 未达软线不压、历史不变 |
| `MaybeCompact_NoUsageNoop` | 无用量遥测（首轮）即便条数多也不压 |
| `Compact_ManualForces` | 手动 `/compact` 无视阈值强制压一次 |

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/run.go` | `streamChat` 开 `IncludeUsage` 并捕获 `prompt_tokens` → `lastPromptTokens` |
| `agent/compact.go` | 比例触发 `maybeCompact`（soft/compact）、手动 `Compact`、`compact`、`tokPerChar`、`SetContextWindow`、阈值常量 |
| `agent/session.go` | `CharCount`、`msgChars`、token 预算版 `SplitForCompaction`（移除按条数的 `NeedsCompaction`）|
| `agent/agent.go` | 新增 `contextWindow/compactRatio/softCompactRatio/lastPromptTokens/softCompactNoticed`，`New` 默认 + 子 Agent 继承 |
| `main.go` | 注册 `/compact` 命令、`DEEPSEEK_CONTEXT_WINDOW` 环境变量 |
| `agent/compact_test.go` | 11 个测试，覆盖触发 / 软线 / 手动 / 退化 / token 切分 / 系数 |

## 效果

- 平时维持 append-only，前缀缓存命中拉满；只有提示真逼近窗口（默认 0.8）才压一次，把缓存打断降到最低。
- 判定基于 provider 回传的真实 token，比字符估算精确；窗口可经环境变量/接口按模型调整。
- 压缩时保留关键上下文（目标 / 事实 / 文件 / 待办）而非粗暴丢弃；摘要失败安全退化为裁剪。
- 提供 `/compact` 让用户主动把握压缩时机。
- `go build ./...` / `go vet ./agent/` / `go test ./agent/ -race` 全绿。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#13 上下文压缩**
> 难度：★★★★☆
