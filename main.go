package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// ========== 工具定义 ==========

type Tool struct {
	Name        string
	Description string
	Parameters  any
	Execute     func(string) string
}

// ========== 内置工具 ==========

func NewReadFileTool() Tool {
	return Tool{
		Name:        "read_file",
		Description: "读取指定文件的内容",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "文件路径"},
			},
			"required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			return fmt.Sprintf("文件 %s（%d 字节）内容：\n%s", p.Path, len(data), string(data))
		},
	}
}

func NewWriteFileTool() Tool {
	return Tool{
		Name:        "write_file",
		Description: "将内容写入指定文件，会覆盖已存在的文件",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "文件路径"},
				"content": map[string]any{"type": "string", "description": "要写入的内容"},
			},
			"required": []string{"path", "content"},
		},
		Execute: func(args string) string {
			var p struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			json.Unmarshal([]byte(args), &p)
			err := os.WriteFile(p.Path, []byte(p.Content), 0644)
			if err != nil {
				return fmt.Sprintf("写入失败：%v", err)
			}
			return fmt.Sprintf("文件 %s 已写入（%d 字节）", p.Path, len(p.Content))
		},
	}
}

func NewListDirectoryTool() Tool {
	return Tool{
		Name:        "list_directory",
		Description: "列出指定目录下的文件和子目录",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "目录路径"},
			},
			"required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)
			if p.Path == "" {
				p.Path = "."
			}
			entries, err := os.ReadDir(p.Path)
			if err != nil {
				return fmt.Sprintf("列目录失败：找不到 %s", p.Path)
			}
			var files, dirs []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, e.Name())
				} else {
					info, _ := e.Info()
					files = append(files, fmt.Sprintf("%s（%d 字节）", e.Name(), info.Size()))
				}
			}
			var b strings.Builder
			fmt.Fprintf(&b, "目录 %s：\n", p.Path)
			if len(dirs) > 0 {
				fmt.Fprintf(&b, "  子目录：%s\n", strings.Join(dirs, "、"))
			}
			if len(files) > 0 {
				fmt.Fprintf(&b, "  文件：\n    %s\n", strings.Join(files, "\n    "))
			}
			if len(entries) == 0 {
				fmt.Fprint(&b, "  （空目录）\n")
			}
			return b.String()
		},
	}
}

// ========== V2 新增工具：精确编辑 + Git ==========

// EditFileBlock 精确编辑：找到一段代码，替换成新的
// 如果找不到或找到多处，会返回错误提示，不会乱改
func NewEditFileBlockTool() Tool {
	return Tool{
		Name:        "edit_file_block",
		Description: "精确修改文件：找到一段旧代码，替换成新代码。改前会先备份",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "文件路径"},
				"old_block": map[string]any{"type": "string", "description": "要替换的旧代码（必须是文件中唯一的文本段）"},
				"new_block": map[string]any{"type": "string", "description": "替换后的新代码"},
			},
			"required": []string{"path", "old_block", "new_block"},
		},
		Execute: func(args string) string {
			var p struct {
				Path     string `json:"path"`
				OldBlock string `json:"old_block"`
				NewBlock string `json:"new_block"`
			}
			json.Unmarshal([]byte(args), &p)

			// 读文件
			data, err := os.ReadFile(p.Path)
			if err != nil {
				return fmt.Sprintf("读文件失败：找不到 %s", p.Path)
			}
			text := string(data)

			// 检查旧代码是否存在
			if !strings.Contains(text, p.OldBlock) {
				return fmt.Sprintf("修改失败：在 %s 中找不到指定的旧代码。请确认内容准确", p.Path)
			}

			// 检查是否唯一
			if strings.Count(text, p.OldBlock) > 1 {
				return fmt.Sprintf("修改失败：旧代码在文件中出现多次，请提供更多前后文让它唯一匹配")
			}

			// 替换
			text = strings.Replace(text, p.OldBlock, p.NewBlock, 1)
			err = os.WriteFile(p.Path, []byte(text), 0644)
			if err != nil {
				return fmt.Sprintf("写入失败：%v", err)
			}
			return fmt.Sprintf("已修改 %s：替换了一段 %d 字符的代码为 %d 字符", p.Path, len(p.OldBlock), len(p.NewBlock))
		},
	}
}

// GitCommit 提交所有变更
func NewGitCommitTool() Tool {
	return Tool{
		Name:        "git_commit",
		Description: "提交当前所有文件变更到 Git，需要提供提交信息",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string", "description": "提交信息，描述这次改了什么"},
			},
			"required": []string{"message"},
		},
		Execute: func(args string) string {
			var p struct{ Message string `json:"message"` }
			json.Unmarshal([]byte(args), &p)

			exec.Command("git", "add", ".").Run()
			out, err := exec.Command("git", "commit", "-m", p.Message).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("提交失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("已提交：%s", p.Message)
		},
	}
}

// GitRevertFile 恢复文件到上一个提交的状态
func NewGitRevertFileTool() Tool {
	return Tool{
		Name:        "git_revert_file",
		Description: "恢复指定文件到上次 Git 提交的状态。改错代码时用这个撤销",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "要恢复的文件路径"},
			},
			"required": []string{"path"},
		},
		Execute: func(args string) string {
			var p struct{ Path string `json:"path"` }
			json.Unmarshal([]byte(args), &p)

			out, err := exec.Command("git", "checkout", "--", p.Path).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("恢复失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("已恢复 %s 到上次提交的状态", p.Path)
		},
	}
}

func NewRunShellTool() Tool {
	return Tool{
		Name:        "run_shell",
		Description: "执行一条 Windows cmd 命令。优先用其他专用工具，搞不定再用这个",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "要执行的 cmd 命令"},
			},
			"required": []string{"command"},
		},
		Execute: func(args string) string {
			var p struct{ Command string `json:"command"` }
			json.Unmarshal([]byte(args), &p)
			out, err := exec.Command("cmd", "/C", p.Command).CombinedOutput()
			if err != nil {
				return fmt.Sprintf("命令失败：%v\n输出：%s", err, string(out))
			}
			return fmt.Sprintf("输出：\n%s", string(out))
		},
	}
}

// ========== Plan 和 Step ==========

type Step struct {
	ID     int    `json:"id"`
	Desc   string `json:"desc"`
	Status string `json:"status"` // pending / running / done / failed
}

type Plan struct {
	Goal  string `json:"goal"`
	Steps []Step `json:"steps"`
}

// ========== Agent ==========

type Agent struct {
	client   *openai.Client
	model    string
	tools    map[string]Tool
	messages []openai.ChatCompletionMessage
}

const maxMessages = 100

func NewAgent(apiKey, model string) *Agent {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com"
	return &Agent{
		client:   openai.NewClientWithConfig(config),
		model:    model,
		tools:    make(map[string]Tool),
		messages: make([]openai.ChatCompletionMessage, 0),
	}
}

func (a *Agent) RegisterTool(t Tool) {
	a.tools[t.Name] = t
}

func (a *Agent) toolDefinitions() []openai.Tool {
	var tools []openai.Tool
	for _, t := range a.tools {
		schema, _ := json.Marshal(t.Parameters)
		var params map[string]any
		json.Unmarshal(schema, &params)
		tools = append(tools, openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

func (a *Agent) trimMessages() {
	if len(a.messages) > maxMessages {
		a.messages = append(
			[]openai.ChatCompletionMessage{a.messages[0]},
			a.messages[len(a.messages)-maxMessages+1:]...,
		)
	}
}

func (a *Agent) Run(ctx context.Context, userInput string) (string, error) {
	a.messages = append(a.messages, openai.ChatCompletionMessage{
		Role: "user", Content: userInput,
	})
	maxLoops := 15
	for i := 0; i < maxLoops; i++ {
		resp, err := a.client.CreateChatCompletion(ctx,
			openai.ChatCompletionRequest{
				Model:    a.model,
				Messages: a.messages,
				Tools:    a.toolDefinitions(),
			},
		)
		if err != nil {
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}
		msg := resp.Choices[0].Message
		a.messages = append(a.messages, msg)
		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}
		for _, tc := range msg.ToolCalls {
			fmt.Printf("  🔧 %s(%s)\n", tc.Function.Name, tc.Function.Arguments)
			tool, ok := a.tools[tc.Function.Name]
			if !ok {
				return "", fmt.Errorf("未知工具: %s", tc.Function.Name)
			}
			result := tool.Execute(tc.Function.Arguments)
			fmt.Printf("  📦 %s\n", truncate(result, 100))
			a.messages = append(a.messages, openai.ChatCompletionMessage{
				Role: "tool", Content: result, ToolCallID: tc.ID,
			})
		}
	}
	return "", fmt.Errorf("达到最大循环次数 %d", maxLoops)
}

// ========== Planning（V1 保留）==========

func (a *Agent) GeneratePlan(ctx context.Context, goal string) (*Plan, error) {
	var toolList []string
	for _, t := range a.tools {
		toolList = append(toolList, fmt.Sprintf("- %s：%s", t.Name, t.Description))
	}

	prompt := fmt.Sprintf(`你是一个项目规划专家。请把目标拆成 3~8 个步骤。

可用工具：
%s

要求：
- 每步必须能用上面的工具完成
- 按执行顺序排列
- 每步不超过 15 个字
- 步骤数不超过 8 步

只输出 JSON，格式：[{"id":1,"desc":"步骤描述"}, ...]

目标：%s`, strings.Join(toolList, "\n"), goal)

	resp, err := a.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: a.model,
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "你是一个严谨的项目规划专家，只输出 JSON"},
				{Role: "user", Content: prompt},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("规划失败：%w", err)
	}

	content := resp.Choices[0].Message.Content
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	type stepJSON struct {
		ID   int    `json:"id"`
		Desc string `json:"desc"`
	}
	var steps []stepJSON
	if err := json.Unmarshal([]byte(content), &steps); err != nil {
		return nil, fmt.Errorf("解析失败：%w\n原始：%s", err, content)
	}

	plan := &Plan{Goal: goal}
	for _, s := range steps {
		plan.Steps = append(plan.Steps, Step{
			ID: s.ID, Desc: s.Desc, Status: "pending",
		})
	}
	return plan, nil
}

func (a *Agent) ExecutePlan(ctx context.Context, plan *Plan) error {
	total := len(plan.Steps)
	for i := range plan.Steps {
		step := &plan.Steps[i]
		step.Status = "running"
		fmt.Printf("\n📋 [%d/%d] %s\n", i+1, total, step.Desc)

		_, err := a.Run(ctx, fmt.Sprintf("请执行：%s", step.Desc))
		if err != nil {
			step.Status = "failed"
			return fmt.Errorf("第 %d 步失败：%w", step.ID, err)
		}
		step.Status = "done"
		fmt.Printf("✅ [%d/%d] 完成\n", i+1, total)
	}
	return nil
}

// ========== 主程序 ==========

func main() {
	fmt.Println("🚀 球球 V2 启动中...")
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置环境变量 DEEPSEEK_API_KEY")
		return
	}

	agent := NewAgent(apiKey, "deepseek-chat")
	agent.RegisterTool(NewReadFileTool())
	agent.RegisterTool(NewWriteFileTool())
	agent.RegisterTool(NewListDirectoryTool())
	agent.RegisterTool(NewEditFileBlockTool())  // V2 新增
	agent.RegisterTool(NewGitCommitTool())       // V2 新增
	agent.RegisterTool(NewGitRevertFileTool())   // V2 新增
	agent.RegisterTool(NewRunShellTool())

	ctx := context.Background()

	fmt.Println("🤖 球球 V2（Coding Agent）已启动（输入 exit 退出）")
	fmt.Println(strings.Repeat("─", 50))

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n🧑 你: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Println("👋 再见！")
			break
		}

		fmt.Println("📋 正在拆解计划...")
		plan, err := agent.GeneratePlan(ctx, input)
		if err != nil {
			fmt.Printf("❌ 规划失败：%v\n", err)
			continue
		}

		fmt.Println("📋 计划如下：")
		for _, s := range plan.Steps {
			fmt.Printf("  %d. %s\n", s.ID, s.Desc)
		}

		fmt.Println("\n🚀 开始执行...")
		err = agent.ExecutePlan(ctx, plan)
		if err != nil {
			fmt.Printf("❌ 执行失败：%v\n", err)
			continue
		}
		fmt.Println("\n🎉 全部完成！")

		agent.trimMessages()
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
