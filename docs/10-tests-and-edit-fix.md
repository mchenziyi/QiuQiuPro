# 10 — 补单元测试 + 修复 edit_file_block

## 为什么要做

1. 项目有 `/test` 命令、README 也讲了 TDD，但仓库里**一个 `_test.go` 都没有**（P0/P1 起了
   头），核心纯逻辑没有任何回归保护。
2. 补测试过程中，给「精确编辑」旗舰工具 `edit_file_block` 写测试时，**当场发现它是坏的**——
   按 LLM 实际调用方式（`old_block` / `new_block`）调用，永远返回「旧代码出现多次」，根本改不了文件。

## 做了什么

### 1. 修复 edit_file_block 缺 json tag 的 bug（TDD 红 → 绿）

工具的参数 schema 用 snake_case（`old_block` / `new_block`），但 Execute 里的结构体没带 tag：

```go
var p struct{ Path, OldBlock, NewBlock string }   // ❌ 无 tag
```

Go 的 `encoding/json` 反序列化做的是**大小写不敏感**匹配，但**不会忽略下划线**：

- `path` → `Path`：能匹配（无下划线），所以路径是对的；
- `old_block` → `OldBlock`：匹配不上 → `OldBlock` 恒为 `""`。

后果：`strings.Contains(text, "")` 恒 `true`、`strings.Count(text, "")` 恒 `> 1`，于是
**任何一次编辑都走进「旧代码出现多次」分支**，工具实质不可用。

修复——补上 json tag（与同文件 `write_file` / `search_files` 的写法一致）：

```go
var p struct {
	Path     string `json:"path"`
	OldBlock string `json:"old_block"`
	NewBlock string `json:"new_block"`
}
```

`tool/edit_tools_test.go` 用 LLM 真实调用方式（snake_case key）覆盖 4 种情形：唯一匹配替换成功、
找不到旧代码、旧代码出现多次、文件不存在。修复前前两个用例失败（实证 bug），修复后 4 个全过。

### 2. 重构：抽出 stripCodeFence（DRY）

`plan.go` 里清洗 LLM 输出 ``` 代码块围栏的 5 行逻辑（`TrimSpace` → 去 ```` ```json ```` /
```` ``` ```` 前缀 → 去 ```` ``` ```` 后缀 → `TrimSpace`）在 `GeneratePlan` / `ReviewPlan` /
`RePlan` **重复了 3 次**。抽成一个纯函数：

```go
func stripCodeFence(s string) string { ... }
```

三处调用点统一改为 `stripCodeFence(...)`，行为不变、可单测。

### 3. 补核心纯函数单测

`agent/agent_test.go`：

- `TestIsHighRiskTool`——高危集合（write/edit/run_shell/run_powershell）判 true，其余（含未知工具）判 false；
- `TestTruncate`——边界（短于/等于/超出上限、空串、中文按 rune 截断）；
- `TestStripCodeFence`——```json / ``` 围栏、无围栏、同行围栏、纯 JSON 各情形。

加上 P0 的 `memory_test.go`、P1 的 `input_test.go`，现在 `agent` + `tool` 两个包共 **15 个测试函数全绿**。

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/edit_tools.go` | **bug 修复**：edit_file_block 结构体补 json tag（old_block/new_block 不再绑空） |
| `tool/edit_tools_test.go` | 新增：edit_file_block 4 例（成功 / 找不到 / 多次 / 文件缺失） |
| `agent/plan.go` | 重构：抽出 `stripCodeFence`，3 处重复清洗逻辑合一 |
| `agent/agent_test.go` | 新增：IsHighRiskTool / truncate / stripCodeFence 单测 |

## 效果

- `edit_file_block` 真正能用了——这是 Agent 改代码的核心能力，之前形同虚设。
- LLM 输出围栏清洗逻辑去重，少一处「改一个忘改俩」的隐患。
- 核心纯函数有了回归网；`go build` / `go test ./agent/ ./tool/` / `go vet` 全绿。

## 相关 TODO

> TODO-reasonix.md — 待修问题 **P3**（缺少测试）
> 难度：★★☆☆☆（附带挖出并修掉一个 P0 级的工具 bug）
