# QiuQiuPro 开发路线图

> 当前状态：核心功能稳定，准备套 UI 显示层。
> 分三层推进：**稳定底层 → 暴露接口 → 构建上层**。

---

## 第一层：稳定底层（套 UI 前必修）

修复不依赖接口变更的缺陷，纯内部改动。

### -- 工具层 --

- [ ] **工具参数解析加 error 检查**（14 处）
      `tool/all_tools.go` 每个工具的 `Execute` 函数中 `json.Unmarshal(args, &p)` 全部忽略 error，
      LLM 传畸形参数时零值执行（如 `os.ReadFile("")`）。
      改为统一返回 `fmt.Errorf("参数解析失败：%v", err)`。
      参考 `install_tools.go` 中已有的正确写法。

- [ ] **MCP 客户端工具参数解析加 error 检查**
      `mcp/client.go:72` 同样吞掉 `json.Unmarshal` 错误。

### -- 资源管理 --

- [ ] **MCP 子进程生命周期管理**
      - 给 `MCPClient` 添加 `Close()` 方法，关闭 stdio 连接
      - `Manager.Refresh()` 时关闭旧连接再建新连接
      - `main.go` 启动时发现的工具注册后释放连接（或改为延迟连接）
        - 当前每次启动为每个 MCP Server 创建 `MCPClient` 发现工具后丢弃，子进程永久驻留

### -- Agent 稳定性 --

- [ ] **Plan mode 只读工具为空时死循环保护**
      当 `availableTools()` 中没有 ReadOnly 工具时，Plan 调研阶段 LLM 反复尝试写工具→被拒→再试。
      方案：调研前检查可用读工具数量，为 0 时提示用户安装读工具或切换模式。

- [ ] **工具输出截断的 UTF-8 安全**
      `bash` 工具 32KB 截断（`all_tools.go:601`）和 `web_fetch` 工具 16000 字节截断（`all_tools.go:543`）
      可能从多字节字符中间切分，发给 LLM 的 content 含非法 UTF-8。
      方案：截断时按 rune 边界回退，保证合法 UTF-8。

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
      - `Agent.Usage() TokenUsage` — Token 用量
      - `Agent.CurrentSkill() *skill.Skill` — 当前人格
      - `Agent.IsPlanMode() bool` — 是否规划调研中
      - `Agent.SessionID() string` — 已有

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

- [ ] **`allTools` map 并发保护**
      当前热安装/刷新与工具执行在同一 goroutine 串行发生，但代码不防未来。
      方案：换成 `sync.Map` 或加 `sync.RWMutex`。

- [ ] **`web_fetch` HTTP Client 复用**
      每次调用新建 `http.Client`，高频抓取时浪费连接。
      方案：包级别复用 `http.DefaultClient` 或全局 client。

- [ ] **`event/store.go` 错误处理**
      `Append()` 中 `json.Marshal` 和 `f.WriteString` 的错误被忽略。
      `Load()` 中 `json.Unmarshal` 错误被忽略。

- [ ] **`SaveCheckpoint` 中 `session.Snapshot()` 错误被忽略**

---

## 标记说明

- `[ ]` 待办
- `[x]` 已完成
- `[-]` 已决定不做
