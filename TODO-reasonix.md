# QiuQiuPro 优化清单（按优先级排序）

> ✅ = 已完成。

---

## ✅ 已完成

- [x] **CoT（思维链）**
- [x] **流式输出**
- [x] **Reflexion（自我反思）**
- [x] **提示词外部化**
- [x] **Ask/Plan 双模式**
- [x] **提示词全面优化**（system + plan + skill）
- [x] **run_shell 跨平台兼容**（Windows/macOS/Linux 自动适配）
- [x] **P0-P3 工程问题修复**（跨轮工具结果 / stdin 收口 / go.mod 卫生 / 补测试）
- [x] **web_fetch 工具**
- [x] **更好的 run_shell**（流式输出 + 退出码）
- [x] **code_search**（语义搜索，定义+引用子集）
- [x] **/cleanup 熵管理命令**
- [x] **Gate 权限门 + 只读模式**
- [x] **Skill 人格扩展**（pm / backend_dev / tester / devops）
- [x] **Session 独立管理**
- [x] **并行工具执行**（只读并发，写串行）
- [x] **事件驱动输出**（Event / Sink）
- [x] **拆分 agent.go**（按职责分文件）
- [x] **上下文压缩**（窗口比例 + 真实用量驱动）
- [x] **DeepSeek V4 迁移**（thinking + max）
- [x] **Token 用量追踪**
- [x] **maxSteps + 暂停恢复**
- [x] **Hook 机制**
- [x] **长期记忆（偏好/规则型）**
- [x] **感知层规范化**（DetectMode 自动判断 Ask/Plan）

---

## ⏸️ 暂缓 / 不做

- **LLM Provider 抽象** — 暂缓（当前只需 DeepSeek）
- **RAG / 知识型记忆** — 不做（coding Agent 不需要）
