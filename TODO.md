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

## 第三层：随用随修

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
