package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// --------------- 网络 ---------------

func stripHTML(s string) string {
	s = regexp.MustCompile(`(?is)<(?:script|style)[^>]*>.*?</(?:script|style)>`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<!--.*?-->`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, "")
	repl := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ")
	s = repl.Replace(s)
	return regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`).ReplaceAllString(s, "\n\n")
}

func NewWebFetchTool() Tool {
	return Tool{
		Name: "web_fetch", Description: "HTTP GET 抓取 URL", ReadOnly: true,
		Parameters: objParams(
			prop("url", "string", ""),
		).Required("url"),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var p struct{ URL string }
			if err := json.Unmarshal(args, &p); err != nil {
				return "", fmt.Errorf("参数解析失败：%v", err)
			}
			if p.URL == "" {
				return "", fmt.Errorf("url required")
			}
			client := &http.Client{Timeout: 15 * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", p.URL, nil)
			if err != nil {
				return "", fmt.Errorf("request: %v", err)
			}
			req.Header.Set("User-Agent", "QiuQiuPro/1.0")
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("fetch: %v", err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if err != nil {
				return "", fmt.Errorf("read: %v", err)
			}
			out := string(body)
			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			if strings.Contains(ct, "text/html") || strings.Contains(out, "<!doctype") || strings.Contains(out, "<html") {
				out = stripHTML(out)
			}
			if len(out) > 16000 {
				out = safeTruncate(out, 16000)
			}
			return fmt.Sprintf("HTTP %s\n%s", resp.Status, strings.TrimSpace(out)), nil
		},
	}
}
