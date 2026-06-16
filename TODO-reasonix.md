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



---

## 🆕 Reasonix 对比发现的差距（2025-06-16）

> 对比分析详见 docs/29-reasonix-gap-analysis.md

### 🥇 优先做

#### 20. 提示词全面优化（参照 Reasonix）
- system prompt：从"身份声明"改为"行为指令 + 工具使用场景"
- plan prompt：细化步骤粒度判断 + 依赖感知
- 词库补齐：complexIntentTerms 补漏
- 难度：★★★☆☆

#### 21. 工具接口改造（ctx + error 返回）
- `Execute func(args string) string` → `Execute(ctx, args) (string, error)`
- 没了 context，工具不能超时取消；没了 error，错误靠字符串猜
- 难度：★★★★☆（所有工具都要改签名）

#### 22. 工具输出截断（32KB cap）
- 一次 read_file 可能灌爆上下文窗口
- 参照 Reasonix：head+tail 截断 + 引导 LLM 如何重取
- 难度：★★☆☆☆

### 🥈 后续做

#### 23. Evidence 账本
- 记录每次工具执行的 receipt，防止模型编造"已完成"
- 难度：★★★★☆

#### 24. 子 Agent 事件嵌套
- 子 Agent 工具调用和父级混在一起，输出一团乱
- 难度：★★★☆☆

#### 25. 更多 Hook 扩展点（PreCompact/SubagentStop/PostLLMCall）
- 难度：★★★☆☆

#### 26. Compaction 归档 + 卡住检测
- 压缩后旧消息直接丢 → 写入 archive.jsonl
- 连续压缩无法降低 prompt → 暂停并警告
- 难度：★★☆☆☆

#### 27. finish_reason 映射
- 模型被截断/内容过滤了都不知道
- 难度：★☆☆☆☆
