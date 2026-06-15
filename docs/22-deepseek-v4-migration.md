# 22 — 迁移到 deepseek-v4-flash（开启 thinking + max）

## 为什么要做

DeepSeek 已发布 V4：`deepseek-v4-flash` / `deepseek-v4-pro`，默认 **1M 上下文**。旧模型名
`deepseek-chat` / `deepseek-reasoner` 将于 **2026-07-24 15:59 UTC 下线**。项目原先硬编码
`deepseek-chat`，需迁移到 V4。

V4 把旧的「chat vs reasoner」合并成**请求参数**：

- `thinking`：`{"thinking":{"type":"enabled|disabled"}}` —— 是否开启思考链（默认 enabled）。
- `reasoning_effort`：`high` / `max` —— 思考强度（思考开启时才有意义；默认 high）。

本项目希望用上 V4 的强推理，因此默认**开启 thinking + max**，并把思考链（reasoning）流式展示出来。

## 做了什么

### 1. 默认模型改为 deepseek-v4-flash（`main.go`）

```go
model := "deepseek-v4-flash"
if v := os.Getenv("DEEPSEEK_MODEL"); v != "" { model = v } // 可切 deepseek-v4-pro 等
a := agent.New(apiKey, model)
```

### 2. 开启思考并设强度（`agent/deepseek.go` + `agent/run.go`）

- **thinking 开关**走请求体顶层 `{"thinking":{"type":"enabled"}}`。go-openai v1.41.2 的类型化请求
  没有该字段，于是用一个 `http.RoundTripper` 装饰器 `bodyFieldInjector`，往出站的
  `/chat/completions` JSON 请求体里注入（流式/非流式都经此路径；不覆盖请求已显式设置的同名键；
  克隆请求并重置 Body/ContentLength/GetBody 以兼容重试）。显式注入而非依赖默认值，避免服务端默认变化。
- **reasoning_effort** 用 go-openai 自带的 `ReasoningEffort` 字段按请求设置（`run.go` 的 streamChat
  请求里设为 `a.reasoningEffort`，默认 `max`）。注意 go-openai 的 reasoning 校验只约束 o1/o3/o4/gpt-5
  前缀模型，DeepSeek 不受影响。

```go
config.HTTPClient = newDeepSeekHTTPClient(thinking) // thinking=true → 注入 enabled
// ...streamChat 请求：
ReasoningEffort: a.reasoningEffort, // "max"
```

### 3. 流式展示思考链（`agent/run.go` + `agent/sink.go`）

思考模式下，DeepSeek 会先流出 `reasoning_content`（思考链）再流出 `content`（最终答案）。
streamChat 分别累计：

- `reasoning_content` → 新事件 `EventReasoning`，`ConsoleSink` 以 ANSI 灰显，和最终答案在视觉上区分；
  标记为 Verbose（安静模式 `-quiet` 下隐藏思考链，但模型仍照常思考）。
- 思考链**不入历史**：DeepSeek 下一轮会忽略 `reasoning_content`，留着只会白占上下文。返回的
  assistant 消息只含最终 `content`（+ tool_calls）。

## 配置

| 变量 | 默认 | 说明 |
|------|------|------|
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | 模型名；可设 `deepseek-v4-pro` |
| `DEEPSEEK_REASONING_EFFORT` | `max` | 思考强度：`max` / `high` |
| `DEEPSEEK_THINKING` | `enabled` | 设 `disabled` 可关闭思考（沿用旧 deepseek-chat 的非思考、更省 token）|
| `DEEPSEEK_CONTEXT_WINDOW` | `1000000` | 压缩触发用的上下文窗口（见 docs/21）|

> 成本提示：thinking + max 会产生较多 reasoning 输出 token、延迟更高，但推理质量最强。
> 想省钱/提速可把 `DEEPSEEK_REASONING_EFFORT=high` 或 `DEEPSEEK_THINKING=disabled`。

## 测试（`agent/deepseek_test.go`）

| 用例 | 验证 |
|------|------|
| `BodyFieldInjector_InjectsOnChat` | chat 请求体被注入 thinking 字段，原字段保留 |
| `BodyFieldInjector_SkipsNonChat` | 非 chat 路径不被改写 |
| `BodyFieldInjector_DoesNotOverride` | 已显式设置的 thinking 不被覆盖 |
| `NewDeepSeekHTTPClient_ThinkingToggle` | thinking=true/false 分别注入 enabled/disabled |
| `DeepSeekThinkingConfig` | 默认 thinking+max；环境变量可覆盖 |

用假的下游 `RoundTripper` 捕获真正发出的请求体，全程无网络。

## 改动文件

| 文件 | 改动 |
|------|------|
| `main.go` | 默认模型 `deepseek-v4-flash` + `DEEPSEEK_MODEL` 环境变量 |
| `agent/deepseek.go` | `bodyFieldInjector`、`newDeepSeekHTTPClient(thinking)`、`deepSeekThinkingConfig`（默认 thinking+max）|
| `agent/agent.go` | `New` 按配置装配客户端 + `reasoningEffort` 字段；子 Agent 继承 |
| `agent/run.go` | streamChat 设 `ReasoningEffort` + 累计/灰显 `reasoning_content` |
| `agent/sink.go` | 新增 `EventReasoning` 事件与渲染、`emitReasoning` |
| `agent/deepseek_test.go` | 注入器与配置单测（无网络）|
| `README.md` / `STRUCTURES.md` | 文档同步 |

## 效果

- 迁移到不会下线的 V4 模型，默认拿到 1M 上下文。
- 默认开启 thinking + max，用满 V4 的强推理；思考链实时灰显、与答案区分，且不污染历史。
- thinking 开关与强度均可经环境变量调整（含一键关闭回到非思考、省 token）。
- `go build ./...` / `go vet ./agent/` / `go test ./agent/ -race` 全绿。
