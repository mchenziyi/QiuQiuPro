// Package agent 是球球的核心：Agent 结构体、对话循环、规划与执行
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	allTools     map[string]tool.Tool   // 全部注册的工具（V5：不受 Skill 限制）
	activeTools  []string               // V5：当前 Skill 允许的工具名列表（空 = 全部）
	messages     []openai.ChatCompletionMessage
	store        *event.Store
	session      string
	currentSkill *skill.Skill           // V5：当前 Skill
	sysPrompt    string                 // V5：当前 SystemPrompt
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

func (a *Agent) RegisterTool(t tool.Tool) {
	a.allTools[t.Name] = t
}

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

// ========== V5：Skill 切换 ==========

// ApplySkill 切换到指定 Skill，更换 SystemPrompt 并限制可用工具
func (a *Agent) ApplySkill(s skill.Skill) {
	a.currentSkill = &s
	a.sysPrompt = s.SystemPrompt

	// 有白名单则只保留白名单内的工具
	if len(s.ToolWhitelist) > 0 {
		a.activeTools = make([]string, 0)
		for _, name := range s.ToolWhitelist {
			if _, ok := a.allTools[name]; ok {
				a.activeTools = append(a.activeTools, name)
			}
		}
	} else {
		// 空白名单 = 全部可用
		a.activeTools = nil
	}

	fmt.Printf("🎯 切换到 [%s] 模式：%s\n", s.Name, s.Description)
}

// CurrentSkillName 返回当前 Skill 名
func (a *Agent) CurrentSkillName() string {
	if a.currentSkill != nil {
		return a.currentSkill.Name
	}
	return "default"
}

// 获取当前可用的工具
func (a *Agent) availableTools() []tool.Tool {
	if len(a.activeTools) == 0 {
		// 没有限制则返回全部
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

// 工具定义转 LLM 格式
func (a *Agent) toolDefinitions() []openai.Tool {
	var tools []openai.Tool
	for _, t := range a.availableTools() {
		schema, _ := json.Marshal(t.Parameters)
		var params map[string]any
		json.Unmarshal(schema, &params)
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

func (a *Agent) trimMessages() {
	if len(a.messages) > maxMessages {
		a.messages = append(
			[]openai.ChatCompletionMessage{a.messages[0]},
			a.messages[len(a.messages)-maxMessages+1:]...,
		)
	}
}

func (a *Agent) recordEvent(eventType, content, toolName string) {
	e := event.Event{
		ID:        fmt.Sprintf("%s_%d", a.session, time.Now().UnixNano()),
		Type:      eventType, Content: content, ToolName: toolName,
		Timestamp: time.Now(),
	}
	a.store.Append(a.session, e)
}

// Run 处理一轮用户输入，每步都记录事件
// 运行时自动注入当前 Skill 的 SystemPrompt
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.recordEvent("user", userInput, "")

	// 构建消息列表：如果有 sysPrompt 先插入一条 system 消息
	reqMessages := make([]openai.ChatCompletionMessage, 0)
	if a.sysPrompt != "" {
		reqMessages = append(reqMessages, openai.ChatCompletionMessage{
			Role: "system", Content: a.sysPrompt,
		})
	}
	reqMessages = append(reqMessages, a.messages...)
	reqMessages = append(reqMessages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})

	maxLoops := 15
	for i := 0; i < maxLoops; i++ {
		resp, err := a.client.CreateChatCompletion(ctx,
			openai.ChatCompletionRequest{
				Model:    a.model,
				Messages: reqMessages,
				Tools:    a.toolDefinitions(),
			},
		)
		if err != nil {
			a.recordEvent("error", err.Error(), "")
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}
		msg := resp.Choices[0].Message
		if msg.Content != "" {
			a.recordEvent("assistant", msg.Content, "")
		}
		reqMessages = append(reqMessages, msg)

		if len(msg.ToolCalls) == 0 {
			// 把本轮对话追加到历史
			a.messages = append(a.messages, openai.ChatCompletionMessage{
				Role: "user", Content: userInput,
			})
			a.messages = append(a.messages, msg)
			return msg.Content, nil
		}
		for _, tc := range msg.ToolCalls {
			a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
			fmt.Printf("  🔧 %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			tool, ok := a.allTools[tc.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具: %s", tc.Function.Name)
			}
			result := tool.Execute(tc.Function.Arguments)
			a.recordEvent("tool_result", result, tc.Function.Name)
			fmt.Printf("  📦 %s\n", truncate(result, 100))
			reqMessages = append(reqMessages, openai.ChatCompletionMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
	}
	// 超时，但把最后的 user 消息和历史记录下来
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// ========== Planning（V1）==========

type Step struct {
	ID     int    `json:"id"`
	Desc   string `json:"desc"`
	Status string `json:"status"`
}

type Plan struct {
	Goal  string `json:"goal"`
	Steps []Step `json:"steps"`
}

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

// ========== Event 操作 ==========

func (a *Agent) SessionID() string {
	return a.session
}

func (a *Agent) EventStore() *event.Store {
	return a.store
}

func (a *Agent) TrimMessages() {
	a.trimMessages()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
