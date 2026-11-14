package modules

import (
	"fmt"
	"sync"

	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// Registry acts as the central registry for task runners and template definitions.
// High-cohesion & loose-coupling: modules register themselves without coupling with scheduler.
type Registry struct {
	mu             sync.RWMutex
	runners        map[string]TaskRunner
	taskTemplates  []TaskTemplate
	assetTemplates []AssetTemplate
}

// NewRegistry initializes an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		runners:        make(map[string]TaskRunner),
		taskTemplates:  make([]TaskTemplate, 0),
		assetTemplates: make([]AssetTemplate, 0),
	}
}

// RegisterTaskRunner associates a TaskRunner with a specific task type slug.
func (r *Registry) RegisterTaskRunner(taskType string, runner TaskRunner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[taskType] = runner
}

// GetTaskRunner retrieves the TaskRunner for a task type.
func (r *Registry) GetTaskRunner(taskType string) (TaskRunner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[taskType]
	return runner, ok
}

// RegisterTaskTemplate adds a task template definition.
func (r *Registry) RegisterTaskTemplate(tpl TaskTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.taskTemplates = append(r.taskTemplates, tpl)
}

// TaskTemplates returns a copy of all registered task templates.
func (r *Registry) TaskTemplates() []TaskTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TaskTemplate, len(r.taskTemplates))
	copy(out, r.taskTemplates)
	return out
}

// RegisterAssetTemplate adds an asset template definition.
func (r *Registry) RegisterAssetTemplate(tpl AssetTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assetTemplates = append(r.assetTemplates, tpl)
}

// AssetTemplates returns a copy of all registered asset templates.
func (r *Registry) AssetTemplates() []AssetTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AssetTemplate, len(r.assetTemplates))
	copy(out, r.assetTemplates)
	return out
}

// NewDefaultRegistry instantiates and registers all built-in modules and schemas.
func NewDefaultRegistry(st *store.Store, push *pushplus.Client) *Registry {
	reg := NewRegistry()

	// 1. Register Task Runners
	morning := NewMorningModule(st, push)
	reg.RegisterTaskRunner("morning", morning)

	radar := NewRadarModule(st, push)
	reg.RegisterTaskRunner("radar", radar)

	watcher := NewAssetWatcherModule(st, push)
	reg.RegisterTaskRunner("asset_watcher", watcher)

	// 2. Register Builtin Task Templates
	for _, tpl := range BuiltinTemplates() {
		reg.RegisterTaskTemplate(tpl)
	}

	// 3. Register Builtin Asset Templates
	for _, atpl := range BuiltinAssetTemplates() {
		reg.RegisterAssetTemplate(atpl)
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
