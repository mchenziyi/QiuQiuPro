package skill

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Manager provides thread-safe Skill lifecycle: load, list, find, install.
type Manager struct {
	mu         sync.RWMutex
	skills     []Skill
	builtinDir string
	installDir string          // persistent user dir, e.g. ~/.qiuqiu/skills
	external   map[string]bool // skills backed by installDir
}

// NewManager creates a Manager that loads built-in and external skills.
// builtinDir is project-local (e.g. "prompt/skills"), installDir is user-level (e.g. ~/.qiuqiu/skills).
func NewManager(builtinDir, installDir string) *Manager {
	m := &Manager{builtinDir: builtinDir, installDir: installDir, external: map[string]bool{}}
	m.reload()
	return m
}

func (m *Manager) reload() {
	m.skills = nil
	m.external = map[string]bool{}
	if builtins, err := LoadFromDir(m.builtinDir); err == nil {
		m.skills = append(m.skills, builtins...)
	}
	if externals, err := LoadFromDir(m.installDir); err == nil {
		for _, s := range externals {
			m.upsert(s)
			m.external[s.Name] = true
		}
	}
}

func (m *Manager) List() []Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Skill, len(m.skills))
	copy(out, m.skills)
	return out
}

func (m *Manager) Find(name string) (Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.skills {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

// InstallFromJSON parses raw JSON, validates it, persists to installDir, and registers in memory.
func (m *Manager) InstallFromJSON(raw string, overwrite bool) (*Skill, error) {
	var s Skill
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("JSON 格式无效：%w", err)
	}
	return m.install(s, overwrite)
}

// InstallFromMarkdown parses SKILL.md content with YAML-style front matter.
func (m *Manager) InstallFromMarkdown(raw string, overwrite bool) (*Skill, error) {
	s, err := parseMarkdownSkill([]byte(raw))
	if err != nil {
		return nil, err
	}
	return m.install(s, overwrite)
}

// InstallFromPath loads a Skill file from a local path, auto-detecting JSON or SKILL.md.
func (m *Manager) InstallFromPath(path string, overwrite bool) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 Skill 文件失败：%w", err)
	}
	s, err := parseSkillData(data)
	if err != nil {
		return nil, fmt.Errorf("解析 Skill 文件失败：%w\n路径：%s", err, path)
	}
	return m.install(s, overwrite)
}

// InstallFromURL downloads a Skill from URL, auto-detecting JSON or SKILL.md.
func (m *Manager) InstallFromURL(url string, overwrite bool) (*Skill, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载 Skill 失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载 Skill 失败：HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%w", err)
	}
	s, err := parseSkillData(data)
	if err != nil {
		return nil, err
	}
	return m.install(s, overwrite)
}

// Delete removes an externally installed Skill from disk and memory.
// Built-in Skills under prompt/skills cannot be deleted through this API.
func (m *Manager) Delete(name string) error {
	if !safeNameRe.MatchString(name) {
		return fmt.Errorf("Skill name %q 不合法", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.external[name] {
		return fmt.Errorf("Skill %q 不是外部安装的 Skill，不能删除", name)
	}
	path := filepath.Join(m.installDir, name+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Skill %q 文件不存在，不能删除", name)
		}
		return fmt.Errorf("删除 Skill 文件失败：%w", err)
	}
	m.reload()
	return nil
}

var safeNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

func (m *Manager) install(s Skill, overwrite bool) (*Skill, error) {
	if err := validate(s); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.FindLocked(s.Name); ok {
		if !overwrite {
			return nil, fmt.Errorf("Skill %q 已存在，需指定 overwrite=true 覆盖", s.Name)
		}
	}

	m.upsert(s)
	m.external[s.Name] = true
	return &s, m.persist(s)
}

func (m *Manager) FindLocked(name string) (Skill, bool) {
	for _, s := range m.skills {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

func (m *Manager) upsert(s Skill) {
	for i, existing := range m.skills {
		if existing.Name == s.Name {
			m.skills[i] = s
			return
		}
	}
	m.skills = append(m.skills, s)
}

func validate(s Skill) error {
	if s.Name == "" {
		return fmt.Errorf("Skill 缺少 name 字段")
	}
	if !safeNameRe.MatchString(s.Name) {
		return fmt.Errorf("Skill name %q 不合法（需以字母开头，仅含字母/数字/下划线/连字符，最长64字符）", s.Name)
	}
	if s.Description == "" {
		return fmt.Errorf("Skill %q 缺少 description 字段", s.Name)
	}
	if s.SystemPrompt == "" {
		return fmt.Errorf("Skill %q 缺少 system_prompt 字段", s.Name)
	}
	return nil
}

func (m *Manager) persist(s Skill) error {
	if m.installDir == "" {
		return nil
	}
	if err := os.MkdirAll(m.installDir, 0755); err != nil {
		return fmt.Errorf("创建 Skill 目录失败：%w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Skill 失败：%w", err)
	}
	path := filepath.Join(m.installDir, s.Name+".json")
	return os.WriteFile(path, data, 0644)
}

func parseSkillData(data []byte) (Skill, error) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		var s Skill
		if err := json.Unmarshal([]byte(trimmed), &s); err != nil {
			return Skill{}, fmt.Errorf("JSON 格式无效：%w", err)
		}
		return s, nil
	}
	if strings.HasPrefix(trimmed, "---") {
		return parseMarkdownSkill([]byte(trimmed))
	}
	return Skill{}, fmt.Errorf("无法识别 Skill 格式：需要 JSON 或 SKILL.md front matter")
}

func parseMarkdownSkill(data []byte) (Skill, error) {
	raw := strings.TrimSpace(string(data))
	if !strings.HasPrefix(raw, "---") {
		return Skill{}, fmt.Errorf("Markdown Skill 缺少 front matter")
	}
	rest := strings.TrimPrefix(raw, "---")
	parts := strings.SplitN(rest, "---", 2)
	if len(parts) != 2 {
		return Skill{}, fmt.Errorf("Markdown Skill front matter 未闭合")
	}
	meta := parseFrontMatter(parts[0])
	body := strings.TrimSpace(parts[1])
	return Skill{
		Name:         meta["name"],
		Description:  meta["description"],
		SystemPrompt: body,
	}, nil
}

func parseFrontMatter(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out
}
