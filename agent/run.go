package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

// Run 处理一轮用户输入——Agent 核心循环（无硬上限，由风暴检测兜底）
func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.recordEvent("user", userInput, "")

	a.session.Add(openai.ChatCompletionMessage{Role: "user", Content: userInput})

	usageBefore := a.usage
	defer func() { a.reportTurnUsage(a.usage.Sub(usageBefore)) }()

	// 重置风暴状态（每轮用户输入是独立的任务）
	a.stormSig = ""
	a.stormCount = 0

	for {
		a.maybeCompact(ctx)
		msg, err := a.streamChat(ctx, a.session.BuildRequest(a.BuildSystemPrompt()))
		if err != nil {
			a.recordEvent("error", err.Error(), "")
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		if msg.Content != "" {
			a.recordEvent("assistant", msg.Content, "")
		}
		a.session.Add(msg)

		if len(msg.ToolCalls) == 0 {
			a.SaveCheckpoint()
			return msg.Content, nil
		}

		// 执行工具 + 风暴检测
		results, storm := a.dispatchAndDetect(msg.ToolCalls)
		if storm != "" {
			a.recordEvent("loop_guard", storm, "")
			a.noticef("  ⚡ %s\n", storm)
			a.SaveCheckpoint()
			return results[0], fmt.Errorf("%s", storm)
		}
	}
}

// dispatchAndDetect 执行工具调用并做风暴检测：连续 3 次同样的工具以同样的错误失败时，
// 不再回灌原始错误给 LLM，而是注入 [loop guard] 指令让它换方案。
func (a *Agent) dispatchAndDetect(toolCalls []openai.ToolCall) ([]string, string) {
	results := make([]string, len(toolCalls))

	for _, tc := range toolCalls {
		a.recordEvent("tool_call", tc.Function.Arguments, tc.Function.Name)
		a.emitToolCall(tc.Function.Name, tc.Function.Arguments)
	}

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

	for i, tc := range toolCalls {
		if a.canRunParallel(tc) {
			continue
		}
		results[i] = a.executeToolCall(tc)
	}

	wg.Wait()

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

	// 风暴检测
	if storm, hit := a.checkStorm(toolCalls, results); hit {
		subj := toolCalls[0].Function.Name
		results[0] = results[0] + "\n\n" + storm
		return results, fmt.Sprintf("[loop guard] %s has now failed %d times in a row with the same error — change approach or use a different tool", subj, a.stormCount)
	}

	return results, ""
}

const stormThreshold = 3

func (a *Agent) checkStorm(calls []openai.ToolCall, results []string) (string, bool) {
	if len(calls) == 0 {
		a.stormCount = 0
		return "", false
	}
	sig := stormSignature(calls, results)
	if sig == "" {
		a.stormSig, a.stormCount = "", 0
		return "", false
	}
	if sig != a.stormSig {
		a.stormSig, a.stormCount = sig, 1
		return "", false
	}
	a.stormCount++
	if a.stormCount < stormThreshold {
		return "", false
	}
	return fmt.Sprintf(
		"[loop guard] %q has now failed %d times in a row with the same error. Re-sending it — even with the wording changed — will not help: the calls keep failing the same way. Change approach: if an argument is being truncated, write less in one call and split the work into several smaller calls; otherwise fix the arguments, use a different tool, or explain the blocker in your final answer.",
		calls[0].Function.Name, a.stormCount,
	), true
}

func stormSignature(calls []openai.ToolCall, results []string) string {
	var b []string
	for i, tc := range calls {
		r := results[i]
		if !isErrorResult(r) {
			return ""
		}
		b = append(b, fmt.Sprintf("%s:%s", tc.Function.Name, errorPrefix(r)))
	}
	return strings.Join(b, "|")
}

func isErrorResult(r string) bool {
	return strings.Contains(r, "失败") || strings.Contains(r, "❌") ||
		strings.Contains(r, "error") || strings.Contains(r, "Error") ||
		strings.Contains(r, "已拒绝")
}

func errorPrefix(r string) string {
	s := strings.TrimSpace(r)
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

// dispatchToolCalls 保留（测试中直接调用），内部委托给 dispatchAndDetect。
func (a *Agent) dispatchToolCalls(toolCalls []openai.ToolCall) {
	a.dispatchAndDetect(toolCalls)
}

// canRunParallel 判断一个工具调用能否安全并发执行
func (a *Agent) canRunParallel(tc openai.ToolCall) bool {
	if _, ok := a.allTools[tc.Function.Name]; !ok {
		return false
	}
	if !isReadOnlyTool(tc.Function.Name) {
		return false
	}
	gate := a.gate
	if gate == nil {
		gate = ConfirmHighRiskGate{}
	}
	d, _ := gate.Check(tc.Function.Name, tc.Function.Arguments)
	return d == GateAllow
}

// executeToolCall 执行单个工具调用并返回结果文本。
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

	gate := a.gate
	if gate == nil {
		gate = ConfirmHighRiskGate{}
	}
	switch decision, reason := gate.Check(tc.Function.Name, tc.Function.Arguments); decision {
	case GateDeny:
		result := fmt.Sprintf("已拒绝执行 %s：%s。请改用只读手段（如 read_file / grep / code_search）", tc.Function.Name, reason)
		a.noticef("  🚫 %s\n", result)
		return result
	case GateConfirm:
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
			ReasoningEffort: a.reasoningEffort,
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

		if resp.Usage != nil && resp.Usage.PromptTokens > 0 {
			usage = *resp.Usage
		}

		if len(resp.Choices) == 0 {
			continue
		}
		delta := resp.Choices[0].Delta

		if delta.ReasoningContent != "" {
			reasoning += delta.ReasoningContent
			a.emitReasoning(delta.ReasoningContent)
		}

		if delta.Content != "" {
			if reasoning != "" && content == "" {
				a.emitToken("\n")
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
