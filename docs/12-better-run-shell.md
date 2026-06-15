# 12 — 更好的 run_shell（流式输出 + 退出码判定）

## 为什么要做

原 `run_shell` 用 `exec.Command(...).CombinedOutput()`，有几个硬伤：

1. **跑完才出结果**：耗时命令（`go test`、`npm install`）执行期间控制台一片空白，看不到进度。
2. **没有退出码**：只回 `命令失败：exit status 1`，模型难以稳定判断成败。
3. **没有超时**：命令一旦卡住（等输入、跑服务），整个 Agent 跟着永久阻塞。
4. **输出无上限**：海量输出会直接灌进上下文，撑爆窗口。

（TODO-reasonix.md 功能清单 #2，第一梯队、★★★☆☆。）

## 做了什么

把核心逻辑抽成 `runCommandStreaming(timeout, name, args...)`，`run_shell` / `run_powershell`
共用，一次解决四个问题：

### 1. 实时流式输出

```go
w := io.MultiWriter(os.Stdout, &cappedBuffer{buf: &buf, max: runShellCaptureMax})
cmd.Stdout = w
cmd.Stderr = w
```

`Stdout` 与 `Stderr` 指向**同一个 writer**——`os/exec` 会把两路合并到一条管道、由单个
goroutine 写入，天然串行化、无竞态。`MultiWriter` 一路实时打到控制台（边跑边看），一路写进
带上限的缓冲（回灌给 LLM）。

> 不会和 Agent 的结果回显重复：Agent 执行工具后只打印 `📦 <前 100 字符>` 预览，全量输出由工具自己流式呈现。

### 2. 退出码判定成败

```go
switch {
case ctx.Err() == context.DeadlineExceeded:  // ❌ 命令超时（超过 5m0s 被强制终止）
case err == nil:                              // ✅ 命令成功（退出码 0）
default:                                      // ❌ 命令失败（退出码 N） / 命令无法执行
}
```

用 `errors.As` 取 `*exec.ExitError` 拿到真实退出码，并区分「跑了但失败」与「根本没启动」
（命令不存在 / 权限不足）。结论以 ✅/❌ 开头，模型一眼判断成败。

### 3. 超时保护

`exec.CommandContext` + `context.WithTimeout`（默认 5 分钟），超时强制终止进程，不再把
整个 Agent 卡死。

### 4. 输出三重上限

- `cappedBuffer`：捕获缓冲最多 1MB（内存上限；控制台仍全量流式）；
- `formatShellOutput`：回灌给 LLM 的文本按 rune 截断到 16000 字符，空输出提示「（无输出）」。

### 5. 顺带

- 参数结构体补 `json:"command"` tag；
- 空命令直接返回「命令为空」，不再白跑一次空 shell。

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/shell_tools.go` | 重写 run_shell / run_powershell；新增 `runCommandStreaming` / `formatShellOutput` / `cappedBuffer` |
| `tool/shell_tools_test.go` | 新增 6 个单测 |

## 测试

| 用例 | 验证 |
|------|------|
| `TestRunCommandStreaming_Success` | echo → 含输出 + 退出码 0 + 成功 |
| `TestRunCommandStreaming_Failure` | `exit 7` → 退出码 7 + 失败 |
| `TestRunCommandStreaming_Timeout` | 200ms 超时 `sleep 3` → 判超时且 <2s 返回（Windows 跳过）|
| `TestRunCommandStreaming_StartError` | 不存在的命令 → 无法执行 |
| `TestFormatShellOutput` | 空 / 短 / 超长截断 |
| `TestCappedBuffer` | 超 max 截断且 Write 声明全量写入 |

## 效果

- 耗时命令边跑边看输出，不再"假死等待"。
- 模型能根据 ✅/❌ + 退出码稳定判断命令成败，呼应「退出码分析」诉求。
- 卡住的命令 5 分钟兜底终止；输出有内存与上下文双重上限。
- `go build` / `go test ./agent/ ./tool/` / `go vet` 全绿；tool 包测试数 10 → 16。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#2 更好的 run_shell**
> 难度：★★★☆☆
