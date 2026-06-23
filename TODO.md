# QiuQiuPro 开发路线图

> 状态：Web UI 可交互，但有多项已知 bug 和功能缺口。

---

## 🔴 已知 Bug（优先修复）

- [ ] **Thread 消息顺序错乱** — 「模型的对话跑到最上面去了」
      助理回复的文本出现在工具调用之前。根因：`streamChat` 在流式过程中先发 `assistant_delta`，
      而 `tool_call` / `tool_result` 在 `streamChat` 返回后才发。
      → 需要前端或后端调整事件发射顺序。

- [ ] **thinking + 工具折叠组未生效** — `turn-group` 容器未正确渲染。
      上一轮改动的 `ensureTurnGroup` / `appendThinking` / `addToolRow` 未按预期工作。
      → 需要 debug 前端分组逻辑。

- [ ] **Markdown 渲染未生效** — `appendAssistant` 和 `addMsg('assistant')` 的 Markdown 渲染未显示效果。
      → 需要验证 `renderMarkdown` 函数是否被调用、CSS 样式是否正确。

- [ ] **Plan 模式在 Web 下审批流程不完整**
      - 调研阶段可运行，但生成方案后确认框缺少详细步骤展示
      - `GeneratePlan` / `ReviewPlan` 是同步 LLM 调用，耗时较长且无进度反馈
      - 用户容易误以为卡死而切换模式

- [ ] **mode 切换偶发失效** — 快速多次点击 mode 标签可能导致状态不同步。

---

## 🟡 功能缺口

- [ ] **`Agent.IsPlanMode()` — Status Bar 显示 `plan_mode` 状态**
      `state` 事件已含 `plan_mode` 字段，但前端 Status Bar 未展示。

- [ ] **`/api/tools` + `ListTools()` — 工具列表 API**
      Agent 端 `ListTools()` 已实现，但 HTTP 端点和前端展示未完成。

- [ ] **工具执行缺少超时机制**
      `executeToolCall` 没有 context timeout，MCP 工具卡住会阻塞整个 Agent。

- [ ] **风暴检测可靠性**
      `isErrorResult` 靠字符串关键词判断，不够可靠。

- [ ] **`all_tools.go` 文件过大（~22000 字节）**
      15 个工具实现在一个文件里，建议按职责拆分。

- [ ] **`main.go` 命令注册臃肿（~20000 字节）**
      大量命令 Handler 在 main.go 里，建议拆分。

---

## 🔵 前端体验优化

- [ ] **执行过程组默认折叠状态** — `turn-group` 默认展开，但内容太长时应可记住用户偏好。
- [ ] **Assistant 回复的代码语法高亮** — Markdown 渲染的 `<code>` 块加语言标识和浅色高亮。
- [ ] **Thread 消息分组** — 用户消息 + 执行过程 + 助理回复 在视觉上分组更清晰。
- [ ] **Composer 高度自适应** — 多行输入时自动增长。
- [ ] **主题切换增强** — `data-theme="auto"` 跟随系统偏好。

---

## 🟢 内部质量

- [ ] **`session.messages` 并发保护** — 加 RWMutex，防多 goroutine 并发写。
- [ ] **`web_fetch` HTTP Client 复用** — 包级别复用 `http.Client`，减少连接浪费。
- [ ] **`event/store.go` 错误处理** — `Append()` / `Load()` 中忽略的错误。
- [ ] **`SaveCheckpoint` 中 `session.Snapshot()` 错误被忽略**。
- [ ] **文档旧工具名同步** — README / STRUCTURES / docs 中的旧工具名。

---

## 标记说明

- `[ ]` 待办
- `[x]` 已完成
- `[-]` 已决定不做
