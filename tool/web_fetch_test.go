package tool

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.com", "https://example.com"},        // 缺协议 → 补 https
		{"http://x.com", "http://x.com"},               // 已有 http 原样
		{"https://x.com", "https://x.com"},             // 已有 https 原样
		{"  example.com  ", "https://example.com"},     // 去空白后补协议
		{"", ""},                                       // 空串
		{"   ", ""},                                    // 纯空白 → 空
	}
	for _, c := range cases {
		if got := normalizeURL(c.in); got != c.want {
			t.Errorf("normalizeURL(%q)=%q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestFetchURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "hello body")
	}))
	defer srv.Close()

	result := fetchURL(srv.URL)
	if !strings.Contains(result, "hello body") {
		t.Fatalf("应包含正文，实际：%s", result)
	}
	if !strings.Contains(result, "200") {
		t.Fatalf("应包含状态码 200，实际：%s", result)
	}
}

func TestFetchURL_EmptyURL(t *testing.T) {
	if result := fetchURL("  "); !strings.Contains(result, "url 不能为空") {
		t.Fatalf("空 URL 应提示，实际：%s", result)
	}
}

func TestFetchURL_Truncates(t *testing.T) {
	long := strings.Repeat("A", webFetchMaxOutput+500)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, long)
	}))
	defer srv.Close()

	result := fetchURL(srv.URL)
	if !strings.Contains(result, "已截断") {
		t.Fatalf("超长正文应被截断并提示，实际尾部：%s", result[max(0, len(result)-80):])
	}
	// 截断后保留的 A 不应超过上限（正文部分）。
	if strings.Count(result, "A") > webFetchMaxOutput {
		t.Fatalf("截断后 A 的数量应 ≤ %d，实际 %d", webFetchMaxOutput, strings.Count(result, "A"))
	}
}

func TestFetchURL_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "page not found")
	}))
	defer srv.Close()

	result := fetchURL(srv.URL)
	if !strings.Contains(result, "404") {
		t.Fatalf("应包含 404 状态，实际：%s", result)
	}
	if !strings.Contains(result, "page not found") {
		t.Fatalf("非 2xx 也应返回正文，实际：%s", result)
	}
}

func TestFetchURL_ConnError(t *testing.T) {
	// 起一个 server 拿到地址后立刻关闭，对它发请求必然连接失败。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	if result := fetchURL(url); !strings.Contains(result, "抓取失败") {
		t.Fatalf("连接失败应提示「抓取失败」，实际：%s", result)
	}
}
