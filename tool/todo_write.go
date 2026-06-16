package tool

import (
	"encoding/json"
	"fmt"
)

// NewTodoWriteTool 返回 todo_write 工具：记录和更新结构化任务清单。
// 参照 Reasonix builtin/todo.go。
func NewTodoWriteTool() Tool {
	return Tool{
		Name: "todo_write",
		Description: `Record and update a structured task list for the current work. Send the COMPLETE list every call — it replaces the previous one. Use it to plan multi-step work and show progress: keep exactly one item in_progress at a time, and flip an item to completed the moment it's done (don't batch completions). Skip it for trivial single-step tasks. The list is two-level: a level 0 item is a PHASE (a milestone) and the level 1 items after it are its concrete sub-steps; omit level (0) for a flat list. Each item has content (imperative, e.g. "Add the parser"), status (pending|in_progress|completed), activeForm (present-continuous shown while in progress, e.g. "Adding the parser"), and optional level (0 phase | 1 sub-step).`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"todos": map[string]any{
					"type":        "array",
					"description": "The complete task list, in order. Replaces any previous list.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content":    map[string]any{"type": "string", "description": "Imperative description of the task."},
							"status":     map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}, "description": "Task state. Keep at most one in_progress."},
							"activeForm": map[string]any{"type": "string", "description": "Present-continuous form shown while the task is in progress (e.g. \"Running tests\")."},
							"level":      map[string]any{"type": "integer", "enum": []int{0, 1}, "description": "Nesting level: 0 = phase/milestone, 1 = a sub-step of the phase above it. Omit for a flat list."},
						},
						"required": []string{"content", "status"},
					},
				},
			},
			"required": []string{"todos"},
		},
		Execute: func(args string) string {
			var p struct {
				Todos []struct {
					Content    string `json:"content"`
					Status     string `json:"status"`
					ActiveForm string `json:"activeForm,omitempty"`
					Level      int    `json:"level,omitempty"`
				} `json:"todos"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("参数解析失败：%v", err)
			}
			var done, active, pending int
			for _, t := range p.Todos {
				if t.Content == "" {
					return fmt.Sprintf("todo content 不能为空")
				}
				switch t.Status {
				case "completed":
					done++
				case "in_progress":
					active++
				case "pending", "":
					pending++
				default:
					return fmt.Sprintf("无效状态：%s（只支持 pending | in_progress | completed）", t.Status)
				}
			}
			return fmt.Sprintf("Todos updated: %d total — %d completed, %d in progress, %d pending.", len(p.Todos), done, active, pending)
		},
	}
}
