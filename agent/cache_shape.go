package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	openai "github.com/sashabaranov/go-openai"
)

// PrefixShape 哈希请求前缀中影响 DeepSeek 前缀缓存复用的部分。
type PrefixShape struct {
	SystemHash        string
	ToolsHash         string
	PrefixHash        string
	LogRewriteVersion int
	ToolSchemaTokens  int
}

// CacheDiagnostics 描述一轮请求的缓存命中与前缀变化原因。
type CacheDiagnostics struct {
	PrefixHash          string
	PrefixChanged       bool
	PrefixChangeReasons []string
	SystemHash          string
	ToolsHash           string
	LogRewriteVersion   int
	ToolSchemaTokens    int
	CacheHitTokens      int
	CacheMissTokens     int
}

func shortHash(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func normalizeToolDefs(tools []openai.Tool) []openai.Tool {
	out := append([]openai.Tool(nil), tools...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Function == nil || out[j].Function == nil {
			return i < j
		}
		return out[i].Function.Name < out[j].Function.Name
	})
	return out
}

// CaptureShape 快照当前可缓存前缀。
func CaptureShape(systemPrompt string, tools []openai.Tool, rewriteVersion int) PrefixShape {
	normalized := normalizeToolDefs(tools)
	toolsJSON, _ := json.Marshal(normalized)
	return PrefixShape{
		SystemHash: shortHash(systemPrompt),
		ToolsHash:  shortHash(string(toolsJSON)),
		PrefixHash: shortHash(map[string]any{
			"system": systemPrompt,
			"tools":  string(toolsJSON),
		}),
		LogRewriteVersion: rewriteVersion,
		ToolSchemaTokens:  len(toolsJSON) / 4,
	}
}

// CompareShape 对比两轮前缀，结合 usage 产出诊断。
func CompareShape(prev, cur PrefixShape, usage openai.Usage) CacheDiagnostics {
	reasons := []string{}
	if prev.SystemHash != "" && prev.SystemHash != cur.SystemHash {
		reasons = append(reasons, "system")
	}
	if prev.ToolsHash != "" && prev.ToolsHash != cur.ToolsHash {
		reasons = append(reasons, "tools")
	}
	if prev.LogRewriteVersion != cur.LogRewriteVersion {
		reasons = append(reasons, "log_rewrite")
	}
	hit := cacheHitTokens(usage)
	miss := cacheMissTokens(usage)
	return CacheDiagnostics{
		PrefixHash:          cur.PrefixHash,
		PrefixChanged:       len(reasons) > 0,
		PrefixChangeReasons: reasons,
		SystemHash:          cur.SystemHash,
		ToolsHash:           cur.ToolsHash,
		LogRewriteVersion:   cur.LogRewriteVersion,
		ToolSchemaTokens:    cur.ToolSchemaTokens,
		CacheHitTokens:      hit,
		CacheMissTokens:     miss,
	}
}

func cacheHitTokens(u openai.Usage) int {
	if d := u.PromptTokensDetails; d != nil {
		return d.CachedTokens
	}
	return 0
}

func cacheMissTokens(u openai.Usage) int {
	miss := u.PromptTokens - cacheHitTokens(u)
	if miss < 0 {
		return 0
	}
	return miss
}
