# 22 — 迁移到 deepseek-v4-flash（并关闭默认 thinking）

## 为什么要做

DeepSeek 已发布 V4：`deepseek-v4-flash` / `deepseek-v4-pro`，默认 **1M 上下文**。旧模型名
`deepseek-chat` / `deepseek-reasoner` 将于 **2026-07-24 15:59 UTC 下线**（过渡期内分别路由到
v4-flash 的非思考 / 思考模式）。项目原先硬编码 `deepseek-chat`，需迁移到 v4。

## 一个容易踩的坑：V4 默认开启 thinking

V4 把旧的「chat vs reasoner」合并成一个**请求参数**，且 **thinking 默认开启**：

- 旧 `deepseek-chat` = **非思考**：不产 reasoning，最快最省。
- 直接把模型名换成 `deepseek-v4-flash`、不带任何标志 = **thinking(high)**：会先产一段 reasoning，
  多花输出 token、更慢，并返回 `reasoning_content`。

也就是说，**裸换模型名会悄悄打开思考模式、推高成本与延迟**——这与本项目（自带 CoT 提示词、
刚做完成本/缓存优化）的目标相悖。因此迁移时要**显式关闭 thinking**，沿用非思考行为。

关闭开关是请求体顶层的 `{"thinking":{"type":"disabled"}}`（OpenAI 格式）。

## 做了什么

### 1. 默认模型改为 deepseek-v4-flash（`main.go`）

```go
model := "deepseek-v4-flash"
if v := os.Getenv("DEEPSEEK_MODEL"); v != "" { model = v } // 可切 deepseek-v4-pro 等
a := agent.New(apiKey, model)
```

### 2. 在传输层关闭 thinking（`agent/deepseek.go`）

go-openai v1.41.2 的 `ChatCompletionRequest` 没有表达 `thinking` 的字段（只有 `reasoning_effort`，
无法用于关闭）。于是用一个 `http.RoundTripper` 装饰器 `bodyFieldInjector`，往出站的
`/chat/completions` JSON 请求体里注入 `{"thinking":{"type":"disabled"}}`：

- 只改 `chat/completions` 路径，**流式与非流式都经此路径**，一处生效（主 Agent、子 Agent、摘要调用全覆盖）。
- 不覆盖请求已显式设置的同名键（便于将来按需开启思考）。
- 克隆请求并重置 `Body / ContentLength / GetBody`，兼容重试与重定向。

`agent.New` 里把它装进客户端：

```go
config.HTTPClient = newDeepSeekHTTPClient() // 关闭 V4 默认 thinking 模式
```

## 配置

| 变量 | 默认 | 说明 |
|------|------|------|
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | 模型名；可设 `deepseek-v4-pro` 等 |
| `DEEPSEEK_CONTEXT_WINDOW` | `1000000` | 压缩触发用的上下文窗口（见 docs/21）|

> 想开启思考模式：目前默认关闭。后续可加 `DEEPSEEK_THINKING` 开关——关闭注入并配
> `reasoning_effort=high/max`（V4 思考档位）。

## 测试（`agent/deepseek_test.go`）

| 用例 | 验证 |
|------|------|
| `BodyFieldInjector_InjectsOnChat` | chat 请求体被注入 thinking=disabled，原字段保留 |
| `BodyFieldInjector_SkipsNonChat` | 非 chat 路径不被改写 |
| `BodyFieldInjector_DoesNotOverride` | 已显式设置的 thinking 不被覆盖 |
| `NewDeepSeekHTTPClient_DisablesThinking` | 默认客户端装配了关闭 thinking 的传输 |

用假的下游 `RoundTripper` 捕获真正发出的请求体，全程无网络。

## 改动文件

| 文件 | 改动 |
|------|------|
| `main.go` | 默认模型 `deepseek-v4-flash` + `DEEPSEEK_MODEL` 环境变量 |
| `agent/deepseek.go` | 新增 `bodyFieldInjector` 与 `newDeepSeekHTTPClient`（关闭 thinking）|
| `agent/agent.go` | `New` 用 `newDeepSeekHTTPClient()` 装配客户端 |
| `agent/deepseek_test.go` | 4 个注入器单测（无网络）|
| `README.md` / `STRUCTURES.md` | 文档同步 |

## 效果

- 迁移到不会下线的 V4 模型，默认拿到 1M 上下文。
- 显式关闭 thinking，行为与成本与旧 `deepseek-chat` 一致，不被默认思考模式悄悄加价。
- 模型可经环境变量切换；将来开启思考也有清晰路径。
- `go build ./...` / `go vet ./agent/` / `go test ./agent/ -race` 全绿。
