# 09 — go.mod 卫生与版本对齐

## 为什么要做

两处不一致 / 不规范：

1. README 写「Go 1.22+」，但 `go.mod` 是 `go 1.25.5`——对不上，会让想跑的人困惑。
2. `go.mod` 里所有依赖都标了 `// indirect`，但其中 `go-openai` 和 `mcp-go` 是代码**直接
   import** 的，标注不实。

## 做了什么

### 1. 查清真实的 Go 版本下限

`go list -m -f '{{.Path}} {{.GoVersion}}' all` 显示 `github.com/mark3labs/mcp-go` 要求
**go 1.25.5**。也就是说 `go.mod` 的 `go 1.25.5` 是被依赖强制的，**不能降**——该改的是 README。

### 2. README 对齐到 1.25.5+

`README.md` — 两处「Go 1.22+」（前置条件 + 技术栈表）改为「Go 1.25.5+」。

### 3. go.mod 修正 indirect 标注

`go.mod` — 把代码直接 import 的两个依赖单独成块、去掉 `// indirect`：

- `github.com/mark3labs/mcp-go`（`mcp/client.go`）
- `github.com/sashabaranov/go-openai`（`agent/*.go`）

其余依赖保持 `// indirect`，`go 1.25.5` 不变。

> 说明：本仓库内有未跟踪的实验目录 `graph/`（含一个会编译失败的测试），直接 `go mod tidy`
> 会把它纳入分析、可能干扰结果，因此这里按 grep 出的真实直接依赖**手动精确修正**，
> 再用 `go mod verify` + `go build` + `go test` 验证一致性。

## 改动文件

| 文件 | 改动 |
|------|------|
| `go.mod` | 直接依赖（mcp-go / go-openai）去掉 `// indirect` 并单独成块；`go 1.25.5` 不变 |
| `README.md` | 「Go 1.22+」→「Go 1.25.5+」（两处） |

## 效果

- README 与 `go.mod` 一致：照着 README 装 Go 1.25.5+ 就能跑。
- `go.mod` 直接 / 间接标注如实，符合 `go mod tidy` 的规范形态。
- `go mod verify` 通过；`go build` / `go test` / `go vet` 全绿。

## 相关 TODO

> TODO-reasonix.md — 待修问题 **P2**（go.mod 与文档不一致）
> 难度：★☆☆☆☆
