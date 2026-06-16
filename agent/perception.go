package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	openai "github.com/sashabaranov/go-openai"
)

// ========== 感知层：自动判断用户意图走 Ask 还是 Plan ==========
// 三层策略（参照 Reasonix）：
//   1. 启发式打分（0-7）——基于输入长度、结构、关键词
//   2. 分数 0 → 立即 Ask；分数 ≥3 → 立即 Plan
//   3. 分数 1-2 → LLM 分类器（JSON 返回，Temperature=0，3s 超时）

var numberedListRE = regexp.MustCompile(`(?m)^\s*(?:[-*]|\d+[.)])\s+\S`)

// autoPlanScore 对用户输入做启发式打分，0=纯对话，越高越像开发任务。
func autoPlanScore(input string) int {
	text := strings.TrimSpace(input)
	if text == "" || strings.HasPrefix(text, "/") {
		return 0
	}
	lower := strings.ToLower(text)

	// 低风险问题：以提问关键词开头且不含复杂术语 → 直接放行
	if isLowRiskQuestion(lower) {
		return 0
	}

	score := 0
	if utf8.RuneCountInString(text) >= 160 {
		score++
	}
	if numberedListRE.MatchString(text) {
		score++
	}
	if strings.Count(text, "\n") >= 2 {
		score++
	}
	if containsAny(lower, complexIntentTerms) {
		score++
	}
	if containsAny(lower, multiSurfaceTerms) {
		score++
	}
	if containsAny(lower, docsAndIssueTerms) {
		score++
	}
	// 多个文件引用
	if strings.Count(text, "@") >= 2 || strings.Count(lower, ".go")+
		strings.Count(lower, ".ts")+strings.Count(lower, ".tsx")+strings.Count(lower, ".js") >= 2 {
		score++
	}
	return score
}

// DetectMode 自动判断用户意图：ask（直接问答）| plan（规划执行）。
// 走启发式打分 + 兜底 LLM 分类器（仅对模糊输入调用）。
func (a *Agent) DetectMode(ctx context.Context, input string) (string, error) {
	score := autoPlanScore(input)
	if score >= 3 {
		return "plan", nil
	}

	// score 0-2：都走 LLM 分类器（启发式对短任务漏判太多，拿误判换 API 调用不划算）
	needsPlan, reason, err := a.classifyNeedsPlan(ctx, input, score)
	if err != nil {
		// 分类器失败 → 退化到启发式（score ≥2 即 plan）
		if score >= 2 {
			return "plan", nil
		}
		return "ask", nil
	}
	if reason != "" {
		a.noticef("  💡 %s\n", reason)
	}
	if needsPlan {
		return "plan", nil
	}
	return "ask", nil
}

// classifyNeedsPlan 用轻量 LLM 调用判断是否需要 Plan（3s 超时，Temperature=0）。
func (a *Agent) classifyNeedsPlan(ctx context.Context, input string, score int) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`heuristic_score=%d

USER_REQUEST:
%s`, score, input)

	config := openai.DefaultConfig(a.apiKey)
	config.BaseURL = "https://api.deepseek.com"
	config.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	lightClient := openai.NewClientWithConfig(config)

	resp, err := lightClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: classifyPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
		MaxTokens:   80,
	})
	if err != nil {
		return false, "", fmt.Errorf("classifier call: %w", err)
	}

	content := extractJSONObject(resp.Choices[0].Message.Content)
	var out struct {
		NeedsPlan *bool  `json:"needs_plan"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return false, "", fmt.Errorf("decode classifier response: %w", err)
	}
	if out.NeedsPlan == nil {
		return false, "", fmt.Errorf("missing needs_plan in classifier response")
	}
	return *out.NeedsPlan, out.Reason, nil
}

const classifyPrompt = `You classify whether a coding-agent user request should first enter read-only planning mode.
Return ONLY JSON: {"needs_plan":true|false,"reason":"short reason"}.
Use true for multi-step implementation, refactors, migrations, unclear cross-file work, PRD/spec/issue work, or tasks needing investigation before edits.
Use false for explanations, simple questions, single obvious edits, direct commands, or requests that should be answered without changing files.`

// ========== 启发式打分用词库 ==========

var complexIntentTerms = []string{
	"implement", "add support", "refactor", "migrate", "redesign", "end-to-end",
	"e2e", "wire up", "integration", "fix the issue", "build a",
	"实现", "新增", "支持", "重构", "迁移", "改造", "端到端", "联调", "接入",
	"修复这个问题", "修一下这个问题", "补齐", "设计",
}

var multiSurfaceTerms = []string{
	"multiple files", "several files", "across", "frontend", "backend", "config",
	"tests", "docs", "ui", "api", "database", "schema",
	"多个文件", "多处", "前端", "后端", "配置", "测试", "文档", "接口", "数据库",
}

var docsAndIssueTerms = []string{
	"prd", "issue", "requirements", "spec", "proposal", "roadmap",
	"需求", "产品文档", "接口文档", "方案", "规划",
}

// ========== 辅助函数 ==========

func isLowRiskQuestion(lower string) bool {
	lower = strings.TrimSpace(lower)
	if strings.HasPrefix(lower, "解释") || strings.HasPrefix(lower, "说明") ||
		strings.HasPrefix(lower, "怎么看") || strings.HasPrefix(lower, "查一下") ||
		strings.HasPrefix(lower, "运行") || strings.HasPrefix(lower, "run ") ||
		strings.HasPrefix(lower, "show ") || strings.HasPrefix(lower, "what ") ||
		strings.HasPrefix(lower, "why ") || strings.HasPrefix(lower, "how ") {
		return !containsAny(lower, complexIntentTerms)
	}
	return false
}

func containsAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end >= start {
		return s[start : end+1]
	}
	return s
}



