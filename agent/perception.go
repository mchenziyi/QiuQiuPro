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
// 使用独立的輕量客户端（不含 thinking 注入），避免 DeepSeek 思考模式拖慢分类。
func (a *Agent) DetectMode(ctx context.Context, input string) (string, error) {
	// 创建轻量分类客户端（不用 a.client，因为它的 transport 有 thinking 注入）
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
