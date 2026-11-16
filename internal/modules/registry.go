package modules

import (
	"fmt"
	"sync"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// Registry acts as the central registry for task runners and template definitions.
// High-cohesion & loose-coupling: modules register themselves without coupling with scheduler.
type Registry struct {
	mu            sync.RWMutex
	runners       map[string]TaskRunner
	taskTemplates []TaskTemplate
}

// NewRegistry initializes an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		runners:       make(map[string]TaskRunner),
		taskTemplates: make([]TaskTemplate, 0),
	}
}

// RegisterTaskRunner associates a TaskRunner with a specific task type slug.
func (r *Registry) RegisterTaskRunner(taskType string, runner TaskRunner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[taskType] = runner
}

// RegisteredTaskTypes returns all registered runner type slugs in insertion/sorted order.
func (r *Registry) RegisteredTaskTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.taskTemplates))
	seen := make(map[string]bool)
	for _, tpl := range r.taskTemplates {
		if _, ok := r.runners[tpl.Type]; ok && !seen[tpl.Type] {
			seen[tpl.Type] = true
			out = append(out, tpl.Type)
		}
	}
	for k := range r.runners {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

// GetTaskRunner retrieves the TaskRunner for a task type.
func (r *Registry) GetTaskRunner(taskType string) (TaskRunner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[taskType]
	return runner, ok
}

// RegisterTaskTemplate adds a task engine capability definition.
func (r *Registry) RegisterTaskTemplate(tpl TaskTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.taskTemplates = append(r.taskTemplates, tpl)
}

// TaskTemplates returns a copy of all registered task definitions.
func (r *Registry) TaskTemplates() []TaskTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TaskTemplate, len(r.taskTemplates))
	copy(out, r.taskTemplates)
	return out
}

// NewDefaultRegistry instantiates and registers all built-in module runners cleanly.
func NewDefaultRegistry(st *store.Store, push *notifier.Client) *Registry {
	reg := NewRegistry()

	morning := NewMorningModule(st, push)
	reg.RegisterTaskRunner("morning", morning)

	radar := NewRadarModule(st, push)
	reg.RegisterTaskRunner("radar", radar)

	watcher := NewAssetWatcherModule(st, push)
	reg.RegisterTaskRunner("asset_watcher", watcher)

	for _, tpl := range BuiltinTemplates() {
		reg.RegisterTaskTemplate(tpl)
	}

	return reg
}

// EnsureRunnerExists validates if a runner is registered for the specified task type.
func (r *Registry) EnsureRunnerExists(taskType string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.runners[taskType]; !ok {
		return fmt.Errorf("no runner registered for template type: %s", taskType)
	}
	return nil
}
