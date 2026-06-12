# QiuQiuPro 优化清单（按优先级排序）

> ✅ = 已完成，其余待做。

---

## ✅ 已完成

- [x] **CoT（思维链）** — system prompt 加入 CoT 引导
- [x] **流式输出** — CreateChatCompletionStream 实时显示
- [x] **Reflexion（自我反思）** — 失败先反思原因再重规划
- [x] **提示词外部化** — 所有硬编码 prompt 抽到 prompt/ 目录
- [x] **Ask/Plan 双模式** — /mode 命令切换，避免闲聊浪费 Token

---

## 🥇 第一梯队（先做，见效快）

### 1. 熵管理 — /cleanup 命令
- 扫描目录 → 列出垃圾文件（死代码、旧 .jsonl、旧 .ckpt）→ 用户确认后删除
- 难度：★★☆☆☆

### 2. Gate 接口（权限系统）
- 用可插拔 Gate 替换硬编码的 fmt.Scanln
- 难度：★★★☆☆

### 3. Plan Mode（只读模式）
- 拦截非只读工具调用，LLM 先出方案再动手
- 难度：★★☆☆☆

## 🥈 第二梯队

### 4. Skill 人格切换（扩展）
- 扩充更多 Skill：pm / backend_dev / tester / devops
- 难度：★★☆☆☆

### 5. Session 独立管理
- messages 从 run.go 中拆出，交给独立 Session 对象
- 难度：★★★☆☆

### 6. 并行工具执行
- 只读工具并行跑，写工具保持串行
- 难度：★★★☆☆

### 7. 事件驱动输出
- 用 Event/Sink 模式替代 debugf() 打印
- 难度：★★★☆☆

## 🥉 第三梯队

### 8. 拆分 agent.go
- 按职责分文件
- 难度：★★☆☆☆

### 9. LLM Provider 抽象
- 支持 DeepSeek / Claude / Ollama 切换
- 难度：★★★★☆

### 10. 上下文压缩
- 超限时让 LLM 总结旧消息
- 难度：★★★★☆

### 11. Token 用量追踪
- 难度：★★★☆☆

### 12. 可配 maxSteps + 暂停恢复
- 难度：★★★★☆

### 13. Hook 机制
- 工具执行前后可插拔
- 难度：★★★★☆

## 🎯 长期规划

### 14. 长期记忆（偏好型）
### 15. RAG / 知识型长期记忆
### 16. 感知层规范化
