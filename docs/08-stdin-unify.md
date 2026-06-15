# 08 — 统一标准输入（修复 stdin 混用）

## 为什么要做

程序里同时有三个读取器盯着 `os.Stdin`：

- `getAPIKey()` 用 `bufio.NewReader`
- 主循环用 `bufio.NewScanner`
- 高危确认用 `fmt.Scanln`

`bufio` 会**预读并缓冲**，而 `fmt.Scanln` 直接读底层流。两者混用时，缓冲区里的字节
可能被一方预读走、另一方读不到——手动一行行敲通常没事，但**粘贴多行 / 管道输入**时
就会错位：确认提示读错，或把后续输入吞掉。

## 做了什么

### 1. Agent 持有唯一的输入读取器

`agent/agent.go` — Agent 结构体新增 `in *bufio.Reader` 字段；`SpawnSubAgent` 让子 Agent
共用父级的同一个 reader。

### 2. 新增统一输入模块

`agent/input.go`（新文件）：

- `stdin()` — 返回统一 reader，未注入时惰性初始化（子 Agent / 测试也可用）
- `SetInput(r)` — 注入共享 reader
- `ReadLine()` — 读一行（去行尾换行），EOF 返回 `ok=false`
- `confirm()` — 读 `[Y/n]`：空行或非 n 视为确认（默认 Yes），EOF 视为取消（高危更安全）

### 3. 全部输入收口到同一个 reader

`main.go`：

- 启动时只建一个 `stdin := bufio.NewReader(os.Stdin)`
- `getAPIKey(stdin)` 改为接收共享 reader（不再自建）
- `a.SetInput(stdin)` 注入 Agent
- 主循环用 `a.ReadLine()` 取代 `bufio.NewScanner`

`agent/run.go` — 高危确认用 `a.confirm()` 取代 `fmt.Scanln`。

### 4. 单元测试

`agent/input_test.go` — 覆盖 `ReadLine`（多行 / 无换行结尾 / CRLF）与 `confirm`
（空行默认 Yes、n/N/no 取消、带空格、EOF 取消）。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/agent.go` | 新增 `in *bufio.Reader` 字段；子 Agent 共用 |
| `agent/input.go` | 新增：统一输入读取器 + `ReadLine` / `confirm` |
| `agent/run.go` | 高危确认改用 `a.confirm()`（去掉 `fmt.Scanln`） |
| `main.go` | 共享 `stdin` reader；`getAPIKey` 接收 reader；主循环用 `a.ReadLine()` |
| `agent/input_test.go` | 新增：输入读取 / 确认的单元测试 |

## 效果

- 全程只有一个读取器盯着 `os.Stdin`，根除缓冲错位。
- 粘贴多行 / 管道输入也稳定，确认不再读错或吞掉输入。
- 输入逻辑集中、可注入，方便测试（无需真实终端）。

## 相关 TODO

> TODO-reasonix.md — 待修问题 **P1**（stdin 读取方式脆弱）
> 也兑现了 #5 Gate 的一半：输入已收口到统一抽象，将来加权限 Gate 更顺。
> 难度：★★☆☆☆
