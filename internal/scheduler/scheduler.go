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

type ModuleStatus struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	CronExpr    string           `json:"cron_expr"`
	NextRunAt   string           `json:"next_run_at"`
	IsRunning   bool             `json:"is_running"`
	LastRun     *store.ModuleRun `json:"last_run,omitempty"`
}

type Scheduler struct {
	store   *store.Store
	mods    map[string]modules.Module
	order   []string
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
		entries: make(map[string]cron.EntryID),
		running: make(map[string]bool),
		loc:     loc,
	}
	for _, m := range mods {
		s.mods[m.ID()] = m
		s.order = append(s.order, m.ID())
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

	c := cron.New(cron.WithLocation(s.loc))
	entries := make(map[string]cron.EntryID)

	for _, id := range s.order {
		enabled, expr := s.moduleConfig(id)
		if !enabled || expr == "" {
			continue
		}
		modID := id
		entryID, err := c.AddFunc(expr, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			if _, err := s.execute(ctx, modID, "cron"); err != nil {
				log.Printf("[scheduler] cron run %s error: %v", modID, err)
			}
		})
		if err != nil {
			return fmt.Errorf("invalid cron expression for %s (%q): %w", id, expr, err)
		}
		entries[id] = entryID
	}

	c.Start()
	s.cron = c
	s.entries = entries
	return nil
}

func (s *Scheduler) RunNow(ctx context.Context, moduleID string) (modules.RunResult, error) {
	return s.execute(ctx, moduleID, "manual")
}

func (s *Scheduler) execute(ctx context.Context, moduleID, triggerType string) (modules.RunResult, error) {
	s.mu.Lock()
	mod, ok := s.mods[moduleID]
	if !ok {
		s.mu.Unlock()
		return modules.RunResult{}, fmt.Errorf("unknown module: %s", moduleID)
	}
	if s.running[moduleID] {
		s.mu.Unlock()
		return modules.RunResult{}, fmt.Errorf("module %s is already running", moduleID)
	}
	s.running[moduleID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, moduleID)
		s.mu.Unlock()
	}()

	runID, err := s.store.StartModuleRun(mod.ID(), mod.Name(), triggerType)
	if err != nil {
		log.Printf("[scheduler] failed to record start for %s: %v", moduleID, err)
	}

	start := time.Now()
	res, runErr := mod.Run(ctx)
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
			log.Printf("[scheduler] failed to record finish for %s: %v", moduleID, err)
		}
	}

	return res, runErr
}

func (s *Scheduler) Status() ([]ModuleStatus, error) {
	latest, err := s.store.LatestModuleRuns()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ModuleStatus, 0, len(s.order))
	for _, id := range s.order {
		mod := s.mods[id]
		enabled, expr := s.moduleConfig(id)

		nextRun := ""
		if enabled && s.cron != nil {
			if eid, ok := s.entries[id]; ok {
				entry := s.cron.Entry(eid)
				if !entry.Next.IsZero() {
					nextRun = entry.Next.In(s.loc).Format("2006-01-02 15:04:05")
				}
			}
		}

		var lastPtr *store.ModuleRun
		if lr, ok := latest[id]; ok {
			copyRun := lr
			lastPtr = &copyRun
		}

		out = append(out, ModuleStatus{
			ID:          mod.ID(),
			Name:        mod.Name(),
			Description: mod.Description(),
			Enabled:     enabled,
			CronExpr:    expr,
			NextRunAt:   nextRun,
			IsRunning:   s.running[id],
			LastRun:     lastPtr,
		})
	}
	return out, nil
}

func (s *Scheduler) moduleConfig(id string) (bool, string) {
	switch id {
	case "morning":
		en, _ := s.store.GetConfig(store.KeyMorningEnabled)
		cr, _ := s.store.GetConfig(store.KeyMorningCron)
		return en != "false", cr
	case "radar":
		en, _ := s.store.GetConfig(store.KeyRadarEnabled)
		cr, _ := s.store.GetConfig(store.KeyRadarCron)
		return en != "false", cr
	case "asset_watcher":
		en, _ := s.store.GetConfig(store.KeyAssetWatcherEnabled)
		cr, _ := s.store.GetConfig(store.KeyAssetWatcherCron)
		return en != "false", cr
	default:
		return false, ""
	}
}
