package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"agentdemo/tool"
)

func TestMemoryStore_AddListForget(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")

	mem, err := store.Add(MemoryScopeGlobal, MemoryKindPreference, "以后默认用中文回答", "model")
	if err != nil {
		t.Fatal(err)
	}
	if mem.ID == "" || mem.Scope != MemoryScopeGlobal || mem.Kind != MemoryKindPreference {
		t.Fatalf("记忆字段不完整：%+v", mem)
	}

	all, err := store.ListEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Content != "以后默认用中文回答" {
		t.Fatalf("ListEnabled 错误：%+v", all)
	}

	ok, err := store.Forget(mem.ID)
	if err != nil || !ok {
		t.Fatalf("Forget 失败：ok=%v err=%v", ok, err)
	}
	all, err = store.ListEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("删除后不应还有启用记忆：%+v", all)
	}
}

func TestMemoryStore_RejectsKnowledgeMemory(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")

	if _, err := store.Add(MemoryScopeProject, "knowledge", "Gin 路由写在 routers 目录", "model"); err == nil {
		t.Fatal("偏好/规则记忆不应接受 knowledge 类型")
	}
	if _, err := store.Add(MemoryScopeProject, MemoryKindProjectRule, strings.Repeat("x", maxMemoryContentLen+1), "model"); err == nil {
		t.Fatal("过长记忆应被拒绝，避免保存大段知识/代码")
	}
}

func TestMemoryStore_RenderBlockStable(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	if _, err := store.Add(MemoryScopeProject, MemoryKindProjectRule, "本项目只支持 DeepSeek", "model"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(MemoryScopeGlobal, MemoryKindPreference, "提交信息默认中文", "model"); err != nil {
		t.Fatal(err)
	}

	block, err := store.RenderPromptBlock()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## 长期记忆（偏好/规则）", "全局偏好", "提交信息默认中文", "项目规则", "本项目只支持 DeepSeek"} {
		if !strings.Contains(block, want) {
			t.Fatalf("记忆块缺少 %q：\n%s", want, block)
		}
	}
	if strings.Index(block, "全局偏好：") > strings.Index(block, "项目规则：") {
		t.Fatalf("渲染顺序应稳定：全局偏好在项目前，实际\n%s", block)
	}
}

func TestAgent_BuildSystemPromptIncludesMemory(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	if _, err := store.Add(MemoryScopeGlobal, MemoryKindPreference, "回答保持简洁", "model"); err != nil {
		t.Fatal(err)
	}
	a := newDispatchAgent(t, AllowAllGate{})
	a.sysPrompt = "BASE"
	a.SetMemoryStore(store)

	got := a.BuildSystemPrompt()
	if !strings.Contains(got, "BASE") || !strings.Contains(got, "回答保持简洁") {
		t.Fatalf("system prompt 应包含基础提示词与长期记忆：\n%s", got)
	}
}

func TestRememberRuleTool_ModelWritesPreference(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetMemoryStore(store)
	a.RegisterTool(a.NewRememberRuleTool())

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{
		Name:      memoryToolName,
		Arguments: `{"scope":"global","kind":"preference","content":"以后默认用中文回答","reason":"用户表达了长期偏好"}`,
	}})
	if !strings.Contains(got, "已保存长期记忆") {
		t.Fatalf("工具应保存偏好记忆，实际 %q", got)
	}
	all, err := store.ListEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Source != "model" || all[0].Content != "以后默认用中文回答" {
		t.Fatalf("模型工具写入错误：%+v", all)
	}
}

func TestRememberRuleTool_RejectsKnowledge(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	a := newDispatchAgent(t, AllowAllGate{})
	a.SetMemoryStore(store)
	a.RegisterTool(a.NewRememberRuleTool())

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{
		Name:      memoryToolName,
		Arguments: `{"scope":"project","kind":"knowledge","content":"某函数在 run.go","reason":"项目知识"}`,
	}})
	if !strings.Contains(got, "只支持保存 preference 或 project_rule") {
		t.Fatalf("知识型记忆应被拒绝，实际 %q", got)
	}
}

func TestRememberRuleTool_DeniedInReadOnlyMode(t *testing.T) {
	store := NewMemoryStore(t.TempDir()+"/global.json", t.TempDir()+"/project.json")
	a := newDispatchAgent(t, ReadOnlyGate{})
	a.SetMemoryStore(store)
	a.RegisterTool(a.NewRememberRuleTool())

	got := a.executeToolCall(context.Background(), openai.ToolCall{Function: openai.FunctionCall{
		Name:      memoryToolName,
		Arguments: `{"scope":"global","kind":"preference","content":"以后默认用中文回答","reason":"用户表达了长期偏好"}`,
	}})
	if !strings.Contains(got, "只读模式禁止") {
		t.Fatalf("只读模式应拒绝写记忆，实际 %q", got)
	}
	all, err := store.ListEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("只读模式不应写入记忆：%+v", all)
	}
}

func TestMemoryTool_AvailableWithSkillWhitelist(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	a.allTools["read_file"] = tool.Tool{Name: "read_file"}
	a.allTools[memoryToolName] = a.NewRememberRuleTool()
	a.activeTools = []string{"read_file"}

	var names []string
	for _, t := range a.availableTools() {
		names = append(names, t.Name)
	}
	if strings.Join(names, ",") != "read_file,"+memoryToolName {
		t.Fatalf("Skill 白名单下仍应追加 remember_rule，实际 %+v", names)
	}
}

// assertValidToolPairing 校验消息序列对「工具调用 / 工具结果」是配对合法的：
// 每条 tool 结果之前，都必须出现过携带同一 ID tool_call 的 assistant 消息。
// 这正是 DeepSeek/OpenAI 接口的硬性要求——裁剪历史时一旦把二者拆开就会 400。
func assertValidToolPairing(t *testing.T, msgs []openai.ChatCompletionMessage) {
	t.Helper()
	seen := map[string]bool{}
	for i, m := range msgs {
		switch m.Role {
		case "assistant":
			for _, tc := range m.ToolCalls {
				seen[tc.ID] = true
			}
		case "tool":
			if m.ToolCallID == "" || !seen[m.ToolCallID] {
				t.Fatalf("msg[%d] 是孤立的 tool 结果（id=%q），缺少对应的 tool_call", i, m.ToolCallID)
			}
		}
	}
}

// 未超过上限时不应裁剪。
func TestSessionTrim_NoopUnderLimit(t *testing.T) {
	s := NewSession("test")
	for i := 0; i < 10; i++ {
		s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
		s.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "yo"})
	}
	before := s.Len()
	s.Trim()
	if s.Len() != before {
		t.Fatalf("未超过上限不应裁剪：before=%d after=%d", before, s.Len())
	}
}

// 超过上限裁剪后，工具调用/结果的配对必须保持完整，且窗口不能以孤立 tool 开头。
func TestSessionTrim_PreservesToolPairing(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "start"})
	// 1 条 user + 100 组 [assistant(tool_call) + tool] = 201 条，必然超过 maxMessages=100。
	for k := 0; k < 100; k++ {
		id := fmt.Sprintf("t%d", k)
		s.Add(openai.ChatCompletionMessage{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{{
				ID: id, Type: "function",
				Function: openai.FunctionCall{Name: "read_file", Arguments: "{}"},
			}},
		})
		s.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: id, Name: "read_file", Content: "data"})
	}
	if s.Len() <= maxMessages {
		t.Fatalf("测试前置不满足：messages=%d 应 > maxMessages=%d", s.Len(), maxMessages)
	}

	s.Trim()

	if s.Len() > maxMessages {
		t.Fatalf("裁剪后仍超过上限：%d > %d", s.Len(), maxMessages)
	}
	msgs := s.Messages()
	if len(msgs) > 0 && msgs[0].Role == "tool" {
		t.Fatalf("窗口不应以孤立的 tool 消息开头")
	}
	assertValidToolPairing(t, msgs)
}

// 有 system 提示词时，请求应把 system 前置，且不修改历史本身。
func TestSessionBuildRequest_PrependsSystem(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
	s.Add(openai.ChatCompletionMessage{Role: "assistant", Content: "yo"})

	req := s.BuildRequest("SYS")
	if len(req) != s.Len()+1 {
		t.Fatalf("应在最前面加 system：len=%d", len(req))
	}
	if req[0].Role != "system" || req[0].Content != "SYS" {
		t.Fatalf("第一条应为 system，实际 %+v", req[0])
	}
	if s.Len() != 2 || s.Messages()[0].Role != "user" {
		t.Fatalf("BuildRequest 不应修改历史本身")
	}
}

// 无 system 提示词时，不应前置 system 消息。
func TestSessionBuildRequest_NoSystemWhenEmpty(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "hi"})
	req := s.BuildRequest("")
	if len(req) != 1 || req[0].Role != "user" {
		t.Fatalf("空 system 不应前置 system 消息：%+v", req)
	}
}

// Snapshot 序列化、Restore 反序列化应能完整往返历史。
func TestSessionSnapshotRestore_RoundTrip(t *testing.T) {
	src := NewSession("src")
	src.Add(openai.ChatCompletionMessage{Role: "user", Content: "问题"})
	src.Add(openai.ChatCompletionMessage{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID: "call_1", Type: "function",
			Function: openai.FunctionCall{Name: "read_file", Arguments: `{"path":"a.go"}`},
		}},
	})
	src.Add(openai.ChatCompletionMessage{Role: "tool", ToolCallID: "call_1", Name: "read_file", Content: "内容"})

	data, err := src.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot 失败：%v", err)
	}

	dst := NewSession("dst")
	if err := dst.Restore(data); err != nil {
		t.Fatalf("Restore 失败：%v", err)
	}
	if dst.Len() != src.Len() {
		t.Fatalf("恢复后条数不一致：src=%d dst=%d", src.Len(), dst.Len())
	}
	got := dst.Messages()
	if got[0].Content != "问题" || got[2].ToolCallID != "call_1" {
		t.Fatalf("恢复内容不一致：%+v", got)
	}
	assertValidToolPairing(t, got)
}

// Restore 遇到非法 JSON 应返回错误，且不破坏既有历史。
func TestSessionRestore_BadJSON(t *testing.T) {
	s := NewSession("test")
	s.Add(openai.ChatCompletionMessage{Role: "user", Content: "保留我"})
	if err := s.Restore("{not json"); err == nil {
		t.Fatalf("非法 JSON 应返回错误")
	}
	if s.Len() != 1 || s.Messages()[0].Content != "保留我" {
		t.Fatalf("Restore 失败时不应破坏既有历史")
	}
}
