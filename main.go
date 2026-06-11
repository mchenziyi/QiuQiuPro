// 球球 Agent — 主入口
// 功能：初始化 Agent、加载 MCP 配置、加载 Skill、启动交互式对话
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	
	"strings"

	"agentdemo/agent"
	"agentdemo/event"
	"agentdemo/mcp"
	"agentdemo/skill"
	"agentdemo/tool"
)

// MCPConfig 对应 .qiuqiu/mcp_servers.json 中的一条配置
type MCPConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func getAPIKey() string {
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
	reader := bufio.NewReader(os.Stdin)
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)
	if key == "" {
		fmt.Println("API Key 不能为空")
		return getAPIKey()
	}
	os.MkdirAll(home+"/.qiuqiu", 0700)
	os.WriteFile(keyFile, []byte(key), 0600)
	fmt.Println("✅ API Key 已保存到", keyFile)
	return key
}

// loadMCPConfigs 从配置文件读取 MCP Server 列表
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
	apiKey := getAPIKey()

	// 初始化 Agent
	a := agent.New(apiKey, "deepseek-chat")
	a.RegisterTools(tool.AllBuiltInTools())

	ctx := context.Background()

	// ========== 加载 MCP 插件（从配置文件）==========
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

	// ========== 加载 Skill（内置 + 外部）==========
	home, _ := os.UserHomeDir()
	skillsDir := home + "/.qiuqiu/skills"

	allSkills := skill.AllBuiltInSkills()
	externalSkills, _ := skill.LoadFromDir(skillsDir)
	allSkills = append(allSkills, externalSkills...)

	fmt.Println("\n🎯 可用 Skill（输入 use <技能名> 切换）：")
	for _, s := range allSkills {
		origin := "内置"
		found := false
		for _, bs := range skill.AllBuiltInSkills() {
			if bs.Name == s.Name {
				found = true
				break
			}
		}
		if !found {
			origin = "外部"
		}
		fmt.Printf("  - %s [%s]：%s\n", s.Name, origin, s.Description)
	}

	fmt.Printf("\n🤖 球球 Agent 已启动 | 当前模式：[%s]（输入 exit 退出，replay 重放历史）\n", a.CurrentSkillName())
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

		if input == "replay" {
			events, err := a.EventStore().Load(a.SessionID())
			if err != nil {
				fmt.Printf("❌ 读取失败：%v\n", err)
			} else {
				fmt.Println(event.Replay(a.SessionID(), events))
			}
			continue
		}

		if strings.HasPrefix(input, "use ") {
			name := strings.TrimPrefix(input, "use ")
			matched := false
			for _, s := range allSkills {
				if s.Name == name {
					a.ApplySkill(s)
					matched = true
					break
				}
			}
			if !matched {
				fmt.Printf("❌ 未找到 Skill：%s（可用：", name)
				for i, s := range allSkills {
					if i > 0 {
						fmt.Print("、")
					}
					fmt.Print(s.Name)
				}
				fmt.Println("）")
			}
			continue
		}

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
