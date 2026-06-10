// Package mcp 封装 MCP 协议客户端
// MCP = Model Context Protocol，标准化的工具发现和调用协议
// 通信方式：JSON-RPC over stdio（启动外部 Server 进程，通过标准输入输出传 JSON）
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/client"
	mcp "github.com/mark3labs/mcp-go/mcp"

	"agentdemo/tool"
)

// MCPClient 包装一个 MCP Server 连接
type MCPClient struct {
	Name   string         // Server 名称，用作工具名前缀
	client *client.Client // MCP 协议客户端
}

// Connect 启动一个 MCP Server 进程并建立连接
// name：Server 名称，command：启动命令，args：启动参数
// 示例：Connect("filesystem", "npx", "-y", "@modelcontextprotocol/server-filesystem", ".")
func Connect(name, command string, args ...string) (*MCPClient, error) {
	// 启动 MCP Server 子进程（通过 stdio 管道通信）
	mcpClient, err := client.NewStdioMCPClient(command, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("启动 MCP Server %s 失败：%w", name, err)
	}

	// 发送初始化请求（MCP 协议握手，确认双方版本兼容）
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "球球", Version: "0.1.0"}
	_, err = mcpClient.Initialize(context.Background(), initReq)
	if err != nil {
		return nil, fmt.Errorf("初始化 MCP Server %s 失败：%w", name, err)
	}

	return &MCPClient{Name: name, client: mcpClient}, nil
}

// DiscoverTools 获取 Server 暴露的所有工具，包装成球球的 Tool 格式
// 每个工具加前缀（如 filesystem_read_file），避免命名冲突
func (c *MCPClient) DiscoverTools() ([]tool.Tool, error) {
	// 调用 MCP 协议的 ListTools 方法，让 Server 返回它支持的所有工具
	toolsReq := mcp.ListToolsRequest{}
	toolsResp, err := c.client.ListTools(context.Background(), toolsReq)
	if err != nil {
		return nil, fmt.Errorf("获取 MCP Server %s 工具列表失败：%w", c.Name, err)
	}

	// 把 MCP 工具格式转换为球球的 tool.Tool 格式
	var tools []tool.Tool
	for i := range toolsResp.Tools {
		mt := toolsResp.Tools[i] // 取副本，避免闭包捕获循环变量的问题
		t := tool.Tool{
			Name:        fmt.Sprintf("%s_%s", c.Name, mt.Name), // 加前缀，如 "filesystem_read_file"
			Description: mt.Description,
			Parameters:  mt.InputSchema, // MCP 的 InputSchema 就是 JSON Schema 格式，直接复用
			Execute: func(args string) string {
				// 解析参数为 map
				var params map[string]any
				json.Unmarshal([]byte(args), &params)

				// 调用 MCP 工具的 CallTool 方法
				callReq := mcp.CallToolRequest{}
				callReq.Params.Name = mt.Name
				callReq.Params.Arguments = params
				resp, err := c.client.CallTool(context.Background(), callReq)
				if err != nil {
					return fmt.Sprintf("MCP 工具调用失败：%v", err)
				}

				// 收集返回的文本内容（可能有多个 Content 片段）
				var parts []string
				for _, content := range resp.Content {
					if tc, ok := content.(mcp.TextContent); ok {
						parts = append(parts, tc.Text)
					}
				}
				return strings.Join(parts, "\n")
			},
		}
		tools = append(tools, t)
	}
	return tools, nil
}
