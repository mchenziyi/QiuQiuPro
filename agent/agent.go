package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/command"
	"agentdemo/event"
	"agentdemo/skill"
	"agentdemo/tool"
)

// Agent 核心结构
type Agent struct {
	client        *openai.Client
	model         string
	allTools      map[string]tool.Tool
	activeTools   []string
	messages      []openai.ChatCompletionMessage
	store         *event.Store
	session       string
	currentSkill  *skill.Skill
	sysPrompt     string
	cmdRegistry   *command.Registry
	lastEventID   string
	Quiet         bool       // true 时隐藏中间日志（🔧📦等）
	toolCallCount int
}

const maxMessages = 100
const checkpointInterval = 5

func New(apiKey, model string) *Agent {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com"
	a := &Agent{
		client:   openai.NewClientWithConfig(config),
		model:    model,
		allTools: make(map[string]tool.Tool),
		messages: make([]openai.ChatCompletionMessage, 0),
		store:    event.NewStore(".reasonix/sessions"),
		session:  fmt.Sprintf("session_%d", time.Now().Unix()),
		cmdRegistry: command.NewRegistry(),
		sysPrompt: "在输出结论之前，请先一步步展示你的推理过程。",
	}
	a.RestoreFromCheckpoint()
	return a
}

func (a *Agent) RegisterTool(t tool.Tool)       { a.allTools[t.Name] = t }
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

var highRiskTools = map[string]bool{
	"write_file":      true,
	"edit_file_block": true,
	"run_shell":       true,
	"run_powershell":  true,
}

func IsHighRiskTool(name string) bool {
	return highRiskTools[name]
}

func (a *Agent) CommandRegistry() *command.Registry { return a.cmdRegistry }
func (a *Agent) SessionID() string                  { return a.session }
func (a *Agent) EventStore() *event.Store           { return a.store }
func (a *Agent) TrimMessages()                      { a.trimMessages() }

// SaveCheckpoint 保存当前状态快照
func (a *Agent) debugf(format string, args ...interface{}) {
	if !a.Quiet {
		fmt.Printf(format, args...)
	}
}

func (a *Agent) SaveCheckpoint() {
	data, _ := json.Marshal(a.messages)
	a.store.SaveCheckpoint(a.session, a.lastEventID, string(data))
}

// RestoreFromCheckpoint 从快照恢复状态
func (a *Agent) RestoreFromCheckpoint() bool {
	cp, err := a.store.LoadCheckpoint(a.session)
	if err != nil || cp == nil {
		return false
	}
	var msgs []openai.ChatCompletionMessage
	if err := json.Unmarshal([]byte(cp.MessagesJSON), &msgs); err != nil {
		return false
	}
	a.messages = msgs
	a.lastEventID = cp.LastEventID
	fmt.Printf("  💾 从快照恢复 %d 条消息\n", len(msgs))
	return true
}

// SpawnSubAgent 创建一个子 Agent，共享 LLM 客户端和工具，但有独立的对话历史
// 子 Agent 用于执行独立的子任务（如查文档、写测试），不干扰主 Agent 的上下文
func (a *Agent) SpawnSubAgent(ctx context.Context, task string) (string, error) {
	sub := &Agent{
		client:   a.client,
		model:    a.model,
		allTools: a.allTools,
		messages: make([]openai.ChatCompletionMessage, 0),
		store:    a.store,
		session:  fmt.Sprintf("%s_sub_%d", a.session, time.Now().UnixNano()),
		Quiet:    a.Quiet,
	}
	return sub.Run(ctx, task)
}


