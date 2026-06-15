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

### ✅ 8. Session 独立管理 — 已完成
- 新增 `Session` 对象（`agent/session.go`）：会话 ID + 对话历史 + 大小管理，收口 Add / Trim / BuildRequest / Snapshot / Restore
- Agent 的 `messages` 切片 + `session` 字符串 ID 合并为 `session *Session`；run.go / agent.go / helpers.go 各触点改走 Session
- `trimMessages` / `buildRequestMessages` 迁入 Session；测试改为直接测 Session，补 Snapshot/Restore 用例
- 行为零变化（全量保留 + 配对感知裁剪 + checkpoint 一致），为后续多会话打底
- 详见 `docs/17-session-object.md`
- 文件：`agent/session.go`、`agent/agent.go`、`agent/run.go`、`agent/helpers.go`、`agent/memory_test.go`
- 难度：★★★☆☆

### ✅ 9. 并行工具执行 — 已完成
- 抽出 `dispatchToolCalls`：只读工具并发执行、写/高危工具串行，结果按原始顺序回灌（配对不乱）
- `canRunParallel` 三重判定：已注册 ∧ 只读 ∧ 权限门 GateAllow 才并发；并发组绝不碰 stdin
- 新增 `isReadOnlyTool` 谓词并让 `ReadOnlyGate` 复用（「改动类工具」分类单点化，不再漂移）
- 测试以 `-race` 验证：顺序保持 / 只读真并发（屏障法）/ 写串行（峰值并发=1）/ 判定逻辑
- 详见 `docs/18-parallel-tools.md`
- 文件：`agent/run.go`、`agent/agent.go`、`agent/gate.go`、`agent/parallel_test.go`
- 难度：★★★☆☆

### ✅ 10. 事件驱动输出 — 已完成
- 新增 `Event`/`Sink`（`agent/sink.go`）：把「发生了什么」与「怎么渲染」解耦，`ConsoleSink` 为默认实现
- Agent 新增 `sink` 字段 + `SetSink` + 语义化发射器（emitToken/emitToolCall/emitToolResult/emitPrompt/noticef/debugf）
- `run.go`/`agent.go`/`plan.go` 全部打印点改走 Sink；包内唯一的 `fmt.Print*` 只剩 ConsoleSink（渲染单点化）
- `Quiet` 过滤统一到 `emit()`，由每条事件的 `Verbose` 显式决定；行为零变化
- 测试：事件流 / 安静过滤 / token 转交 / 控制台渲染（截 os.Stdout）
- 详见 `docs/19-event-sink.md`
- 文件：`agent/sink.go`、`agent/agent.go`、`agent/run.go`、`agent/plan.go`、`agent/sink_test.go`
- 难度：★★★☆☆

## 🥉 第三梯队

### ✅ 11. 拆分 agent.go — 已完成
- 265 行「杂物间」按职责拆分：`tools.go`（工具+风险分类）/ `skill.go`（人格+模式）/ `checkpoint.go`（存档恢复）
- Agent 侧的门控制方法并入 `gate.go`、事件发射方法并入 `sink.go`（领域类型与操作就近聚合）
- `agent.go` 瘦到 ~90 行（结构体 + New + accessor + SpawnSubAgent），去掉多余 json 导入
- 纯代码搬家、零行为改变；既有测试 + `-race` + `gofmt` 全绿
- 详见 `docs/20-split-agent.md`
- 文件：`agent/agent.go`、`agent/tools.go`、`agent/skill.go`、`agent/checkpoint.go`、`agent/gate.go`、`agent/sink.go`
- 难度：★★☆☆☆

### ⏸️ 12. LLM Provider 抽象 — 暂缓（当前只需支持 DeepSeek）
- 多 Provider（Claude / Ollama）切换暂无需求；保留待将来需要时再做
- 难度：★★★★☆

### ✅ 13. 上下文压缩 — 已完成
- 超限时不再直接丢弃，而是让 LLM 把旧消息总结成摘要、用「摘要 + 近消息」替换历史
- **触发时机对前缀缓存友好（按窗口占比、真实用量驱动）**：DeepSeek 按前缀匹配缓存（命中价约
  未命中 1/50），乱压缩会拉低命中率、抬高成本。故按**占模型窗口的比例**触发（soft 0.5 提醒 /
  compact 0.8 触发），并用 provider 回传的真实 `prompt_tokens` 判定，比字符估算精确；窗口默认
  贴合 DeepSeek V4 的 1M，平时几乎压不到、缓存常热
- Session 压缩原语：`CharCount` / token 预算版 `SplitForCompaction`（配对感知）/ `ApplyCompaction`
- `streamChat` 开 `IncludeUsage` 捕获用量；`tokPerChar` 按真实用量推导每字符 token 数
- Agent `maybeCompact` 编排（接入 Run 循环顶部）；摘要失败安全退化为 `Trim`；压缩后清零遥测防重复压
- 手动口子 `/compact` 命令 + `SetContextWindow` / `DEEPSEEK_CONTEXT_WINDOW` 环境变量
- 摘要器抽成可注入函数缝（默认 `llmSummarize`），测试无需联网即可全链路验证
- 详见 `docs/21-context-compaction.md`
- 文件：`agent/session.go`、`agent/compact.go`、`agent/agent.go`、`agent/run.go`、`main.go`、`agent/compact_test.go`
- 难度：★★★★☆

### ✅ 14. Token 用量追踪 — 已完成
- 按 provider 回传的真实 `usage` 累计，区分**缓存命中输入**与**思考输出** token（呼应 #13：命中越多越省，命中率一眼可见）
- 一处记账 `accountUsage`：streamChat 主循环 + plan/reflect/replan 规划 + compact 摘要全覆盖，口径与账单一致
- 每轮 Run 结束输出「本轮 token」摘要（细节日志，安静模式隐藏）；`/usage` 命令看会话累计
- 可选费用估算：`Pricing` 按缓存命中/未命中/输出三档单价，经环境变量配置；默认不显示（不编造价格）
- 子 Agent 用量在任务结束后并入父级会话总量
- 纯函数（累加/子集/命中率/费用/文案）+ httptest 流式集成测试，全程无网络
- 详见 `docs/23-token-usage.md`
- 文件：`agent/usage.go`、`agent/run.go`、`agent/plan.go`、`agent/compact.go`、`agent/agent.go`、`main.go`、`agent/usage_test.go`
- 难度：★★★☆☆

### ✅ 15. 可配 maxSteps + 暂停恢复 — 已完成
- `DEEPSEEK_MAX_STEPS` / `/maxsteps [n]` 控制一次连续计划执行最多完成多少个 step；`0` 表示不限制
- 达到 maxSteps 后协作式暂停：当前 step 完成后保存执行状态，提示 `/resume` 继续
- `/pause` 请求协作式暂停；`/resume` 从保存的 `NextStepIndex` 继续，不重新生成 Plan、不从头重跑
- 新增执行状态 sidecar：保存目标、步骤列表、下一步索引、状态、暂停原因、更新时间；Session checkpoint 仍保存消息历史
- 计划完成后自动清理执行状态；无可恢复状态时 `/resume` 返回友好错误
- 详见 `docs/24-maxsteps-hooks.md`
- 文件：`agent/execution_state.go`、`agent/plan.go`、`agent/agent.go`、`event/store.go`、`main.go`、`agent/execution_state_test.go`
- 难度：★★★★☆

### ✅ 16. Hook 机制 — 已完成（工具前后）
- 新增 `ToolHook` 接口：`BeforeToolCall` / `AfterToolCall`
- `BeforeToolCall` 可放行或拒绝工具执行；拒绝仍回灌合法 tool result，保持 tool_call/tool_result 配对
- `AfterToolCall` 可观察或改写工具结果，后续可接审计、脱敏、指标、策略限制等
- `executeToolCall` 作为唯一咽喉接入 hook 链，默认无 hook 时行为不变；子 Agent 继承父级 hooks
- 详见 `docs/24-maxsteps-hooks.md`
- 文件：`agent/hooks.go`、`agent/run.go`、`agent/agent.go`、`agent/hooks_test.go`
- 难度：★★★★☆

## 🎯 长期规划

### ✅ 17. 长期记忆（偏好/规则型）— 已完成
- 只保存偏好/规则，不保存知识型内容、代码片段、临时任务细节、日志或秘密
- 写入由模型自主判断：新增受限工具 `remember_rule`，当用户表达长期偏好、默认行为或项目规则时由模型主动调用
- 不提供 `/remember` 手动写入命令；保留 `/memory` 查看与 `/forget <id>` 删除，保证透明和可纠错
- 两层存储：全局 `~/.qiuqiu/memory.json` + 项目 `.reasonix/memory.json`
- system prompt 稳定注入“长期记忆（偏好/规则）”块；切换 Skill 后 `remember_rule` 仍可用
- 只读模式下拒绝写入记忆；默认模式下不弹高危确认，避免打断模型自主沉淀
- 详见 `docs/25-preference-memory.md`
- 文件：`agent/long_memory.go`、`agent/agent.go`、`agent/run.go`、`agent/tools.go`、`main.go`、`agent/memory_test.go`

### 🚫 18. RAG / 知识型长期记忆 — 不做
- 用户已明确：coding Agent 不需要知识型长期记忆
- 不引入向量库、embedding、知识库索引或自动保存项目知识，避免污染上下文与增加复杂度

### 19. 感知层规范化
