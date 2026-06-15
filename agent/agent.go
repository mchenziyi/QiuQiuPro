package agent

import (
	"bufio"
	"context"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/command"
	"agentdemo/event"
	"agentdemo/skill"
	"agentdemo/tool"
)

// Agent 核心结构。按职责拆分到同包的多个文件：
//   - tools.go      工具注册 / 筛选 / 定义 / 风险分类
//   - skill.go      Skill 人格与 plan/ask 模式
//   - gate.go       权限门（接口 + Agent 控制方法）
//   - sink.go       事件驱动输出（Event/Sink + Agent 发射方法）
//   - checkpoint.go 会话快照存档 / 恢复
//   - session.go    会话历史对象
//   - run.go        核心循环 / 工具分发 / 流式
//   - plan.go       规划 / 反思 / 重规划
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
	Quiet         bool   // true 时隐藏中间日志
	Mode          string // 运行模式："plan"（默认）| "ask"（直接问答）
	toolCallCount int
	in            *bufio.Reader // 统一的标准输入读取器（主循环 + 确认 + API Key 共用，避免混用）
	gate          Gate          // 权限门：裁决每次工具调用（放行 / 确认 / 拒绝），可插拔
	sink          Sink          // 输出去向：把运行事件渲染到控制台 / UI / 测试捕获，可插拔
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
		sink:        ConsoleSink{},         // 默认：渲染到控制台，等价于改造前的 fmt.Print
		sysPrompt:   "在输出结论之前，请先一步步展示你的推理过程。",
	}
	if p, err := LoadRawPrompt("prompt/default/system.xml"); err == nil {
		a.sysPrompt = p
	}
	a.RestoreFromCheckpoint()
	return a
}

func (a *Agent) CommandRegistry() *command.Registry { return a.cmdRegistry }
func (a *Agent) SessionID() string                  { return a.session.ID }
func (a *Agent) EventStore() *event.Store           { return a.store }
func (a *Agent) TrimMessages()                      { a.session.Trim() }

// SpawnSubAgent 派生一个共享客户端 / 工具 / 存储的子 Agent，独立会话执行一个子任务。
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
		sink:     a.sink, // 子 Agent 继承父级输出去向
	}
	return sub.Run(ctx, task)
}
