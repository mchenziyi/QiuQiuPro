# 27 — 风暴断路器（Storm Breaker）

## 为什么要做

之前 `Run()` 用硬上限 `maxLoops=15` 控制循环轮数。问题：
1. **上限太低** — 分析类任务可能超过 15 轮（查资料、读文件、交叉验证），到上限就崩
2. **上限太高** — 如果模型陷入死循环（同工具同错误反复重试），15 轮白白浪费 token

参照 Reasonix 的 `stormBreaker`，改为**无硬上限 + 风暴检测**：正常的任务让它自然结束，死循环自动打断。

## 做了什么

### 1. 去掉 maxLoops 硬限制

```go
// 之前
maxLoops := 15
for i := 0; i < maxLoops; i++ { ... }

// 之后
for { ... }  // 自然终止或风暴触发
```

### 2. 新增风暴断路器

`dispatchAndDetect` 方法在每轮工具执行后检查：

```
全部调用都失败 → 构建签名 (toolName, errorPrefix)
签名与上轮相同 → 累计 count
count >= 3     → 注入 [loop guard] 指令到结果 → 停止本轮
任何调用成功   → 重置计数器
```

### 3. 签名策略

只取 `(工具名, 错误前缀 80 字)`，不看参数。因为死循环时模型往往换参数措辞但结果一样——如果看参数就检测不到（参照 Reasonix 注释中的真实案例：被截断的 tool-call args）。

## 改动文件

| 文件 | 改动 |
|------|------|
| `agent/run.go` | 重写 Run 循环 + 新增 dispatchAndDetect/checkStorm/stormSignature |
| `agent/agent.go` | 新增 stormSig/stormCount 字段 |
| `agent/perception.go` | 分类器 prompt 更保守（不确定时偏向 plan） |

## 效果

- 正常 Q&A 自然结束，不会因轮数上限中途崩掉
- 模型陷入死循环时 3 轮后自动打断，不再烧 token
- 与 Reasonix 的 stormBreaker 同等语义
