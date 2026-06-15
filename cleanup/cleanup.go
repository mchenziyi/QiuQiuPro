// Package cleanup 扫描并清理常见垃圾文件（供 /cleanup 命令使用）。
package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// junkExact 按完整文件名精确匹配的垃圾文件（OS / 编辑器残留）。
var junkExact = map[string]bool{
	".DS_Store":   true,
	"Thumbs.db":   true,
	"desktop.ini": true,
}

// junkSuffix 按后缀匹配的垃圾文件（临时 / 备份 / 编辑器 swap）。
var junkSuffix = []string{
	".tmp", ".temp", ".bak", ".orig", ".swp", ".swo", "~",
}

// JunkFile 一个待清理的垃圾文件。
type JunkFile struct {
	Path string
	Size int64
}

// IsJunk 判断文件名是否为垃圾文件（纯函数，便于测试）。
func IsJunk(name string) bool {
	if junkExact[name] {
		return true
	}
	for _, suf := range junkSuffix {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	return false
}

// Scan 递归扫描 root 下的垃圾文件。
// 关键安全约束：绝不进入 .git 目录，避免误删破坏仓库。
func Scan(root string) ([]JunkFile, error) {
	var junk []JunkFile
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的项，不中断整次扫描
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if IsJunk(info.Name()) {
			junk = append(junk, JunkFile{Path: path, Size: info.Size()})
		}
		return nil
	})
	return junk, err
}

// Delete 删除给定的垃圾文件，返回成功删除数与逐个失败的错误。
func Delete(files []JunkFile) (deleted int, errs []error) {
	for _, f := range files {
		if err := os.Remove(f.Path); err != nil {
			errs = append(errs, fmt.Errorf("删除 %s 失败：%w", f.Path, err))
			continue
		}
		deleted++
	}
	return deleted, errs
}

// FormatList 把待清理列表整理成可读文本（含单文件大小与合计）。
func FormatList(files []JunkFile) string {
	var b strings.Builder
	var total int64
	for _, f := range files {
		fmt.Fprintf(&b, "  - %s（%s）\n", f.Path, HumanSize(f.Size))
		total += f.Size
	}
	fmt.Fprintf(&b, "  合计 %d 个文件，%s\n", len(files), HumanSize(total))
	return b.String()
}

// HumanSize 把字节数格式化为人类可读的大小。
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
