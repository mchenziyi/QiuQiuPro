# 02 — 流式输出

## 为什么要做

之前 QiuQiuPro 使用 `CreateChatCompletion`（阻塞式），用户输入指令后要等 LLM 完全生成完才一次性显示。
改为流式输出后，LLM 一边生成一边输出，用户体验明显更好。

## 做了什么

1. **新增 `streamChat()` 方法**
   - `agent/run.go` → 用 `CreateChatCompletionStream` 替代 `CreateChatCompletion`
   - 文本内容实时 `fmt.Print()` 输出到终端
   - 工具调用（tool call）以流式方式下发，按 `Index` 字段积累拼合

2. **修改 `Run()` 方法**
   - `resp, err := a.client.CreateChatCompletion(...)` → `msg, err := a.streamChat(ctx, reqMessages)`
   - 其余流程不变（tool call 执行、checkpoint、循环逻辑）

3. **流式 tool call 积累**
   - OpenAI 流式 API 下，tool call 的 id/name/arguments 分多次下发
   - 用 `map[int]ToolCall` 按 `Index` 积累，流结束后拼成完整列表

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/run.go` | 新增 `streamChat()`，`Run()` 中调用流式接口代替阻塞接口 |

## 效果

- 用户输入指令后，LLM 的回答逐字显示，不再整段等待
- 对工具调用的行为无影响（积累逻辑透明）
- 整体交互体验明显提升

## 相关 TODO

> TODO-reasonix.md — 第一梯队第 2 项
> 难度：★★☆☆☆ | 耗时：40 分钟
