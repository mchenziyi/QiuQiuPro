package agent

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// DetectMode 轻量级意图分类：判断用户输入走 ask（直接问答）还是 plan（规划执行）。
// 用一次极小的 LLM 调用完成，不进入主循环、不触发工具、不记入 Token 用量（成本可忽略）。
func (a *Agent) DetectMode(ctx context.Context, input string) (string, error) {
	prompt := fmt.Sprintf(`## 任务
判断用户输入应该走哪个模式：
- ask：简单对话、问候、闲聊、不需要使用工具的提问
- plan：开发任务、代码修改、文件操作、需要工具执行的复杂请求

## 用户输入
%s

## 输出
只输出 ask 或 plan，不要输出其他内容。`, input)

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "你是一个意图分类器。只输出 ask 或 plan。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 10,
	})
	if err != nil {
		return "plan", fmt.Errorf("意图分类失败（退化到 plan）：%w", err) // 失败时默认 plan（安全侧）
	}

	answer := strings.TrimSpace(resp.Choices[0].Message.Content)
	switch answer {
	case "ask":
		return "ask", nil
	case "plan":
		return "plan", nil
	default:
		return "plan", nil // 意外输出退化为 plan
	}
}
