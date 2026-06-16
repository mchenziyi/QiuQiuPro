# 34 — ask 工具（模型列选项让用户选）

## 为什么要做

Reasonix system prompt 最频繁的指令是"不确定 → 调 ask 工具列 2-4 个选项让用户选"。没有 ask，模型只能猜一个方案或空泛地问"你想要哪种？"。

## 做了什么

### NewAskTool()

```
参数：
  question:     简短问题描述
  options:      2-4 个 {label, description}
  allowMultiple:是否允许多选

执行流程：
  模型调 ask → 展示问题+选项 → 用户输入序号 → 返回给模型
```

### 注册

`main.go` 在启动时注册：

```go
a.RegisterTool(a.NewAskTool())
```

### system prompt 引用

在 prompt/default/system.xml 的"不确定"场景中描述：
```
如果请求有多个合理方案——不要默默选一种，调 ask 工具列 2-4 个选项让用户决定。
```

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/ask.go` | 新增：NewAskTool |
| `main.go` | 注册 ask 工具 |
| `prompt/default/system.xml` | 更新引用（可选） |

## 效果

- 模型现在在不确定时能主动问用户选哪个方案
- 用户体验从"猜错了等纠正"变成"先问清楚了再做"
