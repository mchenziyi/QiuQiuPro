package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
)

// NewInstallSkillTool creates the install_skill tool that the model calls
// when the user asks to install a Skill from JSON, Markdown, local path, or URL.
func (a *Agent) NewInstallSkillTool(mgr *skill.Manager) tool.Tool {
	return tool.Tool{
		Name: "install_skill",
		Description: `Install a new Skill persona so it becomes available via /use <name>. 
Use when user says "安装 skill"/"add skill" or provides a Skill JSON/SKILL.md/path/URL.
The Skill is validated, persisted to ~/.qiuqiu/skills/, and immediately usable.

source_type: "json" | "markdown" | "path" | "url"
source: the JSON string / Markdown SKILL.md content / file path / URL
overwrite: set true to replace an existing Skill with the same name`,
		ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source_type": map[string]any{
					"type":        "string",
					"enum":        []string{"json", "markdown", "path", "url"},
					"description": "来源类型：json（直接JSON）、markdown（SKILL.md 内容）、path（本地路径，自动识别 JSON/Markdown）、url（远程URL，自动识别 JSON/Markdown）",
				},
				"source": map[string]any{
					"type":        "string",
					"description": "Skill 内容来源：JSON 字符串、Markdown 内容、文件路径、或 URL",
				},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "是否覆盖同名 Skill（默认 false）",
				},
			},
			"required": []string{"source_type", "source"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				SourceType string `json:"source_type"`
				Source     string `json:"source"`
				Overwrite  bool   `json:"overwrite"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}

			var s *skill.Skill
			var err error
			switch p.SourceType {
			case "json":
				s, err = mgr.InstallFromJSON(p.Source, p.Overwrite)
			case "markdown":
				s, err = mgr.InstallFromMarkdown(p.Source, p.Overwrite)
			case "path":
				s, err = mgr.InstallFromPath(p.Source, p.Overwrite)
			case "url":
				s, err = mgr.InstallFromURL(p.Source, p.Overwrite)
			default:
				return "", fmt.Errorf("source_type 必须为 json/markdown/path/url，当前：%s", p.SourceType)
			}
			if err != nil {
				return "", err
			}

			a.noticef("  ✅ Skill [%s] 安装成功：%s\n", s.Name, s.Description)
			return fmt.Sprintf("Skill \"%s\" 已安装成功，可通过 /use %s 使用", s.Name, s.Name), nil
		},
	}
}

// NewDeleteSkillTool creates the delete_skill tool that removes externally installed Skills.
func (a *Agent) NewDeleteSkillTool(mgr *skill.Manager) tool.Tool {
	return tool.Tool{
		Name: "delete_skill",
		Description: `Delete an externally installed Skill from ~/.qiuqiu/skills and remove it from the current process.
Use only when user explicitly asks to delete/uninstall/remove a Skill.
Built-in Skills cannot be deleted. If deleting the active Skill, the Agent switches back to default.`,
		ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "要删除的 Skill 名称",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Name == "" {
				return "", fmt.Errorf("name 不能为空")
			}
			wasActive := a.CurrentSkillName() == p.Name
			if err := mgr.Delete(p.Name); err != nil {
				return "", err
			}
			if wasActive {
				a.ClearSkill()
			}
			a.noticef("  ✅ Skill [%s] 已删除\n", p.Name)
			return fmt.Sprintf("Skill %q 已删除", p.Name), nil
		},
	}
}

// NewInstallMCPTool creates the install_mcp tool that the model calls
// when the user asks to install an MCP server.
func (a *Agent) NewInstallMCPTool(mgr *mcp.Manager) tool.Tool {
	return tool.Tool{
		Name: "install_mcp",
		Description: `Install and connect a new MCP server so its tools become available immediately.
Use when user says "安装 MCP"/"add MCP server" or provides MCP config.

install_type: "config" | "npm"
For config: provide name, command, args fields.
For npm: provide npm_package (e.g. "@modelcontextprotocol/server-filesystem") and optional extra_args.
overwrite: set true to replace an existing MCP server with same name.`,
		ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"install_type": map[string]any{
					"type":        "string",
					"enum":        []string{"config", "npm"},
					"description": "安装方式：config（完整配置）或 npm（npm包快捷安装）",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "MCP Server 名称（install_type=config 时必填）",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "启动命令（install_type=config 时必填）",
				},
				"args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "启动参数列表（install_type=config 时）",
				},
				"npm_package": map[string]any{
					"type":        "string",
					"description": "npm 包名（install_type=npm 时必填）",
				},
				"extra_args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "npm 安装后附加的命令行参数（如工作目录 '.'）",
				},
				"overwrite": map[string]any{
					"type":        "boolean",
					"description": "是否覆盖同名 MCP Server（默认 false）",
				},
			},
			"required": []string{"install_type"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				InstallType string   `json:"install_type"`
				Name        string   `json:"name"`
				Command     string   `json:"command"`
				Args        []string `json:"args"`
				NpmPackage  string   `json:"npm_package"`
				ExtraArgs   []string `json:"extra_args"`
				Overwrite   bool     `json:"overwrite"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}

			var toolCount int
			var err error
			var serverName string

			switch p.InstallType {
			case "config":
				if p.Name == "" || p.Command == "" {
					return "", fmt.Errorf("install_type=config 时 name 和 command 必填")
				}
				serverName = p.Name
				toolCount, err = mgr.Install(mcp.Config{
					Name:    p.Name,
					Command: p.Command,
					Args:    p.Args,
				}, p.Overwrite)
			case "npm":
				if p.NpmPackage == "" {
					return "", fmt.Errorf("install_type=npm 时 npm_package 必填")
				}
				serverName = p.NpmPackage
				toolCount, err = mgr.InstallFromNpm(p.NpmPackage, p.ExtraArgs, p.Overwrite)
			default:
				return "", fmt.Errorf("install_type 必须为 config 或 npm，当前：%s", p.InstallType)
			}

			if err != nil {
				return "", err
			}

			a.noticef("  ✅ MCP Server [%s] 安装成功，发现 %d 个工具\n", serverName, toolCount)
			return fmt.Sprintf("MCP Server \"%s\" 已安装成功，发现 %d 个工具，立即可用", serverName, toolCount), nil
		},
	}
}

// NewRefreshMCPTool creates the refresh_mcp tool that reconnects an installed MCP server.
func (a *Agent) NewRefreshMCPTool(mgr *mcp.Manager) tool.Tool {
	return tool.Tool{
		Name: "refresh_mcp",
		Description: `Reconnect an already installed MCP server and refresh its tools in the current process.
Use after installing project-level prerequisites (for example codegraph init) or when a server previously discovered 0 tools.
The server must already exist in ~/.qiuqiu/mcp_servers.json.`,
		ReadOnly: false,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "已安装的 MCP Server 名称，例如 codegraph 或 filesystem",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.Name == "" {
				return "", fmt.Errorf("name 不能为空")
			}
			toolCount, err := mgr.Refresh(p.Name)
			if err != nil {
				return "", err
			}
			a.noticef("  ✅ MCP Server [%s] 已刷新，发现 %d 个工具\n", p.Name, toolCount)
			return fmt.Sprintf("MCP Server %q 已刷新，发现 %d 个工具，立即可用", p.Name, toolCount), nil
		},
	}
}
