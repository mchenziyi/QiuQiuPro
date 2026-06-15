package agent

import (
	"bufio"
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
	store         *event.Store
	session       *Session // 会话状态：ID + 对话历史 + 大小管理
	currentSkill  *skill.Skill
	sysPrompt     string
	cmdRegistry   *command.Registry
	lastEventID   string
	Quiet         bool          // true 时隐藏中间日志
	Mode          string        // 运行模式："plan"（默认）| "ask"（直接问答）
	toolCallCount int
	in            *bufio.Reader // 统一的标准输入读取器（主循环 + 确认 + API Key 共用，避免混用）
	gate          Gate          // 权限门：裁决每次工具调用（放行 / 确认 / 拒绝），可插拔
}

const maxMessages = 100
const checkpointInterval = 5

func New(apiKey, model string) *Agent {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com"
	a := &Agent{
		client:      openai.NewClientWithConfig(config),
		model:       model,
		allTools:    make(map[string]tool.Tool),
		store:       event.NewStore(".reasonix/sessions"),
		session:     NewSession(fmt.Sprintf("session_%d", time.Now().Unix())),
		cmdRegistry: command.NewRegistry(),
		Mode:        "plan",
		gate:        ConfirmHighRiskGate{}, // 默认：高危确认，等价于改造前的行为
		sysPrompt:   "在输出结论之前，请先一步步展示你的推理过程。",
	}
	if p, err := LoadRawPrompt("prompt/default/system.xml"); err == nil {
		a.sysPrompt = p
	}
	a.RestoreFromCheckpoint()
	return a
}

func (a *Agent) RegisterTool(t tool.Tool)       { a.allTools[t.Name] = t }
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

// SetMode 切换 Agent 运行模式：plan（规划执行）| ask（直接问答）
func (a *Agent) SetMode(mode string) {
	if mode != "ask" && mode != "plan" {
		fmt.Printf("  ⚠️  未知模式：%s，可选：plan（规划执行）/ ask（直接问答）\n", mode)
		return
	}
	a.Mode = mode
	fmt.Printf("  🔄 切换到 [%s] 模式\n", mode)
}

func (a *Agent) CurrentMode() string { return a.Mode }

// SetGate 替换权限门。
func (a *Agent) SetGate(g Gate) { a.gate = g }

// GateName 返回当前权限门名字（confirm / read-only / allow-all）。
func (a *Agent) GateName() string {
	if a.gate == nil {
		return "confirm"
	}
	return a.gate.Name()
}

// SetReadOnly 开关只读模式：开启用 ReadOnlyGate，关闭恢复默认的 ConfirmHighRiskGate。
func (a *Agent) SetReadOnly(on bool) {
	if on {
		a.gate = ReadOnlyGate{}
	} else {
		a.gate = ConfirmHighRiskGate{}
	}
}

// IsReadOnly 当前是否处于只读模式。
func (a *Agent) IsReadOnly() bool {
	_, ok := a.gate.(ReadOnlyGate)
	return ok
}

func (a *Agent) CurrentSkillName() string {
	if a.currentSkill != nil {
		return a.currentSkill.Name
	}
	return "default"
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
	for _, name := range a.activeTools {
		if t, ok := a.allTools[name]; ok {
			tools = append(tools, t)
		}
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
func (a *Agent) SessionID() string                  { return a.session.ID }
func (a *Agent) EventStore() *event.Store           { return a.store }
func (a *Agent) TrimMessages()                      { a.session.Trim() }

func (a *Agent) debugf(format string, args ...interface{}) {
	if !a.Quiet {
		fmt.Printf(format, args...)
	}
}

func (a *Agent) SaveCheckpoint() {
	data, _ := a.session.Snapshot()
	a.store.SaveCheckpoint(a.session.ID, a.lastEventID, data)
}

func (a *Agent) RestoreFromCheckpoint() bool {
	cp, err := a.store.LoadCheckpoint(a.session.ID)
	if err != nil || cp == nil {
		return false
	}
	if err := a.session.Restore(cp.MessagesJSON); err != nil {
		return false
	}
	a.lastEventID = cp.LastEventID
	fmt.Printf("  💾 从快照恢复 %d 条消息\n", a.session.Len())
	return true
}

func (a *Agent) SpawnSubAgent(ctx context.Context, task string) (string, error) {
	sub := &Agent{
		client:   a.client,
		model:    a.model,
		allTools: a.allTools,
		store:    a.store,
		session:  NewSession(fmt.Sprintf("%s_sub_%d", a.session.ID, time.Now().UnixNano())),
		Quiet:    a.Quiet,
		in:       a.in,   // 子 Agent 共用父级的输入读取器
		gate:     a.gate, // 子 Agent 继承父级权限门（如只读模式）
	}
	return sub.Run(ctx, task)
}
