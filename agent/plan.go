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

// GeneratePlan 让 LLM 把目标拆成步骤
func (a *Agent) GeneratePlan(ctx context.Context, goal string) (*Plan, error) {
	var toolList []string
	for _, t := range a.availableTools() {
		toolList = append(toolList, fmt.Sprintf("- %s：%s", t.Name, t.Description))
	}

	prompt := fmt.Sprintf(`你是一个项目规划专家。把目标拆成 3~8 个步骤。

可用工具：
%s

要求：每步必须能用上面工具完成，按顺序，每步不超过 15 字，不超过 8 步。
只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]

目标：%s`, strings.Join(toolList, "\n"), goal)

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

	content := resp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

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
// 如果发现问题，返回修正后的 Plan；如果没问题，Plan 保持不变
func (a *Agent) ReviewPlan(ctx context.Context, plan *Plan) (*Plan, error) {
	var stepsText []string
	for _, s := range plan.Steps {
		stepsText = append(stepsText, fmt.Sprintf("%d. %s", s.ID, s.Desc))
	}

	prompt := fmt.Sprintf(`你是一个项目规划评审专家。请检查以下 Plan 的质量。

目标：%s

现有步骤：
%s

检查要求：
1. 是否有遗漏的关键步骤？
2. 步骤顺序是否合理？
3. 每步粒度是否合适？

如果 Plan 没问题，只输出 "OK"。
如果有问题，输出修正后的 JSON：[{"id":1,"desc":"步骤描述"}, ...]`, plan.Goal, strings.Join(stepsText, "\n"))

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

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "OK" {
		fmt.Println("  📋 Plan 审查通过")
		return plan, nil
	}

	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	type stepJSON struct {
		ID   int    `json:"id"`
		Desc string `json:"desc"`
	}
	var steps []stepJSON
	if err := json.Unmarshal([]byte(content), &steps); err != nil || len(steps) == 0 {
		fmt.Println("  ⚠️  Plan 审查结果解析失败，使用原始 Plan")
		return plan, nil
	}

	newPlan := &Plan{Goal: plan.Goal}
	for _, s := range steps {
		newPlan.Steps = append(newPlan.Steps, Step{ID: s.ID, Desc: s.Desc, Status: "pending"})
	}
	fmt.Println("  📋 Plan 已根据审查意见优化")
	return newPlan, nil
}

// ExecutePlan 按顺序执行 Plan 中的每一步
// 某步失败时自动触发重规划（RePlan），不中断整体执行
func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) error {
	for i := 0; i < len(plan.Steps); i++ {
		step := &plan.Steps[i]
		step.Status = "running"
		a.debugf("\n  📋 [%d/%d] %s\n", i+1, len(plan.Steps), step.Desc)
		_, err := a.Run(ctx, fmt.Sprintf("请执行：%s", step.Desc))
		if err != nil {
			step.Status = "failed"
			fmt.Printf("  ❌ [%d/%d] 失败：%v\n", i+1, len(plan.Steps), err)

			// 让 LLM 重新规划剩余步骤
			newPlan, replanErr := a.RePlan(ctx, plan, i)
			if replanErr != nil {
				return fmt.Errorf("第 %d 步失败：%w（重规划也失败：%v）", step.ID, err, replanErr)
			}

			// 保留已完成步骤 + 当前失败步骤，替换后续步骤为新方案
			plan.Steps = append(plan.Steps[:i+1], newPlan.Steps...)
			a.debugf("  🔄 已重新规划剩余步骤（新方案共 %d 步）\n", len(newPlan.Steps))
			continue
		}
		step.Status = "done"
		a.debugf("  ✅ [%d/%d] 完成\n", i+1, len(plan.Steps))
	}
	return nil
}

// RePlan 让 LLM 根据已完成和失败的步骤，重新规划后续方案
func (a *Agent) RePlan(ctx context.Context, plan *Plan, failedIndex int) (*Plan, error) {
	var doneText []string
	for i := 0; i < failedIndex; i++ {
		doneText = append(doneText, fmt.Sprintf("✅ %d. %s", plan.Steps[i].ID, plan.Steps[i].Desc))
	}
	var remainingText []string
	for i := failedIndex; i < len(plan.Steps); i++ {
		remainingText = append(remainingText, fmt.Sprintf("❌ %d. %s", plan.Steps[i].ID, plan.Steps[i].Desc))
	}

	prompt := fmt.Sprintf(`你是一个项目规划专家。执行过程中某步失败了，请重新规划后续步骤。

总目标：%s

已完成：
%s

失败/未完成的步骤：
%s

请根据已完成的内容，重新规划剩余步骤。要求：
- 每步是 LLM 一次能处理的粒度
- 按执行顺序排列
- 每步不超过 15 字
- 步骤数不超过 8 步

只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]`,
		plan.Goal, strings.Join(doneText, "\n"), strings.Join(remainingText, "\n"))

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
		return nil, fmt.Errorf("重规划失败：%w", err)
	}

	content := resp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

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
