package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"agentdemo/tool"
)

// NewAskTool 返回 ask 工具：当用户请求有多个合理方案时，模型调用此工具列出选项让用户选。
// 模型给出问题 + 2-4 个选项，每个选项包含 label 和可选 description。
// 返回值是用户的选择。不可并行、不可在只读模式下调用。
func (a *Agent) NewAskTool() tool.Tool {
	return tool.Tool{
		Name: "ask",
		Description: `When the user's request leaves a real choice — which approach, which library, the scope, or a consequential decision — call this tool to present 2-4 concrete options. Do NOT silently pick one approach. Do NOT ask in prose. Use this tool to let the user decide.

Output: the user's selection (the label they chose, or "cancelled" if they declined).

Input:
- question: a short, neutral question describing what needs to be decided
- options: 2-4 options, each with a short label and optional one-line description
- allowMultiple: if true, the user may select more than one option`,
		ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"question": map[string]any{"type": "string", "description": "简短中立的问题描述，让用户知道要决定什么"},
				"options": map[string]any{
					"type": "array",
					"minItems": 2,
					"maxItems": 4,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"label":       map[string]any{"type": "string", "description": "选项的简短名称（1-3 个字）"},
							"description": map[string]any{"type": "string", "description": "选项的一行说明（可选）"},
						},
						"required": []string{"label"},
					},
				},
				"allowMultiple": map[string]any{"type": "boolean", "description": "是否允许用户选择多个"},
			},
			"required": []string{"question", "options"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Question      string `json:"question"`
				Options       []struct {
					Label       string `json:"label"`
					Description string `json:"description,omitempty"`
				} `json:"options"`
				AllowMultiple bool `json:"allowMultiple,omitempty"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Question == "" {
				return "", fmt.Errorf("question 不能为空")
			}
			if len(p.Options) < 2 || len(p.Options) > 4 {
				return "", fmt.Errorf("options 需要 2-4 个，当前 %d 个", len(p.Options))
			}

			// 展示选项给用户
			notice := fmt.Sprintf("\n  💬 %s\n", p.Question)
			for i, opt := range p.Options {
				notice += fmt.Sprintf("    %d) %s", i+1, opt.Label)
				if opt.Description != "" {
					notice += " — " + opt.Description
				}
				notice += "\n"
			}
			if p.AllowMultiple {
				notice += "  (可多选，用逗号分隔，如 1,3。输入 0 取消)\n"
			} else {
				notice += "  (输入序号，输入 0 取消)\n"
			}
			notice += "  你的选择？"
			a.noticef("%s", notice)

			// 读取用户输入
			a.emitPrompt("")
			line, ok := a.ReadLine()
			if !ok {
				return "cancelled (EOF)", nil
			}
			choice := strings.TrimSpace(line)

			if choice == "0" || choice == "" {
				return "cancelled", nil
			}

			if p.AllowMultiple {
				return fmt.Sprintf("selected: %s", choice), nil
			}

			// 单选的序号
			if idx, err := strconv.Atoi(choice); err == nil && idx >= 1 && idx <= len(p.Options) {
				return fmt.Sprintf("selected: %s", p.Options[idx-1].Label), nil
			}

			return fmt.Sprintf("selected: %s", choice), nil
		},
	}
}
