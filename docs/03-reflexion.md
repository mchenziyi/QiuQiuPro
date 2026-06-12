# 03 — Reflexion（自我反思）

## 为什么要做

之前 ExecutePlan 某步失败时，直接调 RePlan 重新规划剩余步骤，没有让 LLM 先分析失败原因。
相当于"考试做错题，老师只给了新试卷，没告诉错在哪"。

Reflexion 增加了一个反思环节：先分析根本原因，再带着反思重新规划。
这样 Agent 能越错越聪明，而不是在同一类问题上反复失败。

## 做了什么

1. **新增 `Reflect()` 方法**
   - `agent/plan.go` → 新的 LLM 调用，专门分析失败原因
   - 输入：总目标、已完成步骤、失败步骤、错误信息
   - 输出：口语化的反思总结（50-100 字）
   - Prompt 要求 LLM 回答：根本原因？忽略了什么？下次怎么改？

2. **修改 `ExecutePlan()`**
   - 失败时先调 `Reflect()`，再调 `RePlan()`
   - 控制台显示 `💡 反思：xxx`

3. **修改 `RePlan()`**
   - 签名从 `RePlan(ctx, plan, failedIndex)` 改为 `RePlan(ctx, plan, failedIndex, reflection)`
   - Prompt 中注入反思内容：`请结合失败反思，重新规划剩余步骤`

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/plan.go` | 新增 `Reflect()` 方法；修改 `ExecutePlan()` 失败流程；修改 `RePlan()` 签名和 prompt |

## 效果

- ExecutePlan 失败时，LLM 先输出反思再重新规划
- 反思结果帮助 LLM 在重规划时避开同样的错误
- 从"失败就换方案"升级为"先想清楚为什么失败，再换方案"

## 相关 TODO

> TODO-reasonix.md — 第二梯队第 4 项
> 难度：★★★☆☆ | 耗时：30 分钟
