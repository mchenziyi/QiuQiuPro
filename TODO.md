# QiuQiuPro 开发路线图

> 当前状态：核心功能稳定，准备套 UI 显示层。
> 分三层推进：**稳定底层 → 暴露接口 → 构建上层**。

---

## 第一层：稳定底层 ✅（2025-07 完成）

- [x] **工具参数解析加 error 检查**（14 处 + 1 处 MCP）
- [x] **MCP 子进程生命周期管理** — 长期持有 + Close()
- [x] **Plan mode 只读工具为空时死循环保护**
- [x] **工具输出截断的 UTF-8 安全**

---

## 第二层：接口暴露（套 UI 时一起做）

为 UI 层暴露必要的 Agent 内部状态。只加公开方法/字段，不改现有行为。

### -- 读取状态 --

- [ ] **暴露会话历史**
      - 新增 `Session.History() []openai.ChatCompletionMessage` 返回深拷贝
      - UI 可展示完整对话列表

- [ ] **暴露工具列表**
      - 新增 `Agent.ListTools() []tool.Tool` 返回当前可用工具的快照
      - UI 可展示当前可用工具和描述

- [ ] **暴露关键状态 Getter**
      - `Agent.Usage() TokenUsage` — 已有
      - `Agent.CurrentSkill() *skill.Skill` — 当前人格
      - `Agent.IsPlanMode() bool` — 是否规划调研中
      - `Agent.SessionID() string` — 已有
      - 执行时只补缺失 Getter，避免重复实现已有方法。

### -- 控制接口 --

- [ ] **单轮交互接口**
      `Run()` 是整个 tool-call 循环阻塞到底。UI 需要逐步执行：
      - 新增 `RunTurn(ctx, input) (*TurnResult, error)` — 只执行一轮 LLM 调用（含 tool calls），
        返回本轮结果和是否有后续，让 UI 可以"每步审核"或"逐步展示"
      - 内部将 `streamChat` 从私有方法提升或重构

- [ ] **进度/状态回调**
      `Sink` 是单向推送，UI 需要知道当前处于什么阶段：
      - 在 `Sink` 中增加阶段标记事件（如 `EventPhaseStart` / `EventPhaseEnd`）
      - 或提供一个独立的 `StatusListener` 接口

---

## 第四层：Web UI 后端（设计稿驱动）

基于 `docs/ui-spec.md` 和 OpenPencil 设计稿实现。输出：单个 Go 二进制 + `--web` 启动。

### 4.1 SSE 事件流 — 核心实时通道

- [ ] **实现 `SSESink`**
      - 实现 `agent.Sink` 接口
      - 内部持有一组 `http.ResponseWriter`（支持多标签页）
      - `Emit(ev)` 将 `agent.Event` 转为 SSE 事件写入所有连接
- [ ] **将现有 Sink 事件类型映射到 SSE 事件**
      - `EventInput` → `user_message`
      - `EventToken`（普通）→ `assistant_delta`
      - `EventToken`（Suffix=thinking）→ `reasoning_delta`
      - `EventToolCall` → `tool_call`
      - `EventToolResult` → `tool_result`
      - `EventUsage` → `usage`
      - `EventNotice` → `notice`
      - `EventError` → `error`
- [ ] **补充缺失的事件类型**
      - `state` 事件：连接时和状态变化时推送 `{mode, skill, session_id, running}`
      - `assistant_done` 事件：assistant 流式暂完时推送完整文本
      - `confirm_request` 事件：Gate 拦截时推送 `{tool_name, arguments, reason}`
      - `done` 事件：本轮结束时推送
      - `session_updated` 事件：会话切换/新建时广播
      - `selection_hint` 事件：提示 UI 自动聚焦最新工具或 diff

### 4.2 HTTP 端点

- [ ] **`GET /api/events`** — SSE 端点
- [ ] **`POST /api/send`** — 接收 `{text}`，启动 `go Agent.Run(ctx, text)`，SSE 接力输出
- [ ] **`POST /api/interrupt`** — 调用 `Agent.Interrupt()`
- [ ] **`POST /api/confirm`** — 发送 `{approve: bool}` 到 Gate 确认通道
- [ ] **`GET /api/state`** — 返回 mode / skill / session_id / cache rate / running 快照
- [ ] **`GET /api/history`** — 返回当前会话消息列表
- [ ] **`GET /api/tools`** — 返回当前可用工具列表
- [ ] **`GET /api/sessions`** — 列出历史会话
- [ ] **`POST /api/sessions/switch`** — 切换会话 `{session_id}`
- [ ] **`POST /api/sessions/new`** — 新建会话
- [ ] **`GET /api/sessions/{id}/history`** — 获取指定会话历史
- [ ] **静态文件服务** — 用 `go:embed` 内嵌前端单 HTML 文件，`GET /` 渲染

### 4.3 Agent 接口补齐（第二层具体化）

- [ ] **`Agent.IsPlanMode() bool`** — 新增 Getter
- [ ] **`Agent.ListTools() []tool.Tool`** — 暴露当前工具快照
- [ ] **`Session.History() []openai.ChatCompletionMessage`** — 深拷贝版本
- [ ] **`Agent.Usage()`** — 确保返回完整 `TokenUsage` struct
- [ ] **`session.messages` 并发保护** — 由第三层提升至必做，接入 RWMutex

### 4.4 工具层：Diff 结构化输出

- [ ] **edit_file / multi_edit / delete_range 执行成功时产出结构化 diff**
      - 格式：`{path, hunks: [{old_start, new_start, lines: [{op, text}]}]}`
      - diff 随 `tool_result` 事件下推
- [ ] **前端根据 hunks 数据渲染红绿 diff 行**

### 4.5 Gate + 确认态集成

- [ ] **Gate 接入 SSE confirm_request 事件**
      - 确认拦截时发出 `confirm_request` 事件并阻塞等待
      - `POST /api/confirm` 接收结果后放行或拒绝

### 4.6 前端单 HTML 文件

- [ ] **用 `go:embed` 打包 `index.html`**，内嵌 CSS + JS，零构建工具
- [ ] **HTML 骨架**：左侧 Sidebar + 中间 Thread + 右侧 Inspector + 底部 Composer + 顶部 Status Bar
- [ ] **SSE 客户端**：`EventSource` 连接 `/api/events`，按事件类型渲染
- [ ] **Composer**：Enter 发送，Shift+Enter 换行，执行中显示停止按钮
- [ ] **Inspector 详情面板**：工具参数、结果、Diff 红绿对比
- [ ] **Sidebar 会话列表**：按运行中/最近/归档分组，支持切换和新建
- [ ] **Status Bar**：mode / skill / session / token / cache rate
- [ ] **主题切换**：CSS 变量 + `Auto/Dark/Light`，`localStorage` 持久化
- [ ] **Diff 渲染**：根据 hunks 数据渲染红绿行，不依赖外部库

### 4.7 验收标准

- [ ] `go build` 产出单个二进制，`--web` 启动后浏览器可访问
- [ ] 消息发送后 Thread 实时展示 reasoning → assistant → tool call → result
- [ ] 写工具结果在 Thread 中摘要 + Inspector 中完整 diff 红绿对比
- [ ] 高危工具触发确认态，Inspector 展示详情并支持 Approve / Reject
- [ ] Sidebar 展示历史会话，支持切换和新建
- [ ] Status Bar 实时反映运行状态
- [ ] 主题切换可用
- [ ] 不加 `--web` 时仍是纯 CLI 交互，零影响

不阻塞 UI，但值得做。

- [ ] **`session.messages` 并发保护**
      当前架构串行访问不出问题，但 `messages` 裸 slice 无锁且 `Messages()` 返回内部引用。
      方案：加 `sync.RWMutex`，`Messages()` 返回深拷贝。
      一旦开始做 UI / `RunTurn` / 后台任务，这项应提升到第二层必做。

- [ ] **`allTools` map 并发保护**
      当前热安装/刷新与工具执行在同一 goroutine 串行发生，但代码不防未来。
      方案：换成 `sync.Map` 或加 `sync.RWMutex`。
      如果 UI 支持运行中安装 Skill/MCP 或刷新工具，这项应提升到第二层必做。

- [ ] **`web_fetch` HTTP Client 复用**
      每次调用新建 `http.Client`，高频抓取时浪费连接。
      方案：包级别复用 `http.DefaultClient` 或全局 client。

- [ ] **`event/store.go` 错误处理**
      `Append()` 中 `json.Marshal` 和 `f.WriteString` 的错误被忽略。
      `Load()` 中 `json.Unmarshal` 错误被忽略。

- [ ] **`SaveCheckpoint` 中 `session.Snapshot()` 错误被忽略**

- [ ] **文档中的旧工具名同步**
      README / STRUCTURES / 历史 docs 中可能仍有 `list_directory`、`run_shell`、`run_powershell`
      等旧工具名或旧文件结构说明。实现 UI 前统一校准文档，避免用户按旧名称测试。

---

## 标记说明

- `[ ]` 待办
- `[x]` 已完成
- `[-]` 已决定不做
