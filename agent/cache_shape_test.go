package agent

import (
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

func TestCompareShape_DetectsSystemChange(t *testing.T) {
	tools := []openai.Tool{{Type: "function", Function: &openai.FunctionDefinition{Name: "read_file"}}}
	before := CaptureShape("SYS-A", tools, 0)
	after := CaptureShape("SYS-B", tools, 0)
	d := CompareShape(before, after, openai.Usage{PromptTokens: 100, PromptTokensDetails: &openai.PromptTokensDetails{CachedTokens: 20}})
	if !d.PrefixChanged {
		t.Fatal("system 变化应标记 PrefixChanged")
	}
	if len(d.PrefixChangeReasons) != 1 || d.PrefixChangeReasons[0] != "system" {
		t.Fatalf("原因应为 system，实际 %v", d.PrefixChangeReasons)
	}
}

func TestCompareShape_DetectsLogRewrite(t *testing.T) {
	tools := []openai.Tool{{Type: "function", Function: &openai.FunctionDefinition{Name: "grep"}}}
	before := CaptureShape("SYS", tools, 1)
	after := CaptureShape("SYS", tools, 2)
	d := CompareShape(before, after, openai.Usage{PromptTokens: 50})
	if !strings.Contains(strings.Join(d.PrefixChangeReasons, ","), "log_rewrite") {
		t.Fatalf("应检测到 log_rewrite，实际 %v", d.PrefixChangeReasons)
	}
}

func TestToolDefinitions_StableOrder(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.RegisterTools([]tool.Tool{
		{Name: "grep"},
		{Name: "read_file"},
		{Name: "bash"},
	})
	defs := a.toolDefinitions()
	if len(defs) < 3 {
		t.Fatal("应有至少 3 个工具")
	}
	for i := 1; i < len(defs); i++ {
		if defs[i].Function.Name < defs[i-1].Function.Name {
			t.Fatalf("工具定义应按名称排序：%s 在 %s 前", defs[i].Function.Name, defs[i-1].Function.Name)
		}
	}
}
