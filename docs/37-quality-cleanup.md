# 37. 代码质量收口

## 背景

经过 #20-#36 的密集开发，代码中积累了一批技术债：过期 TODO 注释、ctx 未贯穿到工具、
`gofmt` 格式化欠账、死代码（`delete_symbol`）、工具实现过于简易等。本轮统一收口。

## 变更清单

### 1. ctx 贯穿工具执行

**问题**：`Tool.Execute(ctx, args)` 接口已支持 `context.Context`（#33），但 `agent/run.go`
的 `executeToolCall` 仍传 `context.Background()`，导致工具无法响应上游取消与超时。

**修改**：
- `executeToolCall(tc)` → `executeToolCall(ctx, tc)`
- `dispatchAndDetect(calls)` → `dispatchAndDetect(ctx, calls)`
- `Run(ctx)` 的 ctx 一路传递到每个 `Tool.Execute(ctx, args)` 调用
- 仅测试辅助方法 `dispatchToolCalls()`（无 ctx 来源）继续使用 `context.Background()`
- 所有调用 `executeToolCall` 的测试文件同步更新签名

**影响文件**：`agent/run.go`、`agent/hooks_test.go`、`agent/memory_test.go`、`agent/plan_mode_test.go`

### 2. 清理过期注释

**问题**：代码中遍布 `TODO #N` 引用（如 `TODO #13`、`TODO #14`），但这些功能早已完成，
`TODO` 前缀容易误导读者认为仍有未完成工作。

**修改**：
- `agent/agent.go`：移除字段注释中的 `TODO #13`/`#14`/`#15`/`#16`/`#17` 前缀
- `agent/compact.go`：移除 `TODO #13`/`#14` 引用
- `agent/sink.go`：移除 `TODO #10` 引用
- `agent/gate.go`：移除 `TODO #6` 引用
- `agent/plan.go`：移除 `TODO #14` 引用
- `agent/usage.go`：移除 `TODO #14` 引用，改为更准确的描述
- `main.go`：
  - 修正 thinking 模式注释（原注释称「客户端已统一关闭」，实际 thinking 已默认开启 max）
  - 移除重复的 `// 按模式分支` 注释

### 3. 移除 `delete_symbol` 死代码

**问题**：`delete_symbol` 工具在 #32 中已从 `AllBuiltInTools()` 移除（不再注册），
但 `NewDeleteSymbolTool()` 函数仍留在 `tool/all_tools.go` 中作为死代码。

**修改**：删除 `NewDeleteSymbolTool()` 函数。

### 4. 修正 TODO-reasonix.md

**问题**：清单中多处描述与代码现状不符：
- `感知层规范化（DetectMode 自动判断 Ask/Plan）` 标记为已完成，但 auto 模式已在 #35 中移除
- Reasonix 对比差距列表未更新（#21 工具接口改造已完成、#20 已完成等）

**修改**：
- 「已完成」列表更新为准确描述（如移除 DetectMode 引用、补充 ctx 贯穿、风暴检测等）
- 「暂缓/不做」列表新增「感知层自动模式」说明
- Reasonix 差距列表替换为「待改进」列表，反映真实剩余工作

### 5. 整理 `tool/all_tools.go`

**问题**：该文件所有工具挤在一起，缩进混乱，可读性差。

**修改**：
- 按职责分组（文件读写 / 编辑 / 搜索 / 任务管理 / 网络 / Git+Shell），加分隔注释
- 统一代码格式，消除压缩在一行的多条语句
- 引入 `paramBuilder` / `prop()` 辅助函数减少参数定义的冗余嵌套

### 6. 增强 `grep` 和 `glob` 工具

**`grep`**：
- 原实现使用 `strings.Contains`，不支持正则表达式
- 现改为 `regexp.Compile(pattern)` + `re.MatchString(line)`
- 当 pattern 是纯文本时行为不变（正则也匹配字面量）

**`glob`**：
- 原实现仅调用 `filepath.Glob`，不支持 `**` 递归模式
- 新增 `globRecursive()` 函数：当 pattern 包含 `**` 时，按前缀目录 Walk、后缀 Glob 匹配
- 例：`src/**/*.go` 递归搜索 `src/` 下所有 `.go` 文件

### 7. gofmt 全量格式化

对 `gofmt -l .` 报告的所有文件执行 `gofmt -w`，清零格式化欠账。
涉及文件：`agent/` 下多个文件、`command/`、`event/`、`main.go`、`skill/`、`tool/`。

## 验证

```
go build ./...    ✅
go vet ./...      ✅
go test -race ./... ✅ (全部通过)
gofmt -l .        ✅ (无输出)
```
