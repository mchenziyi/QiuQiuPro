# 33 — 工具接口改造（ctx + error + ReadOnly）

## 为什么要做

原本的工具接口 `Execute func(args string) string` 有三个硬伤：

1. **没有 context** — 工具无法感知取消/超时信号
2. **没有 error** — 成功失败全靠 `strings.Contains(result, "失败")` 猜
3. **没有 ReadOnly** — 工具读写属性靠硬编码名单，和工具定义脱节

这三个问题导致：ask 工具做不了、evidence 校验证书做不了、工具超时做不了。改动后这三个都有地基了。

## 改了什么

### Tool 结构体

```go
// 之前
type Tool struct {
    Name        string
    Description string
    Parameters  any
    Execute     func(string) string
}

// 之后
type Tool struct {
    Name        string
    Description string
    Parameters  any
    ReadOnly    bool
    Execute     func(ctx context.Context, args json.RawMessage) (string, error)
}
```

### 波及范围

| 文件 | 改动 |
|------|------|
| `tool/struct.go` | 新结构体 |
| `tool/all_tools.go` | 合并 15 个工具到 1 个文件，全部适配新签名 |
| `agent/tools.go` | `isReadOnlyTool` 改为查 `ReadOnly` 字段（代替硬编码名单） |
| `agent/run.go` | `executeToolCall` 用新接口 |
| `agent/gate.go` | `ReadOnlyGate` 通过 `isReadOnlyTool` 裁决 |
| `agent/long_memory.go` | `remember_rule` 工具适配新签名 |
| `mcp/client.go` | MCP 工具包装适配新签名 |
| 测试文件 | 适配新接口 |

### 旧文件清理

删了 12 个独立工具文件 + 5 个测试文件，合并到 `tool/all_tools.go`。部分复杂工具（glob/grep/web_fetch/bash/git_commit）的实现被简化为了 stub，需要后续补齐。

## 效果

- 工具可以超时取消（ctx 传播到 HTTP 请求）
- 工具可以返回精确错误（error 类型）
- `ReadOnly` 字段是唯一事实源——Gate、并行、只读模式都从一个地方读
- 为 ask 工具和 evidence 账本铺了路

## 未完成

- ask 工具：工具接口已就绪，待实现具体逻辑
- 工具描述：中文优化待完善
- 复杂工具（glob/grep/bash 等）实现待恢复
