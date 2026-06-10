// 球球 Agent — 主入口（V5 支持 Skill 切换）
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

func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Println("请设置环境变量 DEEPSEEK_API_KEY")
		return
	}

	// 初始化 Agent
	a := agent.New(apiKey, "deepseek-chat")
	a.RegisterTools(tool.AllBuiltInTools())

	ctx := context.Background()

	// 加载 MCP 插件
	fmt.Println("🔌 正在加载 MCP 插件...")
	mcpClient, err := mcp.Connect("filesystem", "npx", "-y", "@modelcontextprotocol/server-filesystem", ".")
	if err != nil {
		fmt.Printf("⚠️  MCP 加载失败（不影响基础功能）：%v\n", err)
	} else {
		tools, err := mcpClient.DiscoverTools()
		if err != nil {
			fmt.Printf("⚠️  MCP 工具发现失败：%v\n", err)
		} else {
			a.RegisterMCPTools(mcpClient.Name, tools)
			fmt.Printf("🔌 已加载 %d 个 MCP 工具\n", len(tools))
		}
	}

	// 打印可用 Skill
	skills := skill.AllBuiltInSkills()
	fmt.Println("\n🎯 可用 Skill（输入 use <技能名> 切换）：")
	for _, s := range skills {
		fmt.Printf("  - %s：%s\n", s.Name, s.Description)
	}

	fmt.Printf("\n🤖 球球 V5（Skill 体系）已启动 | 当前模式：[%s]（输入 exit 退出，replay 重放历史）\n", a.CurrentSkillName())
	fmt.Println(strings.Repeat("─", 50))

	// 交互式对话循环
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

		// replay 命令
		if input == "replay" {
			events, err := a.EventStore().Load(a.SessionID())
			if err != nil {
				fmt.Printf("❌ 读取失败：%v\n", err)
			} else {
				fmt.Println(event.Replay(a.SessionID(), events))
			}
			continue
		}

		// use 命令：切换 Skill
		if strings.HasPrefix(input, "use ") {
			name := strings.TrimPrefix(input, "use ")
			matched := false
			for _, s := range skills {
				if s.Name == name {
					a.ApplySkill(s)
					matched = true
					break
				}
			}
			if !matched {
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

		// 正常流程：规划 → 执行
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
