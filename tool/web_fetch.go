package tool

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	webFetchTimeout   = 15 * time.Second // 整次请求超时，避免卡死事件循环
	webFetchMaxBytes  = 1 << 20          // 最多读 1MB，防止超大响应撑爆内存
	webFetchMaxOutput = 16000            // 返回给 LLM 的字符上限，超出截断
)

// NewWebFetchTool 通过 HTTP GET 抓取一个 URL 的内容
// LLM 使用场景："查一下这个库的文档"、"看看这个 API 返回什么"、"读一下这篇文章"
func NewWebFetchTool() Tool {
	return Tool{
		Name:        "web_fetch",
		Description: "通过 HTTP GET 抓取一个 URL 的内容（网页 / API / 在线文档），返回响应正文文本。用于查文档、查 API、查资料",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "要抓取的完整 URL，如 https://example.com。省略协议时默认按 https 处理",
				},
			},
			"required": []string{"url"},
		},
		Execute: func(args string) string {
			// 必须带 json tag：参数名是 url，无 tag 时 Go 也能大小写不敏感匹配到 URL，
			// 但显式写出更稳妥、也和其它工具保持一致。
			var p struct {
				URL string `json:"url"`
			}
			json.Unmarshal([]byte(args), &p)
			return fetchURL(p.URL)
		},
	}
}

// normalizeURL 清洗并补全 URL：去空白、缺协议时补 https://。返回 "" 表示输入为空。
func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return "https://" + raw
	}
	return raw
}

// fetchURL 执行 GET 并把响应整理成 LLM 友好的文本。
// 抽成独立函数（不依赖闭包），方便用 httptest 单测。
func fetchURL(rawURL string) string {
	url := normalizeURL(rawURL)
	if url == "" {
		return "抓取失败：url 不能为空"
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Sprintf("抓取失败：URL 无效：%v", err)
	}
	// 部分站点会拒绝默认的 Go UA，伪装成常见浏览器更稳。
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; QiuQiuAgent/1.0)")

	client := &http.Client{Timeout: webFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("抓取失败：请求出错：%v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBytes))
	if err != nil {
		return fmt.Sprintf("抓取失败：读取响应出错：%v", err)
	}

	content := string(body)
	note := ""
	if runes := []rune(content); len(runes) > webFetchMaxOutput {
		content = string(runes[:webFetchMaxOutput])
		note = fmt.Sprintf("\n…（内容过长已截断，仅显示前 %d 个字符）", webFetchMaxOutput)
	}

	return fmt.Sprintf("GET %s\n状态：%s\nContent-Type：%s\n正文：\n%s%s",
		url, resp.Status, resp.Header.Get("Content-Type"), content, note)
}
