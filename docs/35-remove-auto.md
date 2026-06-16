# 35 — 取消 auto 模式，改为 ask/plan 手动切换

## 现状（before）

模式有三种：
- `ask` — 直接问答
- `plan` — GeneratePlan → ReviewPlan → ExecutePlan 三阶段
- `auto` — 启发式打分 + LLM 分类器自动判断 ask/plan

auto 的问题：
- 分类准确率不稳定，含 URL 的任务（"帮我看看github.com/..."）频频误判为 ask
- 每次输入都要等 LLM 分类器（耗时 1-3s）
- 相当于让模型替用户做决策，用户反而被动

## 改动后（after）

模式有两种：
- `ask` — 直接问答，不走规划。模型可以直接调用工具
- `plan` — 只读调研 → 方案审批 → 执行

### Plan 模式流程（Reasonix 风格）

```
用户输入 → SetPlanMode(true) → Run()（只读调研）
  → 模型用只读工具调研，输出方案文本
  → 展示方案给用户 → 用户批准？
    → Y: SetPlanMode(false) → Run("批准，执行吧") → 模型执行
    → N: 保持 plan 模式，用户修改输入
```

不是老的 GeneratePlan → ReviewPlan → ExecutePlan 三段式，而是让模型自己写方案（只读模式拒绝写工具），用户批准后再执行。

### 只读门控

当 `planMode=true` 时，`executeToolCall` 会拒绝所有非 ReadOnly 工具：
```go
if a.planMode.Load() && !t.ReadOnly {
    return "blocked: writer tool, plan mode is read-only"
}
```

## 删除了什么

| 文件 | 说明 |
|------|------|
| `agent/perception.go` | 整个自动分类层（autoPlanScore + DetectMode + classifyNeedsPlan + 词库） |
| `main.go` 的 auto 分支 | ~100 行分类 + 路由代码 |

## 新增

| 项 | 文件 |
|------|------|
| `planMode atomic.Bool` 字段 | `agent/agent.go` |
| `SetPlanMode(v bool)` 方法 | `agent/agent.go` |
| plan mode 门控（拒绝写工具） | `agent/run.go` executeToolCall |
| Plan 模式：调研→审批→执行 | `main.go` case "plan" |

## 效果

- 没有 auto 了，用户自己选 ask 还是 plan
- Plan 模式：调研→方案→批准→执行，比原来更直观
- 只读门控保证调研期间不会意外改代码
