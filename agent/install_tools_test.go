package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
)

func TestInstallSkillTool_Integration(t *testing.T) {
	home := t.TempDir()
	installDir := filepath.Join(home, "skills")

	mgr := skill.NewManager("", installDir)
	a := newDispatchAgent(t, AllowAllGate{})

	installTool := a.NewInstallSkillTool(mgr)
	a.RegisterTool(installTool)

	// Verify tool is in available tools
	found := false
	for _, tl := range a.availableTools() {
		if tl.Name == "install_skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("install_skill not in available tools")
	}

	// Execute install from JSON
	args := json.RawMessage(`{
		"source_type": "json",
		"source": "{\"name\": \"test_install\", \"description\": \"Test installed\", \"system_prompt\": \"hello\"}",
		"overwrite": false
	}`)
	result, err := installTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("install_skill failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Verify it's now in the manager
	s, ok := mgr.Find("test_install")
	if !ok {
		t.Fatal("installed skill not found in manager")
	}
	if s.Description != "Test installed" {
		t.Fatalf("wrong description: %s", s.Description)
	}

	// Verify file persisted
	if _, err := os.Stat(filepath.Join(installDir, "test_install.json")); err != nil {
		t.Fatalf("skill not persisted to disk: %v", err)
	}
}

func TestInstallSkillTool_InstallsMarkdown(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "skills")
	mgr := skill.NewManager("", installDir)
	a := newDispatchAgent(t, AllowAllGate{})
	installTool := a.NewInstallSkillTool(mgr)

	args := json.RawMessage(`{
		"source_type": "markdown",
		"source": "---\nname: md_tool\ndescription: Markdown tool skill\n---\n\nAlways say MD_TOOL_OK."
	}`)
	result, err := installTool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("install_skill markdown failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if _, ok := mgr.Find("md_tool"); !ok {
		t.Fatal("markdown skill not installed")
	}
}

func TestDeleteSkillTool_Integration(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "skills")
	mgr := skill.NewManager("", installDir)
	if _, err := mgr.InstallFromJSON(`{"name":"delete_tool","description":"Delete tool","system_prompt":"bye"}`, false); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	a := newDispatchAgent(t, AllowAllGate{})
	a.ApplySkill(skill.Skill{Name: "delete_tool", Description: "Delete tool", SystemPrompt: "bye"})

	deleteTool := a.NewDeleteSkillTool(mgr)
	a.RegisterTool(deleteTool)

	found := false
	for _, tl := range a.availableTools() {
		if tl.Name == "delete_skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("delete_skill not in available tools")
	}

	result, err := deleteTool.Execute(context.Background(), json.RawMessage(`{"name":"delete_tool"}`))
	if err != nil {
		t.Fatalf("delete_skill failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if _, ok := mgr.Find("delete_tool"); ok {
		t.Fatal("deleted skill should be removed from manager")
	}
	if got := a.CurrentSkillName(); got != "default" {
		t.Fatalf("deleting active skill should clear current skill, got %s", got)
	}
}

func TestInstallMCPTool_Integration(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "mcp_servers.json")

	var registeredPrefix string
	var registeredTools []tool.Tool

	fakeConn := func(name, command string, args ...string) (*mcp.MCPClient, error) {
		return &mcp.MCPClient{Name: name}, nil
	}
	registrar := func(prefix string, tools []tool.Tool) {
		registeredPrefix = prefix
		registeredTools = tools
	}

	mcpMgr := mcp.NewManager(configPath, fakeConn, registrar)
	a := newDispatchAgent(t, AllowAllGate{})

	installTool := a.NewInstallMCPTool(mcpMgr)
	a.RegisterTool(installTool)

	// Execute with config type - will call fakeConn + DiscoverTools on nil client
	// DiscoverTools on a nil inner client will panic, so test config validation only
	args := json.RawMessage(`{
		"install_type": "config",
		"name": "",
		"command": "echo"
	}`)
	_, err := installTool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	// Test npm with missing package
	args = json.RawMessage(`{"install_type": "npm", "npm_package": ""}`)
	_, err = installTool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for empty npm_package")
	}

	// Since we can't run a real MCP server in tests, verify the tool registered
	found := false
	for _, tl := range a.availableTools() {
		if tl.Name == "install_mcp" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("install_mcp not in available tools")
	}

	_ = registeredPrefix
	_ = registeredTools
}

func TestRefreshMCPTool_RegistersRefreshedTools(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mcp_servers.json")
	if err := os.WriteFile(configPath, []byte(`[{"name":"codegraph","command":"npx","args":["-y","codegraph","serve","--mcp"]}]`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	a := newDispatchAgent(t, AllowAllGate{})
	mcpMgr := mcp.NewManager(configPath, func(name, command string, args ...string) (*mcp.MCPClient, error) {
		return &mcp.MCPClient{
			Name: name,
			DiscoverToolsFunc: func() ([]tool.Tool, error) {
				return []tool.Tool{{
					Name:        "explore",
					Description: "explore code",
					ReadOnly:    true,
					Execute: func(context.Context, json.RawMessage) (string, error) {
						return "ok", nil
					},
				}}, nil
			},
		}, nil
	}, a.RegisterMCPTools)

	refreshTool := a.NewRefreshMCPTool(mcpMgr)
	result, err := refreshTool.Execute(context.Background(), json.RawMessage(`{"name":"codegraph"}`))
	if err != nil {
		t.Fatalf("refresh_mcp failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if _, ok := a.allTools["codegraph_explore"]; !ok {
		t.Fatalf("expected codegraph_explore to be registered, tools=%v", a.allTools)
	}
	if _, ok := a.allTools["codegraph_codegraph_explore"]; ok {
		t.Fatal("MCP tool should not be double-prefixed")
	}
}

func TestRefreshMCPTool_DoesNotDoublePrefixAlreadyPrefixedTools(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mcp_servers.json")
	if err := os.WriteFile(configPath, []byte(`[{"name":"codegraph","command":"npx","args":["-y","codegraph","serve","--mcp"]}]`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	a := newDispatchAgent(t, AllowAllGate{})
	mcpMgr := mcp.NewManager(configPath, func(name, command string, args ...string) (*mcp.MCPClient, error) {
		return &mcp.MCPClient{
			Name: name,
			DiscoverToolsFunc: func() ([]tool.Tool, error) {
				return []tool.Tool{{Name: "codegraph_explore", Description: "explore code", ReadOnly: true}}, nil
			},
		}, nil
	}, a.RegisterMCPTools)

	refreshTool := a.NewRefreshMCPTool(mcpMgr)
	if _, err := refreshTool.Execute(context.Background(), json.RawMessage(`{"name":"codegraph"}`)); err != nil {
		t.Fatalf("refresh_mcp failed: %v", err)
	}
	if _, ok := a.allTools["codegraph_explore"]; !ok {
		t.Fatalf("expected codegraph_explore to be registered, tools=%v", a.allTools)
	}
	if _, ok := a.allTools["codegraph_codegraph_explore"]; ok {
		t.Fatal("already-prefixed MCP tool should not be prefixed again")
	}
}
