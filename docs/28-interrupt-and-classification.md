# 28 — 中断机制（Ctrl+C 打断当前操作，会话继续）

## 为什么要做

通过思维链看到模型跑偏了，只能干等——没有中途打断的手段。Reasonix 和 Claude Code 都有 Ctrl+C 中断处理。

第一版实现错误地把 `cancel(ctx)` 当成中断——这会杀掉整个 context，导致会话终止，完全不符合预期。

## 做了什么

### 1. Agent 新增 `interrupt` channel

```go
type Agent struct {
    ...
    interrupt chan struct{}  // 用户按 Ctrl+C 时关闭，Run 循环检测到后优雅退出
}
```

每次 `Run()` 开始时重建 channel，上一轮的中断不影响下一轮。

### 2. Interrupt() 方法

关闭 channel，Run 循环下一轮检测到后：
1. 打印 `⚡ 已中断当前操作`
2. `SaveCheckpoint()` 保存当前状态
3. 返回 `interrupted` 错误
4. 主循环继续等待用户输入

### 3. 信号绑定

```go
// main.go
signal.Notify(sigCh, os.Interrupt)
go func() {
    for range sigCh {
        a.Interrupt()  // 只打断当前操作
    }
}()
```

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/agent.go` | 新增 `interrupt chan struct{}` 字段 |
| `agent/run.go` | Run 循环头检查 interrupt channel + Interrupt() 方法 |
| `main.go` | signal.Notify → a.Interrupt()（不再 cancel context） |

## 效果

- 按 Ctrl+C → 停当前工具/LLM调用 → 会话继续 → 可以告诉模型你的想法
- 中断后快照已保存，下次启动可恢复
- 每次 Run 独立中断 channel，互不影响
