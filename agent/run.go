package agent

import (
	"context"
	"errors"
	"fmt"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

// Run 处理一轮用户输入——Agent 核心循环
//
// 全量保留：a.session 是唯一的事实源。一轮对话里的所有往返——用户输入、
// 带 tool_calls 的 assistant 消息、以及每条 tool 结果——都按顺序写进会话历史，
// 不再像过去那样只在结束时存「用户问题 + 最终回答」而丢掉中间的工具上下文。
// 这样下一轮 LLM 能看到上一轮读过哪些文件、工具返回了什么，避免重复读取与「失忆」。
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.recordEvent("user", userInput, "")

	// 用户消息进入唯一的消息日志。
	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: userInput})

	maxLoops := 15
	for i := 0; i < maxLoops; i++ {
		// 每轮都从唯一日志重建请求（system 单独前置，不入历史）。
		msg, err := a.streamChat(ctx, a.session.BuildRequest(a.sysPrompt))
		if err != nil {
			a.recordEvent("error", err.Error(), "")
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		if msg.Content != "" {
			a.recordEvent("assistant", msg.Content, "")
		}
		// assistant 消息（可能带 tool_calls）进入日志。
		a.session.Add(msg)

		// 没 ToolCall → 任务完成（保存 Checkpoint）。
		if len(msg.ToolCalls) == 0 {
			a.SaveCheckpoint()
			return msg.Content, nil
		}

		// 有 ToolCall → 依次执行。每个 tool_call 都必须回一条 tool 结果，
		// 否则会留下悬空的 assistant(tool_calls)，破坏「调用/结果」配对、下轮请求即报错。
		for _, tc := range msg.ToolCalls {
			a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
			a.debugf("  🔧 %s(%s)\n", tc.Function.Name, tc.Function.Arguments)

			result := a.executeToolCall(tc)
			a.recordEvent("tool_result", result, tc.Function.Name)
			a.debugf("  📦 %s\n", truncate(result, 100))

			// tool 结果进入日志（与上面的 assistant.tool_calls 按 ToolCallID 配对）。
			a.session.Add(openai.ChatCompletionMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID, Name: tc.Function.Name,
			})

			// 每 N 次工具调用保存 Checkpoint。
			a.toolCallCount++
			if a.toolCallCount%checkpointInterval == 0 {
				a.SaveCheckpoint()
			}
		}
	}

	a.SaveCheckpoint()
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// executeToolCall 执行单个工具调用并返回结果文本。
//
// 不变量：无论成功、未知工具、还是被用户取消，都必须返回一段文本作为 tool 结果
// 回灌给模型——这样每个 tool_call 都有配对的 tool 结果，历史始终合法。
// 这也使未知工具从「直接中断整轮」变成「把错误喂回，让模型自我纠正」。
func (a *Agent) executeToolCall(tc openai.ToolCall) string {
	t, ok := a.allTools[tc.Function.Name]
	if !ok {
		result := fmt.Sprintf("error: 未知工具 %s", tc.Function.Name)
		a.debugf("  ⚠️  %s\n", result)
		return result
	}

	// 经由权限门裁决：放行 / 确认 / 拒绝。Gate 可插拔（默认高危确认，可切换只读等）。
	gate := a.gate
	if gate == nil {
		gate = ConfirmHighRiskGate{}
	}
	switch decision, reason := gate.Check(tc.Function.Name, tc.Function.Arguments); decision {
	case GateDeny:
		// 拒绝也要回灌一条结果，让模型据此改用只读方式，而非中断整轮。
		result := fmt.Sprintf("已拒绝执行 %s：%s。请改用只读手段（如 read_file / grep / code_search）", tc.Function.Name, reason)
		fmt.Printf("  🚫 %s\n", result)
		return result
	case GateConfirm:
		// 确认走统一输入读取器，避免与主循环混用 stdin。
		a.debugf("  🔐 %s：%s(%s)\n", reason, tc.Function.Name, tc.Function.Arguments)
		fmt.Print("  确认执行？[Y/n] ")
		if !a.confirm() {
			result := fmt.Sprintf("用户已取消执行 %s，请换一种方式", tc.Function.Name)
			fmt.Printf("  🚫 %s\n", result)
			return result
		}
	}

	return t.Execute(tc.Function.Arguments)
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
