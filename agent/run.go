package agent

import (
	"context"
	"errors"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// Run 处理一轮用户输入——Agent 核心循环
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
		msg, err := a.streamChat(ctx, reqMessages)
		if err != nil {
			a.recordEvent("error", err.Error(), "")
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		if msg.Content != "" {
			a.recordEvent("assistant", msg.Content, "")
		}
		reqMessages = append(reqMessages, msg)

		// 没 ToolCall → 任务完成（保存 Checkpoint）
		if len(msg.ToolCalls) == 0 {
			a.SaveCheckpoint()
			a.messages = append(a.messages, openai.ChatCompletionMessage{
				Role: "user", Content: userInput,
			})
			a.messages = append(a.messages, msg)
			return msg.Content, nil
		}

		// 有 ToolCall → 依次执行
		for _, tc := range msg.ToolCalls {
			a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
			a.debugf("  🔧 %s(%s)\n", tc.Function.Name, tc.Function.Arguments)

			tool, ok := a.allTools[tc.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具: %s", tc.Function.Name)
			}

			// 高危工具：执行前让用户确认
			if IsHighRiskTool(tc.Function.Name) {
				a.debugf("  🔐 高危操作：%s(%s)\n", tc.Function.Name, tc.Function.Arguments)
				fmt.Print("  确认执行？[Y/n] ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm == "n" || confirm == "N" || confirm == "no" {
					result := fmt.Sprintf("用户已取消执行 %s，请换一种方式", tc.Function.Name)
					fmt.Printf("  🚫 %s\n", result)
					a.recordEvent("tool_result", result, tc.Function.Name)
					reqMessages = append(reqMessages, openai.ChatCompletionMessage{
						Role: "tool", Content: result, ToolCallID: tc.ID,
					})
					continue
				}
			}

			result := tool.Execute(tc.Function.Arguments)
			a.recordEvent("tool_result", result, tc.Function.Name)
			a.debugf("  📦 %s\n", truncate(result, 100))

			// 每 N 次工具调用保存 Checkpoint
			a.toolCallCount++
			if a.toolCallCount%checkpointInterval == 0 {
				a.SaveCheckpoint()
			}

			reqMessages = append(reqMessages, openai.ChatCompletionMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
	}

	a.SaveCheckpoint()
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// streamChat 流式调用 LLM，实时输出文本，同时积累 tool call
func (a *Agent) streamChat(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionMessage, error) {
	stream, err := a.client.CreateChatCompletionStream(ctx,
		openai.ChatCompletionRequest{
			Model:    a.model,
			Messages: messages,
			Tools:    a.toolDefinitions(),
		},
	)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	defer stream.Close()

	var content string
	toolCallAcc := make(map[int]openai.ToolCall)

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}

		if len(resp.Choices) == 0 {
			continue
		}
		delta := resp.Choices[0].Delta

		if delta.Content != "" {
			content += delta.Content
			fmt.Print(delta.Content)
		}

		for _, tc := range delta.ToolCalls {
			if tc.Index == nil {
				continue
			}
			idx := *tc.Index
			existing, ok := toolCallAcc[idx]
			if !ok {
				existing = openai.ToolCall{Index: &idx}
			}
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Type != "" {
				existing.Type = tc.Type
			}
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				existing.Function.Arguments += tc.Function.Arguments
			}
			toolCallAcc[idx] = existing
		}
	}

	if content != "" {
		fmt.Println()
	}

	msg := openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: content,
	}

	if len(toolCallAcc) > 0 {
		msg.ToolCalls = make([]openai.ToolCall, 0, len(toolCallAcc))
		for i := 0; i < len(toolCallAcc); i++ {
			if tc, ok := toolCallAcc[i]; ok {
				msg.ToolCalls = append(msg.ToolCalls, tc)
			}
		}
	}

	return msg, nil
}
