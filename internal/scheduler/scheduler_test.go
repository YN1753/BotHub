package scheduler

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type onceRunner struct {
	hits atomic.Int32
}

func (o *onceRunner) ID() string { return "once_mod" }
func (o *onceRunner) Run(_ context.Context) (modules.RunResult, error) {
	o.hits.Add(1)
	return modules.RunResult{Summary: "一次性提醒已送达"}, nil
}
func (o *onceRunner) RunWithTask(_ context.Context, _ store.Task) (modules.RunResult, error) {
	o.hits.Add(1)
	return modules.RunResult{Summary: "一次性提醒已送达"}, nil
}

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

func TestSchedulerOneShotRunAtAutoDisables(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "oneshot.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	targetAt := time.Now().In(modules.ShanghaiLocation()).Add(-2 * time.Second).Format("2006-01-02 15:04:05")
	if err := st.SaveTask(store.Task{
		ID:      "once_reminder",
		Name:    "喝水提醒",
		Type:    "once_mod",
		RunAt:   targetAt,
		Enabled: true,
		Params:  "{}",
	}); err != nil {
		t.Fatalf("save one-shot task: %v", err)
	}

	runner := &onceRunner{}
	reg := modules.NewRegistry()
	reg.RegisterTaskRunner("once_mod", runner)
	sched := NewWithRegistry(st, reg)
	if err := sched.Start(); err != nil {
		t.Fatalf("start scheduler: %v", err)
	}
	defer sched.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runner.hits.Load() >= 1 {
			latest, _ := st.GetTask("once_reminder")
			if latest != nil && !latest.Enabled {
				return
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	latest, _ := st.GetTask("once_reminder")
	t.Fatalf("expected one-shot reminder to run once and auto-disable; hits=%d task=%+v", runner.hits.Load(), latest)
}
