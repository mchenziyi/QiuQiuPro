package agent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// DetectMode 轻量级意图分类：判断用户输入走 ask（直接问答）还是 plan（规划执行）。
// 策略：先规则快速过滤（零 API 调用），再落轻量 LLM 分类。
func (a *Agent) DetectMode(ctx context.Context, input string) (string, error) {
	// 第一层：规则快速命中 → 零 API 调用
	if isQuickAsk(input) {
		return "ask", nil
	}
	if isQuickPlan(input) {
		return "plan", nil
	}

	// 第二层：LLM 分类（轻量客户端，无 thinking 注入）
	config := openai.DefaultConfig(a.apiKey)
	config.BaseURL = "https://api.deepseek.com"
	config.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	lightClient := openai.NewClientWithConfig(config)

	prompt := fmt.Sprintf(`## 任务
判断用户输入应该走哪个模式：
- ask：简单对话、问候、闲聊、不需要使用工具的提问
- plan：开发任务、代码修改、文件操作、需要工具执行的复杂请求

## 用户输入
%s

## 输出
只输出 ask 或 plan，不要输出其他内容。`, input)

	resp, err := lightClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "你是一个意图分类器。只输出 ask 或 plan。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 10,
	})
	if err != nil {
		return "plan", fmt.Errorf("意图分类失败（退化到 plan）：%w", err)
	}

	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	switch answer {
	case "ask":
		return "ask", nil
	case "plan":
		return "plan", nil
	default:
		return "plan", nil
	}
}

// isQuickAsk 零成本规则：明显是问候/闲聊/身份询问 → 直接走 Ask
func isQuickAsk(input string) bool {
	s := strings.TrimSpace(input)
	lower := strings.ToLower(s)
	switch lower {
	case "你好", "嗨", "hello", "hi", "hey", "在吗", "在不在", "早", "晚上好", "下午好",
		"good morning", "good afternoon", "good evening", "哈喽", "在?":
		return true
	}
	// 极短消息（≤4 个字符，中文为主的招呼/确认）
	runes := []rune(s)
	if len(runes) <= 4 && !strings.ContainsAny(s, "{}()[]<>/\\@#$%^&*") {
		return true
	}
	return false
}

// isQuickPlan 零成本规则：明显是开发任务 → 直接走 Plan
func isQuickPlan(input string) bool {
	planKeywords := []string{
		"写一个", "帮我写", "修改", "重构", "修复", "实现", "添加", "新建", "创建", "删除",
		"优化", "改进", "拆分", "合并", "加入", "集成", "迁移", "升级", "降级",
		"生成", "搭建", "配置", "安装", "部署", "测试", "提交", "commit",
		"代码", "文件", "函数", "方法", "类", "接口", "包", "模块",
		"帮我查", "查一下", "找到", "搜索", "分析", "解释这段代码",
		"fix", "add", "implement", "refactor", "rewrite", "create", "delete",
		"优化这段", "改一下", "加一个", "把这个",
	}
	for _, kw := range planKeywords {
		if strings.Contains(strings.ToLower(input), strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
