package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agentdemo/agent"
	"agentdemo/cleanup"
	"agentdemo/command"
	"agentdemo/event"
	"agentdemo/skill"
)

// registerCommands 注册所有内置命令到 registry。抽离自 main.go 以减小主文件体积。
func registerCommands(registry *command.Registry, a *agent.Agent, ctx context.Context, mgr *skill.Manager) {
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

	// /use — 切换 Skill（已单独抽取，在下方 registerUseCommand 中）
	registerUseCommand(registry, a, mgr)

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

	// /readonly [on|off] — 切换只读模式
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

	// /compact — 手动压缩上下文
	registry.Register(command.Command{
		Name: "compact", Description: "手动压缩上下文：把较早的对话折叠成摘要、保留近消息，主动重置前缀缓存。用法：/compact",
		Handler: func(args string) bool {
			a.Compact(ctx)
			return true
		},
	})

	// /usage — 查看 token 用量
	registry.Register(command.Command{
		Name: "usage", Description: "显示本次会话的 token 用量（输入 / 缓存命中 / 输出 / 思考 / 合计）与估算费用。用法：/usage",
		Handler: func(args string) bool {
			a.ReportUsage()
			return true
		},
	})

	// /memory — 查看长期记忆
	registry.Register(command.Command{
		Name: "memory", Description: "查看长期记忆（仅偏好/规则；写入由模型自主判断）。用法：/memory",
		Handler: func(args string) bool {
			fmt.Println(a.MemoryList())
			return true
		},
	})

	// /forget <id> — 删除长期记忆
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

	// /maxsteps [n] — 设置 step 上限
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

	// /pause — 协作式暂停
	registry.Register(command.Command{
		Name: "pause", Description: "请求协作式暂停：当前 step 完成后暂停，之后可 /resume 继续。用法：/pause",
		Handler: func(args string) bool {
			a.RequestPause()
			fmt.Println("  已请求暂停：当前 step 完成后会停下（若当前没有执行中的计划，则下次计划执行时生效）")
			return true
		},
	})

	// /resume — 从暂停状态继续
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
}

// registerUseCommand 注册 /use <skill>。
func registerUseCommand(registry *command.Registry, a *agent.Agent, mgr *skill.Manager) {
	registry.Register(command.Command{
		Name: "use", Description: "切换 Skill（专业模式）。用法：/use <技能名|default>",
		Handler: func(args string) bool {
			if args == "" {
				fmt.Println("❌ 请指定 Skill 名，如：/use architect 或 /use default")
				fmt.Println("可用 Skill：")
				fmt.Println("  - default：默认 Coding Agent（恢复全量工具）")
				for _, s := range mgr.List() {
					fmt.Printf("  - %s：%s\n", s.Name, s.Description)
				}
				return true
			}
			if args == "default" {
				a.ClearSkill()
				return true
			}
			if s, ok := mgr.Find(args); ok {
				a.ApplySkill(s)
				return true
			}
			fmt.Printf("❌ 未找到 Skill：%s（输入 /use 查看所有可用 Skill）\n", args)
			return true
		},
	})
}

// formatPlanProposal 格式化方案文本用于 CLI 展示。
func formatPlanProposal(research string, plan *agent.Plan) string {
	var b strings.Builder
	if strings.TrimSpace(research) != "" {
		b.WriteString("调研摘要：\n")
		b.WriteString(strings.TrimSpace(research))
		b.WriteString("\n\n")
	}
	b.WriteString("执行步骤：\n")
	for _, s := range plan.Steps {
		fmt.Fprintf(&b, "  %d. %s\n", s.ID, s.Desc)
	}
	return strings.TrimRight(b.String(), "\n")
}
