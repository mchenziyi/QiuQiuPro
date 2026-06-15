# QiuQiuPro 优化清单（按优先级排序）

> ✅ = 已完成，其余待做。

---

## ✅ 已完成

- [x] **CoT（思维链）**
- [x] **流式输出**
- [x] **Reflexion（自我反思）**
- [x] **提示词外部化**
- [x] **Ask/Plan 双模式**
- [x] **提示词全面优化**（system + plan + skill）
- [x] **run_shell 跨平台兼容**（Windows/macOS/Linux 自动适配）

---

## 🐛 待修问题 / 工程卫生

> 以下是**现有代码已存在的问题**，不是新功能。建议在堆功能前先处理（尤其 P0）。
> 独立于下方 1–19 的功能清单单独编号。

### ✅ P0. 跨轮丢失工具结果（正确性）— 已修复
- 方案：**全量保留工具链**（参照 Reasonix，append-only、永不删消息）
- `Run()` 把 user / assistant(tool_calls) / 每条 tool 结果全量写进 `a.messages`；`trimMessages` 改为配对感知
- 体积控制（prune 原地 elide / compact 摘要）留给 #13 上下文压缩
- 详见 `docs/07-full-tool-history.md`
- 文件：`agent/run.go`、`agent/helpers.go`、`agent/memory_test.go`

### ✅ P1. stdin 读取方式脆弱（健壮性）— 已修复
- 方案：Agent 持唯一 `bufio.Reader`，主循环 / 确认 / 读 API Key 全部收口到它
- 去掉 `fmt.Scanln` 与 `bufio.Scanner` 混用；新增 `ReadLine()` / `confirm()` 并加单测
- 顺带兑现 #5 Gate 的一半（输入已抽象，便于将来加权限 Gate）
- 详见 `docs/08-stdin-unify.md`
- 文件：`agent/agent.go`、`agent/input.go`、`agent/run.go`、`main.go`、`agent/input_test.go`

### P2. go.mod 与文档不一致（工程卫生）
- README 写 Go 1.22+，`go.mod` 实为 `go 1.25.5`，对不上
- 依赖全标 `// indirect`，但实际为直接依赖
- 修复：对齐版本说明 + 跑 `go mod tidy`

### P3. 缺少测试（工程卫生）
- 有 `/test` 命令但仓库无任何 `_test.go`
- 建议：先给核心循环补单测（`Run` / `GeneratePlan` / `streamChat` 的 toolcall 增量拼装）

---

## 🥇 第一梯队（先做，见效快）

### 1. web_fetch 工具
- HTTP GET 请求，抓取网页内容（查文档、查 API、搜 Stack Overflow）
- 难度：★☆☆☆☆ | 最实用，一行 HTTP 请求
- 参考：Go `net/http` 包

### 2. 更好的 run_shell
- 交互式执行：能看中间输出、跑耗时命令时边跑边显示
- 退出码分析：工具退出后能自动分析输出判断是否成功
- 难度：★★★☆☆

### 3. code_search（语义代码搜索）
- 不是 grep，而是找到符号定义/引用/调用链
- 可以用 go/* 包解析 AST 或集成 codegraph
- 难度：★★★☆☆

### 4. 熵管理 — /cleanup 命令
- 扫描目录 → 列出垃圾文件 → 用户确认后删除
- 难度：★★☆☆☆

### 5. Gate 接口（权限系统）
- 用可插拔 Gate 替换硬编码的 fmt.Scanln
- 难度：★★★☆☆

### 6. Plan Mode（只读模式）
- 拦截非只读工具调用，LLM 先出方案再动手
- 难度：★★☆☆☆

## 🥈 第二梯队

### 7. Skill 人格切换（扩展）
- 扩充更多 Skill：pm / backend_dev / tester / devops
- 难度：★★☆☆☆

### 8. Session 独立管理
- messages 从 run.go 中拆出，交给独立 Session 对象
- 难度：★★★☆☆

### 9. 并行工具执行
- 只读工具并行跑，写工具保持串行
- 难度：★★★☆☆

### 10. 事件驱动输出
- 用 Event/Sink 模式替代 debugf() 打印
- 难度：★★★☆☆

## 🥉 第三梯队

### 11. 拆分 agent.go
- 按职责分文件
- 难度：★★☆☆☆

### 12. LLM Provider 抽象
- 支持 DeepSeek / Claude / Ollama 切换
- 难度：★★★★☆

### 13. 上下文压缩
- 超限时让 LLM 总结旧消息
- 难度：★★★★☆

### 14. Token 用量追踪
- 难度：★★★☆☆

### 15. 可配 maxSteps + 暂停恢复
- 难度：★★★★☆

### 16. Hook 机制
- 工具执行前后可插拔
- 难度：★★★★☆

## 🎯 长期规划

### 17. 长期记忆（偏好型）
### 18. RAG / 知识型长期记忆
### 19. 感知层规范化
