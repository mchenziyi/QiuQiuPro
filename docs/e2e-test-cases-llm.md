# QiuQiuPro LLM E2E 用例（需模型 + 人工验收）

> **49 条** · 必须启动真实 `qiuqiupro` 并调用 DeepSeek · **请你按步骤操作并勾选通过标准**

## 通用前置

| 项 | 要求 |
|----|------|
| 环境 | 干净 git worktree 或专用测试目录 |
| API | `~/.qiuqiu/key` 或 `DEEPSEEK_API_KEY` 可用 |
| 模式 | **不要用 `-q`**（需看到 `🔧` / `📊 本轮 token` / thinking） |
| 启动 | `go build -o qiuqiupro . && ./qiuqiupro` |
| 记录 | 每条标记 ✅ PASS / ❌ FAIL / ⏸ BLOCKED（503 等） |

### 503 / 模型偏航时

- **503 Service Unavailable** → 记 ⏸ BLOCKED，换时间重跑，不算产品 FAIL
- **模型未调工具 / 答非所问** → 记 ❌ FAIL，粘贴终端关键输出

---

## 一、Agent 模式（7 条）

### TC-MODE-01：Ask 基础问答

**输入序列**：
```
你好
exit
```

**通过标准**：
- [ ] 启动 banner：`模式：[ask]`，`Skill：[default]`
- [ ] 回复问候语，**无** `🔧` 工具调用
- [ ] 出现 `📊 本轮 token`

---

### TC-MODE-02：Ask 多轮上下文

**输入序列**：
```
我叫张三
我叫什么名字？
今天星期几？（任意第 3 轮）
你还记得我叫什么吗？
exit
```

**通过标准**：
- [ ] 第 2 轮回答「张三」
- [ ] 第 4 轮仍记得「张三」
- [ ] 每轮有 token 报告

---

### TC-MODE-03：Ask 触发 read_file

**输入**：
```
读一下 main.go 的前 20 行
exit
```

**通过标准**：
- [ ] 日志出现 `🔧 read_file(...)`
- [ ] 出现 `📦` 结果摘要
- [ ] 回答内容与 main.go 一致（非编造）

---

### TC-MODE-04：Plan 只读调研

**输入**：
```
/mode plan
调研一下 agent/run.go 的结构，不要改任何文件
exit
```
（若出现 `批准执行？[Y/n]` 输入 `n`）

**通过标准**：
- [ ] 切换后提示 `[PLAN]` 或调研提示
- [ ] 工具调用仅 read_file / grep / ls / code_search 等只读
- [ ] 若模型尝试 write → 出现 `blocked: ... plan mode is read-only`
- [ ] 结束时出现 `批准执行？[Y/n]`

---

### TC-MODE-05：Plan 审批后执行

**前置**：空目录或 `/tmp/plan_e2e/` 可写

**输入**：
```
/mode plan
在 /tmp/plan_e2e/ 创建一个文件 hello.txt，内容为 hi，不要其他操作
```
等待 `批准执行？[Y/n]` 后：
```
Y
exit
```

**通过标准**：
- [ ] `✅ 方案已批准，开始执行...`
- [ ] `🎉 执行完成！`
- [ ] `cat /tmp/plan_e2e/hello.txt` → `hi`

---

### TC-MODE-06：Plan 拒绝执行

**输入**：同 MODE-05 直到审批提示，然后：
```
n
exit
```

**通过标准**：
- [ ] `已取消执行，可以修改后重试`
- [ ] `/tmp/plan_e2e/hello.txt` **不存在**（或方案涉及文件未创建）

---

### TC-MODE-08：Plan 步骤失败→反思→重规划

**输入**（需构造会失败的任务）：
```
/mode plan
执行 Plan：第 1 步 read_file 读取 /tmp/absolutely_not_exist_qiuqiu.txt，第 2 步 write_file 写 /tmp/plan_ok.txt 内容 ok
Y
exit
```

**通过标准**：
- [ ] 某步出现 `❌ [x/n] 失败`
- [ ] 出现 `💡 反思：`
- [ ] 出现 `🔄 已重新规划剩余步骤`
- [ ] 最终任务仍尝试完成（不崩溃）

---

## 二、命令 + Agent 联动（3 条）

### TC-CMD-05-2：/subagent 真实子任务

**输入**：
```
/subagent 用 grep 在 agent 目录搜 SetPlanMode，告诉我有几个匹配
exit
```

**通过标准**：
- [ ] `🧩 派生子 Agent 执行`
- [ ] `🧩 子 Agent 返回` + 合理答案

---

### TC-CMD-06-2：/explain

**输入**：
```
/explain main.go
exit
```

**通过标准**：
- [ ] 非空解释，提及 main 包或入口逻辑

---

### TC-CMD-18-3：未知命令交给模型

**输入**：
```
/foobar_baz 这是什么
exit
```

**通过标准**：
- [ ] **不**出现 `❌ 未知命令`（斜杠命令未注册）
- [ ] 模型正常回复（说明未知命令或尝试帮助）

---

## 三、ask 工具（5 条）

> 需模型**主动调用** ask；若模型不调用，记 FAIL 并注明「模型未触发 ask」

### TC-AGENT-TOOL-01：单选

**输入**：
```
我想选技术栈，请用 ask 工具让我选：1 Go 2 Python 3 Rust
```
模型弹出选项后：
```
1
exit
```

**通过标准**：
- [ ] `💬` + 带序号选项
- [ ] 工具结果含 `selected:` 与 Go 相关 label

---

### TC-AGENT-TOOL-02：取消

**触发 ask 后**：
```
0
exit
```
或**直接回车**

**通过标准**：
- [ ] 返回 `cancelled`

---

### TC-AGENT-TOOL-03：多选

**输入**：
```
用 ask 工具让我多选喜欢的语言：Go、Python、Rust、Java，allowMultiple=true
```
```
1,3
exit
```

**通过标准**：
- [ ] 提示可多选
- [ ] `selected: 1,3` 或等价

---

### TC-AGENT-TOOL-05：EOF

**触发 ask 后**按 **Ctrl+D**（需真实终端，非管道）

**通过标准**：
- [ ] `cancelled (EOF)`

---

### TC-AGENT-TOOL-06：超范围输入

**触发 3 选项 ask 后**：
```
gin
exit
```

**通过标准**：
- [ ] `selected: gin`（自由文本）

---

## 四、Skill（5 条）

### TC-SKILL-01-2：architect 人格

**输入**：
```
/use architect
如何设计一个高并发 API 网关？用 3 句话
exit
```

**通过标准**：
- [ ] 切换成功提示
- [ ] 回答偏架构视角（非纯代码堆砌）

---

### TC-SKILL-02：白名单收窄

**输入**：
```
/use tester
列出你现在能用的所有工具名称
exit
```

**通过标准**：
- [ ] 工具列表**少于** default 全量
- [ ] 仍含 remember_rule（若 Skill 设计如此）

---

### TC-SKILL-03 ~ 05

按原用例文档验证 toolDefinitions / 空白名单 / 无效工具名静默 — 需结合 `/use` + 让模型列举或调用工具观察。

---

## 五、记忆（2 条）

### TC-MEM-07-2：/memory 去重展示

**输入**：
```
请记住偏好：回复要简洁
请记住偏好：回复要简洁
/memory
exit
```

**通过标准**：
- [ ] `/memory` 只显示一条相同偏好

---

### TC-MEM-09：偏好影响行为

**输入**：
```
记住规则：以后回答只用一句话
你好，介绍一下你自己
exit
```

**通过标准**：
- [ ] 第 2 轮回答明显简短（主观判断，可 FAIL 若模型忽略）

---

## 六、压缩（2 条）

### TC-COMPACT-01：80% 自动压缩

**操作**：`DEEPSEEK_CONTEXT_WINDOW=8000` 启动，连续多轮长对话（或反复 read 大文件）直到 token 超 80%

**通过标准**：
- [ ] `🗜️ 上下文已压缩（自动）`

---

### TC-COMPACT-02：50% 软提醒

**通过标准**：
- [ ] `📈 上下文已达窗口 50%`
- [ ] 只提醒一次，不立即压缩

---

## 七、用量（3 条）

### TC-USAGE-01：每轮 token 报告

**输入**：`你好` → **通过**：出现 `📊 本轮 token`

### TC-USAGE-02：安静模式

**启动**：`./qiuqiupro -q`，输入 `你好` → **通过**：**无** `📊 本轮 token`

### TC-USAGE-06：子 Agent 用量合并

**输入**：
```
/subagent 说一个简短笑话
/usage
exit
```

**通过标准**：
- [ ] `/usage` 的 Calls ≥ 2（含子 Agent）

---

## 八、maxSteps Plan CLI（2 条）

### TC-STEPS-01：CLI 触发 maxSteps 暂停

**输入**：
```
/maxsteps 2
/mode plan
创建一个目录 /tmp/steps_a、/tmp/steps_b、/tmp/steps_c 各放一个文件
Y
exit
```

**通过标准**：
- [ ] 出现 `⏸️ 已达到 maxSteps=2` 或等价暂停

---

### TC-STEPS-03：Plan 执行中 /pause

**输入**：长 Plan 任务执行中输入 `/pause` → **通过**：当前步完成后暂停提示

---

## 九、风暴检测（5 条）

### TC-STORM-01 ~ 05

**构造方式**：让模型反复读取不存在文件（同一工具同一错误 ≥3 次）

**TC-STORM-01 通过标准**：
- [ ] `⚡ [loop guard]` 或 `loop_guard` 事件
- [ ] Run 终止，不无限循环

其余 STORM 用例按原文档判定计数重置逻辑 — 需观察多轮工具失败模式。

---

## 十、中断（2 条）

### TC-INT-01 / 02

**需真实终端**（非管道）：流式输出中 **Ctrl+C** → 回到提示符 → 再输入 `你好` 正常回复

---

## 十一、Checkpoint（3 条）

### TC-CKPT-01：纯问答写 ckpt

**输入**：`你好` → `exit` → 检查 `.reasonix/sessions/*.ckpt` 存在

### TC-CKPT-02：每 5 次工具调用

**输入**：让模型连续调用 ≥5 次只读工具 → ckpt 更新

### TC-CKPT-04：跨重启恢复（**当前已知 FAIL**）

**步骤**：
1. 对话：`我叫李四` → `exit`
2. 重启 `./qiuqiupro`
3. 问：`我叫什么名字？`

**预期（产品目标）**：
- [ ] 启动时 `💾 从快照恢复 N 条消息`
- [ ] 回答「李四」

**当前实际**：session ID 每次新建，**无法跨进程恢复** — 验收时若仍失败，记为 **已知 Bug B-7**

---

## 十二、子 Agent（3 条）

### TC-SUB-01 / 03 / 04

/subagent 成功、用量合并、失败任务 — 见 USAGE-06 与子任务错误场景

---

## 十三、事件 / Thinking / MCP / Quiet / Prompt / 稳定

| ID | 验收要点 |
|----|---------|
| TC-EVENT-02 | JSONL 含 user/assistant/tool_call/tool_result/error |
| TC-EVENT-05 | ckpt 含 lastEventID |
| TC-THINK-01 | 复杂推理题先灰字 thinking 再答案 |
| TC-THINK-02 | `-q` 无 thinking 输出 |
| TC-THINK-04 | `DEEPSEEK_REASONING_EFFORT=high` 思考量变化 |
| TC-MCP-03/04 | 配置真实 MCP Server，调用 `{server}_{tool}` |
| TC-QUIET-02 | `-q` 仍显示最终答案和高危确认 |
| TC-QUIET-03 | 同输入对比 `-q` 与非 `-q` 输出差异 |
| TC-PROMPT-04 | Plan 全流程使用 generate/review/reflect/replan 模板 |
| TC-STABLE-01 | 20+ 轮对话不崩溃 |
| TC-STABLE-02 | `/mode plan` ↔ `/mode ask` ×5 |
| TC-STABLE-03 | `read_file` 读大文件 + 后续对话正常 |

---

## 验收记录模板

复制到 PR 或 issue：

```markdown
| ID | 结果 | 备注 |
|----|------|------|
| TC-MODE-01 | ✅ | |
| TC-MODE-02 | ⏸ | 503 |
| ... | | |
```

**LLM 通过率** = PASS / (总数 − BLOCKED)
