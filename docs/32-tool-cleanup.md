# 32 — 工具清理：去掉重复/无用工具

## 删了什么

| 工具 | 原因 |
|------|------|
| `count_file_chars` | LLM 几乎不用，Reasonix 也没有 |
| `run_powershell` | `bash` 自动适配系统，Windows 走 cmd，macOS/Linux 走 sh |
| `edit_file_block` | `edit_file` 参数更简洁（old_string/new_string vs old_block/new_block） |

## 改了什么

- 所有工具 Description 统一改为中文
- 工具命名对齐 Reasonix：`list_directory→ls`，`run_shell→bash`
- 工具总数 18 → 15

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/struct.go` | 去掉 3 个工具注册 |
| `tool/file_tools.go` | 删 `count_file_chars` |
| `tool/shell_tools.go` | 删 `run_powershell` |
| `tool/edit_tools.go` | 删 `edit_file_block`（保留 `search_files`） |
| `agent/tools.go` | 更新高危工具名单 |
| `agent/agent_test.go` | 更新测试引用 |
| `agent/gate_test.go` | 更新测试引用 |
| `agent/parallel_test.go` | 更新测试引用 |
| `prompt/skills/*.json` | 更新白名单 |

## 当前工具全集（15 个）

读取: read_file, ls, glob, grep, code_search, web_fetch
编辑: write_file, edit_file, multi_edit
删除: delete_range, delete_symbol
执行: bash, git_commit
跟踪: todo_write
