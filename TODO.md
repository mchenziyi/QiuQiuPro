# QiuQiuPro 开发路线图

> 当前状态：Web UI 后端核心完成，前端界面初版可交互。
> 路线：**完善细节 → 补齐缺口 → 打磨体验**。

---

## 第一层：稳定底层 ✅（2025-07 完成）

- [x] **工具参数解析加 error 检查**（14 处 + 1 处 MCP）
- [x] **MCP 子进程生命周期管理** — 长期持有 + Close()
- [x] **Plan mode 只读工具为空时死循环保护**
- [x] **工具输出截断的 UTF-8 安全**
- [x] **跨平台路径兼容** — filepath.Dir / filepath.ToSlash

---

## 第二层：Agent 接口 ✅（Web UI 实现时补齐）

- [x] **暴露会话历史** — `Agent.SessionMessages()`
- [x] **暴露关键状态 Getter** — `SessionID()`、`CurrentMode()`、`CurrentSkillName()`、`SessionCacheStats()`、`GateName()`
- [x] **暴露会话切换/新建** — `SwitchSession()` / `ResetSession()`
- [x] **Gate 确认通道** — `SetConfirmChan()`，Web 模式替代 stdin

- [-] **单轮交互接口 `RunTurn()`** — 暂不需要，当前 `Run()` + goroutine + SSE 已满足
- [ ] **`Agent.ListTools() []tool.Tool`** — 暴露当前工具快照（影响 /api/tools）
- [ ] **`Agent.IsPlanMode() bool`** — 新增 Getter
- [ ] **`session.messages` 并发保护** — 加 RWMutex，Web 模式多 goroutine 访问

---

## 第三层：随用随修

- [ ] **`web_fetch` HTTP Client 复用** — 包级别复用 http.Client
- [ ] **`event/store.go` 错误处理** — Append/Load 忽略的错误
- [ ] **`SaveCheckpoint` 中 `session.Snapshot()` 错误被忽略**
- [ ] **文档中的旧工具名同步** — README / STRUCTURES / docs

---

## 第四层：Web UI

### 4.1 SSE 事件流 ✅

- [x] **SSESink 实现** — agent.Sink → SSE 广播，多客户端并发安全
- [x] **事件映射** — user_message / assistant_delta / reasoning_delta / tool_call / tool_result / notice / error
- [x] **补充事件** — state / confirm_request / done
- [-] **selection_hint / session_updated** — V1 暂不需要

### 4.2 HTTP 端点

- [x] **`GET /api/events`** — SSE 端点
- [x] **`POST /api/send`** — 发送消息，goroutine 执行 Run
- [x] **`POST /api/interrupt`** — 中断当前执行
- [x] **`POST /api/confirm`** — 高危操作确认
- [x] **`GET /api/state`** — 状态快照
- [x] **`GET /api/history`** — 当前会话消息列表
- [x] **`GET /api/sessions`** — 历史会话列表
- [x] **`POST /api/sessions/switch`** — 切换会话
- [x] **`POST /api/sessions/new`** — 新建会话
- [x] **静态文件服务** — go:embed index.html

- [ ] **`GET /api/tools`** — 返回当前可用工具列表

### 4.3 Agent 接口补齐

- [x] **`SessionMessages()`** — 已实现
- [x] **`SwitchSession()` / `ResetSession()`** — 已实现
- [ ] **`ListTools()`** — 阻塞 /api/tools
- [ ] **`IsPlanMode()`** — 阻塞 Status Bar
- [ ] **`Session.History()` 深拷贝** — 并发安全

### 4.4 Diff 结构化输出 ❌

- [ ] **edit_file / multi_edit / delete_range 执行成功时产出结构化 diff**
      - 格式：`{path, hunks: [{old_start, new_start, lines: [{op, text}]}]}`
      - diff 随 `tool_result` 事件下推
- [ ] **前端根据 hunks 数据渲染红绿 diff 行**

### 4.5 Gate + 确认态 ✅

- [x] confirm_request SSE 事件
- [x] /api/confirm 端点
- [x] 前端确认条（输入框上方）
- [x] Run 互斥锁防止并发会话混乱

### 4.6 前端

- [x] **go:embed 打包 index.html**
- [x] **HTML 骨架** — Sidebar + Thread + Inspector + Composer + Status Bar
- [x] **SSE 客户端** — 按事件类型渲染
- [x] **Sidebar 会话列表** — 运行中/今天/昨天/更早分组，支持切换和新建
- [x] **Status Bar** — mode / skill / session / token / cache rate
- [x] **工具行折叠** — 默认折叠，点击展开参数和结果
- [x] **确认态** — 输入框上方确认条

- [ ] **主题切换** — CSS 变量 + Auto/Dark/Light，localStorage 持久化
- [ ] **Inspector 详情面板** — 点击工具行在右侧显示详情
- [ ] **Composer 停止按钮** — 执行中显示停止按钮
- [ ] **Diff 红绿渲染** — 配合 4.4

### 4.7 验收标准

- [x] `go build` 产出单个二进制，`--web` 启动
- [x] 消息发送后 Thread 实时展示 reasoning → assistant → tool call → result
- [x] 高危工具触发确认态
- [x] Sidebar 展示历史会话，支持切换和新建
- [x] Status Bar 实时反映运行状态
- [x] 不加 `--web` 时仍是纯 CLI 交互

- [ ] 写工具结果在 Thread 中摘要 + Inspector 中完整 diff 红绿对比
- [ ] 主题切换可用

---

## 标记说明

- `[x]` 已完成
- `[ ]` 待办
- `[-]` 已决定不做

---

## 标记说明

- `[ ]` 待办
- `[x]` 已完成
- `[-]` 已决定不做
