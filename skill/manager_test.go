package skill

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestManager_ListAndFind(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{
		"name": "test_skill",
		"description": "A test skill",
		"system_prompt": "test prompt"
	}`), 0644)

	mgr := NewManager("", dir)
	skills := mgr.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "test_skill" {
		t.Fatalf("expected test_skill, got %s", skills[0].Name)
	}

	s, ok := mgr.Find("test_skill")
	if !ok {
		t.Fatal("Find should return true for existing skill")
	}
	if s.Description != "A test skill" {
		t.Fatalf("wrong description: %s", s.Description)
	}

	_, ok = mgr.Find("nonexistent")
	if ok {
		t.Fatal("Find should return false for missing skill")
	}
}

func TestManager_InstallFromJSON(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "skills")

	mgr := NewManager("", installDir)
	raw := `{"name": "new_skill", "description": "New one", "system_prompt": "hello"}`
	s, err := mgr.InstallFromJSON(raw, false)
	if err != nil {
		t.Fatalf("InstallFromJSON failed: %v", err)
	}
	if s.Name != "new_skill" {
		t.Fatalf("name = %s, want new_skill", s.Name)
	}

	// Verify persisted to disk
	path := filepath.Join(installDir, "new_skill.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("skill file not persisted: %v", err)
	}

	// Verify in-memory
	_, ok := mgr.Find("new_skill")
	if !ok {
		t.Fatal("newly installed skill not found in manager")
	}
}

func TestManager_InstallDuplicateRejects(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "skills")
	mgr := NewManager("", installDir)

	raw := `{"name": "dup", "description": "First", "system_prompt": "p"}`
	_, err := mgr.InstallFromJSON(raw, false)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	_, err = mgr.InstallFromJSON(raw, false)
	if err == nil {
		t.Fatal("duplicate install without overwrite should fail")
	}

	raw2 := `{"name": "dup", "description": "Second", "system_prompt": "p2"}`
	s, err := mgr.InstallFromJSON(raw2, true)
	if err != nil {
		t.Fatalf("overwrite install failed: %v", err)
	}
	if s.Description != "Second" {
		t.Fatalf("overwrite didn't update: %s", s.Description)
	}
}

func TestManager_InstallFromPath(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "install")
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "ext.json")
	os.WriteFile(sourcePath, []byte(`{
		"name": "from_path",
		"description": "Loaded from path",
		"system_prompt": "sp"
	}`), 0644)

	mgr := NewManager("", installDir)
	s, err := mgr.InstallFromPath(sourcePath, false)
	if err != nil {
		t.Fatalf("InstallFromPath failed: %v", err)
	}
	if s.Name != "from_path" {
		t.Fatalf("name = %s, want from_path", s.Name)
	}

	// Verify copy exists in installDir
	if _, err := os.Stat(filepath.Join(installDir, "from_path.json")); err != nil {
		t.Fatalf("not persisted: %v", err)
	}
}

func TestManager_InstallFromMarkdownPath(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "install")
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "SKILL.md")
	os.WriteFile(sourcePath, []byte(`---
name: markdown_skill
description: Markdown Skill from front matter
---

# Markdown Skill

Always answer with MARKDOWN_OK.
`), 0644)

	mgr := NewManager("", installDir)
	s, err := mgr.InstallFromPath(sourcePath, false)
	if err != nil {
		t.Fatalf("InstallFromPath markdown failed: %v", err)
	}
	if s.Name != "markdown_skill" {
		t.Fatalf("name = %s, want markdown_skill", s.Name)
	}
	if s.SystemPrompt != "# Markdown Skill\n\nAlways answer with MARKDOWN_OK." {
		t.Fatalf("unexpected system prompt: %q", s.SystemPrompt)
	}
	if _, err := os.Stat(filepath.Join(installDir, "markdown_skill.json")); err != nil {
		t.Fatalf("markdown skill not persisted as json: %v", err)
	}
}

func TestManager_InstallFromURL(t *testing.T) {
	payload := `{"name": "remote_skill", "description": "From URL", "system_prompt": "rp"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	installDir := filepath.Join(t.TempDir(), "install")
	mgr := NewManager("", installDir)

	s, err := mgr.InstallFromURL(srv.URL+"/skill.json", false)
	if err != nil {
		t.Fatalf("InstallFromURL failed: %v", err)
	}
	if s.Name != "remote_skill" {
		t.Fatalf("name = %s, want remote_skill", s.Name)
	}
}

func TestManager_InstallFromMarkdownURL(t *testing.T) {
	payload := `---
name: remote_markdown
description: Remote markdown skill
---

Use remote markdown instructions.
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	installDir := filepath.Join(t.TempDir(), "install")
	mgr := NewManager("", installDir)

	s, err := mgr.InstallFromURL(srv.URL+"/SKILL.md", false)
	if err != nil {
		t.Fatalf("InstallFromURL markdown failed: %v", err)
	}
	if s.Name != "remote_markdown" {
		t.Fatalf("name = %s, want remote_markdown", s.Name)
	}
}

func TestManager_DeleteExternalSkill(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "install")
	mgr := NewManager("", installDir)
	if _, err := mgr.InstallFromJSON(`{"name":"delete_me","description":"Delete me","system_prompt":"bye"}`, false); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if err := mgr.Delete("delete_me"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, ok := mgr.Find("delete_me"); ok {
		t.Fatal("deleted skill should be removed from memory")
	}
	if _, err := os.Stat(filepath.Join(installDir, "delete_me.json")); !os.IsNotExist(err) {
		t.Fatalf("skill file should be deleted, stat err=%v", err)
	}
}

func TestManager_DeleteRefusesBuiltinOnlySkill(t *testing.T) {
	builtinDir := t.TempDir()
	os.WriteFile(filepath.Join(builtinDir, "builtin.json"), []byte(`{
		"name": "builtin_only",
		"description": "Built in",
		"system_prompt": "builtin"
	}`), 0644)
	installDir := filepath.Join(t.TempDir(), "install")
	mgr := NewManager(builtinDir, installDir)

	err := mgr.Delete("builtin_only")
	if err == nil {
		t.Fatal("Delete should refuse builtin-only skill")
	}
	if _, ok := mgr.Find("builtin_only"); !ok {
		t.Fatal("builtin-only skill should remain in memory")
	}
}

func TestManager_ValidationRejects(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "install")
	mgr := NewManager("", installDir)

	cases := []struct {
		name string
		raw  string
	}{
		{"missing name", `{"description": "d", "system_prompt": "p"}`},
		{"missing description", `{"name": "x", "system_prompt": "p"}`},
		{"missing system_prompt", `{"name": "x", "description": "d"}`},
		{"invalid name chars", `{"name": "has space", "description": "d", "system_prompt": "p"}`},
		{"name starts with number", `{"name": "123abc", "description": "d", "system_prompt": "p"}`},
	}
	for _, tc := range cases {
		_, err := mgr.InstallFromJSON(tc.raw, false)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}
