# 23 — Token 用量追踪

## 为什么要做

Agent 每轮都在烧 token，但此前无从得知一次任务花了多少、缓存命中如何、贵在输入还是输出。
\#13 把上下文压缩做成了「缓存友好」（命中价约未命中的 1/50），那就更需要把**缓存命中率**摆到台面上——
省没省到、值不值得，得有数字说话。本项目因此按 provider 回传的**真实 `usage`** 做用量追踪，
口径与账单一致，而非本地估算。

## 做了什么

### 1. 用量数据结构（`agent/usage.go`）

`TokenUsage` 累计：调用次数、输入 token、其中**缓存命中**、输出 token、其中**思考（reasoning）**、合计。

- `Add(openai.Usage)`：把一次调用的真实用量并入；明细指针为 nil（provider 未回传）时子项按 0 计。
- `AddUsage` / `Sub`：分别用于「子 Agent 用量并入父级」与「从会话累计切出本轮增量」。
- `MissTokens()`（未命中 = 输入 − 命中，兜底非负）/ `HitRate()`（命中率，不除零）。

数据来源是接口 `usage` 字段：`prompt_tokens` / `completion_tokens` / `total_tokens` +
`prompt_tokens_details.cached_tokens`（缓存命中）+ `completion_tokens_details.reasoning_tokens`（思考）。

### 2. 一处记账，覆盖所有 LLM 调用

`a.accountUsage(usage)` 是唯一记账口，所有耗 token 的调用都经它计入会话累计：

| 调用 | 位置 |
|------|------|
| 主循环流式对话 | `agent/run.go` streamChat（`StreamOptions.IncludeUsage` 末尾用量包）|
| 规划 / 审查 / 反思 / 重规划 | `agent/plan.go` 四处非流式调用 |
| 上下文压缩摘要 | `agent/compact.go` llmSummarize |

streamChat 原先只从末尾用量包取了 `prompt_tokens`（给 #13 压缩判定）；现在改为捕获**完整** `usage`，
既保留 `lastPromptTokens`，又把含缓存/思考的明细计入累计。

### 3. 展示：每轮摘要 + /usage 汇总

- **每轮 Run 结束**：`reportTurnUsage` 用「结束累计 − 进入基线」算出本轮增量，输出一行
  `📊 本轮 token｜输入 .. (缓存 ..) 输出 .. (思考 ..) 合计 ..`。标记为 Verbose——安静模式（`-q`）隐藏。
  用快照相减而非单设轮次字段，天然不会被规划等「轮外」调用污染。
- **`/usage` 命令**：`ReportUsage` 输出会话累计（含命中率），配置了单价时附带估算费用。

### 4. 可选费用估算（默认关）

`Pricing` 按**缓存命中 / 未命中 / 输出**三档单价（每 1M token）估算花费。三项经环境变量配置，
**默认全不配置**——价格随模型与时间变动，编造金额不如不显（token 数始终展示）。

```
DEEPSEEK_PRICE_INPUT=2          # 输入（未命中缓存），每 1M token
DEEPSEEK_PRICE_CACHE_HIT=0.2    # 输入（命中缓存）
DEEPSEEK_PRICE_OUTPUT=8         # 输出
```

> 单价请按 [DeepSeek 官方定价](https://api-docs.deepseek.com/zh-cn/quick_start/pricing) 填写校准。

### 5. 子 Agent 用量并入父级

`SpawnSubAgent` 在子任务结束后 `a.usage.AddUsage(sub.usage)`，让 `/usage` 反映整个会话（含子任务）。
子 Agent 串行执行，无并发写 `a.usage`。

## 测试（`agent/usage_test.go`）

| 用例 | 验证 |
|------|------|
| `TestTokenUsage_AddAccumulatesDetails` | 累加总量与明细；明细为 nil 时不炸 |
| `TestTokenUsage_MissTokensAndHitRate` | 未命中/命中率；负数与除零兜底 |
| `TestTokenUsage_AddUsageAndSub` | 合并与增量相减 |
| `TestPricing_EnabledAndCost` | 单价是否配置；三档费用计算 |
| `TestTokenUsage_Format` | 紧凑/会话两种文案；未配单价不显费用 |
| `TestStreamChat_AccountsUsage` | 流式末尾用量包被完整捕获、计入累计 |
| `TestRun_ReportsTurnUsage` / `...HiddenWhenQuiet` | 本轮摘要输出；安静模式隐藏 |
| `TestReportUsage` | `/usage` 汇总输出含费用 |
| `TestSpawnSubAgent_RollsUpUsage` | 子 Agent 用量并入父级 |

流式集成测试用 `httptest` 起一个吐 SSE 分片（含末尾 usage 包）的假服务端，全程无网络。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/usage.go` | 新增：`TokenUsage` / `Pricing` + 累加/子集/命中率/费用/文案 + Agent 侧记账与展示 |
| `agent/run.go` | streamChat 捕获完整 `usage` 并记账；Run 用快照相减输出本轮摘要 |
| `agent/plan.go` | 规划/审查/反思/重规划四处调用记账 |
| `agent/compact.go` | 摘要调用记账 |
| `agent/agent.go` | `Agent` 新增 `usage` / `pricing` 字段；`SpawnSubAgent` 用量并入父级 |
| `main.go` | 解析价格环境变量 + 注册 `/usage` 命令 |
| `agent/usage_test.go` | 纯函数 + 流式/Run/子 Agent 集成测试（无网络）|

## 效果

- `/usage` 一眼看清会话烧了多少 token、缓存命中率多少；配了单价还能看估算费用。
- 每轮结束自带一行用量摘要（安静模式隐藏），任务成本即时可见。
- 所有耗 token 路径（对话/规划/摘要/子任务）统一记账，口径与账单一致。
- `go build ./...` / `go vet ./agent/` / `go test ./agent/ -race` 全绿。
