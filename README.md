# 🏀 QiuQiuPro — Agent 即插即用框架

> **从零手写 Agent 系统的实战产物。不是框架，不是 SDK，是一个你能完全读懂的 Agent。**
>
> 基于 Go 实现，核心代码不到 2000 行，每个函数都有中文注释。

---

## 它能做什么

```
你：给 main.go 的 main 函数加一行日志
QiuQiuPro：读取文件 → 找到函数位置 → 精确插入代码 → go build 验证 → git commit
          如果编译失败 → 自动回滚
```

```
你：分析我的项目结构，看看安全风险
QiuQiuPro：先拆成步骤 → 每步独立执行 → 拆完自己检查 Plan 质量
          如果某步失败了 → 自动重新规划剩余步骤继续
```

---

## 快速开始

### 前置条件

- Go 1.22+
- DeepSeek API Key（或其他兼容 OpenAI 接口的模型）

### 启动

```bash
git clone https://github.com/mchenziyi/QiuQiuPro.git
cd QiuQiuPro
go run main.go
```

**首次启动：** 在终端输入你的 DeepSeek API Key，会自动保存到 `~/.qiuqiu/key`，下次不用再输。

### 配置 MCP 工具

编辑 `~/.qiuqiu/mcp_servers.json`：

```json
[
  {"name": "filesystem", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]}
]
```

### 安装外部 Skill

放一个 `.json` 文件到 `~/.qiuqiu/skills/`：

```json
{
  "name": "debug_expert",
  "description": "Debug 专家模式 — 定位 Bug、分析根因",
  "system_prompt": "你是一个 Debug 专家。\n定位问题时你必须：\n1. 先复现问题\n2. 分析根因\n3. 修复并验证",
  "tool_whitelist": ["read_file", "search_files", "run_powershell"],
  "rules": [
    {"name": "先复现", "description": "修改代码前必须先确认能复现问题"}
  ]
}
```

---

## 命令

| 命令 | 作用 |
|------|------|
| `exit` / `quit` | 退出 |
| `replay` | 回放当前会话的事件日志 |
| `use <skill>` | 切换 Skill（如 `use architect`） |

---

## 项目结构

```
D:\QiuQiuPro\
├── main.go                    ← 入口（API Key + MCP 加载 + Skill 加载 + 对话循环）
│
├── agent/                     ← Agent 核心（4 个文件）
│   ├── agent.go               ← 结构体 + 注册 + Skill 切换
│   ├── run.go                 ← Agent 核心循环（调 LLM → 执行 Tool → 再调 LLM）
│   ├── plan.go                ← 拆步骤 + 自我审视 + 执行 + 动态重规划
│   └── helpers.go             ← 辅助函数
│
├── tool/                      ← 工具（5 个文件，7 个内置工具）
│   ├── struct.go              ← Tool 结构体定义
│   ├── file_tools.go          ← read / write / list / count
│   ├── edit_tools.go          ← edit_block + search_files
│   ├── shell_tools.go         ← run_shell + run_powershell
│   └── git_tools.go           ← git_commit
│
├── event/store.go             ← Event Sourcing（JSON Lines）
├── mcp/client.go              ← MCP 协议客户端
├── skill/skill.go             ← Skill 定义 + 内置 + 外部加载
├── .gitignore / go.mod / go.sum
└── OPTIMIZATION_SUMMARY.md    ← 优化过程记录
```

---

## 架构一览

```text
用户输入
  ↓
拆步骤（GeneratePlan）
  ↓
自我审视（ReviewPlan）→ 有问题自动修正
  ↓
按顺序执行（ExecutePlan）
  ├── 某步执行（Run）
  │   ├── 调 LLM
  │   ├── 有 ToolCall → 执行工具（内置 or MCP）→ 结果喂回 → 继续
  │   └── 没 ToolCall → 返回答案
  ├── 成功 → 下一步
  └── 失败 → 重规划（RePlan）→ 替换剩余步骤 → 继续
  ↓
截断历史（TrimMessages）
```

---

## 核心概念

| 概念 | 说明 |
|------|------|
| **Agent** | LLM + Tool + Memory + Planning 的循环执行系统 |
| **Tool** | Agent 能调用的函数（内置 7 个 + 任意 MCP 外部工具） |
| **MCP** | 工具即插即用协议（Model Context Protocol） |
| **Skill** | 人格切换卡 = SystemPrompt + 工具白名单 + 规则 |
| **Plan** | 复杂任务拆成步骤，每步独立执行 |
| **Event Log** | 每步操作记录为不可变事件（JSON Lines），支持重放 |

---

## 内置工具（7 个）

| 工具 | 用途 |
|------|------|
| `read_file` | 读取文件内容 |
| `write_file` | 写入文件 |
| `list_directory` | 列出目录内容 |
| `edit_file_block` | 精确替换代码块（找不到/找到多处就拒绝） |
| `search_files` | 按文件名或内容搜索 |
| `count_file_chars` | 统计文件字符数 |
| `git_commit` | 提交所有变更到 Git |
| `run_powershell` | 执行 PowerShell 命令（Windows 推荐） |
| `run_shell` | 执行 cmd 命令（兜底，不推荐） |

---

## 内置 Skill（3 个）

| Skill | 适合场景 | 可用工具 |
|-------|---------|---------|
| `architect` | 系统设计、技术选型、架构评审 | 只读工具 |
| `code_review` | 代码审查、安全审计、质量检查 | 读 + 编辑 |
| `frontend_design` | UI 设计、组件库开发、前端架构 | 读 + 写 |

---

## 技术栈

| 模块 | 选型 |
|------|------|
| 语言 | Go 1.22+ |
| LLM SDK | `go-openai`（兼容 DeepSeek） |
| MCP 协议 | `mcp-go` |
| 事件存储 | JSON Lines（`.jsonl`） |
| 用户配置 | `~/.qiuqiu/` 目录 |

---

## 学习路线

如果你是从零开始想学 Agent 开发，推荐的学习顺序：

```
Phase 0  名词扫盲 → 认全 Agent / Tool / Memory / Planning / Skill
  ↓
V0   Agent Loop  → 手写 LLM + Tool 循环
  ↓
V1   Planning    → 拆步骤、按顺序执行
  ↓
V2   Coding      → 精确编辑文件、Git 管理
  ↓
V3   Runtime     → Event Log、崩溃恢复
  ↓
V4   MCP         → 外部工具即插即用
  ↓
V5   Skill       → 人格切换
  ↓
优化  QiuQiuPro  → search_files / ReviewPlan / RePlan / 配置化等
```

完整的 6 个月学习规划见 [qiuqiu-agent-plan](https://github.com/mchenziyi/qiuqiu-agent-plan)。

---

## 与 Reasonix / Claude Code 的对应

| 概念 | 球球 | Reasonix | Claude Code |
|------|------|----------|-------------|
| Agent Loop | `Run()` | Agent Loop | Plan→Execute→Verify |
| 工具系统 | `tool/` | Tool Registry | Tool Use |
| 事件日志 | `event/store.go` | Event Log (JSON Lines) | SQLite |
| MCP | `mcp/client.go` | MCP 集成 | MCP 集成 |
| Skill | `skill/skill.go` | Skill 系统 | Behavior 配置 |

---

## License

MIT
