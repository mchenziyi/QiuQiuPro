# 20 — 拆分 agent.go（按职责分文件）

## 为什么要做

`agent.go` 攒成了一个 265 行的「杂物间」：结构体、构造、工具注册、Skill 切换、模式切换、
权限门控制、事件输出、检查点存档、子 Agent 派生……七八种不相干的职责挤在一个文件里。
找一个方法要上下翻半天，改一处容易碰到无关代码。纯结构整理，把它按职责摊开。

（TODO-reasonix.md 功能清单 #11，第三梯队、★★☆☆☆，重构类。）

## 做了什么

把 `agent.go` 里的方法按职责搬到同包的多个文件——**只移动、不改逻辑**：

| 文件 | 职责 | 内容 |
|------|------|------|
| `agent.go` | 核心骨架 | `Agent` 结构体、常量、`New`、零碎 accessor、`SpawnSubAgent` |
| `tools.go` | 工具 | `RegisterTool(s)` / `RegisterMCPTools` / `availableTools` / `toolDefinitions` + 风险分类 `IsHighRiskTool` / `isReadOnlyTool` |
| `skill.go` | 人格与模式 | `ApplySkill` / `CurrentSkillName` / `SetMode` / `CurrentMode` |
| `checkpoint.go` | 持久化 | `SaveCheckpoint` / `RestoreFromCheckpoint` |
| `gate.go`（已存在）| 权限门 | 追加 Agent 侧控制：`SetGate` / `GateName` / `SetReadOnly` / `IsReadOnly` |
| `sink.go`（已存在）| 事件输出 | 追加 Agent 侧发射：`SetSink` / `emit` / `debugf` / `noticef` / `emitToken` / `emitToolCall` / `emitToolResult` / `emitPrompt` |

`gate.go` / `sink.go` 本就持有各自领域的类型（Gate 实现、Event/Sink），把 Agent 上对应的
控制 / 发射方法挪过去——领域类型与其操作终于聚到一起，而不是类型在一处、方法在另一处。

`agent.go` 从 265 行瘦到约 90 行，开头加了一段「去哪找什么」的文件索引注释。

## 行为是否改变

零改变。这是纯粹的代码搬家：没有改任何函数体、签名或调用关系。Go 的包是「文件无关」的，
同包跨文件符号互相可见，所以拆分对编译与运行完全透明。删掉了 `agent.go` 中因方法外迁而
不再需要的 `encoding/json` 导入。

## 怎么验证

既有全部测试（agent / tool / cleanup）原样跑绿即是证明——它们覆盖的是行为，而行为未动：

- `go build ./...`、`go vet`、`go test ./agent/ -race` 全绿；
- `gofmt -l` 对新增 / 改写文件无告警。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/agent.go` | 瘦身：仅留结构体 / 常量 / New / accessor / SpawnSubAgent；去掉 json 导入 |
| `agent/tools.go` | 新增：工具注册 / 筛选 / 定义 / 风险分类 |
| `agent/skill.go` | 新增：Skill 套用与 plan/ask 模式切换 |
| `agent/checkpoint.go` | 新增：检查点存档 / 恢复 |
| `agent/gate.go` | 追加：Agent 侧权限门控制方法 |
| `agent/sink.go` | 追加：Agent 侧事件发射方法 |

## 效果

- 每个文件一个清晰职责，定位与改动都顺手；新增能力知道该往哪个文件放。
- 领域类型与其操作就近聚合（门、输出各成一体）。
- 为后续 #12（Provider 抽象）、#16（Hook）等留出干净的落点。

## 相关 TODO

> TODO-reasonix.md — 功能清单 **#11 拆分 agent.go**
> 难度：★★☆☆☆
