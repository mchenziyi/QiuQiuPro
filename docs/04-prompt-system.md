# 04 — Prompt 外部化（XML + 模板系统）

## 为什么要做

QiuQiuPro 之前有 8 处提示词写死在 3 个 Go 文件中：
- `agent/agent.go` — 默认 system prompt（CoT）
- `agent/plan.go` — GeneratePlan / ReviewPlan / Reflect / RePlan 共 4 个 prompt
- `skill/skill.go` — 3 个内置 Skill 的 system prompt

改提示词需要：找到代码位置 → 修改字符串 → 重新编译 → 重启。
这违反了"提示词是配置不是代码"的原则。

## 做了什么

### 1. 创建 `prompt/` 目录结构

```
prompt/
├── default/
│   └── system.xml              ← 默认 system prompt（CoT）
├── plan/
│   ├── generate.xml            ← GeneratePlan
│   ├── review.xml              ← ReviewPlan
│   ├── reflect.xml             ← Reflect
│   └── replan.xml              ← RePlan
└── skills/
    ├── architect.json          ← 架构师 Skill
    ├── code_review.json        ← 代码审查 Skill
    └── frontend_design.json    ← 前端设计 Skill
```

### 2. 新增 `agent/prompt.go` — Prompt 加载器

- `LoadPrompt(path, vars)` — 读取 XML，解析 `<prompt>` 标签，执行 Go 模板替换
- `LoadRawPrompt(path)` — 读取纯文本 XML（无模板变量）
- 所有加载失败时都有 fallback 到硬编码 prompt，不会影响已有功能

### 3. 移除内置 Skill 硬编码

- `skill/skill.go` — 删除 `Architect()` / `CodeReview()` / `Frontend()` / `AllBuiltInSkills()`
- 内置 Skill 改为从 `prompt/skills/*.json` 加载
- 用户自定义 Skill 从 `~/.qiuqiu/skills/*.json` 加载
- `skill.Manager` 负责 Skill 的加载、查找、安装和内存刷新
- 用户可以让 Agent 调用 `install_skill`，从 JSON / `SKILL.md` / 本地路径 / URL 安装 Skill；安装后会写入 `~/.qiuqiu/skills/<name>.json` 并立即可 `/use <name>`，无需重启
- 用户可以让 Agent 调用 `delete_skill` 删除外部安装的 Skill；内置 Skill 不允许删除
- MCP Server 由 `mcp.Manager` 管理：`install_mcp` 负责安装并立即连接，`refresh_mcp` 可在项目初始化后重连并刷新工具列表

### 4. 改动文件

| 文件 | 改动 |
|------|------|
| `agent/prompt.go` | **新增** — Prompt 加载器 |
| `agent/agent.go` | sysPrompt 从 `prompt/default/system.xml` 加载 |
| `agent/plan.go` | 4 个 prompt 全部从 `prompt/plan/*.xml` 加载 |
| `skill/skill.go` | 删除内置 Skill 函数，保留外部加载 |
| `skill/manager.go` | Skill 加载 / 查找 / 安装 / 热刷新 |
| `agent/install_tools.go` | 新增 `install_skill` / `delete_skill` / `install_mcp` / `refresh_mcp`，支持自然语言管理外部 Skill 与 MCP |
| `main.go` | 通过 `skill.Manager` 加载 Skill，`/use` 动态读取最新列表 |
| `prompt/` | **新增** — 8 个 XML/JSON 文件 |

## 效果

- 改提示词不再需要重新编译——直接改 XML 文件重启即可
- 加新的内置 Skill = 放一个 JSON 文件到 `prompt/skills/`
- 加新的用户 Skill = 对话里让 QiuQiuPro 安装，或手动放到 `~/.qiuqiu/skills/`
- 外部 `SKILL.md` 会自动解析 front matter，正文作为 `system_prompt`
- 删除用户 Skill = 对话里让 QiuQiuPro 删除，仅影响 `~/.qiuqiu/skills/`
- `install_skill` 安装完成后会热加载到当前进程，马上可以 `/use`
- `install_mcp` 安装后会立即注册工具；若后续做了项目初始化，可用 `refresh_mcp` 重新发现工具
- XML 模板支持 `{{.Variable}}` 语法，保持灵活性
- 所有加载失败时有 fallback，不会因为文件缺失而崩溃

## 相关 TODO

> TODO-reasonix.md — 新需求
> 难度：★★★☆☆
