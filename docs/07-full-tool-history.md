# 07 — 全量保留工具链（修复跨轮失忆）

## 为什么要做

之前 `Run()` 一轮结束时，只把「用户问题 + 最终回答」两条写进 `a.messages`，
中间的 assistant(tool_calls) 和每条 tool 结果全部丢弃——它们只活在一次请求的局部
变量 `reqMessages` 里，函数返回就没了。

后果是「跨轮失忆」：下一轮 LLM 看不到上一轮读过哪些文件、工具返回了什么，
于是会重复读同一个文件、重复劳动，多轮连续任务尤其明显。

参照 Reasonix 的做法——单一 append-only 消息日志，工具往返**全量保留、永不删消息**
（只在超限时对旧结果做 prune/compact）——我们也把工具链全量保留下来。

## 做了什么

### 1. `Run()` 改为全量保留

`agent/run.go` — `a.messages` 成为唯一事实源：用户输入、带 `tool_calls` 的 assistant
消息、每条 tool 结果，全部按顺序 append 进 `a.messages`。每轮从 `a.messages` 重建请求，
不再维护局部 `reqMessages`，也删掉了旧的「结束时补存 user + 最终 assistant」逻辑（避免重复）。

### 2. 新增 `buildRequestMessages()`

`agent/helpers.go` — 把 `a.messages` 组装成一次 LLM 请求，system 提示词单独前置、
**不进** `a.messages`。这样 `a.messages` 保持为纯对话历史（只含 user / assistant / tool），
便于裁剪与持久化。

### 3. 新增 `executeToolCall()`

`agent/run.go` — 把单个工具调用的执行抽出来。**不变量**：无论成功、未知工具、还是
被用户取消，都返回一段文本作为 tool 结果回灌给模型。因此每个 `tool_call` 必有配对的
tool 结果，历史始终合法；未知工具也从「直接中断整轮」变成「把错误喂回，让模型自我纠正」。

### 4. `trimMessages()` 改为配对感知

`agent/helpers.go` — 全量保留后，历史里会出现 assistant(tool_calls) 紧跟若干 tool 结果。
裁剪窗口**绝不能以孤立的 tool 消息开头**，否则它与对应的 `tool_call` 失联，
DeepSeek/OpenAI 接口会直接 400。新逻辑：保留最近 `maxMessages` 条；若窗口开头是 tool，
就继续向前丢弃，直到落在 user / assistant 上。顺手修掉旧逻辑「把 `messages[0]` 当 system
保留」的误解（system 在 `sysPrompt`，本就不在 `messages` 里）。

### 5. 单元测试

`agent/memory_test.go` — 新增：

- `TestTrimMessages_PreservesToolPairing`：构造 201 条带工具往返的历史，裁剪后校验
  「调用/结果」配对不被拆散、且不以孤立 tool 开头（先红后绿，正是这次的核心 bug）。
- `TestTrimMessages_NoopUnderLimit`：未超限不裁剪。
- `TestBuildRequestMessages_*`：system 前置 / 空 system 的请求组装行为。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/run.go` | `Run()` 改为全量保留；新增 `executeToolCall()` |
| `agent/helpers.go` | 新增 `buildRequestMessages()`；`trimMessages()` 改为配对感知 |
| `agent/memory_test.go` | 新增：裁剪配对 / 边界、请求组装的单元测试 |

## 效果

- **跨轮不再失忆**：下一轮能看到上一轮的工具上下文，减少重复读文件、重复劳动。
- **历史始终合法**：裁剪不会把 tool 结果与它的 tool_call 拆开，不会触发接口 400。
- **更稳健**：未知工具不再中断整轮，而是把错误喂回，让模型自我纠正。

## 相关 TODO

> TODO-reasonix.md — 待修问题 **P0**（跨轮丢失工具结果）
> 设计参照 Reasonix（全量保留 + 永不删消息）；体积控制（prune 原地 elide / compact 摘要）
> 见 #13 上下文压缩。
> 难度：★★★☆☆
