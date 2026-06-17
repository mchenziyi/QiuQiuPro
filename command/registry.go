// Package command 定义斜杠命令系统
// 斜杠命令以 / 开头，如 /help、/replay
// 命令的 Handler 不直接引用 Agent，而是通过闭包从注册处捕获依赖
package command

import (
	"fmt"
	"strings"
)

// Command 定义一个斜杠命令
type Command struct {
	Name        string                 // 命令名
	Description string                 // 帮助说明
	Handler     func(args string) bool // 处理函数，返回 true 表示已处理
}

// Registry 命令注册表
type Registry struct {
	commands []Command
}

// NewRegistry 创建命令注册表
func NewRegistry() *Registry {
	return &Registry{commands: make([]Command, 0)}
}

// Register 注册一个命令
func (r *Registry) Register(cmd Command) {
	r.commands = append(r.commands, cmd)
}

// List 返回所有命令列表
func (r *Registry) List() []Command {
	return r.commands
}

// Handle 根据输入查找并执行命令
// 输入以 / 开头则尝试匹配，匹配成功返回 true；否则返回 false
func (r *Registry) Handle(input string) bool {
	if !strings.HasPrefix(input, "/") {
		return false
	}

	rest := strings.TrimPrefix(input, "/")
	parts := strings.SplitN(rest, " ", 2)
	name := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	for _, c := range r.commands {
		if c.Name == name {
			return c.Handler(args)
		}
	}

	fmt.Printf("❌ 未知命令：/%s（输入 /help 查看所有命令）\n", name)
	return true
}
