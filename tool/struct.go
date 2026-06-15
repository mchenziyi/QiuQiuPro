// Package tool 定义了 Agent 可调用的工具
package tool

// Tool 定义一个 Agent 可调用的工具
type Tool struct {
	Name        string             // 工具名，LLM 通过它调用
	Description string             // 描述，告诉 LLM 什么时候用
	Parameters  any                // 参数定义（JSON Schema）
	Execute     func(string) string // 执行函数，返回 LLM 友好的文本
}

// AllBuiltInTools 返回所有内置工具列表
func AllBuiltInTools() []Tool {
	return []Tool{
		NewReadFileTool(),
		NewWriteFileTool(),
		NewListDirectoryTool(),
		NewCountFileCharsTool(),
		NewEditFileBlockTool(),
		NewSearchFilesTool(), NewGlobTool(), NewGrepTool(),
		NewCodeSearchTool(),
		NewWebFetchTool(),
		NewGitCommitTool(),
		NewRunPowerShellTool(),
		NewRunShellTool(),
	}
}
