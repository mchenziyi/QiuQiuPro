package agent

import (
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/event"
	"agentdemo/skill"
	"agentdemo/tool"
)

// Agent 核心结构
type Agent struct {
	client       *openai.Client
	model        string
	allTools     map[string]tool.Tool
	activeTools  []string
	messages     []openai.ChatCompletionMessage
	store        *event.Store
	session      string
	currentSkill *skill.Skill
	sysPrompt    string
}

const maxMessages = 100

func New(apiKey, model string) *Agent {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com"
	return &Agent{
		client:   openai.NewClientWithConfig(config),
		model:    model,
		allTools: make(map[string]tool.Tool),
		messages: make([]openai.ChatCompletionMessage, 0),
		store:    event.NewStore(".reasonix/sessions"),
		session:  fmt.Sprintf("session_%d", time.Now().Unix()),
	}
}

func (a *Agent) RegisterTool(t tool.Tool)     { a.allTools[t.Name] = t }
func (a *Agent) RegisterTools(tools []tool.Tool) {
	for _, t := range tools { a.RegisterTool(t) }
}
func (a *Agent) RegisterMCPTools(prefix string, tools []tool.Tool) {
	for _, t := range tools {
		t.Name = fmt.Sprintf("%s_%s", prefix, t.Name)
		a.allTools[t.Name] = t
	}
}

func (a *Agent) ApplySkill(s skill.Skill) {
	a.currentSkill = &s
	a.sysPrompt = s.SystemPrompt
	if len(s.ToolWhitelist) > 0 {
		a.activeTools = make([]string, 0)
		for _, name := range s.ToolWhitelist {
			if _, ok := a.allTools[name]; ok {
				a.activeTools = append(a.activeTools, name)
			}
		}
	} else {
		a.activeTools = nil
	}
	fmt.Printf("🎯 切换到 [%s] 模式：%s\n", s.Name, s.Description)
}

func (a *Agent) CurrentSkillName() string {
	if a.currentSkill != nil { return a.currentSkill.Name }
	return "default"
}

func (a *Agent) availableTools() []tool.Tool {
	if len(a.activeTools) == 0 {
		var tools []tool.Tool
		for _, t := range a.allTools { tools = append(tools, t) }
		return tools
	}
	var tools []tool.Tool
	for _, name := range a.activeTools {
		if t, ok := a.allTools[name]; ok { tools = append(tools, t) }
	}
	return tools
}

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

// highRiskTools 定义需要用户确认的高危工具
// 这些工具会修改文件或执行命令，LLM 可能误用
var highRiskTools = map[string]bool{
	"write_file":      true,
	"edit_file_block": true,
	"run_shell":       true,
	"run_powershell":  true,
}

// IsHighRiskTool 判断工具是否高危
func IsHighRiskTool(name string) bool {
	return highRiskTools[name]
}

func (a *Agent) SessionID() string            { return a.session }
func (a *Agent) EventStore() *event.Store      { return a.store }
func (a *Agent) TrimMessages()                 { a.trimMessages() }
