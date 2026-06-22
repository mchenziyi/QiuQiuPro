package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/command"
	"agentdemo/event"
	"agentdemo/skill"
	"agentdemo/tool"
)

const defaultSystemPrompt = "你是球球（QiuQiuPro），一个 Coding Agent。始终用中文回答，代码和术语保留原文。"

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
	client           *openai.Client
	apiKey           string
	model            string
	allTools         map[string]tool.Tool
	activeTools      []string
	store            *event.Store
	session          *Session // 会话状态：ID + 对话历史 + 大小管理
	currentSkill     *skill.Skill
	sysPrompt        string
	defaultSysPrompt string
	cmdRegistry      *command.Registry
	lastEventID      string
	Quiet            bool        // true 时隐藏中间日志
	Mode             string      // 运行模式："plan"（规划执行）| "ask"（直接问答）
	planMode         atomic.Bool // true=只读调研模式，写工具被拒绝
	toolCallCount    int
	in               *bufio.Reader // 统一的标准输入读取器（主循环 + 确认 + API Key 共用，避免混用）
	gate             Gate          // 权限门：裁决每次工具调用（放行 / 确认 / 拒绝），可插拔
	sink             Sink          // 输出去向：把运行事件渲染到控制台 / UI / 测试捕获，可插拔
	summarizer       summarizeFunc // 上下文压缩时产出摘要（默认走 LLM，可注入便于测试）
	reasoningEffort  string        // DeepSeek V4 思考强度："max"（默认）/ "high"；thinking 关闭时被忽略

	// 上下文压缩：按「占模型窗口的比例」触发，靠真实用量驱动，对前缀缓存友好。
	contextWindow       int     // 模型上下文窗口（token）；<=0 关闭自动压缩
	compactRatio        float64 // 提示达到窗口该比例时触发压缩
	softCompactRatio    float64 // 达到该比例时提醒一次（不压缩）
	compactForceRatio   float64 // 高水位强制压缩
	lastPromptTokens    int     // 上一轮 LLM 调用的真实 prompt_tokens
	softCompactNoticed  bool
	consecutiveCompacts int
	compactStuck        bool

	// 前缀缓存：稳定 system 段 + turn-tail 记忆 + 诊断与会话级命中统计（对齐 Reasonix）。
	cachedSystemPrompt  string
	pendingMemory       []string
	lastPrefixShape     PrefixShape
	haveLastPrefixShape bool
	sessCacheHit        atomic.Int64
	sessCacheMiss       atomic.Int64

	// Token 用量追踪：累计所有 LLM 调用的真实用量；pricing 可选，配置后 /usage 展示费用。
	usage   TokenUsage
	pricing Pricing

	// 可控长任务执行：maxSteps 限制一次连续执行的 Plan step 数；pauseRequested 协作式暂停。
	maxSteps       int
	pauseRequested bool

	// 工具 Hook：所有工具执行前后统一经过 hook 链。
	toolHooks []ToolHook

	// 风暴检测：跟踪连续同类工具错误的次数（参照 Reasonix stormBreaker）。
	stormSig   string
	stormCount int

	// 中断控制：用户按 Ctrl+C 时设为 1，ReadLine 检测后重置为 0 继续等待；
	// Run 循环每轮检测，若为 1 则停止当前操作并返回。
	interrupted int32

	// confirmChan：Web UI 模式下，GateConfirm 通过此通道异步等待用户批准。
	// CLI 模式下为 nil，回退到 stdin 确认。
	confirmChan chan bool

	// 偏好/规则型长期记忆：由模型通过受限工具自主写入，system prompt 稳定注入。
	memoryStore     *MemoryStore
	qiuqiuRuleFiles []QiuqiuRuleFile
}

const maxMessages = 100
const checkpointInterval = 5

// ErrNoSessionToResume 表示 -c/--continue 时磁盘上没有可恢复的会话。
var ErrNoSessionToResume = errors.New("没有可恢复的会话")

// New 创建 Agent。continueSession 为 true 时恢复最近一次 checkpoint（对齐 Reasonix -c/--continue）。
func New(apiKey, model string, continueSession bool) (*Agent, error) {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com"
	thinking, effort := deepSeekThinkingConfig() // 默认开启 thinking + max，可经环境变量调整
	config.HTTPClient = newDeepSeekHTTPClient(thinking)
	store := event.NewStore(".reasonix/sessions")
	var sessionID string
	if continueSession {
		sessionID = store.ResolveSessionID()
		if sessionID == "" {
			return nil, ErrNoSessionToResume
		}
	} else {
		sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
	}
	a := &Agent{
		client:      openai.NewClientWithConfig(config),
		apiKey:      apiKey,
		model:       model,
		allTools:    make(map[string]tool.Tool),
		store:       store,
		session:     NewSession(sessionID),
		cmdRegistry: command.NewRegistry(),
		Mode:        "ask",
		gate:        ConfirmHighRiskGate{}, // 默认：高危确认，等价于改造前的行为
		sink:        ConsoleSink{},         // 默认：渲染到控制台，等价于改造前的 fmt.Print
		sysPrompt:   defaultSystemPrompt,

		contextWindow:     defaultContextWindow,
		compactRatio:      defaultCompactRatio,
		softCompactRatio:  defaultSoftRatio,
		compactForceRatio: defaultCompactForce,
		reasoningEffort:   effort,
		memoryStore:       DefaultMemoryStore(),
		qiuqiuRuleFiles:   DefaultQiuqiuRuleFiles(),
	}
	if p, err := LoadRawPrompt("prompt/default/system.xml"); err == nil {
		a.sysPrompt = p
	}
	a.defaultSysPrompt = a.sysPrompt
	a.composeCachedSystemPrompt()
	a.summarizer = a.llmSummarize // 默认摘要器：走真实 LLM
	if continueSession {
		if !a.RestoreFromCheckpoint() {
			return nil, ErrNoSessionToResume
		}
		if cp, _ := store.LoadCheckpoint(sessionID); cp != nil {
			a.maybeColdResumePrune(cp.CreatedAt)
		}
	}
	return a, nil
}

func (a *Agent) CommandRegistry() *command.Registry { return a.cmdRegistry }
func (a *Agent) SessionID() string                  { return a.session.ID }
func (a *Agent) EventStore() *event.Store           { return a.store }
func (a *Agent) TrimMessages()                      { a.session.Trim() }

// SpawnSubAgent 派生一个共享客户端 / 工具 / 存储的子 Agent，独立会话执行一个子任务。
func (a *Agent) SpawnSubAgent(ctx context.Context, task string) (string, error) {
	sub := &Agent{
		client:           a.client,
		model:            a.model,
		allTools:         a.allTools,
		store:            a.store,
		session:          NewSession(fmt.Sprintf("%s_sub_%d", a.session.ID, time.Now().UnixNano())),
		Quiet:            a.Quiet,
		in:               a.in,   // 子 Agent 共用父级的输入读取器
		gate:             a.gate, // 子 Agent 继承父级权限门（如只读模式）
		sink:             a.sink, // 子 Agent 继承父级输出去向
		sysPrompt:        a.sysPrompt,
		defaultSysPrompt: a.defaultSysPrompt,

		contextWindow:     a.contextWindow, // 继承上下文压缩配置，子任务过长时同样兜底
		compactRatio:      a.compactRatio,
		softCompactRatio:  a.softCompactRatio,
		compactForceRatio: a.compactForceRatio,
		reasoningEffort:   a.reasoningEffort, // 思考强度随父级（thinking 开关随共享的 client）
		pricing:           a.pricing,         // 沿用父级单价，便于子任务费用并入后口径一致
		maxSteps:          a.maxSteps,        // 子任务同样受步数上限保护，但不继承暂停请求
		toolHooks:         a.toolHooks,
		memoryStore:       a.memoryStore,
		qiuqiuRuleFiles:   a.qiuqiuRuleFiles,
	}
	sub.composeCachedSystemPrompt()
	sub.summarizer = sub.llmSummarize
	result, err := sub.Run(ctx, task)
	// 子任务用量并入父级会话总量（子 Agent 串行执行，无并发写 a.usage）。
	a.usage.AddUsage(sub.usage)
	return result, err
}

func (a *Agent) SetPlanMode(v bool) { a.planMode.Store(v) }
