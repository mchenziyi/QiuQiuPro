package agent

import (
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

// 工具相关：注册、按 Skill 白名单筛选、转成 LLM 的 function 定义，以及风险分类。

func (a *Agent) RegisterTool(t tool.Tool) { a.allTools[t.Name] = t }

func (a *Agent) RegisterTools(tools []tool.Tool) {
	for _, t := range tools {
		a.RegisterTool(t)
	}
}

func (a *Agent) RegisterMCPTools(prefix string, tools []tool.Tool) {
	for _, t := range tools {
		t.Name = fmt.Sprintf("%s_%s", prefix, t.Name)
		a.allTools[t.Name] = t
	}
}

// availableTools 返回当前生效的工具：无 Skill 白名单时给全集，否则只给白名单内的。
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

// toolDefinitions 把生效工具转成 OpenAI/DeepSeek 接口要求的 function 定义。
func (a *Agent) toolDefinitions() []openai.Tool {
	var tools []openai.Tool
	for _, t := range a.availableTools() {
		data, _ := json.Marshal(t.Parameters)
		var params map[string]any
		json.Unmarshal(data, &params)
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name: t.Name, Description: t.Description, Parameters: params,
			},
		})
	}
	return tools
}

var highRiskTools = map[string]bool{
	"write_file":      true,
	"edit_file_block": true,
	"bash":       true,
	"run_powershell":  true,
}

// IsHighRiskTool 判断工具是否高危（写文件 / 编辑 / 执行命令），需用户确认。
func IsHighRiskTool(name string) bool {
	return highRiskTools[name]
}

// isReadOnlyTool 判断工具是否「只读、无副作用、不读 stdin」——即可安全并发执行。
// 集合与 ReadOnlyGate 放行的一致：非高危（写文件 / 编辑 / 执行命令）且不改动仓库
// （git_commit）。新增改动类工具时只需更新 highRiskTools，这里与只读门会一并跟上。
func isReadOnlyTool(name string) bool {
	if name == memoryToolName {
		return false
	}
	return !IsHighRiskTool(name) && name != "git_commit"
}

