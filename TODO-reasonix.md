# QiuQiuPro 优化清单（按优先级排序）

> ✅ = 已完成。

---

## ✅ 已完成

- [x] **CoT（思维链）**（DeepSeek V4 原生 thinking 替代手动提示）
- [x] **流式输出**
- [x] **Reflexion（自我反思）**
- [x] **提示词外部化**
- [x] **Ask/Plan 双模式**（手动切换，Plan 带只读调研门）
- [x] **提示词全面优化**（Reasonix 风格：场景→工具→行为）
- [x] **bash 跨平台兼容**（Windows PowerShell / macOS+Linux sh）
- [x] **P0-P3 工程问题修复**（跨轮工具结果 / stdin 收口 / go.mod 卫生 / 补测试）
- [x] **web_fetch 工具**（HTTP GET + HTML strip + 截断）
- [x] **bash 工具**（流式输出 + 退出码 + 32KB 截断）
- [x] **code_search**（按符号名搜索 Go 代码）
- [x] **/cleanup 熵管理命令**
- [x] **Gate 权限门 + 只读模式**
- [x] **Skill 人格扩展**（pm / backend_dev / tester / devops / architect / code_review / frontend_design）
- [x] **Session 独立管理**
- [x] **并行工具执行**（只读并发，写串行）
- [x] **事件驱动输出**（Event / Sink 可插拔）
- [x] **拆分 agent.go**（按职责分文件：run / plan / tools / gate / sink / hooks / memory ...）
- [x] **上下文压缩**（窗口比例 + 真实用量驱动，对前缀缓存友好）
- [x] **DeepSeek V4 迁移**（thinking enabled + max reasoning effort）
- [x] **Token 用量追踪**（缓存命中率 + 可选费用估算）
- [x] **maxSteps + 暂停恢复**（协作式 pause/resume + ExecutionState 持久化）
- [x] **Hook 机制**（BeforeToolCall / AfterToolCall 链）
- [x] **长期记忆（偏好/规则型）**（模型通过 remember_rule 自主写入）
- [x] **工具接口改造**（`Execute(ctx, args) (string, error)` + ReadOnly 字段）
- [x] **风暴检测**（连续同类错误自动打断 + 指导换方案）
- [x] **Ctrl+C 中断**（协作式 interrupt，优雅停止当前操作）
- [x] **Plan 模式只读门**（调研只读 → 方案审批 → 放行执行）
- [x] **ctx 贯穿工具执行**（Run → dispatchAndDetect → executeToolCall → Tool.Execute 全链路传递）

---

## ⏸️ 暂缓 / 不做

- **LLM Provider 抽象** — 暂缓（当前只需 DeepSeek）
- **RAG / 知识型记忆** — 不做（coding Agent 不需要）
- **感知层自动模式** — 已移除（实验后发现不如手动切换可控）

---

## 🔜 待改进

#### 工具增强
- `grep`：支持真正的正则表达式（当前是 strings.Contains）
- `glob`：支持递归 `**` 模式（当前只有 filepath.Glob 单层）
- `code_search`：升级为 AST 级别符号搜索
- `todo_write`：任务状态持久化

#### 质量改进
- Evidence 账本：记录工具执行 receipt，防止模型编造"已完成"
- 子 Agent 事件嵌套：子 Agent 输出与父级分层展示
- 更多 Hook 扩展点（PreCompact / SubagentStop / PostLLMCall）
- Compaction 归档 + 卡住检测
- finish_reason 映射

