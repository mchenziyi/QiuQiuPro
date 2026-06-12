# 05 — Ask/Plan 双模式

## 现状（改动前的问题）

之前 QiuQiuPro 只有一套固定的"规划→执行"流程。无论用户输入什么，都强制走
GeneratePlan → ReviewPlan → ExecutePlan。

导致的问题是：用户说"你好"这种简单问候，GeneratePlan 也会把它当成项目任务，
拆出 6 步（识别关键词、搜索文件、列目录结构、读文件……），白白浪费大量 Token。
测试中一次"你好"产生了 1300 行不必要的输出。

## 做了什么

### 1. Agent 新增 Mode 字段

`agent/agent.go` — Agent 结构体新增 `Mode string`，默认值为 `"plan"`。
新增两个方法：
- `SetMode(mode string)` — 切换模式，只接受 `"ask"` 或 `"plan"`
- `CurrentMode() string` — 获取当前模式

### 2. 注册 `/mode` 命令

`main.go` — 新增 `/mode` 斜杠命令：
- `/mode` — 查看当前模式
- `/mode plan` — 切换到规划执行模式（走 Plan 流程）
- `/mode ask` — 切换到直接问答模式（走 Run 流程）

### 3. 提示符显示模式

- 启动时：`Skill：[architect] 模式：[plan]`
- 输入提示：`🧑 [PLAN] 你:` 或 `🧑 [ASK] 你:`
- 当前模式一目了然，不会忘记自己在哪里

### 4. 交互循环按模式分支

```go
switch a.CurrentMode() {
case "ask":
    // 直接 Run()，不走规划
    answer, err := a.Run(ctx, input)

case "plan":
    // 原有流程：GeneratePlan → ReviewPlan → ExecutePlan
}
```

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/agent.go` | Agent 结构体新增 `Mode` 字段 + `SetMode()` / `CurrentMode()` |
| `main.go` | 注册 `/mode` 命令 + 提示符显示模式 + ask/plan 分支 |

## 效果

- 简单聊天（"你好"、"今天天气怎么样"） → Ask 模式 → 直接回答，不浪费 Token
- 复杂任务（"加一个用户登录功能"） → Plan 模式 → 拆步骤执行
- 切换模式只需要 `/mode ask` 或 `/mode plan`，不需要重启

## 相关 TODO

> TODO-reasonix.md — 新需求（来自测试发现）
> 难度：★★☆☆☆
