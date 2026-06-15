package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

// DeepSeek V4（deepseek-v4-flash / -pro）把旧的「chat vs reasoner」合并成请求参数，思考模式
// 默认开启：会先产一段 reasoning（reasoning_content）再给最终答案，配合 reasoning_effort 控制
// 思考强度（high / max）。本项目默认**开启 thinking + max**（最强推理），可经环境变量调整：
//
//	DEEPSEEK_THINKING=disabled        关闭思考（沿用旧 deepseek-chat 的非思考行为，省 token）
//	DEEPSEEK_REASONING_EFFORT=high    思考强度降为 high（默认 max）
//
// thinking 开关是请求体顶层的 {"thinking":{"type":"enabled|disabled"}}，但 go-openai 的类型化
// 请求没有对应字段（只有 reasoning_effort）。于是在 HTTP 传输层往 /chat/completions 的 JSON
// 请求体里注入该字段——流式与非流式都经此路径，一处生效。reasoning_effort 则用 go-openai 自带的
// 字段按请求设置（见 run.go / compact.go）。

// bodyFieldInjector 是 http.RoundTripper 装饰器：把 fields 合并进出站的 chat/completions
// JSON 请求体（不覆盖请求已显式设置的同名键），其余请求原样放行。
type bodyFieldInjector struct {
	base   http.RoundTripper
	fields map[string]any
}

func (t bodyFieldInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.Body == nil || len(t.fields) == 0 || !strings.HasSuffix(req.URL.Path, "/chat/completions") {
		return base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if json.Unmarshal(body, &m) == nil {
		for k, v := range t.fields {
			if _, exists := m[k]; !exists {
				m[k] = v
			}
		}
		if nb, err := json.Marshal(m); err == nil {
			body = nb
		}
	}

	// 克隆请求并重置 Body / ContentLength / GetBody，保证（含重试、重定向）能再次读取请求体。
	r2 := req.Clone(req.Context())
	r2.Body = io.NopCloser(bytes.NewReader(body))
	r2.ContentLength = int64(len(body))
	r2.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return base.RoundTrip(r2)
}

// newDeepSeekHTTPClient 返回一个显式设置 thinking 开关的 HTTP 客户端，用于构造 DeepSeek 客户端。
// thinking=true 注入 enabled、false 注入 disabled——显式声明，避免依赖服务端默认值的变化。
func newDeepSeekHTTPClient(thinking bool) *http.Client {
	mode := "enabled"
	if !thinking {
		mode = "disabled"
	}
	return &http.Client{
		Transport: bodyFieldInjector{
			base:   http.DefaultTransport,
			fields: map[string]any{"thinking": map[string]any{"type": mode}},
		},
	}
}

// deepSeekThinkingConfig 从环境变量读取思考模式配置。默认**开启 thinking + max 努力档**。
func deepSeekThinkingConfig() (thinking bool, effort string) {
	thinking, effort = true, "max"
	switch strings.TrimSpace(strings.ToLower(os.Getenv("DEEPSEEK_THINKING"))) {
	case "disabled", "off", "false", "0", "no":
		thinking = false
	}
	if v := strings.TrimSpace(strings.ToLower(os.Getenv("DEEPSEEK_REASONING_EFFORT"))); v != "" {
		effort = v
	}
	return thinking, effort
}
