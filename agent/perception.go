package agent

import (
	"context"
	"strings"
	"unicode/utf8"
)

// DetectMode 判断用户输入走 ask 还是 plan。
// 策略：默认走 plan（安全侧），只有明显是闲聊/提问才走 ask。
func (a *Agent) DetectMode(ctx context.Context, input string) (string, error) {
	if isConversational(input) {
		return "ask", nil
	}
	return "plan", nil
}

// isConversational 判断输入是否明显是闲聊/提问（不需要文件操作、代码修改）。
func isConversational(input string) bool {
	text := strings.TrimSpace(input)
	runes := utf8.RuneCountInString(text)

	// 非常短 → 很可能是闲聊
	if runes <= 10 {
		return true
	}

	// 提问模式：以常见提问词开头
	lower := strings.ToLower(text)
	questionStarters := []string{
		"什么是", "怎么", "如何", "为什么", "能不能", "可以",
		"what ", "why ", "how ", "can ", "is ",
		"帮我看看", "帮我查", "帮我分析", "帮我解释", "帮我看一下",
		"解释", "说明", "介绍一下",
	}
	for _, q := range questionStarters {
		if strings.HasPrefix(lower, q) {
			// 但如果是"帮我分析这个项目怎么重构" → plan
			if containsCodeAction(lower) {
				return false
			}
			return true
		}
	}

	// 含代码操作词 → 不是闲聊
	if containsCodeAction(lower) {
		return false
	}

	// 默认走 plan
	return false
}

func containsCodeAction(s string) bool {
	actions := []string{
		"写一个", "实现", "修改", "改一下", "重构", "重写", "添加", "新增",
		"创建", "新建", "删除", "删掉", "修复", "修一下", "优化", "升级",
		"安装", "集成", "部署", "配置", "设置", "接入", "迁移",
		"拆分", "合并", "生成", "搭建", "提交", "commit",
		"implement", "refactor", "rewrite", "fix", "add", "create",
		"delete", "install", "configure", "deploy", "migrate",
	}
	for _, act := range actions {
		if strings.Contains(s, act) {
			return true
		}
	}
	return false
}
