package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// Step 计划中的一步
type Step struct {
	ID     int    `json:"id"`
	Desc   string `json:"desc"`
	Status string `json:"status"` // pending / running / done / failed
}

// Plan 执行计划
type Plan struct {
	Goal  string `json:"goal"`
	Steps []Step `json:"steps"`
}

// stripCodeFence 去掉 LLM 输出里包裹 JSON 的 ``` 代码块围栏
// （```json ... ``` 或 ``` ... ```），返回纯 JSON 文本，便于后续 Unmarshal。
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// GeneratePlan 让 LLM 把目标拆成步骤
func (a *Agent) GeneratePlan(ctx context.Context, goal string) (*Plan, error) {
	var toolList []string
	for _, t := range a.availableTools() {
		toolList = append(toolList, fmt.Sprintf("- %s：%s", t.Name, t.Description))
	}

	prompt, err := LoadPrompt("prompt/plan/generate.xml", PromptVars{
		Tools: strings.Join(toolList, "\n"),
		Goal:  goal,
	})
	if err != nil {
		// fallback：文件加载失败时用默认 prompt（不影响已有功能）
		prompt = fmt.Sprintf(`你是一个项目规划专家。把目标拆成 3~8 个步骤。

可用工具：
%s

要求：每步必须能用上面工具完成，按顺序，每步不超过 15 字，不超过 8 步。
只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]

目标：%s`, strings.Join(toolList, "\n"), goal)
	}

	resp, err := a.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: a.model,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "你是一个严谨的项目规划专家，只输出 JSON"},
				{Role: "user", Content: prompt},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("规划失败：%w", err)
	}
	a.accountUsage(resp.Usage)

	content := stripCodeFence(resp.Choices[0].Message.Content)

	type stepJSON struct {
		ID   int    `json:"id"`
		Desc string `json:"desc"`
	}
	var steps []stepJSON
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		return nil, fmt.Errorf("解析失败：%w\n原始内容：%s", err, content)
	}

	plan := &Plan{Goal: goal}
	for _, s := range steps {
		plan.Steps = append(plan.Steps, Step{ID: s.ID, Desc: s.Desc, Status: "pending"})
	}
	return plan, nil
}

// ReviewPlan 让 LLM 自我审查 Plan 的质量
func (a *Agent) ReviewPlan(ctx context.Context, plan *Plan) (*Plan, error) {
	var stepsText []string
	for _, s := range plan.Steps {
		stepsText = append(stepsText, fmt.Sprintf("%d. %s", s.ID, s.Desc))
	}

	prompt, err := LoadPrompt("prompt/plan/review.xml", PromptVars{
		Goal:  plan.Goal,
		Steps: strings.Join(stepsText, "\n"),
	})
	if err != nil {
		// fallback
		prompt = fmt.Sprintf(`你是一个项目规划评审专家。请检查以下 Plan 的质量。

目标：%s

现有步骤：
%s

检查要求：
1. 是否有遗漏的关键步骤？
2. 步骤顺序是否合理？
3. 每步粒度是否合适？

如果 Plan 没问题，只输出 "OK"。
如果有问题，输出修正后的 JSON：[{"id":1,"desc":"步骤描述"}, ...]`, plan.Goal, strings.Join(stepsText, "\n"))
	}

	resp, err := a.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: a.model,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "你是一个严格的规划评审专家，只输出 OK 或修正后的 JSON"},
				{Role: "user", Content: prompt},
			},
		},
	)
	if err != nil {
		return plan, nil
	}
	a.accountUsage(resp.Usage)

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "OK" {
		a.noticef("  📋 Plan 审查通过\n")
		return plan, nil
	}

	content = stripCodeFence(content)

	type stepJSON struct {
		ID   int    `json:"id"`
		Desc string `json:"desc"`
	}
	var steps []stepJSON
	if err := json.Unmarshal([]byte(content), &steps); err != nil || len(steps) == 0 {
		a.noticef("  ⚠️  Plan 审查结果解析失败，使用原始 Plan\n")
		return plan, nil
	}

	newPlan := &Plan{Goal: plan.Goal}
	for _, s := range steps {
		newPlan.Steps = append(newPlan.Steps, Step{ID: s.ID, Desc: s.Desc, Status: "pending"})
	}
	a.noticef("  📋 Plan 已根据审查意见优化\n")
	return newPlan, nil
}

// ExecutePlan 按顺序执行 Plan 中的每一步
func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) error {
	return a.executePlanFrom(ctx, plan, 0)
}

func (a *Agent) executePlanFrom(ctx context.Context, plan *Plan, start int) error {
	if start < 0 {
		start = 0
	}
	if start >= len(plan.Steps) {
		_ = a.ClearExecutionState()
		return nil
	}
	stepsRun := 0
	for i := start; i < len(plan.Steps); i++ {
		step := &plan.Steps[i]
		step.Status = "running"
		_ = a.SaveExecutionState(ExecutionState{
			Goal:          plan.Goal,
			Steps:         append([]Step(nil), plan.Steps...),
			NextStepIndex: i,
			Status:        ExecutionRunning,
		})
		a.debugf("\n  📋 [%d/%d] %s\n", i+1, len(plan.Steps), step.Desc)
		_, err := a.Run(ctx, fmt.Sprintf("请执行：%s", step.Desc))
		if err != nil {
			step.Status = "failed"
			a.noticef("  ❌ [%d/%d] 失败：%v\n", i+1, len(plan.Steps), err)

			reflection := a.Reflect(ctx, plan, i, err)
			newPlan, replanErr := a.RePlan(ctx, plan, i, reflection)
			if replanErr != nil {
				return fmt.Errorf("第 %d 步失败：%w（重规划也失败：%v）", step.ID, err, replanErr)
			}

			plan.Steps = append(plan.Steps[:i+1], newPlan.Steps...)
			a.debugf("  🔄 已重新规划剩余步骤（反思后新方案共 %d 步）\n", len(newPlan.Steps))
			continue
		}
		step.Status = "done"
		a.debugf("  ✅ [%d/%d] 完成\n", i+1, len(plan.Steps))
		stepsRun++

		next := i + 1
		if next < len(plan.Steps) && a.pauseRequested {
			a.pauseRequested = false
			if err := a.savePausedPlan(plan, next, PauseReasonUser); err != nil {
				return err
			}
			a.noticef("  ⏸️  已暂停：当前步骤完成，输入 /resume 继续\n")
			return ErrPlanPaused
		}
		if next < len(plan.Steps) && a.maxSteps > 0 && stepsRun >= a.maxSteps {
			if err := a.savePausedPlan(plan, next, PauseReasonMaxSteps); err != nil {
				return err
			}
			a.noticef("  ⏸️  已达到 maxSteps=%d，输入 /resume 继续\n", a.maxSteps)
			return ErrPlanPaused
		}
	}
	_ = a.ClearExecutionState()
	return nil
}

// Reflect 让 LLM 分析失败原因，输出反思
func (a *Agent) Reflect(ctx context.Context, plan *Plan, failedIndex int, err error) string {
	var doneText []string
	for i := 0; i < failedIndex; i++ {
		doneText = append(doneText, fmt.Sprintf("✅ %d. %s", plan.Steps[i].ID, plan.Steps[i].Desc))
	}

	prompt, perr := LoadPrompt("prompt/plan/reflect.xml", PromptVars{
		Goal:           plan.Goal,
		DoneSteps:      strings.Join(doneText, "\n"),
		FailedStepID:   plan.Steps[failedIndex].ID,
		FailedStepDesc: plan.Steps[failedIndex].Desc,
		Error:          err.Error(),
	})
	if perr != nil {
		// fallback
		prompt = fmt.Sprintf(`你是一个项目复盘专家。执行计划时某步失败了，请分析根本原因。

总目标：%s

已完成步骤：
%s

失败步骤：%d. %s
失败信息：%v

请深刻反思：
1. 错误发生的根本原因是什么？
2. 你忽略了什么？
3. 下一次尝试时的具体修改策略是什么？

输出反思（50-100 字，口语化）：`,
			plan.Goal, strings.Join(doneText, "\n"),
			plan.Steps[failedIndex].ID, plan.Steps[failedIndex].Desc, err)
	}

	resp, rerr := a.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: a.model,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "你是一个经验丰富的项目复盘专家，善于从失败中总结教训"},
				{Role: "user", Content: prompt},
			},
		},
	)
	if rerr != nil {
		return fmt.Sprintf("（反思失败：%v）", rerr)
	}
	a.accountUsage(resp.Usage)

	reflection := strings.TrimSpace(resp.Choices[0].Message.Content)
	a.noticef("  💡 反思：%s\n", truncate(reflection, 120))
	return reflection
}

// RePlan 让 LLM 根据已完成和失败的步骤，结合反思重新规划后续方案
func (a *Agent) RePlan(ctx context.Context, plan *Plan, failedIndex int, reflection string) (*Plan, error) {
	var doneText []string
	for i := 0; i < failedIndex; i++ {
		doneText = append(doneText, fmt.Sprintf("✅ %d. %s", plan.Steps[i].ID, plan.Steps[i].Desc))
	}
	var remainingText []string
	for i := failedIndex; i < len(plan.Steps); i++ {
		remainingText = append(remainingText, fmt.Sprintf("❌ %d. %s", plan.Steps[i].ID, plan.Steps[i].Desc))
	}

	prompt, err := LoadPrompt("prompt/plan/replan.xml", PromptVars{
		Goal:           plan.Goal,
		DoneSteps:      strings.Join(doneText, "\n"),
		RemainingSteps: strings.Join(remainingText, "\n"),
		Reflection:     reflection,
	})
	if err != nil {
		// fallback
		prompt = fmt.Sprintf(`你是一个项目规划专家。执行过程中某步失败了，请重新规划后续步骤。

总目标：%s

已完成：
%s

失败/未完成的步骤：
%s

失败反思（本轮执行遇到的问题）：
%s

请结合失败反思，重新规划剩余步骤。要求：
- 每步是 LLM 一次能处理的粒度
- 按执行顺序排列
- 每步不超过 15 字
- 步骤数不超过 8 步

只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]`,
			plan.Goal, strings.Join(doneText, "\n"), strings.Join(remainingText, "\n"), reflection)
	}

	resp, rerr := a.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: a.model,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "你是一个严谨的项目规划专家，只输出 JSON"},
				{Role: "user", Content: prompt},
			},
		},
	)
	if rerr != nil {
		return nil, fmt.Errorf("重规划失败：%w", rerr)
	}
	a.accountUsage(resp.Usage)

	content := stripCodeFence(resp.Choices[0].Message.Content)

	type stepJSON struct {
		ID   int    `json:"id"`
		Desc string `json:"desc"`
	}
	var steps []stepJSON
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		return nil, fmt.Errorf("解析重规划结果失败：%w\n原始：%s", err, content)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("重规划结果为空")
	}

	newPlan := &Plan{Goal: plan.Goal}
	for _, s := range steps {
		newPlan.Steps = append(newPlan.Steps, Step{ID: s.ID, Desc: s.Desc, Status: "pending"})
	}
	return newPlan, nil
}
