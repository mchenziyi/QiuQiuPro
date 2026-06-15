package agent

// ToolHookContext 描述一次工具调用的上下文，供 Hook 做审计、策略判断或结果处理。
type ToolHookContext struct {
	SessionID string
	Name      string
	Arguments string
}

type ToolHookDecision int

const (
	ToolHookAllow ToolHookDecision = iota
	ToolHookDeny
)

// ToolHook 是工具执行前后的扩展点。BeforeToolCall 可拒绝执行；AfterToolCall 可观察或改写结果。
// 默认没有 hook，行为完全不变。
type ToolHook interface {
	BeforeToolCall(ctx ToolHookContext) (ToolHookDecision, string)
	AfterToolCall(ctx ToolHookContext, result string) string
}

func (a *Agent) RegisterToolHook(h ToolHook) {
	if h != nil {
		a.toolHooks = append(a.toolHooks, h)
	}
}

func (a *Agent) toolHookContext(name, args string) ToolHookContext {
	sessionID := ""
	if a.session != nil {
		sessionID = a.session.ID
	}
	return ToolHookContext{SessionID: sessionID, Name: name, Arguments: args}
}

func (a *Agent) beforeToolHooks(ctx ToolHookContext) (bool, string) {
	for _, h := range a.toolHooks {
		if h == nil {
			continue
		}
		decision, reason := h.BeforeToolCall(ctx)
		if decision == ToolHookDeny {
			if reason == "" {
				reason = "tool hook denied"
			}
			return false, reason
		}
	}
	return true, ""
}

func (a *Agent) afterToolHooks(ctx ToolHookContext, result string) string {
	for _, h := range a.toolHooks {
		if h == nil {
			continue
		}
		result = h.AfterToolCall(ctx, result)
	}
	return result
}
