# QiuQiuPro Self-test 用例（无需模型）

> **149 条可自动验证** + **8 条环境阻塞** · 执行：`./scripts/run-e2e-self.sh`

验证方式缩写：**T**=工具直测 · **U**=单元测试 · **C**=CLI 管道 · **E**=环境变量/启动

---

## 一、Agent 模式（1 可自动 + 0 阻塞）

| ID | 用例 | 验证 | 操作摘要 |
|----|------|------|---------|
| TC-MODE-07 | 模式切换与状态 | C | `/mode plan` → `/mode ask` → `/mode` → `/mode xxx` |

> MODE-01~06、08 见 [LLM 文档](./e2e-test-cases-llm.md)

---

## 二、文件读写（8）— 全部 T

| ID | 预期要点 |
|----|---------|
| TC-FILE-01 | read_file 正常返回内容与字节数 |
| TC-FILE-02 | 不存在文件 → `读取 ... 失败` |
| TC-FILE-03 | write 创建 + 读回 `hello` |
| TC-FILE-04 | 覆盖写入 |
| TC-FILE-05 | ls agent/ 含 agent.go，目录带 `/` |
| TC-FILE-06 | path 空 → 列 `.` |
| TC-FILE-07 | 空目录 → `（空目录）` |
| TC-FILE-08 | 不存在目录 → `读目录失败` |

---

## 三、编辑工具（10）— 全部 T

| ID | 预期要点 |
|----|---------|
| TC-EDIT-01 | 唯一匹配替换 |
| TC-EDIT-02 | old_string 不存在 |
| TC-EDIT-03 | 多次出现 → `出现 N 次` |
| TC-EDIT-04 | 文件不存在 |
| TC-EDIT-05 | multi_edit 3 处批量 |
| TC-EDIT-06 | 中间失败原子回滚 |
| TC-EDIT-07 | replace_all |
| TC-EDIT-08 | delete_range inclusive |
| TC-EDIT-09 | delete_range exclusive |
| TC-EDIT-10 | anchor 异常三种 |

---

## 四、搜索工具（12）— 全部 T

| ID | 预期要点 |
|----|---------|
| TC-SEARCH-01~04 | glob 基础/递归/空/无匹配 |
| TC-SEARCH-05~07 | grep 关键词/正则/非法正则 |
| TC-SEARCH-08 | **不扫 `.reasonix` 等隐藏目录** |
| TC-SEARCH-09 | grep 指定目录 / 默认 `.` |
| TC-SEARCH-10 | search_files term / 空参数 |
| TC-SEARCH-11~12 | code_search 符号 / 路径 |

---

## 五、Shell / Git / Web（10 自动 + 2 阻塞）

| ID | 验证 | 预期要点 |
|----|------|---------|
| TC-EXEC-01 | T+U | bash `go version` 成功 |
| TC-EXEC-02 | U | Gate 确认 `n` → 取消 |
| TC-EXEC-03 | T | 超 32KB → `...(截断)` |
| TC-EXEC-04 | T | stderr 合并到输出 |
| TC-EXEC-05 | T | 空 command 报错 |
| TC-EXEC-07 | T | httpbin 200 |
| TC-EXEC-08 | T | HTML 去标签（可选网络） |
| TC-EXEC-09 | T | 16KB 截断（可选网络） |
| TC-EXEC-10 | T | 空 URL / 无效 URL |
| TC-EXEC-11 | T | clean repo 无变更 → `nothing to commit` |
| TC-EXEC-12 | T | todo_write 统计 |
| TC-EXEC-06 | **阻塞** | Windows PowerShell 分支需 Windows |
| TC-EXEC-08/09 | **阻塞** | 无网络时跳过 |

---

## 六、ask 工具（1 自动 + 5 LLM）

| ID | 验证 | 预期 |
|----|------|------|
| TC-AGENT-TOOL-04 | U | question 空 / options 数量校验 |

---

## 七、工具管线（10）— 全部 U

TC-PIPE-01 ~ TC-PIPE-10 · `go test ./agent/ -run 'Hook|Parallel|Gate|Tool'`

---

## 八、命令系统（19 自动 + 1 LLM）

| ID | 验证 | 操作 |
|----|------|------|
| TC-CMD-01 | C | `/help` |
| TC-CMD-02 | C | 同 TC-MODE-07 |
| TC-CMD-03 | C | `/readonly` on/off/非法 |
| TC-CMD-04 | C | `/use architect` 等切换 |
| TC-CMD-05 | C | `/subagent` 空参数报错 |
| TC-CMD-07 | C | `/test ./command/` |
| TC-CMD-08 | C | `/cleanup` n 取消 |
| TC-CMD-09 | C | `/compact` 空/短历史 |
| TC-CMD-10 | C | `/usage` |
| TC-CMD-11 | C | `/memory` `/forget` |
| TC-CMD-12 | C | `/forget` 用法 / 不存在 ID |
| TC-CMD-13 | C | `/maxsteps` 全套 |
| TC-CMD-14 | C | `/pause` |
| TC-CMD-15 | C | `/resume` 无暂停 |
| TC-CMD-16 | C | `/replay` |
| TC-CMD-17 | C | `exit` / `quit` |
| TC-CMD-18 | C | 空行 / 空格忽略 |
| TC-CMD-05-2 | **LLM** | `/subagent <task>` |
| TC-CMD-06-2 | **LLM** | `/explain main.go` |

---

## 九、权限门（8）— 全部 U

TC-GATE-01 ~ TC-GATE-08 · `plan_mode_test.go` / `agent_test.go` / `hooks_test.go`

---

## 十、Skill（3 自动 + 5 LLM）

| ID | 验证 |
|----|------|
| TC-SKILL-06 | C · 8 个内置 Skill 逐个 `/use` |
| TC-SKILL-07 | C · 外部 custom.json |
| TC-SKILL-08 | C · CurrentSkillName |

---

## 十一、记忆（10 自动 + 2 LLM）

| ID | 验证 |
|----|------|
| TC-MEM-01~02 | U · remember_rule 写入 global/project |
| TC-MEM-03~06 | U · 拒绝 knowledge/非法 scope/过长/空 |
| TC-MEM-07 | U · 去重 |
| TC-MEM-08 | C · `/forget` |
| TC-MEM-10~12 | U · 渲染顺序/20 条上限/source |

---

## 十二、压缩（5 自动 + 2 LLM）

| ID | 验证 |
|----|------|
| TC-COMPACT-03~07 | U/C · 手动 compact / 空 / 短 / 失败退化 / window≤0 |

---

## 十三、用量（4 自动 + 3 LLM）

| ID | 验证 |
|----|------|
| TC-USAGE-03 | C · `/usage` |
| TC-USAGE-04 | U · HitRate 边界 |
| TC-USAGE-05 | E · 单价环境变量 |
| TC-USAGE-07 | U · 各路径记账 |

---

## 十四、maxSteps（4 自动 + 2 LLM）

| ID | 验证 |
|----|------|
| TC-STEPS-01~02 | U · `TestExecutePlan_PausesAtMaxStepsAndResumeContinues` |
| TC-STEPS-04 | C · `/maxsteps 0` |
| TC-STEPS-05 | U · ExecutionState 持久化 |
| TC-STEPS-06 | C · 无暂停 `/resume` |

---

## 十五～十六、风暴 / 中断

| 类型 | 数量 | 说明 |
|------|------|------|
| TC-STORM-* | 0 self | 全部 LLM |
| TC-INT-01~02 | **阻塞** | 需真实 Ctrl+C TTY |
| TC-INT-03 | U | ReadLine 中断感知 |

---

## 十七、Checkpoint（2 自动 + 3 LLM）

| ID | 验证 | 备注 |
|----|------|------|
| TC-CKPT-05 | U | 损坏/缺失 ckpt 静默 |
| TC-STARTUP-02 | U | New() 内 Restore 调用 |
| TC-CKPT-04 | **已知 FAIL** | session ID 每次重启变化，跨进程恢复失败 |

---

## 十八～二十六（其余 Self）

| 模块 | ID 范围 | 验证 |
|------|---------|------|
| 子 Agent | TC-SUB-04 部分 | C · 空 subagent |
| 事件 | TC-EVENT-01,03,04 | U/C |
| Thinking | TC-THINK-03,05 | U/E |
| MCP | TC-MCP-01,02 | C · 无配置/坏配置 |
| MCP-03/04 | **阻塞** | 需有效 MCP Server |
| 安静 | TC-QUIET-01 | C · `-q` |
| 配置 | TC-CFG-01~07 | E/C |
| Prompt | TC-PROMPT-01~03 | U（加载/渲染） |
| Session | TC-SESSION-01~05 | U |
| Sink | TC-SINK-01~04 | U |
| 清理 | TC-CLEAN-01~05 | U/C |
| 输入 | TC-INPUT-01 | U |
| 启动 | TC-STARTUP-01 | C |
| 稳定 | TC-STABLE-04,05 | `go test -race` / 工具元数据 |

---

## 环境阻塞汇总（8）

| ID | 原因 |
|----|------|
| TC-EXEC-06 (Windows) | 需 Windows |
| TC-INT-01~02 | 需交互 TTY Ctrl+C |
| TC-MCP-03~04 | 需有效 MCP Server |
| TC-EXEC-08/09 | 无网络时跳过 |

---

## 自动化脚本覆盖

`scripts/run-e2e-self.sh` 依次执行：

1. `go build ./...`
2. `go test ./... -count=1`
3. `go test -race ./... -count=1`
4. `go test ./tool/ -run 'TestEdit|TestMulti|TestDelete|TestGrep|TestGit'` — 工具回归
5. CLI 命令批次（隔离 HOME）
6. 输出 PASS/FAIL 汇总
