// 球球 Agent — 主入口
// 功能：初始化 Agent、注册工具、加载 MCP、启动交互式对话
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"agentdemo/agent"
	"agentdemo/event"
	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
)

func getAPIKey() string {
	// ① 优先从环境变量读取
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		return key
	}

	// ② 从本地配置文件读取（~/.qiuqiu/key）
	home, _ := os.UserHomeDir()
	keyFile := home + "/.qiuqiu/key"
	if data, err := os.ReadFile(keyFile); err == nil {
		key := strings.TrimSpace(string(data))
		if key != "" {
			return key
		}
	}

	// ③ 都没有 -> 让用户在终端输入
	fmt.Print("首次使用，请输入你的 DeepSeek API Key（输入后自动保存，下次不用再输）: ")
	reader := bufio.NewReader(os.Stdin)
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)
	if key == "" {
		fmt.Println("API Key 不能为空")
		return getAPIKey() // 递归，让用户重新输入
	}

	// 保存到配置文件
	os.MkdirAll(home+"/.qiuqiu", 0700)
	os.WriteFile(keyFile, []byte(key), 0600)
	fmt.Println("✅ API Key 已保存到", keyFile)
	return key
}

func main() {
	// 获取 API Key：环境变量 → 本地文件 → 用户输入
	apiKey := getAPIKey()

	// 初始化 Agent：传入 API Key 和模型名
	a := agent.New(apiKey, "deepseek-chat")
	// 注册所有内置工具（read_file、write_file 等 7 个）
	a.RegisterTools(tool.AllBuiltInTools())

	ctx := context.Background()

	// 加载 MCP 插件（以 filesystem Server 为例）
	fmt.Println("🔌 正在加载 MCP 插件...")
	mcpClient, err := mcp.Connect("filesystem", "npx", "-y", "@modelcontextprotocol/server-filesystem", ".")
	if err != nil {
		// MCP 加载失败不影响基础功能
		fmt.Printf("⚠️  MCP 加载失败（不影响基础功能）：%v\n", err)
	} else {
		// 发现 MCP Server 暴露的工具
		tools, err := mcpClient.DiscoverTools()
		if err != nil {
			fmt.Printf("⚠️  MCP 工具发现失败：%v\n", err)
		} else {
			// 把 MCP 工具注册进 Agent（会自动加前缀避免命名冲突）
			a.RegisterMCPTools(mcpClient.Name, tools)
			fmt.Printf("🔌 已加载 %d 个 MCP 工具\n", len(tools))
		}
	}

	// 打印可用的 Skill（用户可以用 use 命令切换）
	skills := skill.AllBuiltInSkills()
	fmt.Println("\n🎯 可用 Skill（输入 use <技能名> 切换）：")
	for _, s := range skills {
		fmt.Printf("  - %s：%s\n", s.Name, s.Description)
	}

	fmt.Printf("\n🤖 球球 V5 已启动 | 当前模式：[%s]（输入 exit 退出，replay 重放历史）\n", a.CurrentSkillName())
	fmt.Println(strings.Repeat("─", 50))

	// 交互式对话循环：用户一行一行输入
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n🧑 你: ")
		if !scanner.Scan() {
			break // EOF（Ctrl+D / Ctrl+Z）
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue // 空输入跳过
		}
		if input == "exit" || input == "quit" {
			fmt.Println("👋 再见！")
			break
		}

		// 命令：replay — 回放当前 session 的事件日志
		if input == "replay" {
			events, err := a.EventStore().Load(a.SessionID())
			if err != nil {
				fmt.Printf("❌ 读取失败：%v\n", err)
			} else {
				fmt.Println(event.Replay(a.SessionID(), events))
			}
			continue
		}

		// 命令：use <skill_name> — 切换 Skill
		if strings.HasPrefix(input, "use ") {
			name := strings.TrimPrefix(input, "use ")
			matched := false
			for _, s := range skills {
				if s.Name == name {
					a.ApplySkill(s) // 切换 SystemPrompt 和工具白名单
					matched = true
					break
				}
			}
			if !matched {
				// 没找到对应的 Skill，列出所有可用的
				fmt.Printf("❌ 未找到 Skill：%s（可用：", name)
				for i, s := range skills {
					if i > 0 {
						fmt.Print("、")
					}
					fmt.Print(s.Name)
				}
				fmt.Println("）")
			}
			continue
		}

		// 正常流程：先规划（拆步骤），再执行
		fmt.Println("📋 正在拆解计划...")
		plan, err := a.GeneratePlan(ctx, input)
		if err != nil {
			fmt.Printf("❌ 规划失败：%v\n", err)
			continue
		}
		// 展示 LLM 拆出来的步骤列表
		fmt.Println("📋 计划如下：")
		for _, s := range plan.Steps {
			fmt.Printf("  %d. %s\n", s.ID, s.Desc)
		}

		// 让 LLM 自我审查 Plan 质量，有问题则自动修正
		fmt.Println("\n🔍 正在审查计划质量...")
		plan, _ = a.ReviewPlan(ctx, plan)

		fmt.Println("\n🚀 开始执行...")
		err = a.ExecutePlan(ctx, plan)
		if err != nil {
			fmt.Printf("❌ 执行失败：%v\n", err)
			continue
		}
		fmt.Println("\n🎉 全部完成！")

		// 截断消息历史（超过 100 条丢最早的）
		a.TrimMessages()
	}
}
