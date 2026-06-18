package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentdemo/tool"
)

func fakeConnector(name, command string, args ...string) (*MCPClient, error) {
	return &MCPClient{Name: name}, nil
}

func TestManager_InstallConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp_servers.json")
	var registered []string

	mgr := NewManager(configPath, fakeConnector, func(prefix string, tools []tool.Tool) {
		registered = append(registered, prefix)
	})

	// DiscoverTools on fake client returns empty (MCPClient.client is nil)
	// so we override the connector to one that creates a client we can work with.
	// For this test, just validate config persistence and registrar callback.

	// Override with a connector that returns a mock discoverable client
	mgr.connector = func(name, command string, args ...string) (*MCPClient, error) {
		return &MCPClient{Name: name}, nil
	}

	// Since DiscoverTools requires a real MCP client, let's test the config layer
	// by checking persistence and upsert behavior.
	cfg := Config{Name: "test_server", Command: "echo", Args: []string{"hi"}}

	// Write a manual install that skips the actual connect (testing config path only)
	mgr.mu.Lock()
	mgr.configs = append(mgr.configs, cfg)
	err := mgr.persistLocked()
	mgr.mu.Unlock()
	if err != nil {
		t.Fatalf("persist failed: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if len(data) < 10 {
		t.Fatal("config file too short")
	}

	// Reload and verify
	mgr2 := NewManager(configPath, fakeConnector, func(string, []tool.Tool) {})
	configs := mgr2.List()
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Name != "test_server" {
		t.Fatalf("name = %s, want test_server", configs[0].Name)
	}
}

func TestManager_NpmShortcut(t *testing.T) {
	cases := []struct {
		pkg      string
		expected string
	}{
		{"@modelcontextprotocol/server-filesystem", "filesystem"},
		{"@foo/mcp-bar", "bar"},
		{"simple-tool", "simple-tool"},
	}
	for _, tc := range cases {
		got := npmToName(tc.pkg)
		if got != tc.expected {
			t.Errorf("npmToName(%q) = %q, want %q", tc.pkg, got, tc.expected)
		}
	}
}

func TestManager_UpsertOverwrite(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp_servers.json")
	os.WriteFile(configPath, []byte(`[{"name":"existing","command":"cat","args":[]}]`), 0644)

	mgr := NewManager(configPath, fakeConnector, func(string, []tool.Tool) {})

	if len(mgr.List()) != 1 {
		t.Fatalf("expected 1 existing, got %d", len(mgr.List()))
	}

	// Add new without conflict
	mgr.mu.Lock()
	mgr.configs = append(mgr.configs, Config{Name: "new_one", Command: "echo"})
	mgr.persistLocked()
	mgr.mu.Unlock()

	mgr2 := NewManager(configPath, fakeConnector, func(string, []tool.Tool) {})
	if len(mgr2.List()) != 2 {
		t.Fatalf("expected 2 configs after add, got %d", len(mgr2.List()))
	}
}

func TestManager_ValidateConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"missing name", Config{Command: "echo"}},
		{"missing command", Config{Name: "foo"}},
	}
	for _, tc := range cases {
		err := validateConfig(tc.cfg)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestManager_RefreshReconnectsAndRegistersTools(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "mcp_servers.json")
	os.WriteFile(configPath, []byte(`[{"name":"codegraph","command":"npx","args":["-y","codegraph","serve","--mcp"]}]`), 0644)

	connects := 0
	mgr := NewManager(configPath, func(name, command string, args ...string) (*MCPClient, error) {
		connects++
		return &MCPClient{
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
	}, func(prefix string, tools []tool.Tool) {
		if prefix != "codegraph" {
			t.Fatalf("prefix=%q, want codegraph", prefix)
		}
		if len(tools) != 1 || tools[0].Name != "explore" {
			t.Fatalf("registered tools=%+v, want unprefixed explore", tools)
		}
	})

	n, err := mgr.Refresh("codegraph")
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("Refresh discovered %d tools, want 1", n)
	}
	if connects != 1 {
		t.Fatalf("connects=%d, want 1", connects)
	}
}
