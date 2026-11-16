package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/pushstream"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

type TaskStatus struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	BotID       string           `json:"bot_id"`
	Description string           `json:"description"`
	CronExpr    string           `json:"cron_expr"`
	Enabled     bool             `json:"enabled"`
	Params      string           `json:"params"`
	NextRunAt   string           `json:"next_run_at"`
	IsRunning   bool             `json:"is_running"`
	LastRun     *store.ModuleRun `json:"last_run,omitempty"`
}

// ModuleStatus alias for backward compatibility
type ModuleStatus = TaskStatus

// Scheduler coordinates cron jobs and manual task triggers.
// Loosely-coupled with execution logic via modules.Registry.
type Scheduler struct {
	store    *store.Store
	registry *modules.Registry
	hub      *pushstream.Hub
	cron     *cron.Cron
	entries  map[string]cron.EntryID
	running  map[string]bool
	mu       sync.Mutex
	loc      *time.Location
}

func (s *Scheduler) SetHub(hub *pushstream.Hub) {
	s.hub = hub
}

// NewWithRegistry creates a Scheduler backed by a dynamic module Registry.
func NewWithRegistry(st *store.Store, reg *modules.Registry) *Scheduler {
	return &Scheduler{
		store:    st,
		registry: reg,
		entries:  make(map[string]cron.EntryID),
		running:  make(map[string]bool),
		loc:      modules.ShanghaiLocation(),
	}
}

// New retains backward compatibility for existing initializations.
func New(st *store.Store, mods ...modules.Module) *Scheduler {
	reg := modules.NewRegistry()
	for _, m := range mods {
		if r, ok := m.(modules.TaskRunner); ok {
			reg.RegisterTaskRunner(m.ID(), r)
		}
	}
	return NewWithRegistry(st, reg)
}

func (s *Scheduler) Start() error {
	return s.Reload()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	c := s.cron
	s.mu.Unlock()
	if c != nil {
		ctx := c.Stop()
		<-ctx.Done()
	}
}

func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
	}

	tasks, err := s.store.ListTasks()
	if err != nil {
		return fmt.Errorf("list tasks failed: %w", err)
	}

	c := cron.New(cron.WithLocation(s.loc))
	entries := make(map[string]cron.EntryID)

	for _, t := range tasks {
		if !t.Enabled || t.CronExpr == "" {
			continue
		}
		taskID := t.ID
		entryID, err := c.AddFunc(t.CronExpr, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if _, err := s.execute(ctx, taskID, "cron"); err != nil {
				log.Printf("[scheduler] cron run task %s error: %v", taskID, err)
			}
		})
		if err != nil {
			return fmt.Errorf("invalid cron expr for task %s (%q): %w", taskID, t.CronExpr, err)
		}
		entries[taskID] = entryID
	}

	c.Start()
	s.cron = c
	s.entries = entries
	return nil
}

func (s *Scheduler) RunNow(ctx context.Context, taskID string) (modules.RunResult, error) {
	return s.execute(ctx, taskID, "manual")
}

func (s *Scheduler) execute(ctx context.Context, taskID, triggerType string) (modules.RunResult, error) {
	s.mu.Lock()
	if s.running[taskID] {
		s.mu.Unlock()
		return modules.RunResult{}, fmt.Errorf("task %s is already running", taskID)
	}
	s.running[taskID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, taskID)
		s.mu.Unlock()
	}()

	task, err := s.store.GetTask(taskID)
	if err != nil || task == nil {
		return modules.RunResult{}, fmt.Errorf("unknown task: %s", taskID)
	}

	runner, ok := s.registry.GetTaskRunner(task.Type)
	if !ok {
		return modules.RunResult{}, fmt.Errorf("no runner registered for template type: %s", task.Type)
	}

	runID, err := s.store.StartModuleRun(task.ID, task.Name, triggerType)
	if err != nil {
		log.Printf("[scheduler] failed to record start for %s: %v", taskID, err)
	}

	botID := strings.TrimSpace(task.BotID)
	if botID != "" {
		workLabel := "正在执行例程: " + task.Name
		_ = s.store.SetBotState(botID, store.BotStateWorking, workLabel)
		if s.hub != nil {
			s.hub.PublishEvent("bot_state", map[string]any{
				"bot_id":       botID,
				"state":        store.BotStateWorking,
				"last_summary": workLabel,
			})
		}
	}

	start := time.Now()
	res, runErr := runner.RunWithTask(ctx, *task)
	durationMs := time.Since(start).Milliseconds()

	status := "success"
	summary := res.Summary
	if runErr != nil {
		status = "error"
		if summary == "" {
			summary = runErr.Error()
		}
	}
	if runID > 0 {
		if err := s.store.FinishModuleRun(runID, status, summary, res.ItemsPushed, durationMs); err != nil {
			log.Printf("[scheduler] failed to record finish for %s: %v", taskID, err)
		}
	}

	if botID != "" {
		cleanSummary := notifier.StripEmoji(summary)
		finalState := store.BotStateDone
		if runErr != nil {
			finalState = store.BotStateBlocked
		}
		report := fmt.Sprintf("### 例程主动汇报: %s\n\n- **执行状态**: %s (%dms)\n- **结果摘要**: %s",
			task.Name, status, durationMs, cleanSummary)
		msg := store.AgentMessage{
			SessionID:   "bot:" + botID,
			BotID:       botID,
			SenderBotID: botID,
			MsgKind:     "routine",
			Role:        "assistant",
			Content:     report,
		}
		if msgID, aErr := s.store.AppendAgentMessage(msg); aErr == nil {
			msg.ID = msgID
			msg.CreatedAt = store.NowShanghai()
		}
		_ = s.store.SetBotState(botID, finalState, cleanSummary)
		_ = s.store.IncrementBotUnread(botID, cleanSummary)
		if s.hub != nil {
			s.hub.PublishEvent("bot_message", msg)
			s.hub.PublishEvent("bot_state", map[string]any{
				"bot_id":       botID,
				"state":        finalState,
				"last_summary": cleanSummary,
			})
		}
	}

	return res, runErr
}

func (s *Scheduler) Status() ([]TaskStatus, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return nil, err
	}

	latest, _ := s.store.LatestModuleRuns()

	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]TaskStatus, 0, len(tasks))
	for _, t := range tasks {
		nextRun := ""
		if t.Enabled && s.cron != nil {
			if eid, ok := s.entries[t.ID]; ok {
				entry := s.cron.Entry(eid)
				if !entry.Next.IsZero() {
					nextRun = entry.Next.In(s.loc).Format("2006-01-02 15:04:05")
				}
			}
		}

		var lastPtr *store.ModuleRun
		if lr, ok := latest[t.ID]; ok {
			copyRun := lr
			lastPtr = &copyRun
		}

		out = append(out, TaskStatus{
			ID:          t.ID,
			Name:        t.Name,
			Type:        t.Type,
			BotID:       t.BotID,
			Description: t.Description,
			CronExpr:    t.CronExpr,
			Enabled:     t.Enabled,
			Params:      t.Params,
			NextRunAt:   nextRun,
			IsRunning:   s.running[t.ID],
			LastRun:     lastPtr,
		})
	}
	return out, nil
}
