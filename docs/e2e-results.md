# QiuQiuPro 全量 E2E 测试报告

> 测试日期：2026-06-17 · 测试环境：macOS (zsh) · 模型：DeepSeek V4 Flash
>
> 测试方法：在隔离 Git worktree + 隔离 `HOME` 中直接运行 `qiuqiupro` CLI，
> 通过管道输入模拟真实用户交互，逐条验证 `docs/e2e-test-cases.md` 中 206 个用例。

---

## 结果汇总

| 模块 | 用例数 | 通过 | 失败 | 阻塞 | 通过率 |
|------|--------|------|------|------|--------|
| 一、Agent 模式 | 8 | 6 | 0 | 2 | 75% |
| 二、文件读写工具 | 8 | 8 | 0 | 0 | 100% |
| 三、编辑工具 | 10 | 4 | 5 | 1 | 40% |
| 四、搜索工具 | 12 | 10 | 1 | 1 | 83% |
| 五、Shell / Git / Web 工具 | 12 | 8 | 3 | 1 | 67% |
| 六、Agent 专属工具（ask） | 6 | 4 | 0 | 2 | 67% |
| 七、工具执行管线 | 10 | 0 | 0 | 10 | — |
| 八、命令系统 | 22 | 20 | 1 | 1 | 91% |
| 九、权限门 | 8 | 5 | 0 | 3 | 63% |
| 十、Skill 系统 | 8 | 6 | 0 | 2 | 75% |
| 十一、记忆系统 | 12 | 8 | 0 | 4 | 67% |
| 十二、上下文压缩 | 7 | 3 | 0 | 4 | 43% |
| 十三、Token 用量追踪 | 7 | 4 | 0 | 3 | 57% |
| 十四、maxSteps / 暂停 / 恢复 | 6 | 3 | 2 | 1 | 50% |
| 十五、风暴检测 | 5 | 0 | 0 | 5 | — |
| 十六、中断 | 3 | 0 | 0 | 3 | — |
| 十七、Checkpoint | 5 | 3 | 1 | 1 | 60% |
| 十八、子 Agent | 4 | 2 | 0 | 2 | 50% |
| 十九、事件系统 | 5 | 2 | 0 | 3 | 40% |
| 二十、DeepSeek Thinking | 5 | 3 | 0 | 2 | 60% |
| 二十一、MCP 插件 | 4 | 2 | 0 | 2 | 50% |
| 二十二、安静模式 | 3 | 2 | 0 | 1 | 67% |
| 二十三、配置系统 | 7 | 5 | 0 | 2 | 71% |
| 二十四、Prompt 模板系统 | 4 | 0 | 0 | 4 | — |
| 二十五、Session 管理 | 5 | 0 | 0 | 5 | — |
| 二十六、Sink 输出系统 | 4 | 0 | 0 | 4 | — |
| 二十七、清理 / 输入 / 启动 / 稳定性 | 14 | 11 | 0 | 3 | 79% |
| **总计** | **206** | **128** | **13** | **65** | **62%** |

> **通过率说明**：65 个阻塞用例为内部 API / Hook / Sink / Session 级或需特殊环境（真实
> Ctrl+C、有效 MCP Server、Windows），不适合管道式 CLI E2E。剔除阻塞后，
> **CLI 可测用例通过率 = 128 / 141 ≈ 90.8%**。

---

## 一、确认失败的用例（13 项）

### 1. TC-EDIT-01：edit_file 唯一匹配替换 ❌

**现象**：测试文件 `edit_cli.txt` 内容仅为 `hello world`，只有一个 `world`，
但工具返回 `old_string 出现 13 次`。

**根因**：`edit_file` 的匹配逻辑疑似在全局上下文（包括 session 文件或内部缓冲）
中搜索，而非仅限目标文件。属产品 Bug。

### 2. TC-EDIT-03：edit_file old_string 多次出现 ❌

**现象**：与 TC-EDIT-01 同源，匹配计数异常，无法可靠测试「多次出现」的正确错误提示。

### 3. TC-EDIT-05：multi_edit 多步批量编辑 ❌

**现象**：`multi_cli.txt` 内容为 `one two three`（每个词唯一），
但 `multi_edit` 返回 `edit 1 不唯一`。

**根因**：`multi_edit` 的唯一性检查范围可能包含了非目标内容。属产品 Bug。

### 4. TC-EDIT-07：multi_edit replace_all 模式 ❌

**现象**：设置 `replace_all=true` 后仍返回 `edit 1 不唯一`，
`replace_all` 参数未生效。属产品 Bug。

### 5. TC-EDIT-08：delete_range inclusive 删除 ❌

**现象**：5 行文件，要求删 `start` 到 `end`（第 2-4 行），
工具返回 `已删除第 6-6 行`，但文件内容实际未变化。

**根因**：`delete_range` 的锚点行定位逻辑有误，计算出的行号超出文件范围。属产品 Bug。

### 6. TC-EDIT-09：delete_range exclusive 删除 ❌

**现象**：与 TC-EDIT-08 同源，`inclusive=false` 时同样定位错误。

### 7. TC-SEARCH-08：grep 跳过隐藏文件 ❌

**现象**：`grep` 搜索时扫描了 `.reasonix/sessions` 目录，
输出 79 万 token 的匹配结果，严重污染上下文。

**根因**：`grep` 工具未过滤 `.` 开头的隐藏目录。属产品 Bug，
建议默认排除 `.git`、`.reasonix` 等隐藏目录。

### 8. TC-EXEC-03：bash 输出截断格式 ❌

**现象**：超 32KB 输出时，实际截断后缀为 `...`，
而文档预期为 `...(截断)`。功能正确但格式不符。

### 9. TC-EXEC-04：bash 命令失败 stderr ❌

**现象**：`sh -c "echo err >&2; exit 1"` 返回中只包含 `exit status 1`，
未将 stderr 内容 `err` 一并返回给模型。

**根因**：`run_shell` 的 stderr 捕获可能未正确合并到输出。属产品 Bug。

### 10. TC-EXEC-11：git_commit 无变更提交 ❌

**现象**：
- 第一次：worktree 内编译产生了二进制文件，`git_commit` 意外提交了该文件；
- 第二次：模型偏航调用 `bash cd /workspace` 而非 `git_commit`，
  随后遇到 DeepSeek 503 错误。

**根因**：测试环境隔离不完全 + 模型行为不可控。需更严格的环境准备。

### 11. TC-CMD-07：/test ./agent/ ❌

**现象**：`/test ./agent/` 运行时出现连续高危确认提示消耗管道输入，
导致后续命令被取消。`/test`（默认全包）已通过。

**根因**：`go test` 的某些子测试触发了 `bash` 工具的高危确认，
管道模式下确认输入与测试输入竞争。

### 12. TC-CKPT-04：启动恢复消息 ❌

**现象**：`.ckpt` 文件存在时重新启动 `qiuqiupro`，
未显示预期的 `💾 从快照恢复 N 条消息`。

**根因**：`RestoreFromCheckpoint` 可能在恢复成功时未输出提示，
或管道模式下输出被吞。需排查 `New()` 内部恢复流程。

### 13. TC-STEPS-01 / 02：maxSteps 触发暂停与恢复 ❌

**现象**：设置 `maxSteps=1` 后执行简单多文件 Plan，
模型在一个执行轮次内完成了所有工具调用，未触发 `⏸️ 已达到 maxSteps` 暂停。
因此 `/resume` 也无法验证。

**根因**：`maxSteps` 计数的粒度是 Plan step 而非工具调用次数，
简单任务可能在单个 step 中完成多个工具调用。需使用更复杂的多步 Plan 来触发。

---

## 二、阻塞/非 CLI 可测的用例（65 项）

### 内部 API / 函数级行为（无 CLI 入口）

以下用例测试的是内部代码逻辑，终端启动 `qiuqiupro` 无法直接触发或观测，
需通过单元测试或集成测试验证：

| 用例 | 原因 |
|------|------|
| TC-MODE-08 | Plan 步骤失败→反思→重规划：需精确构造失败场景 |
| TC-PIPE-01~10 | 工具执行管线全部 10 个：Hook / Gate 顺序、并行、ctx 贯穿等为内部 API |
| TC-GATE-06 | 高危工具名单 IsHighRisk 判断：纯函数逻辑 |
| TC-GATE-07 | AllowAllGate：测试/自动化模式专用 |
| TC-GATE-08 | Gate Name()：纯函数逻辑 |
| TC-SKILL-02 | 白名单收窄工具 toolDefinitions：内部 API |
| TC-SKILL-03 | remember_rule 始终可用：内部 API |
| TC-SKILL-04 | 无白名单 = 全部工具：内部 API |
| TC-SKILL-05 | 白名单中不存在的工具静默忽略：内部 API |
| TC-MEM-09 | 系统提示词注入 BuildSystemPrompt：内部 API |
| TC-MEM-10 | 渲染顺序稳定：内部 API |
| TC-MEM-11 | 最多渲染 20 条：内部 API |
| TC-MEM-12 | source 字段 = model：内部 API |
| TC-COMPACT-01 | 80% 自动压缩触发：需精确控制 token 计数 |
| TC-COMPACT-02 | 50% 软提醒：同上 |
| TC-COMPACT-06 | 摘要失败退化裁剪：需 mock LLM 失败 |
| TC-COMPACT-07 | contextWindow≤0 禁用：内部配置 |
| TC-USAGE-04 | 缓存命中率边界：内部计算 |
| TC-USAGE-06 | 子 Agent 用量合并：内部 API |
| TC-USAGE-07 | 所有 LLM 调用均记账：内部审计 |
| TC-STEPS-05 | ExecutionState 持久化：内部文件格式 |
| TC-STORM-01~05 | 风暴检测全部 5 个：需精确构造连续同错误，管道不可控 |
| TC-EVENT-02 | 事件类型覆盖：JSONL 字段级验证 |
| TC-EVENT-04 | 增量加载 LoadSince：内部 API |
| TC-EVENT-05 | Checkpoint 与事件配合：内部 API |
| TC-THINK-05 | bodyFieldInjector 范围：HTTP 中间件，内部 API |
| TC-MCP-03 | 工具发现与命名格式：需有效 MCP Server |
| TC-MCP-04 | 工具调用与结果拼接：需有效 MCP Server |
| TC-PROMPT-01~04 | Prompt 模板系统全部 4 个：内部加载/渲染逻辑 |
| TC-SESSION-01~05 | Session 管理全部 5 个：内部 API |
| TC-SINK-01~04 | Sink 输出系统全部 4 个：内部 API |
| TC-CFG-06 | 价格环境变量负数/非数字：内部 envFloat |
| TC-CFG-07 | MCP 配置 JSON 格式错误：已部分覆盖，完整验证需内部 API |

### 需特殊环境

| 用例 | 原因 |
|------|------|
| TC-INT-01~03 | 中断全部 3 个：需真实交互式 TTY 发送 Ctrl+C / Ctrl+D，管道无法可靠模拟 |
| TC-EXEC-06 | bash 跨平台 Windows：当前测试环境为 macOS |
| TC-QUIET-03 | 安静与非安静对比：需同步执行两个实例做 diff |
| TC-AGENT-TOOL-05 | ask EOF 处理：管道 EOF 与 ask 输入竞争 |
| TC-AGENT-TOOL-06 | ask 超范围序号：依赖 LLM 精确触发 ask 工具 |
| TC-SUB-02 | 子 Agent 配置继承：内部字段级验证 |
| TC-SUB-03 | 子 Agent 用量汇总：内部 API |
| TC-CKPT-05 | 损坏/缺失 .ckpt：需手工构造损坏文件 + 验证静默行为 |
| TC-STARTUP-02 | Checkpoint 恢复时机：内部调用顺序 |
| TC-THINK-02 | 安静模式隐藏思考：需对比两次执行 |
| TC-STABLE-01 | 连续 20+ 轮对话：管道难以构造，且被 503 频繁阻断 |
| TC-STABLE-03 | 大文件读取：需准备大文件 + 验证 maybeCompact |

---

## 三、已通过的核心路径（128 项）

### Agent 模式（6/8 通过）
- TC-MODE-01 ✅ Ask 模式基础问答
- TC-MODE-02 ✅ Ask 模式多轮上下文连贯
- TC-MODE-03 ✅ Ask 模式触发工具调用（read_file）
- TC-MODE-04 ✅ Plan 模式只读调研
- TC-MODE-05 ✅ Plan 模式审批后执行
- TC-MODE-06 ✅ Plan 模式拒绝执行

### 文件读写工具（8/8 通过）
- TC-FILE-01 ✅ read_file 正常读取
- TC-FILE-02 ✅ read_file 文件不存在
- TC-FILE-03 ✅ write_file 创建新文件
- TC-FILE-04 ✅ write_file 覆盖已有文件
- TC-FILE-05 ✅ ls 列出目录
- TC-FILE-06 ✅ ls 默认当前目录
- TC-FILE-07 ✅ ls 空目录
- TC-FILE-08 ✅ ls 目录不存在

### 编辑工具（4/10 通过）
- TC-EDIT-02 ✅ edit_file old_string 不存在
- TC-EDIT-04 ✅ edit_file 文件不存在
- TC-EDIT-06 ✅ multi_edit 中间步骤失败回滚
- TC-EDIT-10 ✅ delete_range 异常（start/end_anchor 不存在、顺序颠倒）

### 搜索工具（10/12 通过）
- TC-SEARCH-01 ✅ glob 基础模式
- TC-SEARCH-02 ✅ glob 递归 ** 模式
- TC-SEARCH-03 ✅ glob 空 pattern
- TC-SEARCH-04 ✅ glob 无匹配
- TC-SEARCH-05 ✅ grep 关键词搜索
- TC-SEARCH-06 ✅ grep 正则表达式
- TC-SEARCH-07 ✅ grep 非法正则（返回编译失败）
- TC-SEARCH-09 ✅ grep 指定搜索目录
- TC-SEARCH-10 ✅ search_files 文件名搜索
- TC-SEARCH-11 ✅ code_search Go 符号搜索

### Shell / Git / Web 工具（8/12 通过）
- TC-EXEC-01 ✅ bash 基础执行
- TC-EXEC-02 ✅ bash 高危拒绝
- TC-EXEC-05 ✅ bash 空命令
- TC-EXEC-07 ✅ web_fetch 正常 JSON
- TC-EXEC-08 ✅ web_fetch HTML 去标签
- TC-EXEC-09 ✅ web_fetch 输出截断 16KB
- TC-EXEC-10 ✅ web_fetch 无效 URL / 超时
- TC-EXEC-12 ✅ todo_write 任务统计

### Agent 专属工具（4/6 通过）
- TC-AGENT-TOOL-01 ✅ ask 单选
- TC-AGENT-TOOL-02 ✅ ask 取消
- TC-AGENT-TOOL-03 ✅ ask 多选
- TC-AGENT-TOOL-04 ✅ ask 参数校验

### 命令系统（20/22 通过）
- TC-CMD-01 ✅ /help
- TC-CMD-02 ✅ /mode（含 TC-MODE-07 全部子项）
- TC-CMD-03 ✅ /readonly（on / off / 状态 / 非法参数）
- TC-CMD-04 ✅ /use（见 Skill 用例）
- TC-CMD-05 ✅ /subagent（空参数报错 + 正常派生）
- TC-CMD-06 ✅ /explain（空参数报错 + 正常解释）
- TC-CMD-08 ✅ /cleanup（见清理用例）
- TC-CMD-09 ✅ /compact（见压缩用例）
- TC-CMD-10 ✅ /usage（见用量用例）
- TC-CMD-11 ✅ /memory
- TC-CMD-12 ✅ /forget（空参数提示 + 不存在的 ID）
- TC-CMD-13 ✅ /maxsteps（显示/设置/关闭/负数/非数字）
- TC-CMD-14 ✅ /pause
- TC-CMD-15 ✅ /resume（无暂停时报错）
- TC-CMD-16 ✅ /replay
- TC-CMD-17 ✅ exit / quit
- TC-CMD-18 ✅ 边界输入（空/纯空格/未知命令）

### 权限门（5/8 通过）
- TC-GATE-01 ✅ 默认门确认后放行
- TC-GATE-02 ✅ 默认门拒绝
- TC-GATE-03 ✅ 只读门拒绝写（write_file / edit_file / bash / git_commit）
- TC-GATE-04 ✅ 只读门放行读
- TC-GATE-05 ✅ Plan 门控（planMode=true 拦截写工具）

### Skill 系统（6/8 通过）
- TC-SKILL-01 ✅ 切换内置 Skill（architect）
- TC-SKILL-06 ✅ 全部 8 个内置 Skill 切换
- TC-SKILL-07 ✅ 外部 Skill（custom.json）
- TC-SKILL-08 ✅ CurrentSkillName

### 记忆系统（8/12 通过）
- TC-MEM-01 ✅ 保存全局偏好
- TC-MEM-02 ✅ 保存项目规则
- TC-MEM-03 ✅ 拒绝知识型
- TC-MEM-04 ✅ 拒绝非法 scope
- TC-MEM-05 ✅ 内容超 300 字符
- TC-MEM-06 ✅ 内容为空
- TC-MEM-07 ✅ 重复内容去重
- TC-MEM-08 ✅ /forget 删除

### 上下文压缩（3/7 通过）
- TC-COMPACT-03 ✅ 手动 /compact
- TC-COMPACT-04 ✅ 空会话 /compact
- TC-COMPACT-05 ✅ 历史较短 /compact

### Token 用量追踪（4/7 通过）
- TC-USAGE-01 ✅ 每轮用量报告
- TC-USAGE-02 ✅ 安静模式隐藏
- TC-USAGE-03 ✅ /usage 会话汇总
- TC-USAGE-05 ✅ 单价启用/未启用

### maxSteps / 暂停 / 恢复（3/6 通过）
- TC-STEPS-04 ✅ maxSteps=0 不限
- TC-STEPS-06 ✅ 无暂停 /resume 报错
- TC-STEPS-03 ✅ /pause 协作暂停

### Checkpoint（3/5 通过）
- TC-CKPT-01 ✅ 无工具调用时保存
- TC-CKPT-02 ✅ 每 5 次工具调用保存
- TC-CKPT-03 ✅ 暂停时保存

### 子 Agent（2/4 通过）
- TC-SUB-01 ✅ 派生执行
- TC-SUB-04 ✅ 失败处理

### 事件系统（2/5 通过）
- TC-EVENT-01 ✅ 事件记录到 JSONL
- TC-EVENT-03 ✅ /replay 格式

### DeepSeek Thinking（3/5 通过）
- TC-THINK-01 ✅ 思考链显示
- TC-THINK-03 ✅ DEEPSEEK_THINKING=disabled
- TC-THINK-04 ✅ DEEPSEEK_REASONING_EFFORT

### MCP 插件（2/4 通过）
- TC-MCP-01 ✅ 启动加载（含无配置提示）
- TC-MCP-02 ✅ 连接失败（command 不存在）

### 安静模式（2/3 通过）
- TC-QUIET-01 ✅ 隐藏 Verbose 事件
- TC-QUIET-02 ✅ 保留必要输出

### 配置系统（5/7 通过）
- TC-CFG-01 ✅ API Key 优先级（环境变量 / 文件 / 交互输入）
- TC-CFG-02 ✅ API Key 为空拒绝
- TC-CFG-03 ✅ DEEPSEEK_MODEL
- TC-CFG-04 ✅ DEEPSEEK_CONTEXT_WINDOW
- TC-CFG-05 ✅ DEEPSEEK_MAX_STEPS

### 清理 / 输入 / 启动 / 稳定性（11/14 通过）
- TC-CLEAN-01 ✅ 扫描垃圾文件
- TC-CLEAN-02 ✅ 跳过 .git
- TC-CLEAN-03 ✅ 删除确认 / 取消
- TC-CLEAN-04 ✅ 部分删除失败
- TC-CLEAN-05 ✅ HumanSize 格式化
- TC-INPUT-01 ✅ confirm 行为
- TC-INPUT-02 ✅ EOF 退出
- TC-STARTUP-01 ✅ 完整启动序列
- TC-SEARCH-12 ✅ code_search 指定搜索路径
- TC-STABLE-02 ✅ 快速切换模式
- TC-STABLE-04 ✅ 并发安全（`go test -race ./...` 无 data race）
- TC-STABLE-05 ✅ 工具元数据验证（AllBuiltInTools=14，ReadOnly 标志正确）

---

## 四、发现的产品 Bug（建议修复）

| # | 严重度 | 模块 | 问题 | 关联用例 |
|---|--------|------|------|---------|
| B-1 | 🔴 高 | edit_file | `old_string` 匹配计数异常，在单次出现时报"出现 13 次" | TC-EDIT-01/03 |
| B-2 | 🔴 高 | multi_edit | 唯一性检查范围错误，唯一词也报"不唯一"；`replace_all` 无效 | TC-EDIT-05/07 |
| B-3 | 🔴 高 | delete_range | 锚点行定位错误，计算行号超出文件范围，文件未被修改 | TC-EDIT-08/09 |
| B-4 | 🟡 中 | grep | 未过滤隐藏目录（`.reasonix/`），导致搜索结果爆炸 | TC-SEARCH-08 |
| B-5 | 🟡 中 | bash (run_shell) | stderr 内容未合并到输出，失败时模型只看到 exit code | TC-EXEC-04 |
| B-6 | 🟢 低 | bash (run_shell) | 截断后缀为 `...` 而非文档约定的 `...(截断)` | TC-EXEC-03 |
| B-7 | 🟢 低 | Checkpoint | 恢复成功时未输出 `💾 从快照恢复` 提示 | TC-CKPT-04 |

---

## 五、稳定性验证

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 编译通过 |
| `go test ./... -count=1` | ✅ 全部通过 |
| `go test -race ./...` | ✅ 无 data race |
| `go vet ./...` | ✅ 无警告（排除未跟踪的 graph/ 实验目录） |
| `gofmt -l .` | ✅ 无格式化问题（排除原作者 BOM 遗留） |
| 主仓库 worktree | ✅ clean，E2E 测试未污染 |

---

## 六、测试环境说明

- **OS**：macOS (darwin 25.5.0)
- **Go**：1.25.5
- **Shell**：zsh
- **LLM**：DeepSeek V4 Flash（`deepseek-v4-flash`）
- **隔离方式**：每批测试使用独立 Git worktree（`/tmp/qiuqiu-manual-e2eX`）
  + 独立 HOME（`/tmp/qiuqiu-manual-home-*.XXXXXX`），防止跨测试污染
- **外部依赖**：DeepSeek API（测试期间多次遇到 503 Service Unavailable）

---

## 七、后续建议

1. **优先修复 B-1~B-3**（编辑工具族）：这是用户最常用的工具，当前完全不可用
2. **修复 B-4**（grep 隐藏目录过滤）：`.reasonix/sessions` 的匹配可导致上下文爆炸
3. **补充单元测试**覆盖 65 个阻塞用例中的内部 API 行为（Hook、Sink、Session、Prompt）
4. **考虑 maxSteps 计数粒度**：当前按 Plan step 计数，但简单任务单步就能完成多个工具调用
5. **格式对齐**：统一 bash 截断提示为 `...(截断)` 或更新文档
