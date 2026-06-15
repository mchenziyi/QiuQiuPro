// 球球 Agent — 主入口
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"flag"

	"agentdemo/agent"
	"agentdemo/command"
	"agentdemo/event"
	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
)

type MCPConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func getAPIKey(in *bufio.Reader) string {
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return key
	}
	home, _ := os.UserHomeDir()
	keyFile := home + "/.qiuqiu/key"
	if data, err := os.ReadFile(keyFile); err == nil {
		key := strings.TrimSpace(string(data))
		if key != "" {
			return key
		}
	}
	fmt.Print("首次使用，请输入你的 DeepSeek API Key（输入后自动保存，下次不用再输）: ")
	key, _ := in.ReadString('\n')
	key = strings.TrimSpace(key)
	if key == "" {
		fmt.Println("API Key 不能为空")
		return getAPIKey(in)
	}
	os.MkdirAll(home+"/.qiuqiu", 0700)
	os.WriteFile(keyFile, []byte(key), 0600)
	fmt.Println("✅ API Key 已保存到", keyFile)
	return key
}

func loadMCPConfigs() []MCPConfig {
	home, _ := os.UserHomeDir()
	configFile := home + "/.qiuqiu/mcp_servers.json"
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		fmt.Printf("  ⚠️  读取 MCP 配置失败：%v\n", err)
		return nil
	}
	var configs []MCPConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		fmt.Printf("  ⚠️  解析 MCP 配置失败：%v\n", err)
		return nil
	}
	return configs
}

func main() {
	quiet := flag.Bool("q", false, "安静模式，减少中间日志")
	flag.Parse()

	// 全程只用这一个 stdin 读取器：读 API Key、主循环、高危确认共用，避免混用导致缓冲错位。
	stdin := bufio.NewReader(os.Stdin)

	apiKey := getAPIKey(stdin)
	a := agent.New(apiKey, "deepseek-chat")
	a.SetInput(stdin)
	a.RegisterTools(tool.AllBuiltInTools())
	a.Quiet = *quiet
	ctx := context.Background()

	// ========== 加载 MCP 插件 ==========
	fmt.Println("\n🔌 正在加载 MCP 插件...")
	configs := loadMCPConfigs()
	if len(configs) == 0 {
		fmt.Println("  没有配置 MCP Server（可编辑 ~/.qiuqiu/mcp_servers.json 添加）")
	}
	for _, cfg := range configs {
		mc, err := mcp.Connect(cfg.Name, cfg.Command, cfg.Args...)
		if err != nil {
			fmt.Printf("  ⚠️  [%s] 加载失败：%v\n", cfg.Name, err)
			continue
		}
		tools, err := mc.DiscoverTools()
		if err != nil {
			fmt.Printf("  ⚠️  [%s] 工具发现失败：%v\n", cfg.Name, err)
			continue
		}
		a.RegisterMCPTools(mc.Name, tools)
		fmt.Printf("  ✅ [%s] 已加载 %d 个工具\n", mc.Name, len(tools))
	}

	// ========== 加载 Skill ==========
	home, _ := os.UserHomeDir()
	skillsDir := home + "/.qiuqiu/skills"
	allSkills, _ := skill.LoadFromDir("prompt/skills")
	externalSkills, _ := skill.LoadFromDir(skillsDir)
	allSkills = append(allSkills, externalSkills...)

	fmt.Println("\n🎯 可用 Skill（输入 /use <技能名> 切换）：")
	for _, s := range allSkills {
		fmt.Printf("  - %s\n", s.Name)
	}

	// ========== 注册命令 ==========
	registry := a.CommandRegistry()

	// /help — 列出所有命令
	registry.Register(command.Command{
		Name: "help", Description: "显示所有可用命令",
		Handler: func(args string) bool {
			fmt.Println("\n📖 可用命令：")
			for _, c := range registry.List() {
				fmt.Printf("  /%-10s — %s\n", c.Name, c.Description)
			}
			fmt.Println()
			return true
		},
	})

	// /subagent <task> — 派生子 Agent 执行独立任务
	registry.Register(command.Command{
		Name: "subagent", Description: "派生子 Agent 执行独立任务。用法：/subagent <任务描述>",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Println("❌ 请指定子任务，如：/subagent 查一下 strings.Builder 的用法")
				return true
			}
			fmt.Printf("  🧩 派生子 Agent 执行：%s\n", args)
			result, err := a.SpawnSubAgent(ctx, args)
			if err != nil {
				fmt.Printf("  ❌ 子 Agent 执行失败：%v\n", err)
			} else {
				fmt.Printf("  🧩 子 Agent 返回：%s\n", result)
			}
			return true
		},
	})
	// /replay — 回放事件日志
	registry.Register(command.Command{
		Name: "replay", Description: "回放当前会话的事件日志",
		Handler: func(args string) bool {
			events, err := a.EventStore().Load(a.SessionID())
			if err != nil {
				fmt.Printf("❌ 读取失败：%v\n", err)
			} else {
				fmt.Println(event.Replay(a.SessionID(), events))
			}
			return true
		},
	})

	// /explain <file> — 解释文件内容
	registry.Register(command.Command{
		Name: "explain", Description: "解释指定文件内容。用法：/explain <文件路径>",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Println("❌ 请指定文件路径，如：/explain main.go")
				return true
			}
			fmt.Printf("📖 正在解释 %s ...\n", args)
			answer, err := a.Run(ctx, fmt.Sprintf("请解释文件 %s 的内容和作用", args))
			if err != nil {
				fmt.Printf("❌ 解释失败：%v\n", err)
			} else {
				fmt.Printf("📖 %s\n", answer)
			}
			return true
		},
	})

	// /test — 运行测试
	registry.Register(command.Command{
		Name: "test", Description: "运行当前项目的测试。用法：/test [包路径]",
		Handler: func(args string) bool {
			target := "."
			if args != "" {
				target = args
			}
			fmt.Printf("🧪 正在运行测试 %s ...\n", target)
			answer, err := a.Run(ctx, fmt.Sprintf("请运行 %s 目录下的测试，如果失败则分析原因并修复", target))
			if err != nil {
				fmt.Printf("❌ 测试失败：%v\n", err)
			} else {
				fmt.Printf("🧪 %s\n", answer)
			}
			return true
		},
	})

	// /use <skill> — 切换 Skill
	registry.Register(command.Command{
		Name: "use", Description: "切换 Skill（专业模式）。用法：/use <技能名>",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Println("❌ 请指定 Skill 名，如：/use architect")
				fmt.Println("可用 Skill：")
				for _, s := range allSkills {
					fmt.Printf("  - %s：%s\n", s.Name, s.Description)
				}
				return true
			}
			for _, s := range allSkills {
				if s.Name == args {
					a.ApplySkill(s)
					return true
				}
			}
			fmt.Printf("❌ 未找到 Skill：%s（输入 /use 查看所有可用 Skill）\n", args)
			return true
		},
	})

	// /mode <ask|plan> — 切换运行模式
	registry.Register(command.Command{
		Name: "mode", Description: "切换运行模式：plan（规划执行）/ ask（直接问答）。用法：/mode <模式名>",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Printf("当前模式：%s（可选：plan 规划执行 / ask 直接问答）\n", a.CurrentMode())
				return true
			}
			a.SetMode(args)
			return true
		},
	})

	fmt.Printf("\n🤖 球球 Agent 已启动 | Skill：[%s] 模式：[%s]（输入 /help 查看所有命令）\n",
		a.CurrentSkillName(), a.CurrentMode())
	fmt.Println(strings.Repeat("─", 50))

	// ========== 交互式对话循环 ==========
	for {
		modeTag := strings.ToUpper(a.CurrentMode())
		fmt.Printf("\n🧑 [%s] 你: ", modeTag)
		line, ok := a.ReadLine()
		if !ok {
			break
		}
		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// 先尝试匹配命令（以 / 开头）
		if registry.Handle(input) {
			continue
		}

		// exit 直接退出
		if input == "exit" || input == "quit" {
			fmt.Println("👋 再见！")
			break
		}

		// 按模式分支
		switch a.CurrentMode() {
		case "ask":
			// Ask 模式：直接问答，不走规划
			answer, err := a.Run(ctx, input)
			if err != nil {
				fmt.Printf("❌ 回答失败：%v\n", err)
			} else {
				fmt.Printf("\n🤖 %s\n", answer)
			}

		case "plan":
			// Plan 模式：规划 → 执行（原有流程）
			fmt.Println("📋 正在拆解计划...")
			plan, err := a.GeneratePlan(ctx, input)
			if err != nil {
				fmt.Printf("❌ 规划失败：%v\n", err)
				continue
			}
			fmt.Println("📋 计划如下：")
			for _, s := range plan.Steps {
				fmt.Printf("  %d. %s\n", s.ID, s.Desc)
			}

			fmt.Println("\n🔍 正在审查计划质量...")
			plan, _ = a.ReviewPlan(ctx, plan)

			fmt.Println("\n🚀 开始执行...")
			err = a.ExecutePlan(ctx, plan)
			if err != nil {
				fmt.Printf("❌ 执行失败：%v\n", err)
				continue
			}
			fmt.Println("\n🎉 全部完成！")
			a.TrimMessages()
		}
	}
}
