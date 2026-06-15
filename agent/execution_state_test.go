package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExecutionState_RoundTrip(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	state := ExecutionState{
		Goal:          "做两步",
		Steps:         []Step{{ID: 1, Desc: "第一步", Status: "done"}, {ID: 2, Desc: "第二步", Status: "pending"}},
		NextStepIndex: 1,
		Status:        ExecutionPaused,
		PauseReason:   "maxSteps",
	}

	if err := a.SaveExecutionState(state); err != nil {
		t.Fatal(err)
	}
	got, ok := a.LoadExecutionState()
	if !ok {
		t.Fatal("应能读取保存的执行状态")
	}
	if got.Goal != state.Goal || got.NextStepIndex != 1 || got.Status != ExecutionPaused || got.PauseReason != "maxSteps" {
		t.Fatalf("执行状态往返错误：%+v", got)
	}
	if len(got.Steps) != 2 || got.Steps[0].Status != "done" || got.Steps[1].Desc != "第二步" {
		t.Fatalf("步骤状态未完整保存：%+v", got.Steps)
	}

	if err := a.ClearExecutionState(); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.LoadExecutionState(); ok {
		t.Fatal("清理后不应再读到执行状态")
	}
}

func TestExecutePlan_PausesAtMaxStepsAndResumeContinues(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	a.Quiet = true
	a.SetMaxSteps(1)

	plan := &Plan{Goal: "三步任务", Steps: []Step{
		{ID: 1, Desc: "第一步", Status: "pending"},
		{ID: 2, Desc: "第二步", Status: "pending"},
		{ID: 3, Desc: "第三步", Status: "pending"},
	}}

	err := a.ExecutePlan(context.Background(), plan)
	if !errors.Is(err, ErrPlanPaused) {
		t.Fatalf("达到 maxSteps 应返回 ErrPlanPaused，实际 %v", err)
	}
	state, ok := a.LoadExecutionState()
	if !ok {
		t.Fatal("达到 maxSteps 后应保存暂停状态")
	}
	if state.NextStepIndex != 1 || state.Status != ExecutionPaused || state.PauseReason != PauseReasonMaxSteps {
		t.Fatalf("暂停状态错误：%+v", state)
	}
	if state.Steps[0].Status != "done" || state.Steps[1].Status != "pending" {
		t.Fatalf("步骤状态错误：%+v", state.Steps)
	}

	a.SetMaxSteps(0)
	if err := a.ResumePlan(context.Background()); err != nil {
		t.Fatalf("恢复执行失败：%v", err)
	}
	if _, ok := a.LoadExecutionState(); ok {
		t.Fatal("计划全部完成后应清理执行状态")
	}
	if a.Usage().Calls != 3 {
		t.Fatalf("三步应执行三次 Run，实际 usage calls=%d", a.Usage().Calls)
	}
}

func TestExecutePlan_CooperativePauseAfterCurrentStep(t *testing.T) {
	srv := newSSEServer(t, usageStreamChunks())
	a := newSSEAgent(t, srv)
	a.Quiet = true
	a.RequestPause()

	plan := &Plan{Goal: "两步任务", Steps: []Step{
		{ID: 1, Desc: "第一步", Status: "pending"},
		{ID: 2, Desc: "第二步", Status: "pending"},
	}}

	err := a.ExecutePlan(context.Background(), plan)
	if !errors.Is(err, ErrPlanPaused) {
		t.Fatalf("请求暂停后应在当前 step 完成后返回 ErrPlanPaused，实际 %v", err)
	}
	state, ok := a.LoadExecutionState()
	if !ok {
		t.Fatal("暂停后应保存执行状态")
	}
	if state.NextStepIndex != 1 || state.PauseReason != PauseReasonUser {
		t.Fatalf("应在第一步后暂停，实际 %+v", state)
	}
	if state.Steps[0].Status != "done" || state.Steps[1].Status != "pending" {
		t.Fatalf("协作式暂停应完成当前 step 后停下，实际 %+v", state.Steps)
	}
}

func TestResumePlan_NoPausedState(t *testing.T) {
	a := newDispatchAgent(t, AllowAllGate{})
	err := a.ResumePlan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "没有可恢复") {
		t.Fatalf("无暂停状态应返回友好错误，实际 %v", err)
	}
}
