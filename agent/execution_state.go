package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	ExecutionRunning = "running"
	ExecutionPaused  = "paused"

	PauseReasonUser     = "user"
	PauseReasonMaxSteps = "maxSteps"
)

// ErrPlanPaused 是协作式暂停的哨兵错误：调用方可把它当作正常的“已暂停”状态而非失败。
var ErrPlanPaused = fmt.Errorf("计划已暂停，可使用 /resume 继续")

// ExecutionState 是 Plan 执行状态快照：Session checkpoint 只保存消息历史，这里补齐
// “目标、步骤列表、下一步索引、暂停原因”等执行层状态，供 /resume 精准续跑。
type ExecutionState struct {
	Goal          string `json:"goal"`
	Steps         []Step `json:"steps"`
	NextStepIndex int    `json:"next_step_index"`
	Status        string `json:"status"`
	PauseReason   string `json:"pause_reason,omitempty"`
	UpdatedAt     int64  `json:"updated_at"`
}

func (a *Agent) SaveExecutionState(state ExecutionState) error {
	state.UpdatedAt = time.Now().Unix()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return a.store.SaveExecutionState(a.session.ID, data)
}

func (a *Agent) LoadExecutionState() (ExecutionState, bool) {
	data, err := a.store.LoadExecutionState(a.session.ID)
	if err != nil || len(data) == 0 {
		return ExecutionState{}, false
	}
	var state ExecutionState
	if err := json.Unmarshal(data, &state); err != nil {
		return ExecutionState{}, false
	}
	return state, true
}

func (a *Agent) ClearExecutionState() error {
	return a.store.ClearExecutionState(a.session.ID)
}

// SetMaxSteps 设置一次连续 Plan 执行最多完成多少个 step；<=0 表示不限制。
func (a *Agent) SetMaxSteps(n int) { a.maxSteps = n }

func (a *Agent) MaxSteps() int { return a.maxSteps }

// RequestPause 请求协作式暂停：当前 step 完成后停下并保存执行状态。
func (a *Agent) RequestPause() { a.pauseRequested = true }

// ResumePlan 从最近保存的 paused 执行状态继续执行。
func (a *Agent) ResumePlan(ctx context.Context) error {
	state, ok := a.LoadExecutionState()
	if !ok || state.Status != ExecutionPaused {
		return fmt.Errorf("没有可恢复的暂停计划")
	}
	if state.NextStepIndex >= len(state.Steps) {
		_ = a.ClearExecutionState()
		return fmt.Errorf("没有可恢复的暂停计划")
	}
	a.pauseRequested = false
	plan := &Plan{Goal: state.Goal, Steps: state.Steps}
	return a.executePlanFrom(ctx, plan, state.NextStepIndex)
}

func (a *Agent) savePausedPlan(plan *Plan, nextStepIndex int, reason string) error {
	state := ExecutionState{
		Goal:          plan.Goal,
		Steps:         append([]Step(nil), plan.Steps...),
		NextStepIndex: nextStepIndex,
		Status:        ExecutionPaused,
		PauseReason:   reason,
	}
	if err := a.SaveExecutionState(state); err != nil {
		return err
	}
	a.SaveCheckpoint()
	return nil
}
