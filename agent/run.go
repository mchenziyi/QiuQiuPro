package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

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

	// 本轮用量 = 结束时的会话累计 − 进入时的基线（无需单独的轮次字段，且不被规划等轮外调用污染）。
	usageBefore := a.usage
	defer func() { a.reportTurnUsage(a.usage.Sub(usageBefore)) }()

	maxLoops := 15
	for i := 0; i < maxLoops; i++ {
		// 历史超限时先压缩（LLM 摘要旧消息），避免请求超出上下文窗口。
		a.maybeCompact(ctx)
		// 每轮都从唯一日志重建请求（system 单独前置，不入历史）。
		msg, err := a.streamChat(ctx, a.session.BuildRequest(a.BuildSystemPrompt()))
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

		// 有 ToolCall → 执行。每个 tool_call 都必须回一条 tool 结果，否则会留下悬空的
		// assistant(tool_calls)，破坏「调用/结果」配对、下轮请求即报错。
		// 只读工具并发跑、写/高危工具串行（详见 dispatchToolCalls）。
		a.dispatchToolCalls(msg.ToolCalls)
	}

	a.SaveCheckpoint()
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// dispatchToolCalls 执行一条 assistant 消息里的全部 tool_call，并把结果按**原始顺序**
// 回灌进会话历史（保持「调用/结果」配对，乱序会破坏接口要求的配对约束）。
//
// 并发策略（TODO #9）：只读工具（read_file / grep / glob / code_search / web_fetch 等）
// 互不冲突、不读 stdin、也不向终端流式输出，故并发执行，缩短一轮里多次读取的总耗时；
// 写 / 高危工具（write_file / edit_file_block / run_shell / git_commit）保持串行——
// 既避免文件写竞争与流式输出错乱，也让需要 stdin 确认的高危操作不互相抢输入。
// 两组在时间上重叠：并发读在后台跑的同时，主协程串行处理写操作。
func (a *Agent) dispatchToolCalls(toolCalls []openai.ToolCall) {
	results := make([]string, len(toolCalls))

	// 1) 记录调用事件（串行、有序，便于审计）。
	for _, tc := range toolCalls {
		a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
		a.emitToolCall(tc.Function.Name, tc.Function.Arguments)
	}

	// 2) 并发启动只读工具。每个 goroutine 只写自己那格 results[i]（互不重叠，无共享写）。
	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		if !a.canRunParallel(tc) {
			continue
		}
		wg.Add(1)
		go func(i int, tc openai.ToolCall) {
			defer wg.Done()
			results[i] = a.executeToolCall(tc)
		}(i, tc)
	}

	// 3) 串行执行写 / 高危 / 需确认 / 未知工具（与上面的并发读在时间上重叠）。
	for i, tc := range toolCalls {
		if a.canRunParallel(tc) {
			continue
		}
		results[i] = a.executeToolCall(tc)
	}

	wg.Wait()

	// 4) 按原始顺序回灌结果（串行）：记录结果事件、写入历史、按节奏存档。
	for i, tc := range toolCalls {
		a.recordEvent("tool_result", results[i], tc.Function.Name)
		a.emitToolResult(tc.Function.Name, truncate(results[i], 100))
		a.session.Add(openai.ChatCompletionMessage{
			Role: "tool", Content: results[i], ToolCallID: tc.ID, Name: tc.Function.Name,
		})
		a.toolCallCount++
		if a.toolCallCount%checkpointInterval == 0 {
			a.SaveCheckpoint()
		}
	}
}

// canRunParallel 判断一个工具调用能否安全并发执行：必须是已注册的只读工具，
// 且当前权限门对它直接放行（GateAllow，不需 stdin 确认、也未被拒绝）。其余一律串行。
func (a *Agent) canRunParallel(tc openai.ToolCall) bool {
	if _, ok := a.allTools[tc.Function.Name]; !ok {
		return false // 未知工具：交给串行路径回灌「未知工具」错误
	}
	if !isReadOnlyTool(tc.Function.Name) {
		return false // 写 / 高危：串行，规避竞争与 stdin 抢占
	}
	gate := a.gate
	if gate == nil {
		gate = ConfirmHighRiskGate{}
	}
	d, _ := gate.Check(tc.Function.Name, tc.Function.Arguments)
	return d == GateAllow
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
	hookCtx := a.toolHookContext(tc.Function.Name, tc.Function.Arguments)
	if ok, reason := a.beforeToolHooks(hookCtx); !ok {
		result := fmt.Sprintf("已拒绝执行 %s：%s", tc.Function.Name, reason)
		a.noticef("  🚫 %s\n", result)
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
		a.noticef("  🚫 %s\n", result)
		return result
	case GateConfirm:
		// 确认走统一输入读取器，避免与主循环混用 stdin。
		a.debugf("  🔐 %s：%s(%s)\n", reason, tc.Function.Name, tc.Function.Arguments)
		a.emitPrompt("  确认执行？[Y/n] ")
		if !a.confirm() {
			result := fmt.Sprintf("用户已取消执行 %s，请换一种方式", tc.Function.Name)
			a.noticef("  🚫 %s\n", result)
			return result
		}
	}

	result := t.Execute(tc.Function.Arguments)
	return a.afterToolHooks(hookCtx, result)
}

// streamChat 流式调用 LLM，实时输出文本，同时积累 tool call
func (a *Agent) streamChat(ctx context.Context, messages []openai.ChatCompletionMessage) (openai.ChatCompletionMessage, error) {
	stream, err := a.client.CreateChatCompletionStream(ctx,
		openai.ChatCompletionRequest{
			Model:    a.model,
			Messages: messages,
			Tools:    a.toolDefinitions(),
			// 思考模式强度（DeepSeek V4：high / max）；thinking 关闭时被忽略。空串走服务端默认。
			ReasoningEffort: a.reasoningEffort,
			// 让流式响应在末尾带上用量统计（prompt_tokens 等），用于按窗口比例触发压缩。
			StreamOptions: &openai.StreamOptions{IncludeUsage: true},
		},
	)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}
	defer stream.Close()

	var content, reasoning string
	var usage openai.Usage
	toolCallAcc := make(map[int]openai.ToolCall)

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return openai.ChatCompletionMessage{}, err
		}

		// 用量统计单独走一个 choices 为空的尾包，必须在 continue 之前捕获。
		if resp.Usage != nil && resp.Usage.PromptTokens > 0 {
			usage = *resp.Usage
		}

		if len(resp.Choices) == 0 {
			continue
		}
		delta := resp.Choices[0].Delta

		// 思考模式：reasoning 先于答案流出，灰显展示但**不入历史**（DeepSeek 下一轮会忽略它，
		// 留着只会白占上下文）。
		if delta.ReasoningContent != "" {
			reasoning += delta.ReasoningContent
			a.emitReasoning(delta.ReasoningContent)
		}

		if delta.Content != "" {
			if reasoning != "" && content == "" {
				a.emitToken("\n") // 思考链与最终答案之间空一行
			}
			content += delta.Content
			a.emitToken(delta.Content)
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
		a.emitToken("\n")
	}

	// 记录这次请求的真实用量：prompt_tokens 供 maybeCompact 按窗口比例判定，
	// 完整 usage（含缓存命中 / 思考 token）计入会话累计供 /usage 展示（TODO #14）。
	if usage.PromptTokens > 0 {
		a.lastPromptTokens = usage.PromptTokens
		a.accountUsage(usage)
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

