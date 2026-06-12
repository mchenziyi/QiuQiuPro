package agent

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// PromptVars 所有 prompt 共用的模板变量
type PromptVars struct {
	Tools          string // 可用工具列表（GeneratePlan）
	Goal           string // 目标
	Steps          string // 步骤列表（ReviewPlan）
	DoneSteps      string // 已完成步骤（Reflect/RePlan）
	RemainingSteps string // 未完成步骤（RePlan）
	FailedStepID   int    // 失败步骤 ID（Reflect）
	FailedStepDesc string // 失败步骤描述（Reflect）
	Error          string // 错误信息（Reflect）
	Reflection     string // 反思内容（RePlan）
}

// LoadPrompt 从 XML 文件加载 prompt，执行模板替换
// XML 格式：<prompt>内容 模板语法用 {{.FieldName}}</prompt>
func LoadPrompt(path string, vars PromptVars) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 prompt 文件失败: %w", err)
	}

	// 提取 <prompt> 标签内的内容
	text := extractPromptTag(string(data))
	if text == "" {
		return "", fmt.Errorf("prompt 文件格式错误，缺少 <prompt> 标签: %s", path)
	}

	tmpl, err := template.New("prompt").Parse(text)
	if err != nil {
		return "", fmt.Errorf("解析 prompt 模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("执行 prompt 模板失败: %w", err)
	}

	return buf.String(), nil
}

// LoadRawPrompt 从 XML 文件加载纯文本 prompt（无模板变量）
func LoadRawPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 prompt 文件失败: %w", err)
	}
	text := extractPromptTag(string(data))
	if text == "" {
		return "", fmt.Errorf("prompt 文件格式错误，缺少 <prompt> 标签: %s", path)
	}
	return text, nil
}

// extractPromptTag 提取 <prompt> 和 </prompt> 之间的内容
func extractPromptTag(s string) string {
	start := bytes.Index([]byte(s), []byte("<prompt>"))
	if start < 0 {
		return ""
	}
	start += len("<prompt>")
	end := bytes.Index([]byte(s[start:]), []byte("</prompt>"))
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}
