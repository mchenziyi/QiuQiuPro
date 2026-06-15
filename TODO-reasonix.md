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

### ✅ P2. go.mod 与文档不一致（工程卫生）— 已修复
- 查清 `mcp-go` 强制 go 1.25.5 → 保留 go.mod 版本，改 README「Go 1.22+」为「1.25.5+」（两处）
- go.mod 直接依赖（mcp-go / go-openai）去掉 `// indirect` 并单独成块
- 因未跟踪的 `graph/` 会干扰 `go mod tidy`，改为手动精确修正 + `go mod verify` 验证
- 详见 `docs/09-gomod-hygiene.md`
- 文件：`go.mod`、`README.md`

### ✅ P3. 缺少测试（工程卫生）— 已修复
- 补核心纯函数单测：agent（`IsHighRiskTool` / `truncate` / `stripCodeFence`，加上 P0/P1 的 memory/input）、tool（`edit_file_block` 4 例）；现共 15 个测试函数全绿
- 重构：抽出 `stripCodeFence`，`plan.go` 里重复 3 次的围栏清洗合一
- **附带挖出并修掉 P0 级 bug**：`edit_file_block` 结构体缺 json tag，`old_block`/`new_block` 绑空 → 永远「出现多次」，旗舰编辑工具实质不可用
- 详见 `docs/10-tests-and-edit-fix.md`
- 文件：`tool/edit_tools.go`、`tool/edit_tools_test.go`、`agent/plan.go`、`agent/agent_test.go`

---

## 🥇 第一梯队（先做，见效快）

### ✅ 1. web_fetch 工具 — 已完成
- HTTP GET 抓取 URL 内容（查文档 / API / 资料）；缺协议自动补 https
- 三重上限：超时 15s + 读取 1MB + 输出 16000 字符截断；非 2xx 仍返回正文
- 单测用 httptest，零外部依赖（normalizeURL 纯函数 + fetchURL 6 例）
- 详见 `docs/11-web-fetch.md`
- 文件：`tool/web_fetch.go`、`tool/struct.go`、`tool/web_fetch_test.go`

### ✅ 2. 更好的 run_shell — 已完成
- 流式输出：MultiWriter 一路实时打控制台、一路捕获回灌 LLM（同一 writer 无竞态）
- 退出码判定：✅/❌ + 真实退出码（errors.As 取 *exec.ExitError），区分失败 / 无法启动
- 超时保护（默认 5min，CommandContext 强制终止）+ 输出三重上限（1MB 捕获 / 16000 字符回灌）
- 详见 `docs/12-better-run-shell.md`
- 文件：`tool/shell_tools.go`、`tool/shell_tools_test.go`

### ✅ 3. code_search（语义代码搜索）— 已完成（定义+引用子集）
- 基于 go/ast 解析，按标识符搜索：定位 func/method/type/var/const 定义 + 所有引用
- 两遍遍历：先收集定义并记位置，再把同名标识符里非定义的作为引用；比 grep 准
- searchSymbolInSource 直接吃源码字节、零文件系统依赖，纯单测覆盖 5 种 kind
- 边界：未做类型解析与调用链（需 go/types 或 codegraph），留待后续
- 详见 `docs/13-code-search.md`
- 文件：`tool/code_search.go`、`tool/struct.go`、`tool/code_search_test.go`

### ✅ 4. 熵管理 — /cleanup 命令 — 已完成
- 新增可测的 cleanup 包：IsJunk / Scan / Delete / FormatList / HumanSize
- /cleanup [目录]：扫描 → 列出（含大小合计）→ Confirm 确认 → 删除并汇报
- 安全：Scan 绝不进入 .git；确认复用导出的 Agent.Confirm()（即 P1 已测 confirm）
- 详见 `docs/14-cleanup-command.md`
- 文件：`cleanup/cleanup.go`、`cleanup/cleanup_test.go`、`agent/input.go`、`main.go`

### ✅ 5. Gate 接口（权限系统）— 已完成（与 #6 合并）
- 抽出可插拔 Gate 接口，替换 executeToolCall 里硬编码的「高危即确认」
- 三实现：ConfirmHighRiskGate（默认，行为不变）/ ReadOnlyGate / AllowAllGate
- 作用于 ask + plan 同一咽喉（executeToolCall）；子 Agent 继承父级门
- 详见 `docs/15-gate-readonly.md`
- 文件：`agent/gate.go`、`agent/run.go`、`agent/agent.go`、`main.go`、`agent/gate_test.go`

### ✅ 6. Plan Mode（只读模式）— 已完成（与 #5 合并）
- ReadOnlyGate 拦截一切改动类工具（写 / 编辑 / 执行 / 提交），读类放行；拒绝时回灌引导模型改用只读手段
- /readonly on|off 切换；主循环提示加 🔒 标识
- 边界：MCP 外部写工具无法自动识别（需工具读写元数据）
- 详见 `docs/15-gate-readonly.md`

## 🥈 第二梯队

### ✅ 7. Skill 人格切换（扩展）— 已完成
- 新增 4 个 Skill：pm / backend_dev / tester / devops，工具白名单按角色收窄
- 新增校验测试：加载所有 Skill + 必填字段 + tool_whitelist 工具名真实存在（防笔误被静默过滤）
- 详见 `docs/16-skill-personas.md`
- 文件：`prompt/skills/{pm,backend_dev,tester,devops}.json`、`agent/skill_bundle_test.go`

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
