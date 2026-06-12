# QiuQiuPro 优化清单（按优先级排序）

> 优先级从高到低，难度从易到难。
> ✅ = 已完成，其余待做。

---

## 🥇 第一梯队：高优先 + 低难度（先做，见效快）

### 1. CoT（思维链）
- 在 system prompt 加一句 "Let's think step by step"
- 收益：提升推理准确性，几乎零成本
- 难度：★☆☆☆☆
- 耗时：5 分钟

### 2. 流式输出
- 替换 CreateChatCompletion 为流式 API
- 收益：用户看到逐字输出，体验飞跃
- 难度：★★☆☆☆
- 参考：Reasonix gent.go:417 — prov.Stream()

### 3. 熵管理 — /cleanup 命令
- 扫描目录 → 列出垃圾文件（死代码、旧 .jsonl、旧 .ckpt）→ 用户确认后删除
- 收益：防止项目越来越脏
- 难度：★★☆☆☆
- 注意：只列清单不直接删，用户确认才动手

---

## 🥈 第二梯队：高优先 + 中等难度（核心能力提升）

### 4. Reflexion（自我反思）
- ExecutePlan 失败时，先让 LLM 分析失败原因，再带着反思重规划
- 接入点：gent/plan.go:155-156 — 在 RePlan 之前插入 Reflect 步骤
- 收益：Agent 越错越聪明，不再是"失败就换方案"
- 难度：★★★☆☆
- 参考：菜鸟教程推理与规划页 → Reflexion 章节

### 5. Gate 接口（权限系统）
- 用可插拔 Gate 替换硬编码的 mt.Scanln
- 收益：权限检查可扩展，未来可加 Plan Mode、远程审批
- 难度：★★★☆☆
- 参考：Reasonix gent.go:76-85 — Gate 接口

### 6. Plan Mode（只读模式）
- 拦截非只读工具调用，LLM 先出方案再动手
- 收益：安全，防止 LLM 直接改代码
- 难度：★★☆☆☆
- 参考：Reasonix gent.go:145 — planMode atomic.Bool

### 7. Skill 人格切换（扩展）
- 扩充更多 Skill：pm / backend_dev / tester / devops
- 每个角色一套 system prompt + 工具白名单
- 收益：一人一 Agent 做全栈项目
- 难度：★★☆☆☆

---

## 🥉 第三梯队：中优先 + 中等难度（架构打磨）

### 8. Session 独立管理
- messages 从 run.go 中拆出，交给独立 Session 对象
- 收益：消息管理可持久化、可恢复、可测试
- 难度：★★★☆☆
- 参考：Reasonix internal/agent/session.go

### 9. 并行工具执行
- 只读工具（read_file、grep）并行跑，写工具保持串行
- 收益：搜索/读取场景速度翻倍
- 难度：★★★☆☆
- 参考：Reasonix gent.go:507-553 — executeBatch

### 10. 事件驱动输出
- 用 Event/Sink 模式替代 debugf() 打印
- 收益：前端渲染和 CLI 输出解耦
- 难度：★★★☆☆
- 参考：Reasonix internal/agent/textsink.go

### 11. 拆分 agent.go
- 按职责分文件：executor / storm / types / agent
- 收益：代码更容易读、改、测试
- 难度：★★☆☆☆

---

## 🏅 第四梯队：中优先 + 高难度（基础设施）

### 12. LLM Provider 抽象
- 不绑死 OpenAI，支持 DeepSeek / Claude / Ollama 切换
- 收益：不锁定供应商
- 难度：★★★★☆
- 参考：Reasonix internal/provider/ 包

### 13. 上下文压缩
- 自动检测上下文长度，超限时让 LLM 总结旧消息
- 收益：处理长对话不炸
- 难度：★★★★☆
- 注意：DeepSeek 前缀缓存便宜，优先级低于其他项
- 参考：Reasonix internal/agent/compact.go

### 14. Token 用量追踪
- 记录每轮 Token 消耗，展示用量
- 收益：了解成本
- 难度：★★★☆☆
- 参考：Reasonix gent.go:368 — Usage 事件

### 15. 可配 maxSteps + 暂停恢复
- maxSteps 可配，超限返回"暂停"而非错误，可继续
- 收益：不丢失工作进度
- 难度：★★★★☆
- 参考：Reasonix gent.go:363

### 16. Hook 机制
- 工具执行前后可插拔，支持日志/通知等
- 收益：扩展 Agent 行为不入侵核心代码
- 难度：★★★★☆
- 参考：Reasonix gent.go:87-107 — ToolHooks 接口

---

## 🎯 第五梯队：长期规划（先了解，不急做）

### 17. 长期记忆（偏好型）
- 对话结束后让 LLM 分析用户偏好，持久化存储，下次自动注入
- 难度：★★★★☆
- 参考：MemGPT / Letta

### 18. RAG / 知识型长期记忆
- 引入向量数据库，文档分块存储 + 语义检索
- 难度：★★★★★
- 适用：API 文档、公司规范场景

### 19. 感知层规范化
- 统一的感知入口，未来支持图片/结构化输入
- 难度：★★★★☆
