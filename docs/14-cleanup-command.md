# 14 — /cleanup 命令（熵管理：清理垃圾文件）

## 为什么要做

项目跑久了会沉淀一堆垃圾：`.DS_Store`、编辑器 swap（`.swp`）、临时/备份文件（`.tmp` / `.bak`
/ `.orig` / `~`）。手动找着删很烦。`/cleanup` 一键扫描、列出、确认后删除，给项目「降熵」。

（TODO-reasonix.md 功能清单 #4，第一梯队、★★☆☆☆。）

## 做了什么

### 1. 新增可测的 cleanup 包

`cleanup/cleanup.go`，逻辑与交互分离、纯函数为主，便于单测：

- `IsJunk(name)`：纯函数判定。精确名（`.DS_Store` / `Thumbs.db` / `desktop.ini`）+ 后缀
  （`.tmp` / `.temp` / `.bak` / `.orig` / `.swp` / `.swo` / `~`）。
- `Scan(root)`：递归扫描。**关键安全约束——绝不进入 `.git`**（`filepath.SkipDir`），
  避免误删对象/引用文件破坏仓库。
- `Delete(files)`：逐个删除，返回成功数与逐项错误（单个失败不影响其余）。
- `FormatList` / `HumanSize`：列表展示 + 人类可读大小。

### 2. 注册 /cleanup 命令

`main.go` 增加命令：`/cleanup [目录]`（默认当前目录）。流程：

```
扫描 → 没有则提示「✨ 没有垃圾文件」
     → 有则列出（含每个大小 + 合计）→ 「确认全部删除？[Y/n]」→ 删除并汇报
```

### 3. 复用统一确认逻辑

确认走 `a.Confirm()` —— 新增的导出包装，内部即 P1 已测的 `confirm()`（空/非 n 视为 Yes、
EOF 视为取消）。命令与高危工具确认共用同一套 stdin 收口逻辑，不再各写一份。

## 改动文件

| 文件 | 改动 |
|------|------|
| `cleanup/cleanup.go` | 新增包：`IsJunk` / `Scan` / `Delete` / `FormatList` / `HumanSize` |
| `cleanup/cleanup_test.go` | 新增 6 个单测 |
| `agent/input.go` | 新增导出 `Confirm()`（包装 `confirm()`，供命令复用）|
| `main.go` | 注册 `/cleanup` 命令 |

## 测试

| 用例 | 验证 |
|------|------|
| `TestIsJunk` | 各类垃圾判 true、正常文件（含 `tmp.go`）判 false |
| `TestScan_FindsJunkSkipsGit` | 找到嵌套垃圾，且 **`.git` 下的一律跳过** |
| `TestDelete` / `TestDelete_PartialError` | 删除成功计数 / 部分失败仍删其余并报错 |
| `TestHumanSize` | B / KB / MB 边界 |
| `TestFormatList` | 逐项 + 合计行 |

## 效果

- `/cleanup` 一键清理常见垃圾文件，删除前必须确认，且**永不碰 `.git`**。
- 核心逻辑全在可测的 `cleanup` 包里，交互层只做编排。
- `go build` / `go test ./agent/ ./tool/ ./cleanup/` / `go vet` 全绿；新增 cleanup 包 6 个单测。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#4 熵管理 /cleanup 命令**
> 难度：★★☆☆☆
