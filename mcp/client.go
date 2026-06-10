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

type MCPClient struct {
	Name   string
	client *client.Client
}

func Connect(name, command string, args ...string) (*MCPClient, error) {
	mcpClient, err := client.NewStdioMCPClient(command, nil, args...)
	if err != nil {
		return nil, fmt.Errorf("启动 MCP Server %s 失败：%w", name, err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "球球", Version: "0.1.0"}
	_, err = mcpClient.Initialize(context.Background(), initReq)
	if err != nil {
		return nil, fmt.Errorf("初始化 MCP Server %s 失败：%w", name, err)
	}
	return &MCPClient{Name: name, client: mcpClient}, nil
}

func (c *MCPClient) DiscoverTools() ([]tool.Tool, error) {
	toolsReq := mcp.ListToolsRequest{}
	toolsResp, err := c.client.ListTools(context.Background(), toolsReq)
	if err != nil {
		return nil, fmt.Errorf("获取 MCP Server %s 工具列表失败：%w", c.Name, err)
	}
	var tools []tool.Tool
	for i := range toolsResp.Tools {
		mt := toolsResp.Tools[i]
		t := tool.Tool{
			Name:        fmt.Sprintf("%s_%s", c.Name, mt.Name),
			Description: mt.Description,
			Parameters:  mt.InputSchema,
			Execute: func(args string) string {
				var params map[string]any
				json.Unmarshal([]byte(args), &params)
				callReq := mcp.CallToolRequest{}
				callReq.Params.Name = mt.Name
				callReq.Params.Arguments = params
				resp, err := c.client.CallTool(context.Background(), callReq)
				if err != nil {
					return fmt.Sprintf("MCP 工具调用失败：%v", err)
				}
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
