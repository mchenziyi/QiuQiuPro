package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// capRT 是一个假的下游 RoundTripper：记录它实际收到的请求体与路径，不发真实网络请求。
type capRT struct {
	body []byte
	path string
}

func (c *capRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		c.body, _ = io.ReadAll(req.Body)
	}
	c.path = req.URL.Path
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
}

// sendBody 经注入器发一条请求，返回下游真正收到的 JSON body 解析结果。
func sendBody(t *testing.T, fields map[string]any, path, body string) map[string]any {
	t.Helper()
	cap := &capRT{}
	inj := bodyFieldInjector{base: cap, fields: fields}
	req, err := http.NewRequest("POST", "https://api.deepseek.com"+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := inj.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if cap.path != path {
		t.Fatalf("下游路径=%q，期望 %q", cap.path, path)
	}
	var m map[string]any
	if len(cap.body) > 0 {
		_ = json.Unmarshal(cap.body, &m)
	}
	return m
}

var disableThinking = map[string]any{"thinking": map[string]any{"type": "disabled"}}

// chat/completions 请求体应被注入 thinking=disabled。
func TestBodyFieldInjector_InjectsOnChat(t *testing.T) {
	m := sendBody(t, disableThinking, "/chat/completions", `{"model":"deepseek-v4-flash","messages":[]}`)
	th, ok := m["thinking"].(map[string]any)
	if !ok || th["type"] != "disabled" {
		t.Fatalf("应注入 thinking=disabled，实际 %+v", m["thinking"])
	}
	if m["model"] != "deepseek-v4-flash" {
		t.Fatalf("原有字段应保留，实际 model=%v", m["model"])
	}
}

// 非 chat 路径不应被改写。
func TestBodyFieldInjector_SkipsNonChat(t *testing.T) {
	m := sendBody(t, disableThinking, "/embeddings", `{"input":"x"}`)
	if _, exists := m["thinking"]; exists {
		t.Fatalf("非 chat 路径不应注入，实际 %+v", m)
	}
}

// 请求已显式设置 thinking 时不应被覆盖。
func TestBodyFieldInjector_DoesNotOverride(t *testing.T) {
	m := sendBody(t, disableThinking, "/chat/completions", `{"thinking":{"type":"enabled"}}`)
	th, _ := m["thinking"].(map[string]any)
	if th["type"] != "enabled" {
		t.Fatalf("不应覆盖已有 thinking，实际 %+v", th)
	}
}

// newDeepSeekHTTPClient 按入参注入 thinking enabled / disabled。
func TestNewDeepSeekHTTPClient_ThinkingToggle(t *testing.T) {
	for _, tc := range []struct {
		thinking bool
		want     string
	}{{true, "enabled"}, {false, "disabled"}} {
		c := newDeepSeekHTTPClient(tc.thinking)
		inj, ok := c.Transport.(bodyFieldInjector)
		if !ok {
			t.Fatalf("Transport 应为 bodyFieldInjector，实际 %T", c.Transport)
		}
		th, ok := inj.fields["thinking"].(map[string]any)
		if !ok || th["type"] != tc.want {
			t.Fatalf("thinking=%v 应注入 %q，实际 %+v", tc.thinking, tc.want, inj.fields["thinking"])
		}
	}
}

// 思考配置默认开启 + max，并能被环境变量覆盖。
func TestDeepSeekThinkingConfig(t *testing.T) {
	t.Setenv("DEEPSEEK_THINKING", "")
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "")
	if thinking, effort := deepSeekThinkingConfig(); !thinking || effort != "max" {
		t.Fatalf("默认应 thinking=true effort=max，实际 %v / %q", thinking, effort)
	}

	t.Setenv("DEEPSEEK_THINKING", "disabled")
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "high")
	if thinking, effort := deepSeekThinkingConfig(); thinking || effort != "high" {
		t.Fatalf("环境变量应覆盖为 thinking=false effort=high，实际 %v / %q", thinking, effort)
	}
}
