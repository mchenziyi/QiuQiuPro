# <img src="qiuqiu-icon.svg" width="32" height="32" style="vertical-align: middle;"> QiuQiuPro — Agent 即插即用框架

> **从零手写 Agent 系统的实战产物。不是框架，不是 SDK，是一个你能完全读懂的 Agent。**
>
> 基于 Go 实现，核心代码不到 2500 行，每个函数都有中文注释。

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

- Go 1.25.5+
- DeepSeek API Key（或其他兼容 OpenAI 接口的模型）

### 启动

```bash
git clone https://github.com/mchenziyi/QiuQiuPro.git
cd QiuQiuPro
go run main.go
```

**首次启动：** 在终端输入你的 DeepSeek API Key，会自动保存到 `~/.qiuqiu/key`，下次不用再输。

### 环境变量（可选）

| 变量 | 默认 | 说明 |
|------|------|------|
| `DEEPSEEK_API_KEY` | — | API Key（也可首次启动时手动输入）|
| `DEEPSEEK_MODEL` | `deepseek-v4-flash` | 模型名；可设为 `deepseek-v4-pro` 等。旧 `deepseek-chat` 将于 2026-07-24 下线 |
| `DEEPSEEK_REASONING_EFFORT` | `max` | 思考强度：`max` / `high` |
| `DEEPSEEK_THINKING` | `enabled` | 设 `disabled` 关闭思考模式（更省 token、更快）|
| `DEEPSEEK_CONTEXT_WINDOW` | `1000000` | 上下文窗口（token），用于自动压缩的触发判定；切到更小窗口的模型时务必调小 |
| `DEEPSEEK_MAX_STEPS` | `0` | 单次连续计划执行的 step 上限；`0` 表示不限制，达到上限会暂停，可 `/resume` 继续 |
| `DEEPSEEK_PRICE_INPUT` | — | 输入（未命中缓存）单价，每 1M token；配置后 `/usage` 展示估算费用 |
| `DEEPSEEK_PRICE_CACHE_HIT` | — | 输入（命中缓存）单价，每 1M token |
| `DEEPSEEK_PRICE_OUTPUT` | — | 输出单价，每 1M token |

> 注：默认开启 V4 的 thinking（思考模式）+ max 强度，推理最强但更费 token、更慢；思考链会实时灰显、与最终答案区分。想省钱/提速可设 `DEEPSEEK_REASONING_EFFORT=high` 或 `DEEPSEEK_THINKING=disabled`。
>
> 价格三项默认不配置（不显示费用）——单价随模型与时间变动，请按 [DeepSeek 官方定价](https://api-docs.deepseek.com/zh-cn/quick_start/pricing) 自行填入校准；`/usage` 的 token 数始终展示。

### 安装 MCP 工具

可以直接在对话里告诉 QiuQiuPro：

```text
帮我安装 @modelcontextprotocol/server-filesystem 这个 MCP，并把当前目录作为参数
```

Agent 会调用 `install_mcp` 工具，在确认后写入 `~/.qiuqiu/mcp_servers.json`、立即连接 MCP Server、
发现工具并注册到当前会话，不需要重启。如果某个 MCP 需要先做项目级初始化（例如 `codegraph init`），
初始化后可以让 Agent “刷新这个 MCP”，它会调用 `refresh_mcp` 重新连接并注册最新工具。

也可以手动编辑 `~/.qiuqiu/mcp_servers.json`，下次启动时自动加载：

```json
[
  {"name": "filesystem", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "."]}
]
```

### 安装外部 Skill

可以直接在对话里说“帮我安装这个 Skill”，然后粘贴 JSON / `SKILL.md`、提供本地路径或 URL。Agent 会调用
`install_skill` 工具，在确认后写入 `~/.qiuqiu/skills/<name>.json`，并热加载到当前进程，随后即可：

```text
/use debug_expert
```

`install_skill` 会自动识别两种格式：

- QiuQiuPro JSON：包含 `name`、`description`、`system_prompt`
- Markdown Skill：`SKILL.md`，用 YAML front matter 声明 `name` / `description`，正文作为 `system_prompt`

也可以手动放一个 `.json` 文件到 `~/.qiuqiu/skills/`，下次启动时自动加载：

```json
{
  "name": "debug_expert",
  "description": "Debug 专家模式 — 定位 Bug、分析根因",
  "system_prompt": "你是一个 Debug 专家。\n定位问题时你必须：\n1. 先复现问题\n2. 分析根因\n3. 修复并验证",
  "tool_whitelist": ["read_file", "search_files", "glob", "grep", "run_powershell"],
  "rules": [
    {"name": "先复现", "description": "修改代码前必须先确认能复现问题"}
  ]
}
```

卸载外部 Skill 时，可以直接说：

```text
帮我删除 seedance 这个 Skill
```

Agent 会调用 `delete_skill`，仅允许删除 `~/.qiuqiu/skills/` 下的外部 Skill；内置 `prompt/skills/` 不会被删除。若删除的是当前激活 Skill，会自动回到 `default`。

---

## 命令

| 命令 | 作用 |
|------|------|
| `/help` | 显示所有可用命令 |
| `/replay` | 回放当前会话的事件日志 |
| `/explain <文件>` | 让 LLM 解释指定文件的内容和作用 |
| `/test` | 运行当前项目的测试 |
| `/use <skill>` | 切换 Skill（如 `/use architect`）；`/use default` 恢复默认 Agent |
| `/compact` | 手动压缩上下文（折叠旧对话为摘要，主动重置前缀缓存）|
| `/usage` | 查看本次会话的 token 用量（输入 / 缓存命中 / 输出 / 思考 / 合计）与估算费用 |
| `/maxsteps [n]` | 设置单次连续计划执行的 step 上限；`0` 表示不限制 |
| `/pause` | 请求协作式暂停：当前 step 完成后暂停 |
| `/resume` | 从上次暂停的 Plan step 继续执行 |
| `/memory` | 查看模型自主沉淀的长期记忆（仅偏好/规则）|
| `/forget <id>` | 删除一条长期记忆 |
| `exit` / `quit` | 退出 |

所有命令以 `/` 开头。不在列表中的输入会作为正常任务交给 Agent 处理。

长期记忆只保存**偏好/规则**，不做知识型 RAG。人工维护的长规则写在 Markdown 文件中：
全局规则为 `~/.qiuqiu/QIUQIU.md`，项目规则为仓库根目录 `QIUQIU.md`。模型自动沉淀的短偏好仍写入
JSON：全局记忆为 `~/.qiuqiu/memory.json`，项目记忆为 `.reasonix/memory.json`；用户可用 `/memory` 审计、
`/forget` 删除这些 JSON 记忆。`remember_rule` 默认不会弹高危确认，便于模型自动沉淀短偏好；开启只读模式时仍会拒绝写入。

### 上下文与前缀缓存

QiuQiuPro 按 DeepSeek 的前缀缓存规则优化对话历史：正常对话、工具循环和 loop guard 都保持 append-only，让下一轮请求尽量复用上一轮 prompt 前缀。连续相同工具错误触发 loop guard 时，Agent 会追加一条 assistant 终止边界，告诉模型“下一条用户输入按新任务处理”，避免错误语境污染，同时不回滚历史。

会主动重写历史、从而重置一部分缓存链的操作只有少数几类：`/compact` 或自动压缩、陈旧 tool 结果裁剪、超长历史 `Trim`，以及用户显式 `Ctrl+C` 中断当前轮。`/usage` 会显示会话累计前缀缓存命中率。

---

## 项目结构

```
D:\QiuQiuPro\
├── main.go                    ← 入口（API Key + MCP + Skill + 命令注册 + 对话循环）
│
├── agent/                     ← Agent 核心
│   ├── agent.go               ← 结构体 + 注册 + Skill 切换 + 高危工具名单
│   ├── install_tools.go       ← install_skill / delete_skill / install_mcp / refresh_mcp 热安装工具
│   ├── run.go                 ← Agent 核心循环 + 高危操作用户确认
│   ├── plan.go                ← 拆步骤 + 自我审视 + 执行 + 动态重规划
│   └── helpers.go             ← 辅助函数
│
├── command/                   ← 斜杠命令系统（1 个文件）
│   └── registry.go            ← Command 结构体 + 注册表 + 匹配执行
│
├── tool/                      ← 工具（6 个文件，11 个内置工具）
│   ├── struct.go              ← Tool 结构体定义 + AllBuiltInTools()
│   ├── file_tools.go          ← read / write / list / count
│   ├── edit_tools.go          ← edit_block + search_files
│   ├── glob_tools.go          ← glob（按文件名模式匹配）
│   ├── grep_tools.go          ← grep（按文件内容搜索）
│   ├── shell_tools.go         ← run_shell + run_powershell
│   └── git_tools.go           ← git_commit
│
├── event/store.go             ← Event Sourcing（JSON Lines）
├── mcp/client.go              ← MCP 协议客户端
├── mcp/manager.go             ← MCP 配置持久化 + 热安装 / 热注册
├── skill/skill.go             ← Skill 定义 + 外部加载
├── skill/manager.go           ← Skill 安装 / 列表 / 查找 / 热加载
├── .gitignore / go.mod / go.sum
├── OPTIMIZATION_SUMMARY.md    ← 优化过程记录
```

---

## 架构一览

```text
用户输入
  ├── 以 / 开头 → 匹配斜杠命令 → 执行
  └── 正常输入
        ↓
      拆步骤（GeneratePlan）
        ↓
      自我审视（ReviewPlan）→ 有问题自动修正
        ↓
      按顺序执行（ExecutePlan）
        ├── 某步执行（Run）
        │   ├── 调 LLM
        │   ├── 有 ToolCall → 权限门检查 → 必要时 [Y/n] 确认 → 执行工具（内置 or MCP）→ 结果喂回 → 继续
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
| **Tool** | Agent 能调用的函数（内置 11 个 + 任意 MCP 外部工具） |
| **MCP** | 工具即插即用协议（Model Context Protocol） |
| **Skill** | 人格切换卡 = SystemPrompt + 工具白名单 + 规则，可热安装 |
| **Plan** | 复杂任务拆成步骤，每步独立执行 |
| **Event Log** | 每步操作记录为不可变事件（JSON Lines），支持重放 |
| **斜杠命令** | 以 `/` 开头的内置快捷操作，可扩展注册 |

---

## 内置工具（11 个）

| 工具 | 用途 | 高危？ |
|------|------|--------|
| `read_file` | 读取文件内容 | |
| `write_file` | 写入文件 | ✅ 需确认 |
| `list_directory` | 列出目录内容 | |
| `edit_file_block` | 精确替换代码块（找不到/找到多处就拒绝） | ✅ 需确认 |
| `search_files` | 按文件名或内容搜索 | |
| `glob` | 按文件名模式匹配（如 `*.go`、`**/*.md`） | |
| `grep` | 在文件内容中搜索关键词 | |
| `count_file_chars` | 统计文件字符数 | |
| `git_commit` | 提交所有变更到 Git | |
| `run_powershell` | 执行 PowerShell 命令（Windows 推荐） | ✅ 需确认 |
| `run_shell` | 执行 cmd 命令（兜底，不推荐） | ✅ 需确认 |

---

## 内置 Skill（3 个）

| Skill | 适合场景 | 可用工具 |
|-------|---------|---------|
| `architect` | 系统设计、技术选型、架构评审 | 读文件 + glob |
| `code_review` | 代码审查、安全审计、质量检查 | 读 + 编辑 + glob + grep |
| `frontend_design` | UI 设计、组件库开发、前端架构 | 读 + 写 + glob + grep |

---

## 优化历程

| # | 优化点 | 说明 |
|---|--------|------|
| 1 | **search_files 工具** | 按文件名或内容搜索文件 |
| 2 | **PowerShell 兼容性** | 新增 run_powershell，引导 LLM 优先用 |
| 3 | **API Key 自动保存** | 首次输入保存到 `~/.qiuqiu/key`，后续免配置 |
| 4 | **Plan 自我审视** | LLM 拆完步骤后自己检查质量，有问题自动修正 |
| 5 | **动态重规划** | 执行中某步失败，自动重新规划剩余步骤并继续 |
| 6 | **MCP 可配置 / 热安装** | 从 `~/.qiuqiu/mcp_servers.json` 读取，也可由 `install_mcp` 写入并立即注册；初始化后可用 `refresh_mcp` 刷新工具 |
| 7 | **Skill 外部加载 / 热安装** | `~/.qiuqiu/skills/*.json` 启动时自动加载，也可由 `install_skill` 写入并立即 `/use`；外部 Skill 可由 `delete_skill` 删除 |
| 8 | **Glob + Grep 工具** | 拆分为独立工具，LLM 更容易选中正确工具 |
| 9 | **安全拦截防线** | 高危工具（写文件/执行命令）执行前弹 `[Y/n]` 确认；偏好记忆默认自动写入，只读模式拒绝 |
| 10 | **斜杠命令系统** | `/help`、`/explain`、`/test`、`/use` 等可扩展命令 |
| 11 | **前缀缓存友好上下文** | 正常对话 append-only；loop guard 追加终止边界而不回滚历史，避免污染下一轮并保持缓存链 |

详细优化过程见 [OPTIMIZATION_SUMMARY.md](./OPTIMIZATION_SUMMARY.md)。

---

## 技术栈

| 模块 | 选型 |
|------|------|
| 语言 | Go 1.25.5+ |
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
优化  QiuQiuPro  → 10 项功能与体验优化
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
| 斜杠命令 | `command/registry.go` | - | Slash Command |
| 安全确认 | `agent/run.go` | - | 权限系统 |

---

## License

MIT
