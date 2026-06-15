package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agentdemo/tool"
)

const (
	MemoryScopeGlobal  = "global"
	MemoryScopeProject = "project"

	MemoryKindPreference  = "preference"
	MemoryKindProjectRule = "project_rule"

	memoryToolName       = "remember_rule"
	maxMemoryContentLen  = 300
	maxRenderedMemories  = 20
	defaultProjectMemory = ".reasonix/memory.json"
)

// Memory 只保存偏好/规则，不保存项目知识、代码片段或一次性任务上下文。
type Memory struct {
	ID        string `json:"id"`
	Scope     string `json:"scope"`
	Kind      string `json:"kind"`
	Content   string `json:"content"`
	Source    string `json:"source"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	Enabled   bool   `json:"enabled"`
}

type memoryFile struct {
	Memories []Memory `json:"memories"`
}

// MemoryStore 管理全局与项目两层偏好/规则记忆。
type MemoryStore struct {
	GlobalPath  string
	ProjectPath string
}

func NewMemoryStore(globalPath, projectPath string) *MemoryStore {
	return &MemoryStore{GlobalPath: globalPath, ProjectPath: projectPath}
}

func DefaultMemoryStore() *MemoryStore {
	home, _ := os.UserHomeDir()
	return NewMemoryStore(filepath.Join(home, ".qiuqiu", "memory.json"), defaultProjectMemory)
}

func (s *MemoryStore) Add(scope, kind, content, source string) (Memory, error) {
	scope = strings.TrimSpace(scope)
	kind = strings.TrimSpace(kind)
	content = strings.Join(strings.Fields(content), " ")
	source = strings.TrimSpace(source)
	if source == "" {
		source = "model"
	}
	if scope != MemoryScopeGlobal && scope != MemoryScopeProject {
		return Memory{}, fmt.Errorf("scope 只支持 global 或 project")
	}
	if kind != MemoryKindPreference && kind != MemoryKindProjectRule {
		return Memory{}, fmt.Errorf("只支持保存 preference 或 project_rule，不保存知识型长期记忆")
	}
	if content == "" {
		return Memory{}, fmt.Errorf("记忆内容不能为空")
	}
	if len(content) > maxMemoryContentLen {
		return Memory{}, fmt.Errorf("记忆内容过长，只保存简短偏好/规则")
	}

	memories, err := s.load(scope)
	if err != nil {
		return Memory{}, err
	}
	now := time.Now().Unix()
	for i := range memories {
		if memories[i].Kind == kind && memories[i].Content == content {
			memories[i].UpdatedAt = now
			memories[i].Enabled = true
			memories[i].Source = source
			if err := s.save(scope, memories); err != nil {
				return Memory{}, err
			}
			return memories[i], nil
		}
	}
	mem := Memory{
		ID:        fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		Scope:     scope,
		Kind:      kind,
		Content:   content,
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
		Enabled:   true,
	}
	memories = append(memories, mem)
	if err := s.save(scope, memories); err != nil {
		return Memory{}, err
	}
	return mem, nil
}

func (s *MemoryStore) ListEnabled() ([]Memory, error) {
	var all []Memory
	for _, scope := range []string{MemoryScopeGlobal, MemoryScopeProject} {
		memories, err := s.load(scope)
		if err != nil {
			return nil, err
		}
		for _, m := range memories {
			if m.Enabled {
				all = append(all, m)
			}
		}
	}
	sortMemories(all)
	if len(all) > maxRenderedMemories {
		all = all[:maxRenderedMemories]
	}
	return all, nil
}

func (s *MemoryStore) Forget(id string) (bool, error) {
	id = strings.TrimSpace(id)
	for _, scope := range []string{MemoryScopeGlobal, MemoryScopeProject} {
		memories, err := s.load(scope)
		if err != nil {
			return false, err
		}
		changed := false
		found := false
		for i := range memories {
			if memories[i].ID == id {
				memories[i].Enabled = false
				memories[i].UpdatedAt = time.Now().Unix()
				changed, found = true, true
				break
			}
		}
		if changed {
			return found, s.save(scope, memories)
		}
	}
	return false, nil
}

func (s *MemoryStore) RenderPromptBlock() (string, error) {
	memories, err := s.ListEnabled()
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("## 长期记忆（偏好/规则）\n")
	b.WriteString("以下记忆只包含用户偏好与项目规则；不要把它们当作外部知识库。\n")
	writeGroup := func(title string, pred func(Memory) bool) {
		wrote := false
		for _, m := range memories {
			if !pred(m) {
				continue
			}
			if !wrote {
				b.WriteString(title + "：\n")
				wrote = true
			}
			b.WriteString("- " + m.Content + "\n")
		}
	}
	writeGroup("全局偏好", func(m Memory) bool { return m.Scope == MemoryScopeGlobal && m.Kind == MemoryKindPreference })
	writeGroup("全局规则", func(m Memory) bool { return m.Scope == MemoryScopeGlobal && m.Kind == MemoryKindProjectRule })
	writeGroup("项目偏好", func(m Memory) bool { return m.Scope == MemoryScopeProject && m.Kind == MemoryKindPreference })
	writeGroup("项目规则", func(m Memory) bool { return m.Scope == MemoryScopeProject && m.Kind == MemoryKindProjectRule })
	return strings.TrimRight(b.String(), "\n"), nil
}

func (s *MemoryStore) FormatList() (string, error) {
	memories, err := s.ListEnabled()
	if err != nil {
		return "", err
	}
	if len(memories) == 0 {
		return "暂无长期记忆（偏好/规则）", nil
	}
	var b strings.Builder
	b.WriteString("长期记忆（偏好/规则）：\n")
	for _, m := range memories {
		fmt.Fprintf(&b, "- %s [%s/%s] %s\n", m.ID, m.Scope, m.Kind, m.Content)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (s *MemoryStore) load(scope string) ([]Memory, error) {
	path := s.path(scope)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f memoryFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	for i := range f.Memories {
		f.Memories[i].Scope = scope
	}
	return f.Memories, nil
}

func (s *MemoryStore) save(scope string, memories []Memory) error {
	path := s.path(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(memoryFile{Memories: memories}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *MemoryStore) path(scope string) string {
	if scope == MemoryScopeGlobal {
		return s.GlobalPath
	}
	return s.ProjectPath
}

func sortMemories(memories []Memory) {
	scopeRank := map[string]int{MemoryScopeGlobal: 0, MemoryScopeProject: 1}
	kindRank := map[string]int{MemoryKindPreference: 0, MemoryKindProjectRule: 1}
	sort.SliceStable(memories, func(i, j int) bool {
		a, b := memories[i], memories[j]
		if scopeRank[a.Scope] != scopeRank[b.Scope] {
			return scopeRank[a.Scope] < scopeRank[b.Scope]
		}
		if kindRank[a.Kind] != kindRank[b.Kind] {
			return kindRank[a.Kind] < kindRank[b.Kind]
		}
		return a.CreatedAt < b.CreatedAt
	})
}

func (a *Agent) SetMemoryStore(store *MemoryStore) { a.memoryStore = store }

func (a *Agent) BuildSystemPrompt() string {
	base := a.sysPrompt
	if a.memoryStore == nil {
		return base
	}
	block, err := a.memoryStore.RenderPromptBlock()
	if err != nil || block == "" {
		return base
	}
	return strings.TrimRight(base, "\n") + "\n\n" + block
}

func (a *Agent) MemoryList() string {
	if a.memoryStore == nil {
		return "长期记忆未启用"
	}
	out, err := a.memoryStore.FormatList()
	if err != nil {
		return fmt.Sprintf("读取长期记忆失败：%v", err)
	}
	return out
}

func (a *Agent) ForgetMemory(id string) string {
	if a.memoryStore == nil {
		return "长期记忆未启用"
	}
	ok, err := a.memoryStore.Forget(id)
	if err != nil {
		return fmt.Sprintf("删除长期记忆失败：%v", err)
	}
	if !ok {
		return fmt.Sprintf("未找到长期记忆：%s", id)
	}
	return fmt.Sprintf("已删除长期记忆：%s", id)
}

func (a *Agent) NewRememberRuleTool() tool.Tool {
	return tool.Tool{
		Name:        memoryToolName,
		Description: "当且仅当用户表达了长期偏好、默认行为或项目规则时调用，用来保存偏好/规则型长期记忆。不要保存知识型内容、代码片段、临时任务细节、日志、秘密或一次性事实。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scope":   map[string]any{"type": "string", "enum": []string{MemoryScopeGlobal, MemoryScopeProject}, "description": "global=跨项目用户偏好；project=当前项目规则/偏好"},
				"kind":    map[string]any{"type": "string", "enum": []string{MemoryKindPreference, MemoryKindProjectRule}, "description": "preference=用户偏好；project_rule=项目规则"},
				"content": map[string]any{"type": "string", "description": "简短、可长期复用的一条偏好或规则，不超过 300 字符"},
				"reason":  map[string]any{"type": "string", "description": "为什么判断这是一条应保存的长期偏好/规则"},
			},
			"required": []string{"scope", "kind", "content", "reason"},
		},
		Execute: func(args string) string {
			if a.memoryStore == nil {
				return "长期记忆未启用"
			}
			var p struct {
				Scope   string `json:"scope"`
				Kind    string `json:"kind"`
				Content string `json:"content"`
				Reason  string `json:"reason"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return fmt.Sprintf("保存长期记忆失败：参数不是合法 JSON：%v", err)
			}
			mem, err := a.memoryStore.Add(p.Scope, p.Kind, p.Content, "model")
			if err != nil {
				return fmt.Sprintf("保存长期记忆失败：%v", err)
			}
			return fmt.Sprintf("已保存长期记忆：%s [%s/%s] %s", mem.ID, mem.Scope, mem.Kind, mem.Content)
		},
	}
}
