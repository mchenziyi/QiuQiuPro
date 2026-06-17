# QiuQiuPro E2E 测试用例索引

> 版本：**4.0** · 共 **206 用例**，按是否需要 DeepSeek 模型能力拆分为两类。

---

## 为什么要拆

| 类型 | 文档 | 执行者 | 说明 |
|------|------|--------|------|
| **Self-test** | [e2e-test-cases-self.md](./e2e-test-cases-self.md) | 自动化脚本 / `go test` | 工具、命令、引擎零件；**不依赖模型决策** |
| **LLM E2E** | [e2e-test-cases-llm.md](./e2e-test-cases-llm.md) | **你人工验收**（我搭环境） | Agent 作为 Agent：理解意图、选工具、多轮、Plan |

测试结果见 [e2e-results.md](./e2e-results.md)。

---

## 用例分布

| 类型 | 数量 | 自动化 |
|------|------|--------|
| Self-test（可自动） | **149** | `scripts/run-e2e-self.sh` |
| Self-test（环境阻塞） | **8** | Windows / Ctrl+C / 有效 MCP Server 等 |
| LLM E2E（需人工） | **49** | 按 llm 文档逐步操作 |
| **总计** | **206** | |

---

## 快速执行

```bash
# Self-test 全自动（约 1～3 分钟）
./scripts/run-e2e-self.sh

# LLM E2E：启动 qiuqiupro，按 e2e-test-cases-llm.md 逐条验收
go build -o qiuqiupro . && ./qiuqiupro
```

---

## 模块对照

| 模块 | Self | LLM | 备注 |
|------|------|-----|------|
| Agent 模式 | 1 | 7 | `/mode` 命令 vs Ask/Plan 全流程 |
| 文件读写 | 8 | 0 | 工具直测 |
| 编辑工具 | 10 | 0 | 工具直测 |
| 搜索工具 | 12 | 0 | 工具直测 |
| Shell/Git/Web | 10 | 2 | EXEC-01/02 工具+Gate；11 工具直测 |
| ask 工具 | 1 | 5 | 参数校验 vs 模型触发 ask |
| 工具管线 | 10 | 0 | 单元测试 |
| 命令系统 | 19 | 3 | `/subagent` 任务、`/explain`、未知命令 |
| 权限门 | 8 | 0 | 单元测试 |
| Skill | 3 | 5 | 切换命令 vs 人格/白名单效果 |
| 记忆 | 10 | 2 | 工具+单测 vs 模型行为 |
| 压缩 | 5 | 2 | 手动/单测 vs 长对话自动触发 |
| 用量 | 4 | 3 | `/usage` vs 多轮/子 Agent |
| maxSteps | 4 | 2 | 单测+命令 vs Plan CLI 暂停 |
| 风暴检测 | 0 | 5 | 需模型反复调错工具 |
| 中断 | 1 | 2 | ReadLine 单测 vs Ctrl+C |
| Checkpoint | 2 | 3 | 损坏 ckpt vs 跨重启恢复 |
| 子 Agent | 1 | 3 | 空参数 vs 真实子任务 |
| 事件 | 3 | 2 | JSONL/replay vs 全类型覆盖 |
| Thinking | 2 | 3 | 环境变量单测 vs 思考链展示 |
| MCP | 2 | 2 | 配置错误 vs 真实 Server |
| 安静模式 | 1 | 2 | `-q` 隐藏 vs 对比 |
| 配置 | 7 | 0 | 环境变量 + CLI 启动 |
| Prompt 模板 | 3 | 1 | 加载单测 vs Plan 全链路 |
| Session | 5 | 0 | 单元测试 |
| Sink | 4 | 0 | 单元测试 |
| 清理/输入/启动/稳定 | 11 | 3 | 大部分自动 vs 长对话/大文件 |

---

## 附录：回归清单

| # | 历史问题 | Self 用例 |
|---|---------|----------|
| R-01 | Trim 后孤立 tool | TC-SESSION-02 |
| R-02 | delete_symbol 残留 | TC-STABLE-05 |
| R-03 | thinking 注释不一致 | TC-THINK-03 |
| R-05 | ctx 未贯穿工具 | TC-PIPE-09 |
| R-06 | gofmt 欠账 | STABLE-04 |
| R-08 | Plan 下 remember_rule | TC-GATE-05 |
