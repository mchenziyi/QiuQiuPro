# QiuQiuPro 全量 E2E 测试用例

> 版本：3.0 · **超级全面版** · 共 **206 用例 / 27 模块**
>
> 覆盖：Agent 核心循环、全部工具、全部命令、权限门、Skill、Hook、记忆、压缩、用量、
> Plan 执行链、风暴检测、中断、Checkpoint、子 Agent、事件系统、DeepSeek Thinking、
> MCP、安静模式、配置、清理、Prompt 模板、Session 管理、Sink 输出、输入系统、
> 工具管线、组合场景、稳定性与边界

---

## 一、Agent 模式（8 用例）

### TC-MODE-01：Ask 模式—基础问答
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 启动 Agent | 默认 `[ASK]` 模式，Skill 为 `[default]` |
| 2 | 输入 `你好` | 模型回复问候，不调用任何工具 |
| 3 | 检查日志 | 显示 `📊 本轮 token`（非安静模式），无 `🔧` 工具调用日志 |

### TC-MODE-02：Ask 模式—多轮上下文连贯
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `我叫张三` | 模型记住名字 |
| 2 | `我叫什么名字？` | 模型回答「张三」 |
| 3 | 连续对话 5 轮 | 每轮上下文连贯，每轮有 token 报告 |

### TC-MODE-03：Ask 模式—触发工具调用
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `读一下 main.go 的内容` | 调用 `read_file`，返回文件内容 |
| 2 | 检查日志 | `🔧 read_file(...)` 和 `📦` 结果摘要 |

### TC-MODE-04：Plan 模式—只读调研
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/mode plan` | 提示符变 `[PLAN]`，显示 `📋 正在调研方案...（只读模式，不会修改代码）` |
| 2 | 模型使用只读工具调研 | 日志只有 read_file / grep / ls 等 |
| 3 | 模型尝试调 write 类工具 | 被拦截 `blocked: ... plan mode is read-only` |
| 4 | 调研结束输出方案 | 出现 `批准执行？[Y/n]` |

### TC-MODE-05：Plan 模式—审批后执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 审批提示后输入 `Y` | plan 模式关闭，显示 `✅ 方案已批准，开始执行...` |
| 2 | 执行完成 | 显示 `🎉 执行完成！` |
| 3 | 文件已修改 | 按方案改动 |

### TC-MODE-06：Plan 模式—拒绝执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 审批提示后输入 `n` | 显示 `已取消执行，可以修改后重试` |
| 2 | 文件无变更 | 无副作用 |

### TC-MODE-07：模式切换与状态
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/mode plan` | `planMode=true`，提示 `🔄 切换到 [plan] 模式` |
| 2 | `/mode ask` | `planMode=false`，提示 `🔄 切换到 [ask] 模式` |
| 3 | `/mode` | 显示 `当前模式：ask` |
| 4 | `/mode xxx` | 显示 `⚠️ 未知模式：xxx，可选：plan / ask` |

### TC-MODE-08：Plan API—步骤失败→反思→重规划
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 某步执行失败 | `❌ [x/n] 失败` |
| 2 | 自动反思 | `💡 反思：...`（120 字截断） |
| 3 | 自动重规划 | `🔄 已重新规划剩余步骤（反思后新方案共 N 步）` |
| 4 | 新步骤继续执行 | 不中断 |

---

## 二、内置工具—文件读写（8 用例）

### TC-FILE-01：read_file—正常读取
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 读 main.go | 返回 `文件 main.go（N 字节）内容：...` |

### TC-FILE-02：read_file—文件不存在
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 读 `/tmp/qiuqiu_nonexist.txt` | 返回错误 `读取 ... 失败` |

### TC-FILE-03：write_file—创建新文件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 写 `/tmp/qiuqiu_e2e.txt` 内容 `hello` | 返回 `已写入 ...` |
| 2 | 读取验证 | 内容为 `hello` |

### TC-FILE-04：write_file—覆盖已有文件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 文件已存在，写新内容 | 旧内容被覆盖 |

### TC-FILE-05：ls—列出目录
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `ls agent/` | 包含 agent.go、run.go 等 |
| 2 | 输出区分子目录和文件 | 子目录带 `/` 后缀 |

### TC-FILE-06：ls—默认当前目录
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | path 为空 | 默认列出 `.` |

### TC-FILE-07：ls—空目录
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 列一个空目录 | 返回 `（空目录）` |

### TC-FILE-08：ls—目录不存在
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 列不存在的目录 | 返回 `读目录失败: ...` |

---

## 三、内置工具—编辑（10 用例）

### TC-EDIT-01：edit_file—唯一匹配替换
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 文件含 `hello world`，替换 `world` 为 `qiuqiu` | 返回 `已编辑`，内容为 `hello qiuqiu` |

### TC-EDIT-02：edit_file—old_string 不存在
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 替换不存在的文本 | 返回错误 `未找到 old_string` |

### TC-EDIT-03：edit_file—old_string 多次出现
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 文件中有 3 处匹配 | 返回错误 `old_string 出现 3 次` |

### TC-EDIT-04：edit_file—文件不存在
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 编辑不存在的文件 | 返回错误 `读取失败: ...` |

### TC-EDIT-05：multi_edit—多步批量编辑
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 同一文件 3 处替换 | 返回 `已编辑 ...（3 条）`，三处均正确 |

### TC-EDIT-06：multi_edit—中间步骤失败回滚
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 第 2 条 old_string 不存在 | 返回 `edit 2 未找到`，文件**未被修改**（原子性） |

### TC-EDIT-07：multi_edit—replace_all 模式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 一条 edit 设 replace_all=true | 文件中所有匹配处全部替换 |

### TC-EDIT-08：delete_range—inclusive 删除
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 5 行文件，删第 2-4 行（inclusive=true） | 锚点行本身也被删除，剩 2 行 |

### TC-EDIT-09：delete_range—exclusive 删除
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | inclusive=false | 锚点行保留，只删中间内容 |

### TC-EDIT-10：delete_range—异常
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | start_anchor 不存在 | 返回 `未找到 start_anchor` |
| 2 | end_anchor 不存在 | 返回 `未找到 end_anchor` |
| 3 | start 在 end 后面 | 返回 `anchor 顺序颠倒` |

---

## 四、内置工具—搜索（12 用例）

### TC-SEARCH-01：glob—基础模式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `docs/*.md` | 返回 docs 下的 .md 文件 |

### TC-SEARCH-02：glob—递归 ** 模式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `**/*_test.go` | 递归匹配全部目录下的测试文件 |
| 2 | `agent/**/*.go` | 只递归 agent 目录 |

### TC-SEARCH-03：glob—空 pattern
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | pattern 为空 | 返回 `pattern required` |

### TC-SEARCH-04：glob—无匹配
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `**/*.xyz` | 返回 `无匹配` |

### TC-SEARCH-05：grep—关键词搜索
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 搜 `SetPlanMode` | 返回 `文件名:行号: 内容` 格式 |

### TC-SEARCH-06：grep—正则表达式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | pattern=`func.*New.*Tool` | 返回所有 NewXxxTool 函数定义 |

### TC-SEARCH-07：grep—非法正则
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | pattern=`[invalid` | 返回 `正则编译失败: ...` |

### TC-SEARCH-08：grep—跳过隐藏文件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `.` 开头的文件不被搜索 | 结果中无 `.gitignore` 等 |

### TC-SEARCH-09：grep—指定搜索目录
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | path=`agent` | 只搜 agent 目录 |
| 2 | path 为空 | 默认搜 `.` |

### TC-SEARCH-10：search_files—文件名搜索
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | term=`agent` | 返回所有名字含 agent 的文件 |
| 2 | pattern 和 term 都为空 | 返回 `需要 pattern 或 term` |

### TC-SEARCH-11：code_search—Go 符号搜索
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | symbol=`TokenUsage` | 仅搜 .go 文件 |
| 2 | 不存在的符号 | 返回 `未找到符号 xxx` |

### TC-SEARCH-12：code_search—指定搜索路径
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | path=`agent` | 只搜 agent 目录 |
| 2 | path 为空 | 默认 `.` |

---

## 五、内置工具—Shell / Git / Web（12 用例）

### TC-EXEC-01：bash—基础执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `go version` | 高危确认 → Y → 返回 Go 版本 |

### TC-EXEC-02：bash—高危拒绝
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 确认时输入 `n` | `🚫 用户已取消执行 bash，请换一种方式` |

### TC-EXEC-03：bash—输出截断 32KB
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 输出超 32KB | 末尾 `...(截断)` |

### TC-EXEC-04：bash—命令失败
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `false` 或 `exit 1` | 返回错误信息 |
| 2 | 有 stderr 输出的失败 | 返回 stderr 内容 |

### TC-EXEC-05：bash—空命令
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | command 为空 | 返回 `command required` |

### TC-EXEC-06：bash—跨平台
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | macOS/Linux | 使用 `/bin/sh -c` |
| 2 | Windows | 使用 PowerShell |

### TC-EXEC-07：web_fetch—正常 JSON
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 抓取 `https://httpbin.org/get` | 返回 `HTTP 200 OK` + JSON |

### TC-EXEC-08：web_fetch—HTML 去标签
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 抓取 HTML 页面 | script/style 被去除，返回纯文本 |
| 2 | Content-Type 含 `text/html` | 触发去标签 |
| 3 | 内容含 `<!doctype` 或 `<html` | 也触发去标签 |

### TC-EXEC-09：web_fetch—输出截断 16KB
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 响应超 16KB | 末尾 `...(truncated)` |

### TC-EXEC-10：web_fetch—无效 URL / 超时
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 空 URL | `url required` |
| 2 | 无效 URL | `request: ...` 错误 |
| 3 | 15 秒超时 | `fetch: ...` 超时错误 |

### TC-EXEC-11：git_commit—提交与恢复
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 改文件后提交 | `git add -A` + `git commit` 成功 |
| 2 | 无变更时提交 | `git commit failed: nothing to commit` |

### TC-EXEC-12：todo_write—任务统计
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 3 个 todo（1 done, 1 active, 1 pending） | 返回 `Todos: 1 done, 1 active, 1 pending` |

---

## 六、Agent 专属工具（6 用例）

### TC-AGENT-TOOL-01：ask—单选
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 模型调 ask，展示 3 个选项 | `💬` + 问题 + 带序号的选项 |
| 2 | 输入 `1` | 返回 `selected: <第一个选项的 label>` |

### TC-AGENT-TOOL-02：ask—取消
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 输入 `0` | 返回 `cancelled` |
| 2 | 直接回车（空） | 返回 `cancelled` |

### TC-AGENT-TOOL-03：ask—多选
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | allowMultiple=true | 提示 `可多选，用逗号分隔` |
| 2 | 输入 `1,3` | 返回 `selected: 1,3` |

### TC-AGENT-TOOL-04：ask—参数校验
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | question 为空 | 返回 `question 不能为空` |
| 2 | options < 2 个 | 返回 `options 需要 2-4 个，当前 1 个` |
| 3 | options > 4 个 | 返回 `options 需要 2-4 个，当前 5 个` |

### TC-AGENT-TOOL-05：ask—EOF 处理
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 等待选择时 EOF (Ctrl+D) | 返回 `cancelled (EOF)` |

### TC-AGENT-TOOL-06：ask—输入超范围序号
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 3 个选项，输入 `5` | 返回 `selected: 5`（当作自由输入） |
| 2 | 输入任意文本 `gin` | 返回 `selected: gin` |

---

## 七、工具执行管线（10 用例）

### TC-PIPE-01：执行顺序—planMode → Hook → Gate → Execute → Hook After
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | planMode=true，写工具 | 第一层 planMode 拦截 |
| 2 | planMode=false，Hook Deny | 第二层 Hook 拦截 |
| 3 | Hook Allow，Gate Deny | 第三层 Gate 拦截 |
| 4 | 全放行 | Execute → Hook After → 返回结果 |

### TC-PIPE-02：Hook Before 拒绝
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Hook 返回 Deny | `🚫 已拒绝执行 xxx：hook denied` |
| 2 | Execute 未被调用 | / |
| 3 | After Hook 不执行 | / |

### TC-PIPE-03：Hook After 改写结果
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Execute 返回 `RAW` | / |
| 2 | After Hook 返回 `REWRITTEN` | 模型收到 `REWRITTEN` |

### TC-PIPE-04：多 Hook 链式执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 注册 Hook A 和 Hook B | Before 按注册顺序执行（A→B） |
| 2 | Hook A Deny | Hook B 不执行 |
| 3 | 都 Allow | After 按注册顺序执行（A→B） |

### TC-PIPE-05：Hook 在 Gate 之前执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Hook Deny + Gate ReadOnly，调 write_file | 拒绝理由来自 Hook，而非 Gate |

### TC-PIPE-06：并行执行—只读并发
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 多个 read_file/grep 同时调用 | 并发执行（goroutine） |
| 2 | `go test -race` 无 data race | / |

### TC-PIPE-07：并行执行—写工具串行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 多个 write_file 同时调用 | 串行执行 |

### TC-PIPE-08：未知工具
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 调用不存在的工具名 | `⚠️ error: 未知工具 xxx` |

### TC-PIPE-09：ctx 贯穿—工具响应取消
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | bash `sleep 60` 后取消 | 工具因 ctx 取消而终止 |

### TC-PIPE-10：canRunParallel 判断
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | ReadOnly=true + GateAllow | 可并行 |
| 2 | ReadOnly=false | 不可并行 |
| 3 | GateConfirm | 不可并行 |
| 4 | 未知工具 | 不可并行 |

---

## 八、命令系统（22 用例）

### TC-CMD-01：/help
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/help` | 列出全部 16 个命令，格式 `/%-10s — %s` |

### TC-CMD-02：/mode（见 TC-MODE-07）

### TC-CMD-03：/readonly
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/readonly` | 显示当前状态和权限门名 |
| 2 | `/readonly on` | `🔒 已开启只读模式`，提示符 `[🔒ASK]` |
| 3 | `/readonly off` | `🔓 已关闭只读模式` |
| 4 | `/readonly abc` | `⚠️ 用法：/readonly on|off` |

### TC-CMD-04：/use（见 TC-SKILL-01~08）

### TC-CMD-05：/subagent
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/subagent` | `❌ 请指定子任务` |
| 2 | `/subagent 查一下 strings.Builder` | `🧩 派生子 Agent 执行` → `🧩 子 Agent 返回` |

### TC-CMD-06：/explain
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/explain` | `❌ 请指定文件路径` |
| 2 | `/explain main.go` | 模型解释 main.go |

### TC-CMD-07：/test
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/test` | 默认跑 `.` 的测试 |
| 2 | `/test ./agent/` | 只跑 agent 包 |

### TC-CMD-08：/cleanup（见 TC-CLEAN-01~05）

### TC-CMD-09：/compact（见 TC-COMPACT-01~07）

### TC-CMD-10：/usage（见 TC-USAGE-01~07）

### TC-CMD-11：/memory（见 TC-MEM-01~12）

### TC-CMD-12：/forget
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/forget` | `⚠️ 用法：/forget <memory_id>，可先用 /memory 查看` |
| 2 | `/forget mem_xxx` | 成功 `已删除` 或不存在 `未找到` |

### TC-CMD-13：/maxsteps
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/maxsteps` | 显示当前值 |
| 2 | `/maxsteps 5` | `已设置 maxSteps=5；达到上限会暂停，可输入 /resume 继续` |
| 3 | `/maxsteps 0` | `已关闭 maxSteps 限制` |
| 4 | `/maxsteps -1` | `⚠️ 用法：/maxsteps <非负整数>` |
| 5 | `/maxsteps abc` | 同上 |

### TC-CMD-14：/pause
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/pause` | `已请求暂停：当前 step 完成后会停下` |

### TC-CMD-15：/resume
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 无暂停 `/resume` | `❌ 恢复失败：没有可恢复的暂停计划` |

### TC-CMD-16：/replay
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/replay` | `📋 Session xxx（共 N 条事件）` + 事件列表 + `✅ 重放完成` |

### TC-CMD-17：exit / quit
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `exit` | `👋 再见！` + 进程退出 |
| 2 | `quit` | 同上 |

### TC-CMD-18：边界输入
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 空输入（直接回车） | 忽略，继续等待 |
| 2 | 纯空格 | 忽略 |
| 3 | `/unknown_cmd` | 非命令，交给模型处理 |

---

## 九、权限门（8 用例）

### TC-GATE-01：默认门—确认后放行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | bash 调用 | `🔐 高危操作：bash(...)` + `确认执行？[Y/n]` |
| 2 | 输入 `Y` | 放行执行 |
| 3 | 直接回车 | 也放行（confirm 默认 yes） |

### TC-GATE-02：默认门—拒绝
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 输入 `n` | `🚫 用户已取消执行 bash` |
| 2 | 输入 `no` | 同上 |
| 3 | 输入 `N` | 同上（大小写不敏感） |

### TC-GATE-03：只读门—拒绝写
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | write_file | `🚫 已拒绝执行 write_file：只读模式禁止改动类操作` |
| 2 | edit_file | 同样拒绝 |
| 3 | bash | 同样拒绝 |
| 4 | git_commit | 同样拒绝 |
| 5 | remember_rule | 同样拒绝（非只读） |

### TC-GATE-04：只读门—放行读
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | read_file / grep / ls / glob / code_search / web_fetch / search_files / todo_write | 全部放行 |

### TC-GATE-05：Plan 门控
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | planMode=true，write_file | `blocked: "write_file" is a writer tool and plan mode is read-only. Keep exploring with read-only tools.` |
| 2 | ReadOnly=true 的工具 | 放行 |

### TC-GATE-06：高危工具名单
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | write_file | IsHighRisk=true |
| 2 | edit_file | IsHighRisk=true |
| 3 | bash | IsHighRisk=true |
| 4 | remember_rule | IsHighRisk=true |
| 5 | read_file | IsHighRisk=false |
| 6 | git_commit | IsHighRisk=false（但 canRunParallel=false） |

### TC-GATE-07：AllowAllGate
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 测试/自动化模式 | 一切工具直接放行，无确认 |

### TC-GATE-08：Gate 名字
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | ConfirmHighRiskGate | Name()=`confirm` |
| 2 | ReadOnlyGate | Name()=`read-only` |
| 3 | AllowAllGate | Name()=`allow-all` |

---

## 十、Skill 系统（8 用例）

### TC-SKILL-01：切换内置 Skill
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/use architect` | `🎯 切换到 [architect] 模式：...` |
| 2 | 回答以架构师视角 | / |

### TC-SKILL-02：白名单收窄工具
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 切换到有白名单的 Skill | toolDefinitions 只含白名单工具 |
| 2 | 白名单外的工具对模型不可见 | / |

### TC-SKILL-03：remember_rule 始终可用
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 切换到任何有白名单的 Skill | remember_rule 仍在 toolDefinitions 中 |

### TC-SKILL-04：无白名单 = 全部工具
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | ToolWhitelist 为空的 Skill | 所有注册工具均可用 |

### TC-SKILL-05：白名单中不存在的工具
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 白名单包含 `nonexist_tool` | 静默忽略，不报错 |

### TC-SKILL-06：全部 8 个内置 Skill 切换
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 逐个切换 architect / backend_dev / code_review / devops / frontend_design / pm / tester / superpowers | 全部成功 |

### TC-SKILL-07：外部 Skill
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `~/.qiuqiu/skills/custom.json` 存在 | 启动时列出 |
| 2 | `/use custom` | 切换成功 |

### TC-SKILL-08：CurrentSkillName
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 启动时 | `default` |
| 2 | `/use architect` 后 | `architect` |

---

## 十一、记忆系统（12 用例）

### TC-MEM-01：保存全局偏好
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | remember_rule(scope=global, kind=preference) | 写入 `~/.qiuqiu/memory.json` |

### TC-MEM-02：保存项目规则
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | remember_rule(scope=project, kind=project_rule) | 写入 `.reasonix/memory.json` |

### TC-MEM-03：拒绝知识型
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | kind=knowledge | 返回 `只支持保存 preference 或 project_rule，不保存知识型长期记忆` |

### TC-MEM-04：拒绝非法 scope
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | scope=`session` | 返回 `scope 只支持 global 或 project` |

### TC-MEM-05：内容超 300 字符
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | content 301 字符 | 返回 `记忆内容过长，只保存简短偏好/规则` |

### TC-MEM-06：内容为空
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | content 为空字符串 | 返回 `记忆内容不能为空` |

### TC-MEM-07：重复内容去重
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 保存相同 kind+content 两次 | 不新增，更新 UpdatedAt |
| 2 | `/memory` | 只显示一条 |

### TC-MEM-08：/forget 删除
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/forget mem_xxx` | `已删除长期记忆：mem_xxx`（Enabled=false 软删除） |
| 2 | `/memory` | 不再显示 |

### TC-MEM-09：系统提示词注入
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 保存偏好后 | BuildSystemPrompt() 追加 `## 长期记忆（偏好/规则）` 块 |
| 2 | 模型行为符合偏好 | / |

### TC-MEM-10：渲染顺序稳定
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 同时有 global preference、global rule、project preference、project rule | 渲染顺序固定：全局偏好 → 全局规则 → 项目偏好 → 项目规则 |

### TC-MEM-11：最多渲染 20 条
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 超过 20 条启用记忆 | RenderPromptBlock 只取前 20 条 |

### TC-MEM-12：source 字段
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 模型调 remember_rule | source=`model` |

---

## 十二、上下文压缩（7 用例）

### TC-COMPACT-01：自动压缩—80% 触发
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | lastPromptTokens ≥ contextWindow × 0.8 | `🗜️ 上下文已压缩（自动）：N 条旧消息折叠为摘要，保留近 M 条` |

### TC-COMPACT-02：软提醒—50% 只提醒
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | lastPromptTokens ≥ contextWindow × 0.5 | `📈 上下文已达窗口 50%...到 80% 才会压缩，期间保持前缀缓存` |
| 2 | 提醒只出现一次 | 回落后重置 |
| 3 | 不触发压缩 | 消息数不变 |

### TC-COMPACT-03：手动 /compact
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/compact` | `上下文已压缩（手动）` |

### TC-COMPACT-04：空会话 /compact
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 会话无消息 | `🗜️ 会话为空，无需压缩` |

### TC-COMPACT-05：历史较短 /compact
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 只有 1-2 条消息 | `🗜️ 历史较短，无需压缩` |

### TC-COMPACT-06：摘要失败退化
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | LLM 摘要返回空 | `🗜️ 摘要失败，退化为裁剪` |
| 2 | Trim 到 100 条上限 | 配对感知（不以 tool 开头） |

### TC-COMPACT-07：contextWindow≤0 禁用
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | SetContextWindow(0) | maybeCompact 直接返回，永不自动压缩 |

---

## 十三、Token 用量追踪（7 用例）

### TC-USAGE-01：每轮用量报告
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 非安静模式对话 | `📊 本轮 token｜输入 X（缓存 X）· 输出 X（思考 X）· 合计 X` |

### TC-USAGE-02：安静模式隐藏
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `-q` 启动 | 不显示 `📊 本轮 token` |

### TC-USAGE-03：/usage 会话汇总
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/usage` | `📊 会话 token 用量（共 N 次调用）` + 输入/缓存命中/命中率/输出/思考/合计 |

### TC-USAGE-04：缓存命中率
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 多轮对话 | HitRate > 0% |
| 2 | 无输入 token 时 | HitRate = 0（不除零） |

### TC-USAGE-05：单价启用/未启用
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 设置 `DEEPSEEK_PRICE_INPUT=2` | `/usage` 显示 `估算费用 x.xxxx` |
| 2 | 不设任何单价 | 不显示费用行 |

### TC-USAGE-06：子 Agent 用量合并
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/subagent` 后 `/usage` | Calls 和 token 包含子 Agent 贡献 |

### TC-USAGE-07：所有 LLM 调用均记账
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | streamChat 主循环 | 记账 |
| 2 | GeneratePlan / ReviewPlan / Reflect / RePlan | 记账 |
| 3 | llmSummarize（压缩） | 记账 |

---

## 十四、maxSteps / 暂停 / 恢复（6 用例）

### TC-STEPS-01：maxSteps 触发暂停
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/maxsteps 2`，执行 5 步 Plan | `⏸️ 已达到 maxSteps=2，输入 /resume 继续` |

### TC-STEPS-02：/resume 继续
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/resume` | 从第 3 步继续 |

### TC-STEPS-03：/pause 协作暂停
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Plan 执行中 `/pause` | 当前步完成后 `⏸️ 已暂停：当前步骤完成，输入 /resume 继续` |

### TC-STEPS-04：maxSteps=0 不限
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 默认或 `/maxsteps 0` | Plan 跑完全部步骤不暂停 |

### TC-STEPS-05：ExecutionState 持久化
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 暂停后 | `.exec.json` 存在，含 goal/steps/next_step_index/status=paused/pause_reason |
| 2 | 全部完成后 | `.exec.json` 被清除 |

### TC-STEPS-06：无暂停 /resume
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 无 paused 状态 | `没有可恢复的暂停计划` |

---

## 十五、风暴检测（5 用例）

### TC-STORM-01：3 次同类错误触发
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 同工具同错误前缀连续 3 次 | `⚡ [loop guard] xxx has now failed 3 times` |
| 2 | Run 终止 | / |

### TC-STORM-02：不同错误不触发
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 每次错误不同 | stormCount 重置 |

### TC-STORM-03：成功调用重置
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 失败 2 次后成功 | 计数归零 |

### TC-STORM-04：新用户输入重置
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Run 开头 | stormSig="" stormCount=0 |

### TC-STORM-05：错误判定
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 结果含 `失败` `❌` `error` `Error` `已拒绝` | isErrorResult=true |
| 2 | 正常结果 | isErrorResult=false，不计入风暴 |

---

## 十六、中断（3 用例）

### TC-INT-01：Ctrl+C 中断
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 流式输出中 Ctrl+C | 回到输入提示符 |

### TC-INT-02：中断后继续
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 中断后输入新消息 | Agent 正常响应 |

### TC-INT-03：ReadLine 中断感知
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | ReadString 返回错误 + interrupted=1 | 重置 interrupted=0，继续等待 |
| 2 | ReadString 返回错误 + interrupted=0 | 当作 EOF |

---

## 十七、Checkpoint（5 用例）

### TC-CKPT-01：无工具调用时保存
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 纯问答一轮 | `.ckpt` 被写入 |

### TC-CKPT-02：每 5 次工具调用保存
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | toolCallCount % 5 == 0 | SaveCheckpoint 触发 |

### TC-CKPT-03：暂停时保存
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Plan 暂停 | SaveCheckpoint 调用 |

### TC-CKPT-04：启动恢复
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `.ckpt` 存在 | `💾 从快照恢复 N 条消息` |
| 2 | 上下文延续 | 模型知道之前对话 |

### TC-CKPT-05：损坏/缺失
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `.ckpt` 不存在 | 静默，从空会话开始 |
| 2 | `.ckpt` JSON 损坏 | 静默忽略，从空会话开始 |

---

## 十八、子 Agent（4 用例）

### TC-SUB-01：派生执行
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `/subagent <task>` | 子 Agent 独立 Session 执行 |
| 2 | Session ID 格式 | `{parent}_sub_{nano}` |

### TC-SUB-02：配置继承
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 继承 | client / allTools / gate / sink / pricing / maxSteps / toolHooks / memoryStore / contextWindow / compactRatio / reasoningEffort |
| 2 | 不继承 | pauseRequested（子 Agent 不继承暂停请求） |

### TC-SUB-03：用量汇总
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 子 Agent 完成后 | `a.usage.AddUsage(sub.usage)` |

### TC-SUB-04：失败处理
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 子 Agent Run 失败 | `/subagent` 显示 `❌ 子 Agent 执行失败：...` |

---

## 十九、事件系统（5 用例）

### TC-EVENT-01：事件记录到 JSONL
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 对话 | `.reasonix/sessions/{id}.jsonl` 每行一个 JSON |
| 2 | 事件字段 | id / type / content / tool_name(可选) / timestamp |
| 3 | ID 格式 | `{sessionID}_{nanoseconds}` |

### TC-EVENT-02：事件类型覆盖
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 用户输入 | type=`user` |
| 2 | 模型回复 | type=`assistant` |
| 3 | 工具调用 | type=`tool_call`，tool_name 非空 |
| 4 | 工具结果 | type=`tool_result`，tool_name 非空 |
| 5 | 错误 | type=`error` |
| 6 | 风暴检测 | type=`loop_guard` |

### TC-EVENT-03：/replay 格式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 各事件类型图标 | user=🧑 assistant=🤖 tool_call=🔧 tool_result=📦 error=❌ 其他=• |
| 2 | 长内容截断 | >80 rune 截断加 `...` |
| 3 | 空事件列表 | `没有事件记录` |

### TC-EVENT-04：增量加载
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | LoadSince(afterEventID) | 只返回该 ID 之后的事件 |
| 2 | afterEventID 为空 | 返回全部事件 |

### TC-EVENT-05：Checkpoint 与事件配合
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | SaveCheckpoint 记录 lastEventID | 恢复后 LoadSince 只重放增量 |

---

## 二十、DeepSeek Thinking（5 用例）

### TC-THINK-01：思考链显示
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 需要推理的问题 | 先灰色思考链（ANSI dim/gray），再最终答案 |
| 2 | 思考链与答案分隔 | 思考链结束后输出 `\n` |

### TC-THINK-02：安静模式隐藏思考
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `-q` 启动 | reasoning 事件被丢弃（Verbose=true） |

### TC-THINK-03：DEEPSEEK_THINKING=disabled
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 环境变量 disabled/off/false/0/no | HTTP 请求注入 `{"thinking":{"type":"disabled"}}` |
| 2 | 无思考链 | 直接输出答案 |

### TC-THINK-04：DEEPSEEK_REASONING_EFFORT
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 默认 | effort=`max` |
| 2 | 设为 `high` | 思考量减少 |

### TC-THINK-05：bodyFieldInjector 范围
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 路径以 `/chat/completions` 结尾 | 注入 thinking 字段 |
| 2 | 其他路径（如 `/models`） | 不注入 |
| 3 | 请求已有 thinking 字段 | 不覆盖 |

---

## 二十一、MCP 插件（4 用例）

### TC-MCP-01：启动加载
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 有效配置 | `✅ [xxx] 已加载 N 个工具` |
| 2 | 无配置 | `没有配置 MCP Server（可编辑 ~/.qiuqiu/mcp_servers.json 添加）` |

### TC-MCP-02：连接失败
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | command 不存在 | `⚠️ [xxx] 加载失败：启动 MCP Server xxx 失败` |
| 2 | 不影响其他 MCP 和 Agent | / |

### TC-MCP-03：工具发现与命名
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 工具名 | `{serverName}_{toolName}` 格式 |
| 2 | Description / Parameters | 来自 MCP Server 的 InputSchema |

### TC-MCP-04：工具调用与结果
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 调用 MCP 工具 | 走 CallTool 协议 |
| 2 | 多个 TextContent 片段 | 用 `\n` 拼接 |
| 3 | 调用失败 | 返回 `MCP failed: ...` |

---

## 二十二、安静模式（3 用例）

### TC-QUIET-01：隐藏 Verbose 事件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `-q` 启动 | 工具调用/结果（`🔧`/`📦`）不显示 |
| 2 | 思考链不显示 | / |
| 3 | 本轮 token 不显示 | / |

### TC-QUIET-02：保留必要输出
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 模型最终答案 | 正常显示 |
| 2 | 高危确认 | 正常弹出 |
| 3 | 命令输出 | `/usage` `/memory` 等正常 |
| 4 | Notice（非 Verbose） | `⏸️ 已暂停` 等正常显示 |

### TC-QUIET-03：安静与非安静对比
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 同一输入，非安静模式 | 多出 `🔧` `📦` `📊 本轮 token` |

---

## 二十三、配置系统（7 用例）

### TC-CFG-01：API Key 优先级
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `DEEPSEEK_API_KEY=xxx` | 使用环境变量 |
| 2 | 无环境变量，`~/.qiuqiu/key` 存在 | 使用文件 |
| 3 | 都不存在 | 提示交互输入 |
| 4 | 交互输入后 | 保存到 `~/.qiuqiu/key`（权限 0600） |

### TC-CFG-02：API Key 为空
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 交互输入空字符串 | `API Key 不能为空`，重新请求 |

### TC-CFG-03：DEEPSEEK_MODEL
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 不设 | 默认 `deepseek-v4-flash` |
| 2 | 设为 `deepseek-v4-pro` | 使用指定模型 |

### TC-CFG-04：DEEPSEEK_CONTEXT_WINDOW
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 设为 `10000` | 压缩触发线 8000 |
| 2 | 不设 | 默认 1,000,000 |

### TC-CFG-05：DEEPSEEK_MAX_STEPS
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 设为 `3` | 启动时 maxSteps=3 |
| 2 | 不设 | 默认 0 |

### TC-CFG-06：价格环境变量
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 三个单价全设 | `/usage` 显示费用 |
| 2 | 负数 | envFloat 返回 0，视为未配置 |
| 3 | 非数字 | envFloat 返回 0 |

### TC-CFG-07：MCP 配置文件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `~/.qiuqiu/mcp_servers.json` 格式正确 | 正常加载 |
| 2 | JSON 格式错误 | `⚠️ 解析 MCP 配置失败` |
| 3 | 文件不存在 | 返回 nil，不报错 |

---

## 二十四、Prompt 模板系统（4 用例）

### TC-PROMPT-01：LoadRawPrompt
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `prompt/default/system.xml` 存在 | 提取 `<prompt>` 标签内容作为 sysPrompt |
| 2 | 文件不存在 | 使用 fallback 默认提示词 |

### TC-PROMPT-02：LoadPrompt 模板渲染
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | generate.xml + PromptVars{Tools, Goal} | `{{.Tools}}` `{{.Goal}}` 正确替换 |
| 2 | reflect.xml + PromptVars{各字段} | 全部变量正确替换 |

### TC-PROMPT-03：Prompt 文件格式错误
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 缺少 `<prompt>` 标签 | 返回 `prompt 文件格式错误，缺少 <prompt> 标签` |
| 2 | Plan 函数 fallback | 使用硬编码的 fallback prompt |

### TC-PROMPT-04：Plan prompt 完整链路
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | generate.xml 含可用工具列表 | 模型能看到所有工具 |
| 2 | review.xml 含已有步骤 | 模型能审查 |
| 3 | reflect.xml 含失败信息 | 模型能反思 |
| 4 | replan.xml 含反思内容 | 模型能重规划 |

---

## 二十五、Session 管理（5 用例）

### TC-SESSION-01：BuildRequest
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | sysPrompt 非空 | system 消息前置在 messages 最前 |
| 2 | sysPrompt 为空 | 不添加 system 消息 |
| 3 | 不修改原历史 | 可反复调用 |

### TC-SESSION-02：Trim 配对感知
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 超 100 条 | 裁剪到 100 条 |
| 2 | 裁剪后首条是 tool 消息 | 继续丢弃直到 user/assistant |

### TC-SESSION-03：SplitForCompaction
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 按 tailBudget token 切分 | old + recent 覆盖全部消息 |
| 2 | 至少保留 2 条近消息 | 即使超 tailBudget |
| 3 | recent 不以孤立 tool 开头 | / |

### TC-SESSION-04：Snapshot / Restore
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Snapshot | 返回 messages 的 JSON |
| 2 | Restore | 从 JSON 恢复 messages |
| 3 | 损坏 JSON | Restore 返回 error |

### TC-SESSION-05：ApplyCompaction
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | summary 非空 | 前置 user 角色摘要消息 `（以下是早前对话的摘要...）` |
| 2 | summary 为空 | 退化为只保留 recent |

---

## 二十六、Sink 输出系统（4 用例）

### TC-SINK-01：ConsoleSink 事件格式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | EventToolCall | `  🔧 {name}({args})` + 换行 |
| 2 | EventToolResult | `  📦 {text}` + 换行 |
| 3 | EventReasoning | ANSI dim/gray 包裹（`\033[90m...\033[0m`），无换行 |
| 4 | EventToken / EventPrompt / EventNotice | 原样输出 |

### TC-SINK-02：Verbose 过滤
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | Quiet=true + Verbose=true | 事件被丢弃 |
| 2 | Quiet=true + Verbose=false | 正常输出 |
| 3 | Quiet=false | 全部输出 |

### TC-SINK-03：自定义 Sink
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | SetSink(customSink) | 事件送往自定义 Sink |
| 2 | sink=nil | fallback 到 ConsoleSink |

### TC-SINK-04：发射方法
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | debugf | Kind=EventNotice, Verbose=true |
| 2 | noticef | Kind=EventNotice, Verbose=false |
| 3 | emitToken | Kind=EventToken |
| 4 | emitReasoning | Kind=EventReasoning, Verbose=true |
| 5 | emitToolCall | Kind=EventToolCall, Verbose=true |
| 6 | emitToolResult | Kind=EventToolResult, Verbose=true |
| 7 | emitPrompt | Kind=EventPrompt |

---

## 二十七、清理命令 & 输入 & 启动 & 稳定性（14 用例）

### TC-CLEAN-01：扫描垃圾文件
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 精确匹配：`.DS_Store` / `Thumbs.db` / `desktop.ini` | 命中 |
| 2 | 后缀匹配：`.tmp` `.temp` `.bak` `.orig` `.swp` `.swo` `~` | 命中 |
| 3 | 正常文件 | 不命中 |

### TC-CLEAN-02：跳过 .git
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `.git/` 目录 | filepath.SkipDir，不扫描 |

### TC-CLEAN-03：删除—确认 / 取消
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `Y` 确认 | 文件被删除，显示成功数 |
| 2 | `n` 取消 | `已取消，未删除任何文件` |

### TC-CLEAN-04：部分删除失败
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 部分文件无权限 | 显示成功删除 N 个 + `⚠️` 失败详情 |

### TC-CLEAN-05：HumanSize 格式化
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | < 1024 | `N B` |
| 2 | ≥ 1024 | `X.X KB` / `X.X MB` 等 |

### TC-INPUT-01：confirm 行为
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `Y` / `y` / 直接回车 / 任意文本 | confirm()=true（默认 yes） |
| 2 | `n` / `no` / `N` / `NO` | confirm()=false |
| 3 | EOF | confirm()=false |

### TC-INPUT-02：EOF 退出
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 主循环 Ctrl+D | ReadLine 返回 ("", false)，`break` 退出 |

### TC-STARTUP-01：完整启动序列
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | API Key 获取 | 优先环境变量 → 文件 → 交互 |
| 2 | Agent 构造 | New() + RegisterTools + RegisterTool(remember_rule) + RegisterTool(ask) |
| 3 | MCP 加载 | `🔌 正在加载 MCP 插件...` |
| 4 | Skill 加载 | `🎯 可用 Skill（输入 /use <技能名> 切换）：` |
| 5 | 命令注册 | 全部 16 个命令 |
| 6 | 启动提示 | `🤖 球球 Agent 已启动 | Skill：[default] 模式：[ask]` |
| 7 | 分隔线 | 50 个 `─` |

### TC-STARTUP-02：Checkpoint 恢复时机
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | New() 内部调 RestoreFromCheckpoint | 在用户交互前完成 |

### TC-STABLE-01：连续长对话
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | 20+ 轮对话 | 不崩溃，压缩/Trim 兜底 |

### TC-STABLE-02：快速切换模式
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | plan → ask × 5 | 每次正常 |

### TC-STABLE-03：大文件读取
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | read_file 大文件 | 完整返回（灌入上下文后 maybeCompact 兜底） |

### TC-STABLE-04：并发安全
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | `go test -race ./...` | 无 data race |

### TC-STABLE-05：工具元数据验证
| # | 操作 | 预期结果 |
|---|------|---------|
| 1 | AllBuiltInTools() | 恰好 14 个工具 |
| 2 | ReadOnly 标志 | read_file=true, write_file=false 等全部正确 |

---

## 汇总

| 模块 | 用例数 |
|------|-------|
| 一、Agent 模式 | 8 |
| 二、文件读写工具 | 8 |
| 三、编辑工具 | 10 |
| 四、搜索工具 | 12 |
| 五、Shell / Git / Web 工具 | 12 |
| 六、Agent 专属工具（ask） | 6 |
| 七、工具执行管线 | 10 |
| 八、命令系统 | 22 |
| 九、权限门 | 8 |
| 十、Skill 系统 | 8 |
| 十一、记忆系统 | 12 |
| 十二、上下文压缩 | 7 |
| 十三、Token 用量追踪 | 7 |
| 十四、maxSteps / 暂停 / 恢复 | 6 |
| 十五、风暴检测 | 5 |
| 十六、中断 | 3 |
| 十七、Checkpoint | 5 |
| 十八、子 Agent | 4 |
| 十九、事件系统 | 5 |
| 二十、DeepSeek Thinking | 5 |
| 二十一、MCP 插件 | 4 |
| 二十二、安静模式 | 3 |
| 二十三、配置系统 | 7 |
| 二十四、Prompt 模板系统 | 4 |
| 二十五、Session 管理 | 5 |
| 二十六、Sink 输出系统 | 4 |
| 二十七、清理 / 输入 / 启动 / 稳定性 | 14 |
| **总计** | **206** |

---

## 附录：回归测试清单

以下为历史已修复的 bug，需确保不再复现：

| # | 历史问题 | 验证方法 |
|---|---------|---------|
| R-01 | Trim 后首条是孤立 tool 导致 API 400 | TC-SESSION-02 |
| R-02 | delete_symbol 工具残留死代码 | TC-STABLE-05（AllBuiltInTools=14） |
| R-03 | thinking 模式注释与实际行为不一致 | TC-THINK-03 |
| R-04 | TODO #N 引用误导为未完成 | 代码搜索无 `TODO #` |
| R-05 | executeToolCall 传 context.Background() | TC-PIPE-09 |
| R-06 | gofmt 格式化欠账 | `gofmt -l .` 无输出 |
| R-07 | 感知层自动模式移除后残留 | TODO-reasonix.md 已标记为移除 |
| R-08 | Plan 模式下 remember_rule 在 ReadOnlyGate 被拒绝 | TC-MEM-12（TC-GATE-03） |
