# 🏀 QiuQiuPro 核心数据结构速查

> 不需要读全部代码，只需要懂这些结构体和它们的设计意图。

---

## tool 包 — 你只需要懂这一个结构体

```go
type Tool struct {
    Name        string             // LLM 调用的工具名（必须唯一）
    Description string             // 告诉 LLM 什么时候用这个工具
    Parameters  any                // 参数定义（JSON Schema 格式）
    Execute     func(string) string // 执行函数：接收 JSON 参数，返回 LLM 友好的结果
}
```

**设计意图：** 所有工具都是这个模板的实例化。符合这个签名就能注册进 Agent。

**核心方法：**

| 函数 | 作用 |
|------|------|
| `AllBuiltInTools()` | 返回所有内置工具列表，main.go 调用它一次性注册 |

**三个固定套路（每个 Execute 都这三步）：**
1. 定义参数结构体 → `var p struct{ Path string \`json:"path"\` }`
2. `json.Unmarshal` 解析参数
3. 执行操作，返回 LLM 友好的文本

---

## event 包 — 你只需要懂这一个结构体

```go
type Event struct {
    ID        string    // 唯一标识（sessionID_时间戳）
    Type      string    // 事件类型：user / assistant / tool_call / tool_result / error
    Content   string    // 事件内容
    ToolName  string    // 工具名（tool_call 和 tool_result 时有值）
    Timestamp time.Time // 发生时间
}
```

**设计意图：** 事件不可变，只追加不修改。这就是 Event Sourcing 的本质。

**核心方法：**

| 方法 | 作用 |
|------|------|
| `Append(sessionID, event)` | 在 JSON Lines 文件末尾追加一行 |
| `Load(sessionID)` | 按行读取整个文件，返回事件列表 |
| `Replay(sessionID, events)` | 格式化事件列表为可读的对话记录 |

**存储格式：** 每行一个 JSON，追加写入，不修改已有内容。

---

## mcp 包 — 你只需要懂这两个结构

```go
// MCPClient 包装一个外部 MCP Server 的连接
type MCPClient struct {
    Name   string         // Server 名称，用作工具名前缀
    client *client.Client // MCP 协议客户端（通过 stdio 通信）
}
```

**核心方法：**

| 方法 | 作用 |
|------|------|
| `Connect(name, command, args...)` | 启动外部进程，建立 MCP 连接 |
| `DiscoverTools()` | 让 Server 返回所有工具，包装成 tool.Tool 格式 |

**设计意图：** 外部工具即插即用，不改代码，不重新编译。

**通信方式：** JSON-RPC over stdio（标准输入输出传 JSON 消息）。

---

## skill 包 — 你只需要懂这一个结构体

```go
type Skill struct {
    Name          string   // 技能名，用户通过 use <name> 切换
    Description   string   // 一句话说明，提示用户这个模式是做什么的
    SystemPrompt  string   // 专业提示词——LLM 的行为核心
    ToolWhitelist []string // 该 Skill 能用的工具名列表（空 = 全部可用）
    Rules         []Rule   // 行为规则
}

type Rule struct {
    Name        string // 规则名，如"必须有 ADR"
    Description string // 规则说明
}
```

**设计意图：** Skill ≠ 插件（MCP 才是）。Skill = SystemPrompt + 工具白名单。切换 Skill 就是换人格。

**核心方法：**

| 函数 | 作用 |
|------|------|
| `AllBuiltInSkills()` | 返回所有内置 Skill 列表 |

---

## agent 包 — 核心结构体

```go
type Agent struct {
    client       *openai.Client      // LLM 客户端（go-openai 封装的 HTTP 调用）
    model        string              // 模型名，如 "deepseek-v4-flash"
    allTools     map[string]Tool     // 全部注册的工具（不随 Skill 改变）
    activeTools  []string            // 当前 Skill 允许的工具（nil = 全部可用）
    messages     []ChatMessage       // 对话历史（跨多轮对话累积）
    store        *event.Store        // 事件存储（JSON Lines）
    session      string              // 当前会话 ID
    currentSkill *Skill              // 当前激活的 Skill
    sysPrompt    string              // 当前 SystemPrompt
}
```

### 核心方法职责

| 方法 | 做了什么 | 对应阶段 |
|------|---------|---------|
| `New(apiKey, model)` | 创建 Agent 实例 | - |
| `RegisterTool(t)` | 注册工具到 allTools | V0 |
| `RegisterTools(tools)` | 批量注册 | V0 |
| `RegisterMCPTools(prefix, tools)` | 注册 MCP 工具（加前缀） | V4 |
| `ApplySkill(s)` | 切换 Skill：换 SystemPrompt + 限制工具 | V5 |
| `Run(ctx, input)` | **核心循环**：调 LLM → 执行 ToolCall → 再调 LLM → 直到返回 | V0 |
| `GeneratePlan(ctx, goal)` | 让 LLM 拆解目标为步骤列表 | V1 |
| `ExecutePlan(ctx, plan)` | 按顺序执行每一步 | V1 |
| `TrimMessages()` | 截断历史（超 100 条丢最早的） | V0 |
| `EventStore()` | 返回事件存储实例 | V3 |
| `SessionID()` | 返回当前会话 ID | V3 |

---

## 核心流程一图流

```text
用户输入
    ↓
GeneratePlan() → 拆成步骤 [1.读文件, 2.分析, 3.输出]
    ↓
ExecutePlan()
    ├── Step 1 → Run("请执行：读文件")
    │               ├── 构建 messages（+ SystemPrompt）
    │               ├── 调 LLM
    │               ├── ↓ 有 ToolCall？ → 执行 → 结果喂回去 → 再调 LLM
    │               └── ↓ 没 ToolCall → 返回最终答案
    ├── Step 2 → Run("请执行：分析")
    └── Step 3 → Run("请执行：输出")
    ↓
TrimMessages() → 截断历史
```

---

## 你现在的位置

> **你知道每个包的数据结构，也知道它们之间怎么连接。剩下的代码只是这些结构的填充和实现细节。**
>
> 加新能力 = 定义新结构体 + 实现必要方法 + 注册进 Agent。
>
> 这就是你从 V0 到 V5 练出来的架构能力。
