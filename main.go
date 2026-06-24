// 球球 Agent — 主入口
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"agentdemo/agent"
	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
	"agentdemo/web"
)

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

func main() {
	var continueSession bool
	flag.BoolVar(&continueSession, "c", false, "恢复最近一次会话")
	flag.BoolVar(&continueSession, "continue", false, "恢复最近一次会话（同 -c）")
	quiet := flag.Bool("q", false, "安静模式，减少中间日志")
	webMode := flag.String("web", "", "启动 Web UI（:端口号，如 :8080）")
	flag.Parse()

	// 全程只用这一个 stdin 读取器：读 API Key、主循环、高危确认共用，避免混用导致缓冲错位。
	stdin := bufio.NewReader(os.Stdin)

	apiKey := getAPIKey(stdin)
	// 默认 deepseek-v4-flash；可经环境变量 DEEPSEEK_MODEL 切换为其他模型。
	// thinking 默认开启（max），可经 DEEPSEEK_THINKING / DEEPSEEK_REASONING_EFFORT 调整。
	model := "deepseek-v4-flash"
	if v := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL")); v != "" {
		model = v
	}
	a, err := agent.New(apiKey, model, continueSession)
	if err != nil {
		if errors.Is(err, agent.ErrNoSessionToResume) {
			fmt.Println("❌ 没有可恢复的会话（先正常对话一轮，或不要加 -c/--continue）")
		} else {
			fmt.Printf("❌ 启动失败：%v\n", err)
		}
		os.Exit(1)
	}
	a.SetInput(stdin)
	if err := a.EnsureQiuqiuRuleFiles(); err != nil {
		fmt.Printf("⚠️ 初始化 QIUQIU.md 失败：%v\n", err)
	}
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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for range sigCh {
			a.Interrupt()
		}
	}()

	// ========== 加载 Skill（通过 Manager，支持热安装） ==========
	home, _ := os.UserHomeDir()
	skillsDir := home + "/.qiuqiu/skills"
	skillMgr := skill.NewManager("prompt/skills", skillsDir)

	// ========== 加载 MCP 插件（通过 Manager，支持热安装） ==========
	mcpConfigPath := home + "/.qiuqiu/mcp_servers.json"
	mcpMgr := mcp.NewManager(mcpConfigPath, mcp.Connect, a.RegisterMCPTools)

	fmt.Println("\n🔌 正在加载 MCP 插件...")
	mcpConfigs := mcpMgr.List()
	if len(mcpConfigs) == 0 {
		fmt.Println("  没有配置 MCP Server（可编辑 ~/.qiuqiu/mcp_servers.json 添加，或让我帮你安装）")
	}
	for _, cfg := range mcpConfigs {
		mc, err := mcp.Connect(cfg.Name, cfg.Command, cfg.Args...)
		if err != nil {
			fmt.Printf("  ⚠️  [%s] 加载失败：%v\n", cfg.Name, err)
			continue
		}
		tools, err := mc.DiscoverTools()
		if err != nil {
			fmt.Printf("  ⚠️  [%s] 工具发现失败：%v\n", cfg.Name, err)
			mc.Close()
			continue
		}
		a.RegisterMCPTools(mc.Name, tools)
		mcpMgr.TrackClient(cfg.Name, mc)
		fmt.Printf("  ✅ [%s] 已加载 %d 个工具\n", mc.Name, len(tools))
	}

	// 注册热安装工具
	a.RegisterTool(a.NewInstallSkillTool(skillMgr))
	a.RegisterTool(a.NewDeleteSkillTool(skillMgr))
	a.RegisterTool(a.NewInstallMCPTool(mcpMgr))
	a.RegisterTool(a.NewRefreshMCPTool(mcpMgr))

	fmt.Println("\n🎯 可用 Skill（输入 /use <技能名> 切换）：")
	for _, s := range skillMgr.List() {
		fmt.Printf("  - %s\n", s.Name)
	}

	// ========== 注册命令 ==========
	registry := a.CommandRegistry()
	registerCommands(registry, a, ctx, skillMgr)

	fmt.Printf("\n🤖 球球 Agent 已启动 | Skill：[%s] 模式：[%s]（输入 /help 查看所有命令）\n",
		a.CurrentSkillName(), a.CurrentMode())
	fmt.Println(strings.Repeat("─", 50))

	// ========== Web UI 模式 ==========
	if *webMode != "" {
		srv := web.NewServer(a)
		addr := *webMode
		if addr[0] == ':' {
			addr = "localhost" + addr
		}
		fmt.Printf("🌐 Web UI 启动于 http://%s\n", addr)

		// Ctrl+C 优雅关闭 HTTP 服务
		shutdownCh := make(chan os.Signal, 1)
		signal.Notify(shutdownCh, os.Interrupt)
		go func() {
			<-shutdownCh
			fmt.Println("\n👋 正在关闭 HTTP 服务...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()

		if err := srv.ListenAndServe(*webMode); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "❌ HTTP 服务异常退出：%v\n", err)
			os.Exit(1)
		}
		return
	}

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
			if errors.Is(err, agent.ErrInterrupted) {
				fmt.Println("  ⚡ 已中断当前操作")
				continue
			}
			if err != nil {
				fmt.Printf("❌ 回答失败：%v\n", err)
			} else {
				fmt.Printf("\n🤖 %s\n", answer)
			}

		case "plan":
			// Plan 模式：只读调研 → GeneratePlan → ReviewPlan → 审批 → ExecutePlan
			goal := input

			if !a.HasReadOnlyTools() {
				fmt.Println("  ⚠️  当前没有可用的只读调研工具（如 read_file / grep / search_files 等），无法进入 Plan 模式。")
				fmt.Println("  请先 /use default 恢复默认工具集，或安装带读工具的 Skill 后再试。")
				continue
			}

			a.SetPlanMode(true)
			fmt.Println("  📋 正在调研方案...（只读模式，不会修改代码）")
			research, err := a.Run(ctx, goal)
			if errors.Is(err, agent.ErrInterrupted) {
				fmt.Println("  ⚡ 已中断当前操作")
				a.SetPlanMode(false)
				continue
			}
			if err != nil {
				fmt.Printf("❌ 调研失败：%v\n", err)
				a.SetPlanMode(false)
				continue
			}

			fmt.Println("  📋 正在生成执行计划...")
			planGoal := goal
			if strings.TrimSpace(research) != "" {
				planGoal = goal + "\n\n调研摘要（供规划参考）：\n" + research
			}
			plan, err := a.GeneratePlan(ctx, planGoal)
			if err != nil {
				fmt.Printf("❌ 规划失败：%v\n", err)
				a.SetPlanMode(false)
				continue
			}
			plan, _ = a.ReviewPlan(ctx, plan)

			fmt.Printf("\n📋 方案建议：\n%s\n", formatPlanProposal(research, plan))
			fmt.Print("  批准执行？[Y/n] ")
			if !a.Confirm() {
				fmt.Println("  已取消执行，可以修改后重试")
				a.SetPlanMode(false)
				continue
			}

			a.SetPlanMode(false)
			fmt.Println("  ✅ 方案已批准，开始执行...")
			if err := a.ExecutePlan(ctx, plan); err != nil {
				if errors.Is(err, agent.ErrPlanPaused) {
					continue
				}
				if errors.Is(err, agent.ErrInterrupted) {
					fmt.Println("  ⚡ 已中断当前操作")
					continue
				}
				fmt.Printf("❌ 执行失败：%v\n", err)
				continue
			}
			fmt.Println("\n🎉 执行完成！")
		}
	}
}
