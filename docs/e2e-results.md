# QiuQiuPro E2E 测试报告

> 测试日期：2026-06-17 · 代码版本：`f9822b4` · 环境：macOS (zsh)
>
> 用例文档 v4.0 已拆分为：
> - [Self-test（无需模型）](./e2e-test-cases-self.md) — `./scripts/run-e2e-self.sh`
> - [LLM E2E（需人工）](./e2e-test-cases-llm.md) — 49 条，需你按步骤验收

---

## 结果汇总

| 类型 | 用例数 | 通过 | 失败 | 阻塞/待验收 | 通过率 |
|------|--------|------|------|-------------|--------|
| **Self-test（自动）** | 149 | **148** | **1 已知 Bug** | 8 环境阻塞 | **99.3%** |
| **LLM E2E（人工）** | 49 | **2 已冒烟** | 0 | **47 待你验收** | — |
| **合计** | 206 | 150 | 1 | 55 | — |

> Self-test 的 1 个失败为 **已知 Bug TC-CKPT-04**（session ID 不跨重启），非回归引入。
> LLM 用例需你在真实终端按 [e2e-test-cases-llm.md](./e2e-test-cases-llm.md) 逐条勾选。

---

## 一、Self-test 执行结果

**命令**：`./scripts/run-e2e-self.sh` · **结果**：✅ 脚本 11/11 通过 + 全量 `go test` / `-race` 通过

| 阶段 | 结果 |
|------|------|
| `go build ./...` | ✅ |
| `go test ./... -count=1` | ✅ agent / tool / cleanup / command / event / skill |
| `go test -race ./...` | ✅ 无 data race |
| 工具回归 TC-FILE/EDIT/SEARCH/EXEC-11 | ✅ 含新增 `TestGitCommitTool_NoChanges` |
| CLI TC-CMD-01/03/13/15/17 | ✅ |
| CLI TC-CMD-07 | ✅（代理：`go test ./command/`，管道 `/test` 不稳定） |
| TC-MCP-02 | ✅ |

### 单元测试覆盖的 Self 模块

PIPE(10) · GATE(8) · SESSION(5) · SINK(4) · STEPS(3) · COMPACT(5) · MEM(10) · USAGE(4) · THINK(2) · 等 — 见 [self 文档](./e2e-test-cases-self.md)

### 已知失败（1）

**TC-CKPT-04**：每次启动新建 `session_{timestamp}`，Restore 查不到上一会话的 `.ckpt` → 无 `💾 从快照恢复` 提示。**Bug B-7**。

### 环境阻塞（8，不计入失败）

TC-EXEC-06 Windows · TC-INT-01/02 Ctrl+C · TC-MCP-03/04 有效 Server · TC-EXEC-08/09 无网络

---

## 二、LLM E2E 执行结果

**自动化冒烟**（真实 DeepSeek，非 `-q`）：

| ID | 结果 | 证据 |
|----|------|------|
| TC-MODE-01 | ✅ | 问候无工具调用，有 `📊 本轮 token` |
| TC-MODE-03 | ✅ | `🔧 read_file({"path":"main.go"})` + `📦` |

**其余 47 条**：请按 [e2e-test-cases-llm.md](./e2e-test-cases-llm.md) 在真实终端验收，填写文档末尾记录表。

优先验收建议：
1. TC-MODE-04~06 Plan 全流程
2. TC-MODE-02 多轮记忆
3. TC-AGENT-TOOL-01 ask 单选
4. TC-CKPT-04 确认 Bug B-7 仍复现

---

## 三、Bug 状态

| # | 问题 | 状态 |
|---|------|------|
| B-1~B-6 | edit/multi_edit/delete_range/grep/bash | ✅ 已修复 f9822b4 |
| B-7 | Checkpoint 跨重启 session ID | ❌ 待修复 |
| B-8 | `/test` 管道 stdin 与 LLM 竞争 | ⚠️ 测试层面已知，Self 用 go test 代理 |

---

## 四、如何复跑

```bash
# Self-test 全自动
./scripts/run-e2e-self.sh

# LLM E2E 人工
go build -o qiuqiupro . && ./qiuqiupro   # 不要加 -q
# 对照 docs/e2e-test-cases-llm.md 逐条操作
```
