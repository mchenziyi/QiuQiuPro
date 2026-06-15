# 13 — code_search 工具（语义级 Go 代码搜索）

## 为什么要做

已有 `grep` 只能按文本匹配，噪音大：搜 `Render` 会把注释、字符串、`Renderer`、别的类型的同名
方法全捞出来。Agent 想准确回答「`Foo` 定义在哪」「谁调用了 `parseConfig`」需要的是**按标识符**
搜索，而不是按字符串。`code_search` 基于 `go/ast` 解析，只匹配真正的符号。

（TODO-reasonix.md 功能清单 #3，第一梯队、★★★☆☆。本次先交付**实用子集**：符号定义 + 引用查找。）

## 做了什么

### 1. 新增 code_search 工具

`tool/code_search.go` — 参数 `symbol`（必填）、`path`（默认 `.`）。对 root 下所有 `.go` 文件
用 `go/parser` 解析，遍历 AST，分别给出：

- **定义**：区分 `func` / `method` / `type` / `var` / `const`，标注 `文件:行` 和该行源码；
- **引用**：所有同名标识符的使用处（已剔除定义名本身）。

### 2. 核心解析逻辑（两遍 AST 遍历）

```go
// 第一遍：FuncDecl（有 Recv 即 method）、GenDecl 里的 TypeSpec / ValueSpec
//         收集定义，并记下定义名标识符的 token.Pos。
// 第二遍：所有同名 *ast.Ident，排除第一遍记录的定义位置，即为引用。
```

`searchSymbolInSource(filename, src, symbol)` **直接吃源码字节、不碰文件系统**，所以能用内联
Go 代码字符串做纯单测，覆盖每一种符号 kind。

### 3. 工程细节

- `collectGoFiles`：递归收集 `.go`，跳过隐藏目录 / `vendor` / `node_modules`；
- 引用按 `文件:行` 排序，最多展示 50 条（超出标注总数），避免刷屏污染上下文；
- 解析失败的文件静默跳过，不影响其它文件；
- 参数结构体带 json tag。

### 4. 注册进内置工具

`tool/struct.go` 的 `AllBuiltInTools()` 加入 `NewCodeSearchTool()`。

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/code_search.go` | 新增：code_search 工具 + `searchSymbol` / `searchSymbolInSource` / `collectGoFiles` / `formatCodeSearch` |
| `tool/struct.go` | 注册 `NewCodeSearchTool()` |
| `tool/code_search_test.go` | 新增 8 个单测 |

## 测试

| 用例 | 验证 |
|------|------|
| `TestSearchSymbolInSource_KindsAndCounts` | type/func/method/var/const 五种 kind + 定义/引用计数 |
| `TestSearchSymbolInSource_ExcludesDefFromRefs` | 定义名不被当成引用 |
| `TestSearchSymbolInSource_ParseError` | 非法源码返回解析错误 |
| `TestSearchSymbol_TempDir` | 端到端：临时目录 → 找到 1 定义 5 引用 |
| `TestSearchSymbol_EmptySymbol` / `_NotFound` | 空符号 / 查无结果 |
| `TestCollectGoFiles_SkipsHiddenAndNonGo` | 跳过隐藏目录与非 .go |
| `TestFormatCodeSearch_CapsRefs` | 引用超 50 条截断 |

## 已知边界（后续可加）

- **不做类型解析**：靠名字匹配标识符，无法区分「不同类型的同名方法 / 字段」。要精确到具体类型，
  需上 `go/types` 做类型检查（成本高），或集成 codegraph。
- **不做调用链**：#3 原本还提到「调用链」，本次未做——那需要构建调用图，属于更重的后续增量。
- 只解析 Go 源码（本项目即 Go）。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#3 code_search（语义代码搜索）**
> 难度：★★★☆☆（本次交付定义 + 引用子集）
