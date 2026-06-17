package tool

import (
	"context"
	"encoding/json"
)

type Tool struct {
	Name        string
	Description string
	Parameters  any
	ReadOnly    bool
	Execute     func(ctx context.Context, args json.RawMessage) (string, error)
}

func AllBuiltInTools() []Tool {
	return []Tool{
		NewReadFileTool(), NewWriteFileTool(), NewListDirectoryTool(),
		NewEditFileTool(), NewMultiEditTool(),
		NewDeleteRangeTool(),
		NewTodoWriteTool(),
		NewSearchFilesTool(), NewGlobTool(), NewGrepTool(),
		NewCodeSearchTool(), NewWebFetchTool(),
		NewGitCommitTool(), NewRunShellTool(),
	}
}
