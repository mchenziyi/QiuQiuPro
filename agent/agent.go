// Package agent 是球球的核心：Agent 结构体、对话循环、规划与执行
// 整合了 tool（工具）、event（事件日志）、skill（行为模式）
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

// Agent 是球球的核心结构体
// 它管理：LLM 客户端、工具注册、对话历史、事件存储、Skill 切换
type Agent struct {
	client       *openai.Client               // LLM 客户端（通过 go-openai SDK 调 DeepSeek API）
	model        string                       // 模型名，如 "deepseek-chat"
	allTools     map[string]tool.Tool         // 全部注册的工具（不随 Skill 切换而改变）
	activeTools  []string                     // 当前 Skill 允许的工具名列表（nil = 全部可用）
	messages     []openai.ChatCompletionMessage // 对话历史（跨多轮 Run 持续累积）
	store        *event.Store                 // 事件存储（V3）
	session      string                       // 当前 session ID，用于事件日志的文件名
	currentSkill *skill.Skill                 // 当前激活的 Skill（V5）
	sysPrompt    string                       // 当前 SystemPrompt（从 Skill 中提取，V5）
}

// maxMessages 对话历史的最大条数
// 超过后丢弃最早的非 system 消息，避免 token 浪费
const maxMessages = 100

// New 创建 Agent 实例
// apiKey: DeepSeek API Key，model: 模型名
func New(apiKey, model string) *Agent {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com" // DeepSeek 兼容 OpenAI 接口
	return &Agent{
		client:   openai.NewClientWithConfig(config),
		model:    model,
		allTools: make(map[string]tool.Tool),
		messages: make([]openai.ChatCompletionMessage, 0),
		store:    event.NewStore(".reasonix/sessions"),
		session:  fmt.Sprintf("session_%d", time.Now().Unix()), // 用时间戳做 session ID
	}
}

// RegisterTool 注册一个工具到 Agent 的 allTools 中
// 工具一旦注册就一直存在，不受 Skill 切换影响
func (a *Agent) RegisterTool(t tool.Tool) {
	a.allTools[t.Name] = t
}

// RegisterTools 批量注册工具
func (a *Agent) RegisterTools(tools []tool.Tool) {
	for _, t := range tools {
		a.RegisterTool(t)
	}
}

// RegisterMCPTools 批量注册从 MCP 发现的工具
// 每个工具名前加 prefix 前缀，避免跟内置工具命名冲突
func (a *Agent) RegisterMCPTools(prefix string, tools []tool.Tool) {
	for _, t := range tools {
		t.Name = fmt.Sprintf("%s_%s", prefix, t.Name) // 如 "filesystem_read_file"
		a.allTools[t.Name] = t
	}
}

// ========== Skill 切换（V5）==========

// ApplySkill 切换到指定 Skill
// 效果：更换 SystemPrompt + 限制可用工具列表（白名单）
func (a *Agent) ApplySkill(s skill.Skill) {
	a.currentSkill = &s
	a.sysPrompt = s.SystemPrompt

	// 如果有工具白名单，只保留在白名单内的工具
	if len(s.ToolWhitelist) > 0 {
		a.activeTools = make([]string, 0)
		for _, name := range s.ToolWhitelist {
			if _, ok := a.allTools[name]; ok {
				a.activeTools = append(a.activeTools, name)
			}
		}
	} else {
		// 空白名单 = 全部工具可用
		a.activeTools = nil
	}

	fmt.Printf("🎯 切换到 [%s] 模式：%s\n", s.Name, s.Description)
}

// CurrentSkillName 返回当前 Skill 的名称
// 如果没有激活 Skill 则返回 "default"
func (a *Agent) CurrentSkillName() string {
	if a.currentSkill != nil {
		return a.currentSkill.Name
	}
	return "default"
}

// availableTools 获取当前 Skill 允许使用的工具
// 根据 activeTools 白名单从 allTools 中筛选
func (a *Agent) availableTools() []tool.Tool {
	if len(a.activeTools) == 0 {
		// 没有限制，返回全部注册的工具
		var tools []tool.Tool
		for _, t := range a.allTools {
			tools = append(tools, t)
		}
		return tools
	}
	// 有限制，只返回在白名单中的工具
	var tools []tool.Tool
	for _, name := range a.activeTools {
		if t, ok := a.allTools[name]; ok {
			tools = append(tools, t)
		}
	}
	return tools
}

// toolDefinitions 把球球的 Tool 定义转换为 LLM API 认识的格式
// LLM 通过这个列表知道"有哪些工具可以用、怎么用"
func (a *Agent) toolDefinitions() []openai.Tool {
	var tools []openai.Tool
	for _, t := range a.availableTools() {
		schema, _ := json.Marshal(t.Parameters)   // JSON Schema → JSON 字符串
		var params map[string]any
		json.Unmarshal(schema, &params)            // 转为 map
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,                // 传给 LLM 的参数定义
			},
		})
	}
	return tools
}

// trimMessages 截断对话历史
// 超过 maxMessages 条时保留第一条 + 最近 maxMessages-1 条
func (a *Agent) trimMessages() {
	if len(a.messages) > maxMessages {
		a.messages = append(
			[]openai.ChatCompletionMessage{a.messages[0]},              // 保留第一条（system prompt）
			a.messages[len(a.messages)-maxMessages+1:]...,              // 保留最近的 N-1 条
		)
	}
}

// recordEvent 记录一条事件到 Event Log
// 每步操作（用户输入、LLM 回复、工具调用、工具结果）都记录
func (a *Agent) recordEvent(eventType, content, toolName string) {
	e := event.Event{
		ID:        fmt.Sprintf("%s_%d", a.session, time.Now().UnixNano()),
		Type:      eventType,
		Content:   content,
		ToolName:  toolName,
		Timestamp: time.Now(),
	}
	a.store.Append(a.session, e)
}

// Run 处理一轮用户输入——这是 Agent 的核心循环
// 流程：构建消息 → 调 LLM → 有 ToolCall 就执行 → 结果喂回去 → 再调 LLM → 直到 LLM 直接回答
// 运行时自动注入当前 Skill 的 SystemPrompt 和工具白名单
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	// 记录用户输入事件
	a.recordEvent("user", userInput, "")

	// 构建消息列表：system prompt（如果有）+ 历史 + 本次用户输入
	reqMessages := make([]openai.ChatCompletionMessage, 0)
	if a.sysPrompt != "" {
		reqMessages = append(reqMessages, openai.ChatCompletionMessage{
			Role: "system", Content: a.sysPrompt,
		})
	}
	reqMessages = append(reqMessages, a.messages...) // 追加历史消息
	reqMessages = append(reqMessages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})

	maxLoops := 15 // 防止无限循环烧 token
	for i := 0; i < maxLoops; i++ {
		// ① 调 LLM：发送消息 + 可用工具列表
		resp, err := a.client.CreateChatCompletion(ctx,
			openai.ChatCompletionRequest{
				Model:    a.model,
				Messages: reqMessages,
				Tools:    a.toolDefinitions(), // 注入当前 Skill 允许的工具
			},
		)
		if err != nil {
			a.recordEvent("error", err.Error(), "")
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		// ② 处理 LLM 的回复
		msg := resp.Choices[0].Message
		if msg.Content != "" {
			a.recordEvent("assistant", msg.Content, "")
		}
		reqMessages = append(reqMessages, msg)

		// ③ 判断：没有 ToolCall → LLM 直接回答了，任务完成
		if len(msg.ToolCalls) == 0 {
			// 把本轮对话记录到持久化的消息历史中
			a.messages = append(a.messages, openai.ChatCompletionMessage{
				Role: "user", Content: userInput,
			})
			a.messages = append(a.messages, msg)
			return msg.Content, nil
		}

		// ④ 有 ToolCall → 遍历每个工具调用，依次执行
		for _, tc := range msg.ToolCalls {
			// 记录工具调用事件
			a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
			fmt.Printf("  🔧 %s(%s)\n", tc.Function.Name, tc.Function.Arguments)

			// 从 allTools 中查找对应工具（不受 Skill 白名单限制，但白名单控制是否暴露）
			tool, ok := a.allTools[tc.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具: %s", tc.Function.Name)
			}
			// 执行工具
			result := tool.Execute(tc.Function.Arguments)
			// 记录工具结果事件
			a.recordEvent("tool_result", result, tc.Function.Name)
			fmt.Printf("  📦 %s\n", truncate(result, 100))

			// 把工具结果加入消息列表，让 LLM 能"看到"结果
			reqMessages = append(reqMessages, openai.ChatCompletionMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
		// ⑤ 继续循环：LLM 看到工具结果后，决定下一步（再调工具 or 直接回答）
	}

	// 超过最大循环次数，返回超时
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// ========== Planning（V1）==========

// Step 表示计划中的一步
type Step struct {
	ID     int    `json:"id"`     // 步骤编号
	Desc   string `json:"desc"`   // 步骤描述，如"读取 router.go"
	Status string `json:"status"` // 状态：pending / running / done / failed
}

// Plan 表示一个执行计划，包含总目标和步骤列表
type Plan struct {
	Goal  string `json:"goal"`  // 总目标，如"添加健康检查接口"
	Steps []Step `json:"steps"` // 步骤列表
}

// GeneratePlan 让 LLM 把用户目标拆解为可执行的步骤
// 把当前可用的工具列表也传给 LLM，确保拆出的步骤能用现有工具完成
func (a *Agent) GeneratePlan(ctx context.Context, goal string) (*Plan, error) {
	// 列出当前可用工具（受 Skill 白名单限制）
	var toolList []string
	for _, t := range a.availableTools() {
		toolList = append(toolList, fmt.Sprintf("- %s：%s", t.Name, t.Description))
	}

	// 构造规划 prompt：告诉 LLM 拆步骤的规则，并给一个示例
	prompt := fmt.Sprintf(`你是一个项目规划专家。把目标拆成 3~8 个步骤。

可用工具：
%s

要求：每步必须能用上面工具完成，按顺序，每步不超过 15 字，不超过 8 步。
只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]

目标：%s`, strings.Join(toolList, "\n"), goal)

	// 调 LLM 拆步骤
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

	// 解析 LLM 返回的 JSON
	content := resp.Choices[0].Message.Content
	// 清理可能的 markdown 代码块标记（```json ... ```）
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

	// 转换为 Plan 结构体
	plan := &Plan{Goal: goal}
	for _, s := range steps {
		plan.Steps = append(plan.Steps, Step{ID: s.ID, Desc: s.Desc, Status: "pending"})
	}
	return plan, nil
}

// ExecutePlan 按顺序执行 Plan 中的每一步
// 每步都通过 Run() 处理（即：每步都是一次完整的 LLM 循环）
func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) error {
	total := len(plan.Steps)
	for i := range plan.Steps {
		step := &plan.Steps[i]
		step.Status = "running" // 标记为执行中
		fmt.Printf("\n📋 [%d/%d] %s\n", i+1, total, step.Desc)

		// 把当前步骤描述作为任务交给 LLM
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

// ========== Event 操作（V3）==========

// SessionID 返回当前 session 的唯一标识
func (a *Agent) SessionID() string {
	return a.session
}

// EventStore 返回事件存储实例，供主程序调用 Replay 等方法
func (a *Agent) EventStore() *event.Store {
	return a.store
}

// TrimMessages 截断消息历史，由主程序在每轮对话后调用
func (a *Agent) TrimMessages() {
	a.trimMessages()
}

// truncate 截断字符串到指定长度（按字符数，不是字节数）
// 用于打印日志时避免输出太长刷屏
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
