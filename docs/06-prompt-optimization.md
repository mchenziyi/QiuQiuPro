# 06 — 提示词全面优化

## 现状（改动前的问题）

之前的 prompt 有 4 个问题：

1. **没有身份定义** — LLM 回答"我是 DeepSeek"，而不是"我是球球"
2. **提示词太简略** — 各 prompt 只有最基础的指令，缺乏结构化指导
3. **Skill 提示词太单薄** — 每个 Skill 只有 3-4 条规则，缺乏专业深度
4. **指令不具体** — 例如"每步不超过 15 字"这种机械限制，没有关注步骤质量

## 改了什么

### default/system.xml（默认系统提示词）

```xml
<prompt>
你是球球（QiuQiuPro），一个基于 Go 开发的 Coding Agent。
你的目标是通过读取、搜索、编辑代码来帮助用户完成项目开发任务。

## 核心规则
- 在输出结论之前，请先一步步展示你的推理过程
- 始终用中文回答，代码和术语保留原文
- 你内置了 11 个工具，也可以通过 MCP 接入外部工具
- 诚实告知能力边界，不要编造
- 代码优先：能给出代码的场景尽量给出可运行的代码
- 每次回答要简洁
</prompt>
```

### prompt/plan/generate.xml（拆解目标）

改动前：3~8 步硬限制，没有依赖感知
改动后：按任务复杂度灵活决定步骤数，强调依赖关系和步骤具体性

### prompt/plan/review.xml（审查计划）

改动前：3 个通用问题
改动后：4 个维度检查，包括遗漏步骤、顺序合理性、粒度、描述明确性

### prompt/plan/reflect.xml（失败反思）

改动前：3 个开放式问题
改动后：4 个维度的结构化分析（规划问题/执行问题/工具问题/根本原因），输出要求更具体

### prompt/plan/replan.xml（重新规划）

改动前：通用重规划要求
改动后：强调"避免犯同样的错误"，根据反思内容针对性调整

### prompt/skills/*.json（三个 Skill）

所有技能都增加了：
- `"你是球球（QiuQiuPro）"` — 身份定义
- 更详细的专业指令
- 结构化输出要求

## 改动文件

| 文件 | 改动 |
|------|------|
| `prompt/default/system.xml` | 完全重写，增加身份+规则 |
| `prompt/plan/generate.xml` | 更灵活的分步策略 |
| `prompt/plan/review.xml` | 4 维度审查 |
| `prompt/plan/reflect.xml` | 4 维度结构化反思 |
| `prompt/plan/replan.xml` | 强调避免重蹈覆辙 |
| `prompt/skills/architect.json` | 增加身份+更具体的架构指导 |
| `prompt/skills/code_review.json` | 增加身份+5 维检查项 |
| `prompt/skills/frontend_design.json` | 增加身份+5 条设计要求 |

## 效果

- Agent 现在会说自己是"球球"而不是"DeepSeek" ✅
- 每一步的指令更加明确，LLM 不容易跑偏
- 计划步骤质量更高（更具体的步骤描述）
- 反思更有针对性（区分规划/执行/工具问题）
- 所有改动不需要重新编译，直接改 XML/JSON 文件即可
