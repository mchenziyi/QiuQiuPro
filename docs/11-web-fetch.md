# 11 — web_fetch 工具（抓取 URL 内容）

## 为什么要做

Agent 此前只能读本地文件 / 跑命令，**没法上网查资料**。查个库的文档、看个 API 返回、读篇文章
都做不到，遇到不懂的只能瞎猜。`web_fetch` 给它补上「联网读取」这一最实用的能力。

（TODO-reasonix.md 功能清单 #1，第一梯队、★☆☆☆☆。）

## 做了什么

### 1. 新增 web_fetch 工具

`tool/web_fetch.go` — 一个标准 GET 工具，参数只有 `url`：

- **缺协议自动补全**：`example.com` → `https://example.com`（`normalizeURL`）。
- **整次超时 15s**：`http.Client{Timeout}`，不让一个卡住的请求拖死事件循环。
- **限制读取 1MB**：`io.LimitReader`，防超大响应撑爆内存。
- **输出截断 16000 字符**：返回给 LLM 的正文超限就截断并标注，避免污染上下文窗口。
- **伪装 User-Agent**：部分站点拒绝默认 Go UA，设成常见浏览器串更稳。
- **错误友好**：URL 无效 / 请求失败 / 读取失败 / 非 2xx，都返回可读的中文说明（非 2xx 仍带回正文，方便看 404 页等）。

### 2. 结构体带 json tag

吸取 P3 里 `edit_file_block` 的教训，参数结构体显式写 `json:"url"`，不靠隐式匹配。

### 3. 注册进内置工具

`tool/struct.go` 的 `AllBuiltInTools()` 加入 `NewWebFetchTool()`，启动即可用。

### 4. 单测（httptest，不依赖真联网）

把 URL 规范化抽成纯函数 `normalizeURL` 单独测；`fetchURL` 用 `httptest` 起本地 server 覆盖：

| 用例 | 验证 |
|------|------|
| `TestNormalizeURL` | 补协议 / 原样 / 去空白 / 空串 |
| `TestFetchURL_Success` | 正常 200，返回正文 + 状态 |
| `TestFetchURL_EmptyURL` | 空 url 提示 |
| `TestFetchURL_Truncates` | 超长正文截断且不超上限 |
| `TestFetchURL_Non2xx` | 404 也返回状态 + 正文 |
| `TestFetchURL_ConnError` | 连接失败提示「抓取失败」（起 server 拿地址后立刻关） |

## 改动文件

| 文件 | 改动 |
|------|------|
| `tool/web_fetch.go` | 新增：web_fetch 工具 + `normalizeURL` / `fetchURL` |
| `tool/struct.go` | `AllBuiltInTools()` 注册 `NewWebFetchTool()` |
| `tool/web_fetch_test.go` | 新增：6 个单测（httptest，零外部依赖） |

## 效果

- Agent 能联网查文档 / API / 资料了，遇到不确定的可以自己去读一手资料。
- 有超时 + 大小 + 输出三重上限，单个请求不会拖死主循环、不会撑爆内存或污染上下文。
- `go build` / `go test ./agent/ ./tool/` / `go vet` 全绿；tool 包测试数从 4 增至 10。

## 已知边界（后续可加）

- 只返回**原始正文**（HTML 不抽正文）；要「干净阅读体验」需再加 HTML→文本提取。
- 未做 SSRF 限制（能访问内网地址）；如果将来跑在不可信环境，应配合 #5 Gate 加访问白名单。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#1 web_fetch 工具**
> 难度：★☆☆☆☆
