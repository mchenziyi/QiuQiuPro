# 16 — 扩充 Skill 人格（pm / backend_dev / tester / devops）

## 为什么要做

Skill 机制已就绪（system_prompt + 工具白名单 + 规则），但只带了 architect / code_review /
frontend_design 三个，覆盖面偏「设计 / 审查」。补齐研发链路上的常用角色，让 `/use` 能一键切到
对应人格，连带把可用工具收窄到该角色该用的范围。

（TODO-reasonix.md 功能清单 #7，第二梯队、★★☆☆☆。）

## 做了什么

### 1. 新增 4 个 Skill（`prompt/skills/*.json`）

| Skill | 定位 | 工具白名单要点 |
|-------|------|----------------|
| `pm` | 产品经理：澄清需求、拆用户故事、写 PRD | 读 + 调研 + `write_file`（写文档），**不**给改代码 / 跑命令 |
| `backend_dev` | 后端开发：实现逻辑、跑测试、提交 | 全量研发工具（读写 / 编辑 / shell / `git_commit`）|
| `tester` | 测试工程师：设计用例、写测试、定位失败 | 读写 + 编辑 + `bash`，**不**给 `git_commit` |
| `devops` | 运维：脚本、构建发布、配置排障 | shell + 配置读写 + `web_fetch` 查文档 |

每个都沿用既有风格：开头「你是球球（QiuQiuPro）…」+ 编号工作流 + 2 条 rules。工具白名单按
「该角色该用什么」收窄（如 pm 不给写代码 / 跑命令，tester 不给提交）。

### 2. 工具白名单校验测试（`agent/skill_bundle_test.go`）

白名单里的工具名若**写错，会被 `ApplySkill` 静默过滤**——不报错、但该工具悄悄没了，很难
发现。新增 `TestBundledSkillsValid` 兜底：

- 加载 `prompt/skills` 下每个 JSON，断言能解析、`name/description/system_prompt` 非空；
- 用 `tool.AllBuiltInTools()` 构造合法工具名集合，校验每个白名单工具都真实存在；
- 断言 7 个预期 Skill 全部就位。

（测试放 `agent` 包，因为它同时 import `skill` 与 `tool`；cwd 为 `agent/`，故路径用 `../prompt/skills`。）

## 改动文件

| 文件 | 改动 |
|------|------|
| `prompt/skills/pm.json` 等 4 个 | 新增 4 个 Skill 人格 |
| `agent/skill_bundle_test.go` | 新增：内置 Skill 加载 + 工具名校验 |

## 效果

- `/use pm` / `/use backend_dev` / `/use tester` / `/use devops` 一键切换人格，可用工具随之收窄。
- 工具名拼写有了回归保护，新增 / 修改 Skill 时不会再因笔误悄悄丢工具。
- `go build` / `go test ./agent/ ./tool/ ./cleanup/` / `go vet` 全绿。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#7 Skill 人格切换（扩展）**
> 难度：★★☆☆☆
