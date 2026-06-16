# 26 — 感知层（自动判断 Ask/Plan 模式）

## 为什么要做

此前 Ask/Plan 模式依赖用户手动 `/mode ask` 或 `/mode plan` 切换。每次对话都要先想「这是个问题还是任务」，忘了切模式就会走错流程——明明是闲聊却走了一遍 Plan 的 6 步链，浪费 Token 和时间。

感知层的职责是让 Agent **自己判断用户意图**，一键都不需要用户按。

## 做了什么

### 1. 新增意图分类器（`agent/perception.go`）

`DetectMode(ctx, input)` 方法用一次极轻量的 LLM 调用做意图分类：

- **prompt 仅 ~80 token**：给 LLM 一条分类规则 + 用户输入，让它只输出 `ask` 或 `plan`
- **MaxTokens = 10**：最多吐 3-5 个 token，分类调用几乎零成本
- 失败时退化到 `plan`（安全侧——宁可多规划，不要漏任务）

### 2. 新增 auto 模式（默认）

- `Agent.Mode` 默认值从 `plan` → `auto`
- `/mode auto` 可随时切回自动模式
- 交互提示显示 `模式：[auto（智能判断）]`

### 3. Auto 分流

主循环新增 `case "auto"`：

```
DetectMode "ask" → 走 Ask 流程（一次 Run）
DetectMode "plan" → 走 Plan 流程（GeneratePlan → ReviewPlan → ExecutePlan）
```

### 4. 手动模式仍保留

`/mode ask` / `/mode plan` 依然有效，用户觉得 auto 不准时可以手动锁定。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/perception.go` | 新增：DetectMode + 分类 prompt |
| `agent/agent.go` | Mode 默认值 `plan` → `auto` |
| `agent/skill.go` | SetMode 校验增加 `auto` |
| `main.go` | 主循环新增 auto 分流 + /mode 命令更新 + 启动文案 |

## 效果

- 用户上来直接说「你好」→ 自动走 Ask，模型直接回复「你好」
- 用户说「帮我把 main.go 里的函数重构一下」→ 自动走 Plan，拆解步骤执行
- 一个分类调用约 100 token，成本可忽略
- 手动模式仍可用，用户可随时覆盖
