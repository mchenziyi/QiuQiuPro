# 36 — plan 模式门控 + 单元测试

## 改动

### 1. Plan mode 门控（`agent/run.go`）

在 `executeToolCall` 的工具查找和 hooks 之间插入了只读门控：

```go
// Plan mode gate: 只读模式下拒绝写工具
if a.planMode.Load() && !t.ReadOnly {
    result := fmt.Sprintf("blocked: %q is a writer tool and plan mode is read-only...")
    return result
}
```

当 `planMode=true` 时：
- ReadOnly 工具（read_file, grep, ls 等）正常运行
- 非 ReadOnly 工具（write_file, bash 等）被拒绝，返回明确错误信息

### 2. 门控联动（`agent/agent.go`）

`SetMode("plan")` 自动调用 `SetPlanMode(true)`，`SetMode("ask")` 自动关闭。

### 3. 单元测试（`agent/plan_mode_test.go`）

新增 4 个测试：

| 测试 | 验证 |
|------|------|
| `TestPlanMode_BlocksWriteTools` | plan 模式拒绝写工具，放行读工具 |
| `TestPlanMode_Off_AllowsWrites` | plan 模式关闭时写工具正常执行 |
| `TestSetPlanMode_Toggles` | SetPlanMode 开关 state 正确 |
| `TestSetMode_SetsPlanMode` | SetMode 切换联动 planMode 正确 |

## 文件清单

| 文件 | 改动 |
|------|------|
| `agent/run.go` | 新增 plan mode 门控 |
| `agent/plan_mode_test.go` | 新增（4 个测试） |

## 验证

- `go build .` ✅
- `go test ./... -count=1` ✅（全部包）
- 新测试全绿
