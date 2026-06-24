package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// --------------- 任务管理 ---------------

func NewTodoWriteTool() Tool {
	return Tool{
		Name: "todo_write", Description: "记录任务清单", ReadOnly: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content":    map[string]any{"type": "string"},
							"status":     map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
							"activeForm": map[string]any{"type": "string"},
							"level":      map[string]any{"type": "integer", "enum": []any{0, 1}},
						},
					},
				},
			},
			"required": []string{"todos"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Todos []struct {
					Content, Status, ActiveForm string
					Level                       int
				}
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			var done, active, pending int
			for _, t := range p.Todos {
				switch t.Status {
				case "completed":
					done++
				case "in_progress":
					active++
				default:
					pending++
				}
			}
			return fmt.Sprintf("Todos: %d done, %d active, %d pending", done, active, pending), nil
		},
	}
}
