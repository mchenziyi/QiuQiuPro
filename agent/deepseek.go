package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// DeepSeek V4（deepseek-v4-flash / -pro）默认开启 thinking 模式：会先产出一段 reasoning，
// 多花输出 token、更慢，且返回 reasoning_content。本项目沿用旧 deepseek-chat 的「非思考」行为、
// 并自带 CoT 提示词（system.xml），不需要原生思考链，故迁移到 V4 后要显式关闭 thinking。
//
// 关闭开关是请求体顶层的 {"thinking":{"type":"disabled"}}，但 go-openai 的类型化请求结构没有
// 对应字段（只有 reasoning_effort，无法用于关闭）。于是在 HTTP 传输层往 /chat/completions 的
// JSON 请求体里注入该字段——流式与非流式都经此路径，一处生效。

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

// newDeepSeekHTTPClient 返回一个会关闭 V4 thinking 模式的 HTTP 客户端，用于构造 DeepSeek 客户端，
// 让迁移到 deepseek-v4-flash 后行为与成本与旧 deepseek-chat（非思考）保持一致。
func newDeepSeekHTTPClient() *http.Client {
	return &http.Client{
		Transport: bodyFieldInjector{
			base:   http.DefaultTransport,
			fields: map[string]any{"thinking": map[string]any{"type": "disabled"}},
		},
	}
}
