package agent

// Gate（权限门）决定一次工具调用是放行、需确认、还是直接拒绝。
//
// 过去这套判断硬编码在 executeToolCall 里（「是高危就 fmt.Scanln 确认」），
// 既不可配、也无法表达「只读模式拒绝写操作」。抽成可插拔接口后：
//   - 默认 ConfirmHighRiskGate：等价于改造前的行为（高危确认，其余放行）；
//   - ReadOnlyGate：拒绝一切改动类工具，实现「只读模式」（TODO #6）；
//   - AllowAllGate：全放行，适合自动化 / 测试。

// Decision 是 Gate 对一次工具调用的裁决。
type Decision int

const (
	GateAllow   Decision = iota // 直接放行
	GateConfirm                 // 需要用户确认
	GateDeny                    // 直接拒绝
)

// Gate 是权限门的统一接口。
type Gate interface {
	// Check 给出裁决；reason 用于确认提示或拒绝时回灌给模型说明原因。
	Check(toolName, args string) (Decision, string)
	// Name 返回门的名字，用于展示当前权限模式。
	Name() string
}

// ConfirmHighRiskGate 默认门：高危工具需确认，其余放行。等价于改造前的行为。
type ConfirmHighRiskGate struct{}

func (ConfirmHighRiskGate) Name() string { return "confirm" }

func (ConfirmHighRiskGate) Check(toolName, _ string) (Decision, string) {
	if IsHighRiskTool(toolName) {
		return GateConfirm, "高危操作"
	}
	return GateAllow, ""
}

// ReadOnlyGate 只读门：拒绝一切会改动文件 / 系统 / 仓库的工具，只放行读类工具。
type ReadOnlyGate struct{}

func (ReadOnlyGate) Name() string { return "read-only" }

func (ReadOnlyGate) Check(toolName, _ string) (Decision, string) {
	// 非只读即写文件 / 编辑 / 执行命令 / 改动仓库（git_commit），一律拒绝。
	if !isReadOnlyTool(toolName) {
		return GateDeny, "只读模式禁止改动类操作"
	}
	return GateAllow, ""
}

// AllowAllGate 全放行门：不确认、不拦截。
type AllowAllGate struct{}

func (AllowAllGate) Name() string { return "allow-all" }

func (AllowAllGate) Check(string, string) (Decision, string) { return GateAllow, "" }
