package agent

import (
	"encoding/json"
	"context"
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

// ExecutePlan 按顺序执行 Plan 中的每一步
func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) error {
	total := len(plan.Steps)
	for i := range plan.Steps {
		step := &plan.Steps[i]
		step.Status = "running"
		fmt.Printf("\n📋 [%d/%d] %s\n", i+1, total, step.Desc)
		_, err := a.Run(ctx, fmt.Sprintf("请执行：%s", step.Desc))
		if err != nil {
			step.Status = "failed"
			return fmt.Errorf("第 %d 步失败：%w", step.ID, err)
		}
		step.Status = "done"
		fmt.Printf("✅ [%d/%d] 完成\n", i+1, total)
	}
	return nil
}
