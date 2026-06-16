# 28 — 中断机制 + 分类简化

## 为什么要做

### 中断

用户通过思维链看到模型跑偏了，但只能干等——没有中途打断的手段。Reasonix 和 Claude Code 都有 Ctrl+C 中断处理。

### 分类简化

LLM 分类器本质不可靠：prompt 写得再好也有概率判错。"帮我把 codeGraph 装到项目中"被判 ask，一轮烧了 97 万 token。

## 做了什么

### 1. Ctrl+C 中断

```
main.go: signal.Notify(os.Interrupt) → cancel context
  ↓
Run() 循环每轮检查 ctx.Done()
  ↓
收到中断 → SaveCheckpoint → 优雅退出
```

中断后上下文已保存，下次启动可以从快照恢复。

### 2. 分类策略砍掉 LLM，改默认 Plan

```
// 之前：LLM 分类器 + 启发式打分（100+ 行，不可靠）
DetectMode → LLM API 调用 → 返回 ask/plan

// 之后：简单规则（40 行，确定性强）
DetectMode → isConversational? → ask : plan
```

判断逻辑：
- ≤10 字符 → ask（你好、嗯、谢谢）
- 提问模式开头 AND 不含代码操作词 → ask（"什么是context"、"帮我分析"）
- 其余全部 → plan

核心原则：**Go to plan wrongly is harmless（多走几步计划流程）。Go to ask wrongly costs millions of tokens（在 Run 里无限工具循环）。**

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/perception.go` | 砍掉 LLM 分类器，简化为纯规则 40 行 |
| `main.go` | signal.Notify + 所有模式分支加 ctx.Err() 检查 |
| `agent/run.go` | Run 循环每轮检查 ctx.Done() |

## 效果

- Ctrl+C 可在任意时刻中断（LLM 请求中除外——HTTP 连接需等服务端响应）
- 分类不再依赖 LLM，零成本、零误判
- 命令行体验与 Reasonix/Claude Code 一致
