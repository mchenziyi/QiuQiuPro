package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

func (a *Agent) RegisterTool(t tool.Tool) { a.allTools[t.Name] = t }

func (a *Agent) RegisterTools(tools []tool.Tool) {
	for _, t := range tools {
		a.RegisterTool(t)
	}
}

func (a *Agent) RegisterMCPTools(prefix string, tools []tool.Tool) {
	for _, t := range tools {
		if !strings.HasPrefix(t.Name, prefix+"_") {
			t.Name = fmt.Sprintf("%s_%s", prefix, t.Name)
		}
		a.allTools[t.Name] = t
	}
}

func (a *Agent) availableTools() []tool.Tool {
	if len(a.activeTools) == 0 {
		var tools []tool.Tool
		for _, t := range a.allTools {
			tools = append(tools, t)
		}
		return tools
	}
	var tools []tool.Tool
	seen := map[string]bool{}
	for _, name := range a.activeTools {
		if t, ok := a.allTools[name]; ok {
			tools = append(tools, t)
			seen[name] = true
		}
	}
	if t, ok := a.allTools[memoryToolName]; ok && !seen[memoryToolName] {
		tools = append(tools, t)
	}
	return tools
}

// HasReadOnlyTools 检查当前是否有可用的只读工具，供 Plan 调研阶段使用。
// remember_rule 是写工具，不计入。
func (a *Agent) HasReadOnlyTools() bool {
	for _, t := range a.allTools {
		if t.Name == memoryToolName {
			continue
		}
		if t.ReadOnly {
			return true
		}
	}
	return false
}

func (a *Agent) toolDefinitions() []openai.Tool {
	tools := a.availableTools()
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	var out []openai.Tool
	for _, t := range tools {
		data, _ := json.Marshal(t.Parameters)
		var params map[string]any
		json.Unmarshal(data, &params)
		out = append(out, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name: t.Name, Description: t.Description, Parameters: params,
			},
		})
	}
	return out
}

// isReadOnlyTool 从工具的 ReadOnly 字段判断是否为只读/无副作用工具。
// 对内置工具直接查字段，未知工具按名称降级为旧名单判断。
func (a *Agent) isReadOnlyTool(name string) bool {
	if name == memoryToolName {
		return false
	}
	if t, ok := a.allTools[name]; ok {
		return t.ReadOnly
	}
	// 未知工具降级：不在旧高危名单里且不是 git_commit 就放行
	return !highRiskTools[name] && name != "git_commit"
}

var highRiskTools = map[string]bool{
	"write_file": true,
	"edit_file":  true,
	"bash":       true,
}

func IsHighRiskTool(name string) bool {
	return highRiskTools[name]
}
