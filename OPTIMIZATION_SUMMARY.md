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

---

## 各优化点的核心思路

### 1. search_files

```
为什么：LLM 原来只能用 list_directory 翻目录，大项目里找不到代码
怎么做：filepath.Glob（文件名）+ filepath.Walk（内容搜索）
设计：返回 LLM 友好的文本，文件名搜到带大小，内容搜到带行号
```

### 2. PowerShell 兼容性

```
为什么：Windows 的 cmd 引号问题多，LLM 写 cmd 命令频繁出错
怎么做：新增 run_powershell（powershell -NoProfile -Command）
      修改 run_shell 描述为 "【不推荐】优先用 run_powershell"
设计：不改代码逻辑，只改描述引导 LLM 的选择
```

### 3. API Key 自动保存

```
为什么：每次启动都要手动 export，烦
怎么做：环境变量 → ~/.qiuqiu/key 文件 → 首次输入
      用 os.UserHomeDir() 定位，不用担心跨平台
设计：按优先级降级，文件权限设为 0600（仅本用户可读）
```

### 4. Plan 自我审视

```
为什么：LLM 拆步骤时偶尔漏关键步骤或顺序不对
怎么做：GeneratePlan 之后、ExecutePlan 之前，调一次 ReviewPlan
      LLM 返回 "OK" 则通过，返回 JSON 则替换 Plan
设计：审查失败不阻塞，保留原 Plan 继续执行
```

### 5. 动态重规划

```
为什么：ExecutePlan 原来一步失败就全盘结束
怎么做：for 循环改为动态长度，失败时调 RePlan 根据已完成内容重新规划
      保留已完成 + 失败步骤，替换后续步骤为新方案
设计：重规划失败才返回 error，重规划成功则继续执行
```

### 6. MCP 可配置

```
为什么：MCP Server 硬编码在 main.go 里，加新工具要改代码
怎么做：从 ~/.qiuqiu/mcp_servers.json 读取配置列表
      JSON 格式：[{"name":"","command":"","args":[]}]
设计：配置缺失不影响启动，没有 Server 就跳过
```

### 7. Skill 外部加载

```
为什么：Skill 写在 Go 代码里，加新 Skill 要改代码
怎么做：扫描 ~/.qiuqiu/skills/*.json，自动注册
      JSON 包含 name / description / system_prompt / tool_whitelist / rules
      CLI 列表中标记 [内置] 和 [外部] 区分来源
设计：加载失败只跳过单个文件，不影响其他 Skill
```

---

## 最终项目结构

```
D:\QiuQiuPro\
├── main.go                    ← 入口：API Key 获取 + MCP 加载 + Skill 加载 + 对话循环
│
├── agent/                     ← Agent 核心（4 个文件）
│   ├── agent.go               ← 结构体 + 注册 + Skill 切换
│   ├── run.go                 ← Run() 核心循环
│   ├── plan.go                ← GeneratePlan + ReviewPlan + ExecutePlan + RePlan
│   └── helpers.go             ← trimMessages + recordEvent + truncate
│
├── tool/                      ← 工具定义 + 实现（5 个文件）
│   ├── struct.go              ← Tool 结构体 + AllBuiltInTools()
│   ├── file_tools.go          ← read / write / list / count
│   ├── edit_tools.go          ← edit_block + search_files
│   ├── shell_tools.go         ← run_shell + run_powershell
│   └── git_tools.go           ← git_commit
│
├── event/store.go             ← Event + Store + Replay
├── mcp/client.go              ← MCP Client 封装
├── skill/skill.go             ← Skill 结构体 + 内置 Skill + 外部加载器
├── .gitignore / go.mod / go.sum
│
└── ~/.qiuqiu/                 ← 用户配置文件（不在 Git 仓库中）
    ├── key                     ← API Key（首次输入自动保存）
    ├── mcp_servers.json        ← MCP Server 配置
    └── skills/                 ← 外部 Skill 目录
        └── debug_expert.json   ← 示例 Skill
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
| Token 滑动窗口 | 按 token 数截断而不是按消息条数 | ⭐⭐⭐ |
| Checkpoint 快照 | 定期保存状态快照，加速崩溃恢复 | ⭐⭐⭐ |
| 日志输出控制 | `--quiet` 模式，减少中间日志 | ⭐⭐ |
| Skill 自动选择 | 根据用户输入自动匹配 Skill | ⭐⭐ |
| Tool 自动重试 | 工具调用失败自动重试 1-2 次 | ⭐⭐ |
| Skill 从 URL 加载 | `LoadFromURL()` 实现 | ⭐ |
| 测试用例 | 为 Agent 功能写测试 | ⭐ |
