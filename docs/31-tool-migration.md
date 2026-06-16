# 31 — 工具搬迁：从 Reasonix 搬运 5 个核心工具

## 为什么要做

对比分析（#29）发现 QiuQiuPro 缺了 Reasonix 的几个高频工具。system prompt 重写后（#30）提到"不确定时列选项"、"跟踪进度"等场景，但旧工具集里没有对应的工具可用。

## 搬了什么

| 工具 | 来源 | 行数 | 用途 |
|------|------|------|------|
| `todo_write` | Reasonix `builtin/todo.go` | 80 | 任务清单跟踪，始终保持一个 in_progress |
| `edit_file` | Reasonix `builtin/editfile.go` | 65 | 精确文本替换，old_string 必须唯一 |
| `multi_edit` | Reasonix `builtin/multiedit.go` | 90 | 批量编辑，原子性——任意一步失败则文件不动 |
| `delete_range` | Reasonix `builtin/delete_range.go` | 125 | 按起止行锚点删除连续区域 |
| `delete_symbol` | Reasonix `builtin/delete_symbol.go` | 210 | Go AST 解析，按符号名精确删除函数/类型/变量 |

## 适配差异

Reasonix 的工具接口是 `Execute(ctx, json.RawMessage) (string, error)`，QiuQiuPro 是 `Execute func(string) string`。适配时：
- 去掉了 context 和 error 返回值
- 错误用 `fmt.Sprintf("xxx 失败：%v", err)` 内嵌在返回字符串里
- `delete_range` 去掉了 `diff.Build` 依赖，改纯文本摘要
- `todo_write` 去掉了 `evidence.Ledger` 校验，保留 JSON 结构验证

## 未搬的

| 工具 | 原因 |
|------|------|
| `complete_step` | 依赖 evidence.Ledger（待 #23） |
| `bash_output` / `kill_shell` / `wait` | 依赖后台任务管理器（待 #6） |
| `notebook_edit` | Jupyter 专用，不需要 |

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/todo_write.go` | 新增 |
| `tool/edit_file_tool.go` | 新增 |
| `tool/multi_edit_tool.go` | 新增 |
| `tool/delete_range_tool.go` | 新增 |
| `tool/delete_symbol_tool.go` | 新增 |
| `tool/struct.go` | 注册 5 个新工具 |
| `prompt/default/system.xml` | 更新引用 |

## 效果

- 工具总数 13 → 18
- `edit_file` 比 `edit_file_block` 参数更简单（不需要 `block` 命名）
- `multi_edit` 允许原子性地批量编辑
- `todo_write` 让模型有进度可追踪
