# 25 — 偏好/规则型长期记忆

## 为什么只做偏好/规则

Coding Agent 的长期记忆应该帮助它遵守用户偏好和项目约定，而不是变成知识库。项目知识、代码事实、日志和一次性任务上下文
会快速过期，放进长期记忆容易污染上下文，也会影响 DeepSeek 前缀缓存稳定性。

因此本项目明确：

- 做：用户偏好、默认行为、项目规则
- 不做：RAG、向量库、embedding、知识型长期记忆、代码片段沉淀

## 写入方式：模型自主判断

不提供 `/remember` 这种显式写入命令。写入由模型在对话中自行判断，通过受限工具 `remember_rule` 完成。

工具只应在这些场景调用：

- 用户表达长期偏好：如“以后默认用中文回答”
- 用户设置默认行为：如“提交信息默认中文”
- 用户明确项目规则：如“当前项目只支持 DeepSeek”

工具不应保存：

- 项目知识或代码事实
- 一次性任务细节
- 日志、报错、临时调试信息
- 大段代码或文档
- 密钥、token、隐私信息

## 存储

两层 JSON 存储：

| 作用域 | 路径 | 用途 |
|------|------|------|
| 全局 | `~/.qiuqiu/memory.json` | 跨项目用户偏好 |
| 项目 | `.reasonix/memory.json` | 当前仓库的项目规则/偏好 |

每条记忆结构：

```json
{
  "id": "mem_...",
  "scope": "global",
  "kind": "preference",
  "content": "以后默认用中文回答",
  "source": "model",
  "created_at": 123,
  "updated_at": 123,
  "enabled": true
}
```

只允许两类 `kind`：

- `preference`
- `project_rule`

`knowledge` 等知识型类型会被拒绝。

## Prompt 注入

每轮 `Run` 构造 system prompt 时，会把启用的长期记忆渲染成稳定块追加到基础 system prompt 后：

```text
## 长期记忆（偏好/规则）
以下记忆只包含用户偏好与项目规则；不要把它们当作外部知识库。
全局偏好：
- ...
项目规则：
- ...
```

记忆块只在记忆文件变化后变化；平时对话仍保持 append-only，尽量不破坏前缀缓存。

## 命令

| 命令 | 作用 |
|------|------|
| `/memory` | 查看当前启用的偏好/规则长期记忆 |
| `/forget <id>` | 删除（禁用）一条长期记忆 |

没有 `/remember`：写入必须由模型通过 `remember_rule` 自主判断完成。

## 只读模式

`remember_rule` 会写 JSON 文件，因此不是只读工具：

- 默认模式：允许模型自主写入，不弹高危确认
- `/readonly on`：拒绝写入记忆
- 工具调度：不参与只读工具并发，避免并发写 JSON

## 测试

`agent/memory_test.go` 覆盖：

- `MemoryStore` 添加、列出、删除
- 拒绝知识型记忆和过长内容
- 记忆块稳定渲染
- system prompt 注入长期记忆
- 模型工具 `remember_rule` 写入偏好
- `remember_rule` 拒绝知识型记忆
- 只读模式拒绝写记忆

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/long_memory.go` | MemoryStore、prompt 渲染、`remember_rule` 工具、`/memory`/`/forget` 辅助方法 |
| `agent/agent.go` | Agent 新增 `memoryStore`，默认启用全局/项目两层记忆 |
| `agent/run.go` | `Run` 使用 `BuildSystemPrompt()` 注入长期记忆 |
| `agent/tools.go` | Skill 白名单下仍保留 `remember_rule`，且该工具非只读、不并发 |
| `main.go` | 注册 `remember_rule` 工具、`/memory`、`/forget` |
| `agent/memory_test.go` | 偏好/规则记忆测试 |
| `README.md` / `TODO-reasonix.md` | 文档同步 |
