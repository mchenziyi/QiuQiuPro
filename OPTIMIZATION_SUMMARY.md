# 🏀 QiuQiuPro 优化阶段总结

> 基于球球 V5 的架构，在 QiuQiuPro 上进行的系列优化记录。
> **目标：把"结构完整的 Agent"打磨成"功能完整的 Agent"。**

---

## 优化清单

| # | 优化点 | 文件 | 说明 | 状态 |
|---|--------|------|------|------|
| 1 | **search_files 工具** | `tool/edit_tools.go` | 支持按文件名（glob）和按文件内容搜索 | ✅ |
| 2 | **PowerShell 兼容性** | `tool/shell_tools.go` | 新增 run_powershell，修改 run_shell 描述引导 LLM 优先用 PowerShell | ✅ |
| 3 | **API Key 自动保存** | `main.go → getAPIKey()` | 首次使用在终端输入 Key，自动保存到 `~/.qiuqiu/key`，后续免配置 | ✅ |
| 4 | **Plan 自我审视** | `agent/plan.go → ReviewPlan()` | LLM 拆完步骤后自己检查一遍，漏步骤或顺序不对时自动修正 | ✅ |
| 5 | **动态重规划** | `agent/plan.go → RePlan()` | 执行中某步失败，LLM 根据已完成内容重新规划剩余步骤并继续执行 | ✅ |
| 6 | **MCP 可配置** | `main.go → loadMCPConfigs()` | MCP Server 从 `~/.qiuqiu/mcp_servers.json` 读取，改配置不用改代码 | ✅ |
| 7 | **Skill 外部加载** | `skill/skill.go → LoadFromDir()` | `~/.qiuqiu/skills/*.json` 启动时自动加载为 Skill | ✅ |
| 8 | **Glob + Grep 工具** | `tool/glob_tools.go` + `tool/grep_tools.go` | 把搜索拆为两个独立工具，LLM 更容易选对 | ✅ |
| 9 | **安全拦截防线** | `agent/agent.go` + `agent/run.go` | 高危工具（写文件/执行命令）执行前弹 `[Y/n]` 确认 | ✅ |
| 10 | **斜杠命令系统** | `command/registry.go` | `/help`、`/explain`、`/test`、`/use`、`/replay` 可扩展命令 | ✅ |

---

## 各优化点的核心思路

### 8. Glob + Grep

```
为什么：原来的 search_files 扛了文件名搜索和内容搜索两件事，
        LLM 要传 search_content:true 才能搜内容，不够直观
怎么做：拆成 glob（文件名匹配）和 grep（内容搜索）两个独立工具
        glob 支持 *.go、**/*.md 等通配符，可指定搜索目录
        grep 支持关键词搜索，可指定搜索目录，返回行号+行内容
设计：工具名越精确，LLM 选对的概率越高。
      跟 Claude Code 的 Glob + Grep 工具设计一致
```

### 9. 安全拦截防线

```
为什么：LLM 可能误调 write_file 或 run_shell，造成破坏
怎么做：在 agent.go 中定义 highRiskTools 名单
       在 run.go 中执行工具前判断是否高危，是则弹出 [Y/n] 确认
       用户取消时不报错，而是返回"用户已取消"给 LLM，让它换方式
设计：map[string]bool 查找 O(1)，四个工具写死不必配置化
      用户取消后 Agent 不会崩溃，会尝试其他方案
```

### 10. 斜杠命令系统

```
为什么：原来 replay、use 等命令散落在 main.go 的 if 分支里
怎么做：新建 command/ 包，Command 结构体 + Registry（Register/List/Handle）
       每个命令是一个 Name + Description + Handler
       Handler 通过闭包从 main.go 捕获 Agent 等依赖，避免循环引用
       内置 5 个命令：/help /replay /explain /test /use
设计：加新命令只需在 main.go 注册区加几行，不改流程代码
```

---

## 最终项目结构

```
D:\QiuQiuPro\
├── main.go                    ← 入口
│
├── agent/                     ← Agent 核心（4 个文件）
│   ├── agent.go               ← 结构体 + 注册 + Skill 切换 + 高危工具名单
│   ├── run.go                 ← 核心循环 + 高危操作用户确认
│   ├── plan.go                ← 拆步骤 + 自我审视 + 执行 + 动态重规划
│   └── helpers.go             ← 辅助函数
│
├── command/                   ← 斜杠命令系统（1 个文件）
│   └── registry.go            ← Command + Registry + Handle
│
├── tool/                      ← 工具（7 个文件，11 个工具）
│   ├── struct.go              ← Tool 定义 + AllBuiltInTools()
│   ├── file_tools.go          ← read / write / list / count
│   ├── edit_tools.go          ← edit_block + search_files
│   ├── glob_tools.go          ← glob 文件名匹配
│   ├── grep_tools.go          ← grep 内容搜索
│   ├── shell_tools.go         ← run_shell + run_powershell
│   └── git_tools.go           ← git_commit
│
├── event/store.go             ← Event Sourcing（JSON Lines）
├── mcp/client.go              ← MCP 协议客户端
├── skill/skill.go             ← Skill 定义 + 内置 + 外部加载
├── .gitignore / go.mod / go.sum
└── OPTIMIZATION_SUMMARY.md
```

---

## 用户配置路径

所有用户配置统一放在 `~/.qiuqiu/` 目录下：

| 文件 | 用途 | 自动创建？ |
|------|------|-----------|
| `~/.qiuqiu/key` | API Key | ✅ 首次输入时自动创建 |
| `~/.qiuqiu/mcp_servers.json` | MCP Server 列表 | ❌ 需要手动创建（有示例） |
| `~/.qiuqiu/skills/*.json` | 外部 Skill | ❌ 手动放入 `.json` 文件 |

---

## 后续可能的优化方向

| 方向 | 说明 | 优先级 |
|------|------|--------|
| 上下文摘要压缩 | 消息超限时让 LLM 总结最早几轮对话，替换原文 | ⭐⭐⭐ |
| Checkpoint 快照 | 定期保存状态快照，加速崩溃恢复 | ⭐⭐⭐ |
| 日志输出控制 | `--quiet` 模式，减少中间日志 | ⭐⭐ |
| Skill 自动选择 | 根据用户输入自动匹配 Skill | ⭐⭐ |
| Tool 自动重试 | 工具调用失败自动重试 1-2 次 | ⭐⭐ |
| Git Worktree 隔离 | 代码修改在隔离分支进行，不污染工作区 | ⭐⭐ |
| SubAgent | 主 Agent 派生子 Agent 并行执行独立子任务 | ⭐⭐ |
| Skill 从 URL 加载 | `LoadFromURL()` 实现 | ⭐ |
