package agent

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

// Run 处理一轮用户输入——Agent 核心循环
// 流程：构建消息 → 调 LLM → 有 ToolCall 就执行 → 结果喂回 → 再调 LLM → 直到 LLM 直接回答
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.recordEvent("user", userInput, "")

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

		// 没 ToolCall → 任务完成
		if len(msg.ToolCalls) == 0 {
			a.messages = append(a.messages, openai.ChatCompletionMessage{
				Role: "user", Content: userInput,
			})
			a.messages = append(a.messages, msg)
			return msg.Content, nil
		}

		// 有 ToolCall → 依次执行
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

	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}
