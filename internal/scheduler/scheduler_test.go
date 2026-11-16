package scheduler

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type panicRunner struct{}

func (panicRunner) ID() string { return "panic_mod" }
func (panicRunner) Run(_ context.Context) (modules.RunResult, error) {
	panic("simulated module panic")
}
func (panicRunner) RunWithTask(_ context.Context, _ store.Task) (modules.RunResult, error) {
	panic("simulated task panic")
}

func TestSchedulerPanicRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "sched_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SaveTask(store.Task{
		ID:       "panic_task",
		Name:     "异常恢复测试",
		Type:     "panic_mod",
		BotID:    "butler",
		CronExpr: "0 8 * * *",
		Enabled:  false,
	})

	reg := modules.NewRegistry()
	reg.RegisterTaskRunner("panic_mod", panicRunner{})
	sched := NewWithRegistry(st, reg)

	_, runErr := sched.RunNow(context.Background(), "panic_task")
	if runErr == nil {
		t.Fatalf("expected error returned from recovered panic")
	}

	// Ensure running lock was released and second invocation recovers cleanly too
	_, runErr2 := sched.RunNow(context.Background(), "panic_task")
	if runErr2 == nil {
		t.Fatalf("expected error returned on second recovered run, not lock deadlock")
	}
}
