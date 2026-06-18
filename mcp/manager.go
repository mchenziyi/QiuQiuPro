package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"agentdemo/tool"
)

// Config describes a single MCP server entry in ~/.qiuqiu/mcp_servers.json.
type Config struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// Connector abstracts MCP server connection for testability.
type Connector func(name, command string, args ...string) (*MCPClient, error)

// ToolRegistrar receives tools discovered from a newly connected MCP server.
type ToolRegistrar func(prefix string, tools []tool.Tool)

// Manager provides hot install/list/connect for MCP servers.
type Manager struct {
	mu         sync.RWMutex
	configs    []Config
	configPath string
	connector  Connector
	registrar  ToolRegistrar
}

// NewManager creates an MCP Manager.
// configPath is the persistent JSON file (e.g. ~/.qiuqiu/mcp_servers.json).
// connector is the connection function (typically mcp.Connect).
// registrar is called to register discovered tools into the agent.
func NewManager(configPath string, connector Connector, registrar ToolRegistrar) *Manager {
	m := &Manager{
		configPath: configPath,
		connector:  connector,
		registrar:  registrar,
	}
	m.configs = m.loadConfigs()
	return m
}

func (m *Manager) loadConfigs() []Config {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil
	}
	var configs []Config
	json.Unmarshal(data, &configs)
	return configs
}

func (m *Manager) List() []Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Config, len(m.configs))
	copy(out, m.configs)
	return out
}

// Install adds or updates an MCP server config, persists it, connects immediately,
// discovers tools, and registers them. Returns the number of tools discovered.
func (m *Manager) Install(cfg Config, overwrite bool) (int, error) {
	if err := validateConfig(cfg); err != nil {
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, existing := range m.configs {
		if existing.Name == cfg.Name {
			if !overwrite {
				return 0, fmt.Errorf("MCP Server %q 已存在，需指定 overwrite=true 覆盖", cfg.Name)
			}
			idx = i
			break
		}
	}

	if idx >= 0 {
		m.configs[idx] = cfg
	} else {
		m.configs = append(m.configs, cfg)
	}

	if err := m.persistLocked(); err != nil {
		return 0, fmt.Errorf("保存 MCP 配置失败：%w", err)
	}

	return m.connectDiscoverRegisterLocked(cfg)
}

// InstallFromNpm expands an npm package name into a standard npx config and installs it.
// Example: "@modelcontextprotocol/server-filesystem" with workDir "." becomes:
//
//	name=server-filesystem, command=npx, args=[-y, @modelcontextprotocol/server-filesystem, .]
func (m *Manager) InstallFromNpm(pkg string, extraArgs []string, overwrite bool) (int, error) {
	name := npmToName(pkg)
	args := []string{"-y", pkg}
	args = append(args, extraArgs...)
	return m.Install(Config{Name: name, Command: "npx", Args: args}, overwrite)
}

// Refresh reconnects a persisted MCP server and registers its currently discovered tools.
// Use this after project-level initialization changes what a server exposes.
func (m *Manager) Refresh(name string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cfg := range m.configs {
		if cfg.Name == name {
			return m.connectDiscoverRegisterLocked(cfg)
		}
	}
	return 0, fmt.Errorf("MCP Server %q 不存在", name)
}

func (m *Manager) connectDiscoverRegisterLocked(cfg Config) (int, error) {
	mc, err := m.connector(cfg.Name, cfg.Command, cfg.Args...)
	if err != nil {
		return 0, fmt.Errorf("连接 MCP Server 失败：%w", err)
	}
	tools, err := mc.DiscoverTools()
	if err != nil {
		return 0, fmt.Errorf("发现工具失败：%w", err)
	}
	m.registrar(mc.Name, tools)
	return len(tools), nil
}

func npmToName(pkg string) string {
	parts := strings.Split(pkg, "/")
	name := parts[len(parts)-1]
	name = strings.TrimPrefix(name, "server-")
	name = strings.TrimPrefix(name, "mcp-")
	if name == "" {
		name = pkg
	}
	return strings.ReplaceAll(name, "@", "")
}

func validateConfig(cfg Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("MCP Server 缺少 name 字段")
	}
	if cfg.Command == "" {
		return fmt.Errorf("MCP Server %q 缺少 command 字段", cfg.Name)
	}
	return nil
}

func (m *Manager) persistLocked() error {
	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return err
	}
	dir := m.configPath[:strings.LastIndex(m.configPath, "/")]
	os.MkdirAll(dir, 0755)
	return os.WriteFile(m.configPath, data, 0644)
}
