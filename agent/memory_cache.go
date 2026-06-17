package agent

import (
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// composeCachedSystemPrompt 在启动时一次性合成 system prompt（含磁盘上的长期记忆）。
// 会话内 remember/forget 通过 turn-tail 注入，不改动此稳定前缀（对齐 Reasonix Compose）。
func (a *Agent) composeCachedSystemPrompt() {
	base := a.sysPrompt
	if a.memoryStore == nil {
		a.cachedSystemPrompt = base
		return
	}
	block, err := a.memoryStore.RenderPromptBlock()
	if err != nil || block == "" {
		a.cachedSystemPrompt = base
		return
	}
	a.cachedSystemPrompt = strings.TrimRight(base, "\n") + "\n\n" + block
}

// queueMemoryNote 将会话内记忆变更排队，下一轮用户输入时以 turn-tail 注入。
func (a *Agent) queueMemoryNote(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	a.pendingMemory = append(a.pendingMemory, note)
}

// composeUserTurn 把排队的记忆变更 prepend 到用户输入，然后清空队列。
func (a *Agent) composeUserTurn(text string) string {
	notes := a.pendingMemory
	a.pendingMemory = nil
	if len(notes) == 0 {
		return text
	}
	var b strings.Builder
	b.WriteString("<memory-update>\n")
	b.WriteString("The following memory changes were just made and apply from now on:\n")
	for _, n := range notes {
		b.WriteString("- " + n + "\n")
	}
	b.WriteString("</memory-update>\n\n")
	b.WriteString(text)
	return b.String()
}

func (a *Agent) capturePrefixShape() PrefixShape {
	return CaptureShape(a.BuildSystemPrompt(), a.toolDefinitions(), a.session.RewriteVersion())
}

func (a *Agent) accumulateSessionCache(u openai.Usage) {
	a.sessCacheHit.Add(int64(cacheHitTokens(u)))
	a.sessCacheMiss.Add(int64(cacheMissTokens(u)))
}

// SessionCacheStats 返回会话累计缓存命中/未命中 token（压缩不 reset，对齐 Reasonix）。
func (a *Agent) SessionCacheStats() (hit, miss int64) {
	return a.sessCacheHit.Load(), a.sessCacheMiss.Load()
}

func (a *Agent) sessionHitRate() float64 {
	hit, miss := a.SessionCacheStats()
	denom := hit + miss
	if denom <= 0 {
		return 0
	}
	return float64(hit) / float64(denom)
}

func formatCacheDiagnostics(d CacheDiagnostics) string {
	if d.CacheHitTokens+d.CacheMissTokens == 0 {
		return ""
	}
	rate := 0.0
	if d.CacheHitTokens+d.CacheMissTokens > 0 {
		rate = float64(d.CacheHitTokens) / float64(d.CacheHitTokens+d.CacheMissTokens) * 100
	}
	if !d.PrefixChanged {
		return fmt.Sprintf("缓存 %.1f%%", rate)
	}
	return fmt.Sprintf("缓存 %.1f%%（前缀变化: %s）", rate, strings.Join(d.PrefixChangeReasons, "+"))
}
