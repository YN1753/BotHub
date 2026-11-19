package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"ok":         true,
		"service":    "vesper-bothub",
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"time":       store.NowShanghai(),
	}
	if s.push != nil && s.push.Hub() != nil {
		resp["sse_clients"] = s.push.Hub().SubscriberCount()
		resp["sse_dropped"] = s.push.Hub().DroppedCount()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) schedulerTasksList() ([]any, error) {
	tasks := make([]any, 0)
	if s.scheduler != nil {
		tList, err := s.scheduler.Status()
		if err != nil {
			return nil, err
		}
		for _, t := range tList {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (s *Server) safeMaskedConfigs() map[string]string {
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)
	delete(cfgs, store.KeyFreeModelPool)
	delete(cfgs, store.KeyModelProviders)
	delete(cfgs, store.KeyPooledModels)
	store.MaskSensitiveConfigs(cfgs)
	return cfgs
}

func (s *Server) revisionsSnapshot() map[string]int64 {
	if s != nil && s.push != nil && s.push.Hub() != nil {
		return s.push.Hub().RevisionsSnapshot()
	}
	return map[string]int64{}
}

// handleDashboardSlice serves targeted partial refreshes (workhub, ops, agents, or comma-separated kinds)
// so SSE assistant_update bursts only query the changed SQLite tables instead of all 16 dashboard tables.
func (s *Server) handleDashboardSlice(w http.ResponseWriter, r *http.Request) {
	rawKinds := strings.TrimSpace(r.PathValue("slice"))
	if q := strings.TrimSpace(r.URL.Query().Get("kinds")); q != "" {
		if rawKinds != "" && rawKinds != "slice" {
			rawKinds = rawKinds + "," + q
		} else {
			rawKinds = q
		}
	}
	if rawKinds == "" || rawKinds == "slice" {
		writeError(w, http.StatusBadRequest, "缺少 slice 或 kinds 参数")
		return
	}

	want := make(map[string]bool)
	for _, part := range strings.Split(rawKinds, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "workhub":
			want["todos"] = true
			want["notes"] = true
			want["stash"] = true
		case "ops":
			want["tasks"] = true
			want["assets"] = true
			want["push_logs"] = true
			want["stats"] = true
		case "agents":
			want["bots"] = true
			want["skills"] = true
			want["memories"] = true
			want["model_health"] = true
			want["models"] = true
		case "model_pool":
			want["models"] = true
		case "todos", "notes", "stash", "memories", "assets", "tasks", "bots", "skills", "notifications", "push_logs", "stats", "model_health", "models", "configs":
			want[strings.ToLower(strings.TrimSpace(part))] = true
		default:
			writeError(w, http.StatusBadRequest, "不支持的 dashboard slice: "+part)
			return
		}
	}

	resp := map[string]any{
		"ok":         true,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"server_now": store.NowShanghai(),
		"revisions":  s.revisionsSnapshot(),
	}
	if want["todos"] {
		todos, _ := s.store.ListTodos("all", "", 120)
		resp["todos"] = todos
	}
	if want["notes"] {
		notes, _ := s.store.ListNotes("", "", 80)
		resp["notes"] = notes
	}
	if want["stash"] {
		stashItems, _ := s.store.SearchStashItems("", "all", "", 100)
		stashTags, _ := s.store.ListTopStashTags(14)
		resp["stash"] = stashItems
		resp["stash_tags"] = stashTags
	}
	if want["memories"] {
		memories, _ := s.store.ListAgentMemories("", 50)
		resp["agent_memories"] = memories
	}
	if want["assets"] {
		assets, _ := s.store.ListAssets()
		resp["assets"] = assets
		resp["asset_categories"] = s.store.ListAssetCategories()
		if !want["stats"] {
			want["stats"] = true
		}
	}
	if want["tasks"] {
		tasks, err := s.schedulerTasksList()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取任务状态失败")
			return
		}
		resp["tasks"] = tasks
		resp["modules"] = tasks
	}
	if want["bots"] {
		bots, _ := s.store.ListBots()
		resp["bots"] = bots
	}
	if want["skills"] {
		skills, _ := s.store.ListSkills()
		resp["skills"] = skills
	}
	if want["notifications"] {
		notifications, _ := s.store.ListNotifications(0, 40, false)
		resp["notifications"] = notifications
	}
	if want["push_logs"] {
		logs, _ := s.store.ListPushLogs(40)
		resp["push_logs"] = logs
	}
	if want["stats"] {
		stats, _ := s.store.CountStats()
		resp["stats"] = stats
	}
	if want["models"] {
		providers, pooledModels, poolStrategies := s.store.GetModelConfig()
		usageSummary, _ := s.store.SummarizeModelUsage(7)
		resp["model_pool"] = store.MaskModelPool(s.store.GetModelPool())
		resp["model_providers"] = store.MaskModelProviders(providers)
		resp["pooled_models"] = pooledModels
		resp["pool_strategies"] = poolStrategies
		resp["model_usage_summary"] = usageSummary
		if s.agent != nil {
			resp["model_options"] = s.agent.ModelOptionsPayload()
			resp["model_health"] = s.agent.GetModelHealth()
		}
	} else if want["model_health"] && s.agent != nil {
		resp["model_health"] = s.agent.GetModelHealth()
	}
	if want["configs"] {
		resp["configs"] = s.safeMaskedConfigs()
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.schedulerTasksList()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取任务状态失败")
		return
	}
	cfgs := s.safeMaskedConfigs()

	logs, _ := s.store.ListPushLogs(40)
	assets, _ := s.store.ListAssets()
	stats, _ := s.store.CountStats()
	sessions, _ := s.store.ListAgentSessions(30)
	memories, _ := s.store.ListAgentMemories("", 50)
	notifications, _ := s.store.ListNotifications(0, 40, false)
	bots, _ := s.store.ListBots()
	skills, _ := s.store.ListSkills()
	todos, _ := s.store.ListTodos("all", "", 120)
	notes, _ := s.store.ListNotes("", "", 80)
	stashItems, _ := s.store.SearchStashItems("", "all", "", 100)
	stashTags, _ := s.store.ListTopStashTags(14)
	modelPool := store.MaskModelPool(s.store.GetModelPool())
	providers, pooledModels, poolStrategies := s.store.GetModelConfig()
	usageSummary, _ := s.store.SummarizeModelUsage(7)
	var modelHealth any
	var modelOptions any
	if s.agent != nil {
		modelHealth = s.agent.GetModelHealth()
		modelOptions = s.agent.ModelOptionsPayload()
	}

	var templates any
	if s.scheduler != nil {
		templates = s.scheduler.TaskTemplates()
	} else {
		templates = modules.BuiltinTemplates()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":             s.version,
		"commit":              s.commit,
		"build_time":          s.buildTime,
		"modules":             tasks, // 兼容前端历史模块字段
		"tasks":               tasks,
		"bots":                bots,
		"skills":              skills,
		"todos":               todos,
		"notes":               notes,
		"stash":               stashItems,
		"stash_tags":          stashTags,
		"model_pool":          modelPool,
		"model_providers":     store.MaskModelProviders(providers),
		"pooled_models":       pooledModels,
		"pool_strategies":     poolStrategies,
		"model_usage_summary": usageSummary,
		"model_options":       modelOptions,
		"model_health":        modelHealth,
		"available_tools": agent.AvailableToolsPayload(),
		"templates":       templates,
		"agent_sessions":  sessions,
		"agent_memories":  memories,
		"notifications":   notifications,
		"configs":          cfgs,
		"assets":           assets,
		"asset_categories": s.store.ListAssetCategories(),
		"push_logs":       logs,
		"stats":           stats,
		"revisions":       s.revisionsSnapshot(),
		"uptime_sec":      int64(time.Since(s.startedAt).Seconds()),
		"server_now":      store.NowShanghai(),
	})
}
