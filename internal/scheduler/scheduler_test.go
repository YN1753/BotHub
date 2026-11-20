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

func (o *onceRunner) RunWithTask(_ context.Context, _ store.Task) (modules.RunResult, error) {
	o.hits.Add(1)
	return modules.RunResult{Summary: "一次性提醒已送达"}, nil
}

type panicRunner struct{}

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

	// Verify panic recovery finalized module_runs status as "error" (never dangling "running")
	runs, err := st.LatestModuleRuns()
	if err != nil || runs["panic_task"].Status != "error" {
		t.Fatalf("expected LatestModuleRuns[panic_task] finalized as error, got %+v (%v)", runs["panic_task"], err)
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

func TestSchedulerSkipsMalformedTaskWithoutBlockingHealthyTasks(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sched_bad_cron.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SaveTask(store.Task{
		ID:       "bad_cron",
		Name:     "损坏规则",
		Type:     "once_mod",
		CronExpr: "invalid cron expr",
		Enabled:  true,
		Params:   "{}",
	})
	_ = st.SaveTask(store.Task{
		ID:       "good_cron",
		Name:     "正常定时任务",
		Type:     "once_mod",
		CronExpr: "0 9 * * *",
		Enabled:  true,
		Params:   "{}",
	})

	runner := &onceRunner{}
	reg := modules.NewRegistry()
	reg.RegisterTaskRunner("once_mod", runner)
	sched := NewWithRegistry(st, reg)
	if err := sched.Start(); err != nil {
		t.Fatalf("expected scheduler start to skip bad cron instead of failing: %v", err)
	}
	defer sched.Stop()

	statuses, err := sched.Status()
	if err != nil {
		t.Fatalf("status error: %v", err)
	}
	foundNext := false
	for _, ts := range statuses {
		if ts.ID == "good_cron" && ts.NextRunAt != "" {
			foundNext = true
		}
	}
	if !foundNext {
		t.Fatalf("expected good_cron to have scheduled NextRunAt despite bad_cron, got %+v", statuses)
	}
}

type mockWatcherRunner struct {
	pushed  int
	summary string
}

func (m *mockWatcherRunner) RunWithTask(_ context.Context, _ store.Task) (modules.RunResult, error) {
	return modules.RunResult{
		Summary:     m.summary,
		ItemsPushed: m.pushed,
	}, nil
}

func TestSchedulerSilentWhenZeroItemsPushedAndLegacyCleanup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "silent_patrol.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	_ = st.SaveBot(store.Bot{
		ID:          "butler",
		Name:        "长庚执更",
		RoleTitle:   "全能生活与基础设施管家",
		LastSummary: "上次真实对话摘要",
	})
	_ = st.SaveTask(store.Task{
		ID:       "asset_watcher",
		Name:     "数字资产与会员守望",
		Type:     "asset_watcher",
		BotID:    "butler",
		CronExpr: "0 9 * * *",
		Enabled:  true,
	})

	runner := &mockWatcherRunner{
		pushed:  0,
		summary: "巡检 6 项资产（同步 0 项，发送 0 项临期强提醒）",
	}
	reg := modules.NewRegistry()
	reg.RegisterTaskRunner("asset_watcher", runner)
	sched := NewWithRegistry(st, reg)

	// 1. Zero-alert run MUST remain completely silent in IM chat while recording execution telemetry
	res, err := sched.RunNow(context.Background(), "asset_watcher")
	if err != nil || res.ItemsPushed != 0 {
		t.Fatalf("unexpected zero-alert run result: res=%+v err=%v", res, err)
	}
	msgs, _ := st.ListBotMessages("butler", 20)
	if len(msgs) != 0 {
		t.Fatalf("expected 0 IM messages when ItemsPushed == 0, got %d: %+v", len(msgs), msgs)
	}
	botAfterSilent, _ := st.GetBot("butler")
	if botAfterSilent == nil || botAfterSilent.UnreadCount != 0 || botAfterSilent.State != store.BotStateIdle || botAfterSilent.LastSummary != "上次真实对话摘要" {
		t.Fatalf("expected bot state/summary/unread preserved on silent patrol, got %+v", botAfterSilent)
	}
	execLogs, _ := st.ListBotExecutions("butler", 10)
	if len(execLogs) != 1 || execLogs[0].Status != "success" {
		t.Fatalf("expected routine execution log recorded even when silent, got %+v", execLogs)
	}

	// 2. Expiring-asset alert (ItemsPushed > 0) pushes to 服务通知 (notifications) without polluting IM chat timeline
	runner.pushed = 2
	runner.summary = "巡检 6 项资产（同步 1 项，发送 2 项临期强提醒）"
	if _, err := sched.RunNow(context.Background(), "asset_watcher"); err != nil {
		t.Fatalf("alert run failed: %v", err)
	}
	msgsAfterAlert, _ := st.ListBotMessages("butler", 20)
	if len(msgsAfterAlert) != 0 {
		t.Fatalf("expected 0 IM chat messages when routine pushes to 服务通知, got %d", len(msgsAfterAlert))
	}
	botAfterAlert, _ := st.GetBot("butler")
	if botAfterAlert == nil || botAfterAlert.UnreadCount != 0 || botAfterAlert.State != store.BotStateIdle {
		t.Fatalf("expected bot chat unread/state untouched when alert goes to 服务通知, got %+v", botAfterAlert)
	}

	// 3. Seed legacy routine message + real chat message, reopen DB, verify ALL routine messages are purged while real chat remains
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:butler",
		BotID:     "butler",
		MsgKind:   "routine",
		Role:      "assistant",
		Content:   "### 例程主动汇报: 数字资产与会员守望\n\n- **执行状态**: success (6ms)",
	})
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:butler",
		BotID:     "butler",
		MsgKind:   "chat",
		Role:      "assistant",
		Content:   "真实聊天回复保留",
	})
	_ = st.Close()

	stReopened, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer stReopened.Close()
	cleanedMsgs, _ := stReopened.ListBotMessages("butler", 20)
	if len(cleanedMsgs) != 1 || cleanedMsgs[0].MsgKind != "chat" {
		t.Fatalf("expected all legacy routine messages purged while keeping real chat, got %d: %+v", len(cleanedMsgs), cleanedMsgs)
	}
}
