package agent

import (
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Token 用量追踪（TODO #14）：按 provider 回传的真实 usage 累计，区分「缓存命中输入」与
// 「思考输出」，并可选地按单价估算花费。数字全部来自接口的 usage 字段而非本地估算，
// 因此口径与账单一致——这也呼应 #13 的缓存友好策略：命中越多越省钱，命中率一眼可见。

// TokenUsage 是一段（单次调用 / 单轮 / 整个会话）的累计 token 用量。
//
// CachedTokens 是命中前缀缓存的输入 token（PromptTokens 的子集，计费远低于未命中）；
// ReasoningTokens 是思考模式产出的输出 token（CompletionTokens 的子集，DeepSeek V4 回传）。
type TokenUsage struct {
	Calls            int // 累计 LLM 调用次数
	PromptTokens     int // 输入 token（含缓存命中）
	CachedTokens     int // 其中命中前缀缓存的输入 token
	CompletionTokens int // 输出 token（含思考）
	ReasoningTokens  int // 其中思考模式产出的输出 token
	TotalTokens      int // 输入 + 输出
}

// Add 把一次 LLM 调用的真实用量并入。明细指针为 nil 时（provider 未回传）对应子项按 0 计。
func (u *TokenUsage) Add(usage openai.Usage) {
	u.Calls++
	u.PromptTokens += usage.PromptTokens
	u.CompletionTokens += usage.CompletionTokens
	u.TotalTokens += usage.TotalTokens
	if d := usage.PromptTokensDetails; d != nil {
		u.CachedTokens += d.CachedTokens
	}
	if d := usage.CompletionTokensDetails; d != nil {
		u.ReasoningTokens += d.ReasoningTokens
	}
}

// AddUsage 合并另一段累计用量（用于把子 Agent 的用量并入父级）。
func (u *TokenUsage) AddUsage(o TokenUsage) {
	u.Calls += o.Calls
	u.PromptTokens += o.PromptTokens
	u.CachedTokens += o.CachedTokens
	u.CompletionTokens += o.CompletionTokens
	u.ReasoningTokens += o.ReasoningTokens
	u.TotalTokens += o.TotalTokens
}

// Sub 返回相对基线 base 的增量（用于从会话累计里切出「本轮」用量，无需另设轮次字段）。
func (u TokenUsage) Sub(base TokenUsage) TokenUsage {
	return TokenUsage{
		Calls:            u.Calls - base.Calls,
		PromptTokens:     u.PromptTokens - base.PromptTokens,
		CachedTokens:     u.CachedTokens - base.CachedTokens,
		CompletionTokens: u.CompletionTokens - base.CompletionTokens,
		ReasoningTokens:  u.ReasoningTokens - base.ReasoningTokens,
		TotalTokens:      u.TotalTokens - base.TotalTokens,
	}
}

// MissTokens 返回未命中缓存的输入 token（按更贵的「未命中」价计费）。异常时兜底为 0。
func (u TokenUsage) MissTokens() int {
	if m := u.PromptTokens - u.CachedTokens; m > 0 {
		return m
	}
	return 0
}

// HitRate 返回输入 token 的缓存命中率（0~1）；无输入时为 0（不除零）。
func (u TokenUsage) HitRate() float64 {
	if u.PromptTokens <= 0 {
		return 0
	}
	return float64(u.CachedTokens) / float64(u.PromptTokens)
}

// Pricing 是每百万 token 的单价（货币单位自定，通常为元）。输入按缓存命中 / 未命中分别计价——
// DeepSeek 命中价远低于未命中。零值表示「未配置」：此时不展示费用，避免给出编造的金额。
type Pricing struct {
	InputMiss float64 // 未命中缓存的输入，每 1M token
	InputHit  float64 // 命中缓存的输入，每 1M token
	Output    float64 // 输出，每 1M token
}

// Enabled 表示是否配置了任一单价。
func (p Pricing) Enabled() bool { return p.InputMiss > 0 || p.InputHit > 0 || p.Output > 0 }

// Cost 按单价估算累计花费（货币单位同 Pricing）。
func (u TokenUsage) Cost(p Pricing) float64 {
	const perMillion = 1_000_000.0
	return float64(u.MissTokens())/perMillion*p.InputMiss +
		float64(u.CachedTokens)/perMillion*p.InputHit +
		float64(u.CompletionTokens)/perMillion*p.Output
}

// FormatCompact 渲染一行紧凑摘要（用于每轮结束的提示）。
func (u TokenUsage) FormatCompact() string {
	return fmt.Sprintf("输入 %d（缓存 %d）· 输出 %d（思考 %d）· 合计 %d",
		u.PromptTokens, u.CachedTokens, u.CompletionTokens, u.ReasoningTokens, u.TotalTokens)
}

// FormatSession 渲染多行会话用量摘要（用于 /usage）；配置了单价时附带估算费用。
func (u TokenUsage) FormatSession(p Pricing) string {
	s := fmt.Sprintf(`📊 会话 token 用量（共 %d 次调用）
   输入 %d（缓存命中 %d，命中率 %.1f%%）
   输出 %d（思考 %d）
   合计 %d`,
		u.Calls,
		u.PromptTokens, u.CachedTokens, u.HitRate()*100,
		u.CompletionTokens, u.ReasoningTokens,
		u.TotalTokens)
	if p.Enabled() {
		s += fmt.Sprintf("\n   估算费用 %.4f（缓存命中已折价；单价可经环境变量校准）", u.Cost(p))
	}
	return s
}

// ----- Agent 侧的用量记账 -----

// accountUsage 把一次 LLM 调用的真实用量计入会话累计。
// 所有 LLM 调用（streamChat 主循环、plan/reflect/replan 规划、compact 摘要）都经此一处记账，
// 故 /usage 汇总与账单口径一致。仅在串行阶段调用（流式收尾、规划），无需加锁。
func (a *Agent) accountUsage(u openai.Usage) { a.usage.Add(u) }

// Usage 返回会话累计 token 用量的快照。
func (a *Agent) Usage() TokenUsage { return a.usage }

// SetPricing 配置 token 单价（供 /usage 估算费用）。零值单价则不展示费用。
func (a *Agent) SetPricing(p Pricing) { a.pricing = p }

// ReportUsage 把会话累计用量（含费用，若已配置单价）输出到 Sink，供 /usage 命令使用。
func (a *Agent) ReportUsage() { a.noticef("%s\n", a.usage.FormatSession(a.pricing)) }

// reportTurnUsage 在一轮 Run 结束后输出该轮新增用量（细节日志，安静模式隐藏）。
func (a *Agent) reportTurnUsage(turn TokenUsage) {
	if turn.Calls <= 0 {
		return // 本轮没有真实用量遥测（如未联网 / provider 未回传），不输出空行
	}
	a.emit(Event{Kind: EventNotice, Text: fmt.Sprintf("  📊 本轮 token｜%s\n", turn.FormatCompact()), Verbose: true})
}
