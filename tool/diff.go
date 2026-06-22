package tool

import (
	"strings"
)

// DiffLine 表示 diff 中的一行。
type DiffLine struct {
	Op   string `json:"op"`   // "ctx" | "add" | "del"
	Text string `json:"text"` // 行内容（不含前缀 + / - / ）
}

// DiffHunk 表示一段连续的变更。
type DiffHunk struct {
	OldStart int        `json:"old_start"`
	NewStart int        `json:"new_start"`
	Lines    []DiffLine `json:"lines"`
}

// DiffResult 表示一次写操作产生的完整 diff。
type DiffResult struct {
	Path  string     `json:"path"`
	Hunks []DiffHunk `json:"hunks"`
}

// ComputeLineDiff 计算两个字符串的逐行 diff，返回 hunks。
func ComputeLineDiff(before, after, path string, context int) DiffResult {
	oldLines := splitLines(before)
	newLines := splitLines(after)
	edits := diffLines(oldLines, newLines)

	// 标记哪些位置属于 diff 区（非 ctx）
	inDiff := make([]bool, len(edits))
	hasChange := false
	for i, e := range edits {
		if e.Op != "ctx" {
			inDiff[i] = true
			hasChange = true
		}
	}
	if !hasChange {
		return DiffResult{Path: path}
	}

	// 将 diff 行分组为 hunks，含 context 行
	var hunks []DiffHunk
	i := 0
	for i < len(edits) {
		// 跳过纯 ctx 段
		if edits[i].Op == "ctx" {
			i++
			continue
		}

		// hunk 起始：向前包含 context 行
		start := i - context
		if start < 0 {
			start = 0
		}

		// hunk 结束：向后包含 context 行
		end := i
		for end < len(edits) {
			if edits[end].Op != "ctx" {
				// 找到非 ctx 行，向后扩展 context
				extendEnd := end
				for extendEnd < len(edits) && extendEnd <= end+context {
					extendEnd++
				}
				end = extendEnd
				i = end
				continue
			}
			end++
			i = end
			break
		}
		if end > len(edits) {
			end = len(edits)
		}

		// 计算 old/new 行号
		oldLine := 0
		newLine := 0
		for j := 0; j < end; j++ {
			if j >= start && (edits[j].Op == "ctx" || edits[j].Op == "del") {
				oldLine++
			}
			if j >= start && (edits[j].Op == "ctx" || edits[j].Op == "add") {
				newLine++
			}
		}

		var lines []DiffLine
		for j := start; j < end; j++ {
			lines = append(lines, edits[j])
		}

		if len(lines) > 0 {
			hunks = append(hunks, DiffHunk{
				OldStart: oldLine,
				NewStart: newLine,
				Lines:    lines,
			})
		}
	}

	return DiffResult{Path: path, Hunks: hunks}
}

// diffLines 对两行数组做 LCS 推导，标注每个位置是 ctx / add / del。
func diffLines(oldLines, newLines []string) []DiffLine {
	m, n := len(oldLines), len(newLines)
	// 计算 LCS 长度表
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	// 回溯构建 diff
	var result []DiffLine
	i, j := m, n
	var reverse []DiffLine
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			reverse = append(reverse, DiffLine{Op: "ctx", Text: oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			reverse = append(reverse, DiffLine{Op: "add", Text: newLines[j-1]})
			j--
		} else if i > 0 {
			reverse = append(reverse, DiffLine{Op: "del", Text: oldLines[i-1]})
			i--
		}
	}

	// 反转
	for k := len(reverse) - 1; k >= 0; k-- {
		result = append(result, reverse[k])
	}
	return result
}

// splitLines 将字符串按换行符分割，保留每行内容（不含换行符）。
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}
