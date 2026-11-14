package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

type TaskStatus struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Type        string           `json:"type"`
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

type Scheduler struct {
	store   *store.Store
	mods    map[string]modules.Module
	runners map[string]modules.TaskRunner
	cron    *cron.Cron
	entries map[string]cron.EntryID
	running map[string]bool
	mu      sync.Mutex
	loc     *time.Location
}

func New(st *store.Store, mods ...modules.Module) *Scheduler {
	loc := modules.ShanghaiLocation()
	s := &Scheduler{
		store:   st,
		mods:    make(map[string]modules.Module),
		runners: make(map[string]modules.TaskRunner),
		entries: make(map[string]cron.EntryID),
		running: make(map[string]bool),
		loc:     loc,
	}
	for _, m := range mods {
		s.mods[m.ID()] = m
		if r, ok := m.(modules.TaskRunner); ok {
			s.runners[m.ID()] = r
		}
	}
	return s
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
		// Fallback check legacy module ID
		if mod, ok := s.mods[taskID]; ok {
			task = &store.Task{
				ID:   mod.ID(),
				Name: mod.Name(),
				Type: mod.ID(),
			}
		} else {
			return modules.RunResult{}, fmt.Errorf("unknown task: %s", taskID)
		}
	}

	runner, ok := s.runners[task.Type]
	if !ok {
		// Try match legacy module
		if mod, mOk := s.mods[task.Type]; mOk {
			if r, rOk := mod.(modules.TaskRunner); rOk {
				runner = r
			}
		}
	}
	if runner == nil {
		return modules.RunResult{}, fmt.Errorf("no runner registered for template type: %s", task.Type)
	}

	runID, err := s.store.StartModuleRun(task.ID, task.Name, triggerType)
	if err != nil {
		log.Printf("[scheduler] failed to record start for %s: %v", taskID, err)
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
