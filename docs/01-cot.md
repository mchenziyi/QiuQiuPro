# 01 — CoT（思维链）

## 为什么要做

CoT（Chain of Thought，思维链）的核心思想是：强制要求模型在输出最终答案前，
先显式地输出中间的推理步骤。这种做法能显著激活模型在复杂逻辑、数学推理和
代码审查中的潜力。

QiuQiuPro 之前没有引导 LLM 展示推理过程，对于一些需要多步分析的问题，
LLM 可能直接跳到结论，跳过中间逻辑推演，影响回答质量。

## 做了什么

1. **设置默认 system prompt**
   - `agent/agent.go` → `New()` 中设置默认 system prompt：
     `"在输出结论之前，请先一步步展示你的推理过程。"`
   - 这样在任何模式（包括无 Skill 时）下，LLM 都会先推理再回答

2. **每个 Skill 的 system prompt 中也加入 CoT**
   - `skill/skill.go` → Architect / CodeReview / Frontend 三个 Skill
     都在第一条加入同一句 CoT 引导
   - 切换 Skill 时 CoT 依然生效，不会被 Skill 的 prompt 覆盖

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/agent.go` | `New()` 中设置默认 sysPrompt 包含 CoT |
| `skill/skill.go` | 三个内置 Skill 各加一句 CoT 引导 |

## 效果

Agent 现在会在回答前先输出推理步骤，而不是直接跳到最终答案。
对于架构决策、代码审查等需要多步分析的任务，回答质量会有提升。

## 相关 TODO

> TODO-reasonix.md — 第一梯队第 1 项
> 难度：★☆☆☆☆ | 耗时：5 分钟
