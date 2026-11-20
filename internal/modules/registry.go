package modules

import (
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
func NewDefaultRegistry(st *store.Store, push *notifier.Client, summarizers ...LightSummarizer) *Registry {
	reg := NewRegistry()

	morning := NewMorningModule(st, push)
	radar := NewRadarModule(st, push)
	if len(summarizers) > 0 && summarizers[0] != nil {
		morning.SetSummarizer(summarizers[0])
		radar.SetSummarizer(summarizers[0])
	}

	reg.RegisterTaskRunner("morning", morning)
	reg.RegisterTaskRunner("radar", radar)
	reg.RegisterTaskRunner("asset_watcher", NewAssetWatcherModule(st, push))
	reg.RegisterTaskRunner("bot_prompt", NewBotPromptModule(st, push))

	for _, tpl := range BuiltinTemplates() {
		reg.RegisterTaskTemplate(tpl)
	}

	return reg
}

// SetLightSummarizer injects the shared light LLM summarizer into MorningModule and RadarModule.
func (r *Registry) SetLightSummarizer(sum LightSummarizer) {
	if r == nil {
		return
	}
	if runner, ok := r.GetTaskRunner("morning"); ok {
		if mm, ok := runner.(*MorningModule); ok {
			mm.SetSummarizer(sum)
		}
	}
	if runner, ok := r.GetTaskRunner("radar"); ok {
		if rm, ok := runner.(*RadarModule); ok {
			rm.SetSummarizer(sum)
		}
	}
}

// SetBotRoutineExecutor injects the agent routine executor into the registered bot_prompt module.
func (r *Registry) SetBotRoutineExecutor(exec BotRoutineExecutor) {
	if runner, ok := r.GetTaskRunner("bot_prompt"); ok {
		if bp, ok := runner.(*BotPromptModule); ok {
			bp.SetExecutor(exec)
		}
	}
}
