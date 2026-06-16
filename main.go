// 球球 Agent — 主入口
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"agentdemo/agent"
	"agentdemo/cleanup"
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

// envFloat 读取一个非负浮点环境变量；缺省 / 非法 / 为负时返回 0（视为未配置）。
func envFloat(key string) float64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return 0
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
	// 默认 deepseek-v4-flash（旧 deepseek-chat 将于 2026-07-24 下线）；可经环境变量切换为
	// deepseek-v4-pro 等。注意：V4 默认开启 thinking，客户端已统一关闭以沿用非思考行为与成本。
	model := "deepseek-v4-flash"
	if v := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL")); v != "" {
		model = v
	}
	a := agent.New(apiKey, model)
	a.SetInput(stdin)
	a.RegisterTools(tool.AllBuiltInTools())
	a.RegisterTool(a.NewRememberRuleTool())
	a.RegisterTool(a.NewAskTool())
	a.Quiet = *quiet
	// 上下文窗口可经环境变量覆盖（默认贴合 DeepSeek V4 的 1M）；切到更小窗口的模型时务必调小。
	if v := strings.TrimSpace(os.Getenv("DEEPSEEK_CONTEXT_WINDOW")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			a.SetContextWindow(n)
		}
	}
	// 单次连续计划执行的 step 上限。0 表示不限制；达到上限会协作式暂停，输入 /resume 继续。
	if v := strings.TrimSpace(os.Getenv("DEEPSEEK_MAX_STEPS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			a.SetMaxSteps(n)
		}
	}
	// Token 单价（每 1M token，货币单位自定）。配置任一项后 /usage 才展示估算费用，
	// 默认不配置——价格随模型与时间变动，编造金额不如不显（参见 DeepSeek 官方定价页校准）。
	a.SetPricing(agent.Pricing{
		InputMiss: envFloat("DEEPSEEK_PRICE_INPUT"),
		InputHit:  envFloat("DEEPSEEK_PRICE_CACHE_HIT"),
		Output:    envFloat("DEEPSEEK_PRICE_OUTPUT"),
	})
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

	// /cleanup [目录] — 扫描并清理垃圾文件
	registry.Register(command.Command{
		Name: "cleanup", Description: "扫描并清理垃圾文件（.DS_Store / *.tmp / *.bak / *.swp 等）。用法：/cleanup [目录]",
		Handler: func(args string) bool {
			dir := strings.TrimSpace(args)
			if dir == "" {
				dir = "."
			}
			files, err := cleanup.Scan(dir)
			if err != nil {
				fmt.Printf("❌ 扫描失败：%v\n", err)
				return true
			}
			if len(files) == 0 {
				fmt.Printf("✨ %s 下没有发现垃圾文件\n", dir)
				return true
			}
			fmt.Printf("🗑️  在 %s 下发现 %d 个垃圾文件：\n", dir, len(files))
			fmt.Print(cleanup.FormatList(files))
			fmt.Print("  确认全部删除？[Y/n] ")
			if !a.Confirm() {
				fmt.Println("  已取消，未删除任何文件")
				return true
			}
			deleted, errs := cleanup.Delete(files)
			fmt.Printf("  ✅ 已删除 %d 个文件\n", deleted)
			for _, e := range errs {
				fmt.Printf("  ⚠️  %v\n", e)
			}
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

	// /readonly [on|off] — 切换只读模式（拒绝一切写操作）
	registry.Register(command.Command{
		Name: "readonly", Description: "切换只读模式：on 拒绝一切写 / 执行 / 提交操作，off 恢复默认（高危确认）。用法：/readonly [on|off]",
		Handler: func(args string) bool {
			switch strings.ToLower(strings.TrimSpace(args)) {
			case "on":
				a.SetReadOnly(true)
				fmt.Println("  🔒 已开启只读模式：写文件 / 编辑 / 运行命令 / 提交 将被拒绝")
			case "off":
				a.SetReadOnly(false)
				fmt.Println("  🔓 已关闭只读模式：恢复默认（高危操作需确认）")
			case "":
				state := "关闭"
				if a.IsReadOnly() {
					state = "开启"
				}
				fmt.Printf("  当前只读模式：%s（权限门：%s）。用法：/readonly on|off\n", state, a.GateName())
			default:
				fmt.Println("  ⚠️  用法：/readonly on|off")
			}
			return true
		},
	})

	// /compact — 手动压缩上下文（在前缀缓存自然填满前主动重置一次）
	registry.Register(command.Command{
		Name: "compact", Description: "手动压缩上下文：把较早的对话折叠成摘要、保留近消息，主动重置前缀缓存。用法：/compact",
		Handler: func(args string) bool {
			a.Compact(ctx)
			return true
		},
	})

	// /usage — 查看本次会话累计 token 用量（及估算费用，若已配置单价）
	registry.Register(command.Command{
		Name: "usage", Description: "显示本次会话的 token 用量（输入 / 缓存命中 / 输出 / 思考 / 合计）与估算费用。用法：/usage",
		Handler: func(args string) bool {
			a.ReportUsage()
			return true
		},
	})

	// /memory — 查看模型自主沉淀的偏好/规则长期记忆（写入由 remember_rule 工具完成，无手动 /remember）。
	registry.Register(command.Command{
		Name: "memory", Description: "查看长期记忆（仅偏好/规则；写入由模型自主判断）。用法：/memory",
		Handler: func(args string) bool {
			fmt.Println(a.MemoryList())
			return true
		},
	})

	// /forget <id> — 删除一条长期记忆，给用户审计和纠错的出口。
	registry.Register(command.Command{
		Name: "forget", Description: "删除一条长期记忆。用法：/forget <memory_id>",
		Handler: func(args string) bool {
			id := strings.TrimSpace(args)
			if id == "" {
				fmt.Println("  ⚠️  用法：/forget <memory_id>，可先用 /memory 查看")
				return true
			}
			fmt.Println(a.ForgetMemory(id))
			return true
		},
	})

	// /maxsteps [n] — 配置一次连续执行最多跑多少个 Plan step；0 表示不限制。
	registry.Register(command.Command{
		Name: "maxsteps", Description: "设置单次连续计划执行的 step 上限；0 表示不限制。用法：/maxsteps [n]",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Printf("  当前 maxSteps：%d（0 表示不限制）\n", a.MaxSteps())
				return true
			}
			n, err := strconv.Atoi(strings.TrimSpace(args))
			if err != nil || n < 0 {
				fmt.Println("  ⚠️  用法：/maxsteps <非负整数>，0 表示不限制")
				return true
			}
			a.SetMaxSteps(n)
			if n == 0 {
				fmt.Println("  已关闭 maxSteps 限制")
			} else {
				fmt.Printf("  已设置 maxSteps=%d；达到上限会暂停，可输入 /resume 继续\n", n)
			}
			return true
		},
	})

	// /pause — 协作式暂停：当前 step 完成后停下并保存执行状态。
	registry.Register(command.Command{
		Name: "pause", Description: "请求协作式暂停：当前 step 完成后暂停，之后可 /resume 继续。用法：/pause",
		Handler: func(args string) bool {
			a.RequestPause()
			fmt.Println("  已请求暂停：当前 step 完成后会停下（若当前没有执行中的计划，则下次计划执行时生效）")
			return true
		},
	})

	// /resume — 从暂停状态继续执行 Plan。
	registry.Register(command.Command{
		Name: "resume", Description: "从上次暂停的 Plan step 继续执行。用法：/resume",
		Handler: func(args string) bool {
			if err := a.ResumePlan(ctx); err != nil {
				if errors.Is(err, agent.ErrPlanPaused) {
					return true
				}
				fmt.Printf("  ❌ 恢复失败：%v\n", err)
			}
			return true
		},
	})

	fmt.Printf("\n🤖 球球 Agent 已启动 | Skill：[%s] 模式：[%s]（输入 /help 查看所有命令）\n",
		a.CurrentSkillName(), a.CurrentMode())
	fmt.Println(strings.Repeat("─", 50))

	// ========== 交互式对话循环 ==========
	for {
		modeTag := strings.ToUpper(a.CurrentMode())
		if a.IsReadOnly() {
			modeTag = "🔒" + modeTag
		}
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

		case "auto":
			// Auto 模式：轻量分类后自动路由
			detected, err := a.DetectMode(ctx, input)
			if err != nil {
				fmt.Printf("  ⚠️  意图分类失败，退化到 plan：%v\n", err)
				detected = "plan"
			}
			if detected == "ask" {
				fmt.Printf("  🔍 检测到简单对话，走 Ask 模式\n")
				answer, err := a.Run(ctx, input)
				if err != nil {
					fmt.Printf("❌ 回答失败：%v\n", err)
				} else {
					fmt.Printf("\n🤖 %s\n", answer)
				}
			} else {
				fmt.Printf("  🔍 检测到开发任务，走 Plan 模式\n")
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
					if errors.Is(err, agent.ErrPlanPaused) {
						continue
					}
					fmt.Printf("❌ 执行失败：%v\n", err)
					continue
				}
				fmt.Println("\n🎉 全部完成！")
				a.TrimMessages()
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
				if errors.Is(err, agent.ErrPlanPaused) {
					continue
				}
				fmt.Printf("❌ 执行失败：%v\n", err)
				continue
			}
			fmt.Println("\n🎉 全部完成！")
			a.TrimMessages()
		}
	}
}




