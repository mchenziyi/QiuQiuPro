package tool

import (
	"unicode/utf8"
)

// --------------- 参数构建辅助 ---------------

type paramBuilder struct {
	props    map[string]any
	required []string
}

func objParams(props ...map[string]any) *paramBuilder {
	merged := map[string]any{}
	for _, p := range props {
		for k, v := range p {
			merged[k] = v
		}
	}
	return &paramBuilder{props: merged}
}

func (b *paramBuilder) Required(names ...string) map[string]any {
	b.required = names
	return b.Build()
}

func (b *paramBuilder) Build() map[string]any {
	m := map[string]any{"type": "object", "properties": b.props}
	if len(b.required) > 0 {
		m["required"] = b.required
	}
	return m
}

func prop(name, typ, desc string) map[string]any {
	p := map[string]any{"type": typ}
	if desc != "" {
		p["description"] = desc
	}
	return map[string]any{name: p}
}

// safeTruncate 按字节截断字符串，同时保证不破坏 UTF-8 多字节字符边界。
func safeTruncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	for maxBytes > 0 && !utf8.RuneStart(b[maxBytes]) {
		maxBytes--
	}
	return string(b[:maxBytes]) + "…(截断)"
}
