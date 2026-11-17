package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
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

var sensitiveConfigKeys = map[string]bool{
	store.KeyCFAPIToken:        true,
	store.KeyBiliSessdata:      true,
	store.KeyDeepSeekAPIKey:    true,
	store.KeyLightAPIKey:       true,
	store.KeyAppBearerToken:    true,
	store.KeyNezhaWebhookToken: true,
}

func maskSecret(val string) string {
	return store.MaskSecretValue(val)
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
		case "todos", "notes", "memories", "assets", "tasks", "bots", "skills", "notifications", "push_logs", "stats", "model_health":
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
	}
	if want["todos"] {
		todos, _ := s.store.ListTodos("all", "", 120)
		resp["todos"] = todos
	}
	if want["notes"] {
		notes, _ := s.store.ListNotes("", "", 80)
		resp["notes"] = notes
	}
	if want["memories"] {
		memories, _ := s.store.ListAgentMemories("", 50)
		resp["agent_memories"] = memories
	}
	if want["assets"] {
		assets, _ := s.store.ListAssets()
		resp["assets"] = assets
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
	if want["model_health"] && s.agent != nil {
		resp["model_health"] = s.agent.GetModelHealth()
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.schedulerTasksList()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取任务状态失败")
		return
	}
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)
	delete(cfgs, store.KeyFreeModelPool)
	for k := range sensitiveConfigKeys {
		if raw, ok := cfgs[k]; ok && strings.TrimSpace(raw) != "" {
			cfgs[k] = maskSecret(raw)
		}
	}

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
	modelPool := store.MaskModelPool(s.store.GetModelPool())
	var modelHealth any
	if s.agent != nil {
		modelHealth = s.agent.GetModelHealth()
	}

	var templates any
	if s.scheduler != nil {
		templates = s.scheduler.TaskTemplates()
	} else {
		templates = modules.BuiltinTemplates()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":         s.version,
		"commit":          s.commit,
		"build_time":      s.buildTime,
		"modules":         tasks, // 兼容前端历史模块字段
		"tasks":           tasks,
		"bots":            bots,
		"skills":          skills,
		"todos":           todos,
		"notes":           notes,
		"model_pool":      modelPool,
		"model_health":    modelHealth,
		"available_tools": agent.AvailableToolsPayload(),
		"templates":       templates,
		"agent_sessions":  sessions,
		"agent_memories":  memories,
		"notifications":   notifications,
		"configs":         cfgs,
		"assets":          assets,
		"push_logs":       logs,
		"stats":           stats,
		"uptime_sec":      int64(time.Since(s.startedAt).Seconds()),
		"server_now":      store.NowShanghai(),
	})
}

func validateHTTPURL(raw string) bool {
	val := strings.TrimSpace(raw)
	if val == "" {
		return true
	}
	parsed, err := url.Parse(val)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON")
		return
	}

	allowedKeys := map[string]bool{
		"cf_api_token":        true,
		"nezha_url":           true,
		"nezha_db_path":       true,
		"nezha_webhook_token": true,
		"bili_sessdata":       true,
		"deepseek_api_key":    true,
		"deepseek_base_url":   true,
		"ai_model":            true,
		"active_persona":      true,
		"light_api_key":       true,
		"light_base_url":      true,
		"light_model":         true,
		"pool_for_light":      true,
		"pool_fallback":       true,
		"app_bearer_token":    true,
	}

	_ = s.store.SetConfig(store.KeyPushChannel, "native")
	urlConfigKeys := map[string]bool{
		"deepseek_base_url": true,
		"light_base_url":    true,
		"nezha_url":         true,
	}
	for k, v := range updates {
		if !allowedKeys[k] {
			continue
		}
		val := strings.TrimSpace(v)
		if urlConfigKeys[k] && !validateHTTPURL(val) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("配置项 %s 必须为有效的 http/https URL", k))
			return
		}
		if sensitiveConfigKeys[k] && strings.Contains(val, "****") {
			continue
		}
		_ = s.store.SetConfig(k, val)
	}

	if s.scheduler != nil {
		if err := s.scheduler.Reload(); err != nil {
			writeError(w, http.StatusBadRequest, "重载调度失败: "+err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestPush(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	channelName := "原生 APP 通知通道 (Inbox + SSE)"
	now := store.NowShanghai()
	content := fmt.Sprintf("### Vesper BotHub 通知链路自检\n\n- **活跃通道**：`%s`\n- **触发时间**：`%s`\n- **链路状态**：正常直连\n\n---\n*来自 Vesper BotHub*", channelName, now)
	if err := s.push.SendMarkdown(ctx, "system_test", "Vesper BotHub 原生 APP 通知测试", content); err != nil {
		writeError(w, http.StatusBadGateway, "推送失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "测试通知已成功推送至原生 APP 收件箱与 SSE 实时通道"})
}

type BackupData struct {
	Version       string              `json:"version"`
	ExportedAt    string              `json:"exported_at"`
	Configs       map[string]string   `json:"configs"`
	Assets        []store.Asset       `json:"assets"`
	Tasks         []store.Task        `json:"tasks"`
	Bots          []store.Bot         `json:"bots,omitempty"`
	Skills        []store.Skill       `json:"skills,omitempty"`
	Todos         []store.TodoItem    `json:"todos,omitempty"`
	Notes         []store.WorkNote    `json:"notes,omitempty"`
	AgentMemories []store.AgentMemory `json:"agent_memories,omitempty"`
}

func (s *Server) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	assets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "导出资产失败: "+err.Error())
		return
	}
	tasks, err := s.store.ListTasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "导出任务失败: "+err.Error())
		return
	}
	bots, _ := s.store.ListBots()
	skills, _ := s.store.ListSkills()
	todos, _ := s.store.ListTodos("all", "", 300)
	notes, _ := s.store.ListNotes("", "", 200)
	memories, _ := s.store.ListAgentMemories("", 500)
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)
	maskSecrets := r.URL.Query().Get("mask_secrets") == "1" || r.URL.Query().Get("mask_secrets") == "true"
	if maskSecrets {
		for k := range sensitiveConfigKeys {
			if raw, ok := cfgs[k]; ok && strings.TrimSpace(raw) != "" {
				cfgs[k] = maskSecret(raw)
			}
		}
		if rawPool, ok := cfgs[store.KeyFreeModelPool]; ok && strings.TrimSpace(rawPool) != "" {
			if maskedPoolBytes, mErr := json.Marshal(store.MaskModelPool(s.store.GetModelPool())); mErr == nil {
				cfgs[store.KeyFreeModelPool] = string(maskedPoolBytes)
			}
		}
	}

	if assets == nil {
		assets = []store.Asset{}
	}
	if tasks == nil {
		tasks = []store.Task{}
	}
	if bots == nil {
		bots = []store.Bot{}
	}
	if skills == nil {
		skills = []store.Skill{}
	}
	if todos == nil {
		todos = []store.TodoItem{}
	}
	if notes == nil {
		notes = []store.WorkNote{}
	}
	if memories == nil {
		memories = []store.AgentMemory{}
	}
	if cfgs == nil {
		cfgs = map[string]string{}
	}

	backup := BackupData{
		Version:       s.version,
		ExportedAt:    store.NowShanghai(),
		Configs:       cfgs,
		Assets:        assets,
		Tasks:         tasks,
		Bots:          bots,
		Skills:        skills,
		Todos:         todos,
		Notes:         notes,
		AgentMemories: memories,
	}

	filename := fmt.Sprintf("bothub_backup_%s.json", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_ = json.NewEncoder(w).Encode(backup)
}

func (s *Server) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(2 * time.Minute))
	_ = rc.SetWriteDeadline(time.Now().Add(2 * time.Minute))

	var backup BackupData
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5*1024*1024)).Decode(&backup); err != nil {
		writeError(w, http.StatusBadRequest, "无效备份文件: "+err.Error())
		return
	}

	// 1. Restore Configs (never overwrite stored secrets with masked "****" placeholders)
	allowedKeys := map[string]bool{
		"cf_api_token":        true,
		"nezha_url":           true,
		"nezha_db_path":       true,
		"nezha_webhook_token": true,
		"bili_sessdata":       true,
		"deepseek_api_key":    true,
		"deepseek_base_url":   true,
		"ai_model":            true,
		"active_persona":      true,
		"light_api_key":       true,
		"light_base_url":      true,
		"light_model":         true,
		"free_model_pool":     true,
		"pool_for_light":      true,
		"pool_fallback":       true,
		"app_bearer_token":    true,
	}
	_ = s.store.SetConfig(store.KeyPushChannel, "native")
	urlConfigKeys := map[string]bool{
		"deepseek_base_url": true,
		"light_base_url":    true,
		"nezha_url":         true,
	}
	for k, v := range backup.Configs {
		if !allowedKeys[k] {
			continue
		}
		val := strings.TrimSpace(v)
		if urlConfigKeys[k] && !validateHTTPURL(val) {
			continue
		}
		if sensitiveConfigKeys[k] && strings.Contains(val, "****") {
			continue
		}
		if k == store.KeyFreeModelPool {
			var entries []store.ModelPoolEntry
			if json.Unmarshal([]byte(val), &entries) == nil {
				_ = s.store.SaveModelPool(entries)
			}
			continue
		}
		_ = s.store.SetConfig(k, val)
	}

	// 2. Restore Assets
	assetsCount := 0
	for _, a := range backup.Assets {
		a.ID = strings.TrimSpace(a.ID)
		a.Name = notifier.StripEmoji(strings.TrimSpace(a.Name))
		if a.ID == "" || a.Name == "" {
			continue
		}
		a.Spec = notifier.StripEmoji(strings.TrimSpace(a.Spec))
		a.Price = notifier.StripEmoji(strings.TrimSpace(a.Price))
		a.Category = notifier.StripEmoji(strings.TrimSpace(a.Category))
		if err := s.store.SaveAsset(a); err == nil {
			assetsCount++
		}
	}

	// 3. Restore Tasks
	tasksCount := 0
	for _, t := range backup.Tasks {
		t.ID = strings.TrimSpace(t.ID)
		t.Name = notifier.StripEmoji(strings.TrimSpace(t.Name))
		if t.ID == "" || t.Name == "" {
			continue
		}
		t.Description = notifier.StripEmoji(strings.TrimSpace(t.Description))
		if err := s.store.SaveTask(t); err == nil {
			tasksCount++
		}
	}

	// 4. Restore Agent Memories (preserve scope, clamped importance, pinned)
	for _, m := range backup.AgentMemories {
		key := notifier.StripEmoji(strings.TrimSpace(m.FactKey))
		content := notifier.StripEmoji(strings.TrimSpace(m.Content))
		if key == "" || content == "" {
			continue
		}
		scope := notifier.StripEmoji(strings.TrimSpace(m.Scope))
		if scope == "" {
			scope = "global"
		}
		cat := notifier.StripEmoji(strings.TrimSpace(m.Category))
		if cat == "" {
			cat = "general"
		}
		_ = s.store.SaveAgentMemoryDetailed(scope, cat, key, content, clampMemoryImportance(m.Importance), m.Pinned)
	}

	// 5. Restore Custom Skills
	skillsCount := 0
	for _, sk := range backup.Skills {
		if strings.TrimSpace(sk.ID) == "" || strings.TrimSpace(sk.Name) == "" {
			continue
		}
		if err := s.store.SaveSkill(sk); err == nil {
			skillsCount++
		}
	}

	// 6. Restore Custom Bots & Group Configurations
	botsCount := 0
	for _, b := range backup.Bots {
		if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.Name) == "" {
			continue
		}
		if err := s.store.SaveBot(b); err == nil {
			botsCount++
		}
	}

	// 7. Restore Todos & Notes
	todosCount := 0
	for _, td := range backup.Todos {
		if strings.TrimSpace(td.Title) == "" {
			continue
		}
		if _, err := s.store.SaveTodo(td); err == nil {
			todosCount++
		}
	}
	notesCount := 0
	for _, nt := range backup.Notes {
		if strings.TrimSpace(nt.Title) == "" {
			continue
		}
		if _, err := s.store.SaveNote(nt); err == nil {
			notesCount++
		}
	}

	// 8. Reload scheduler and notify connected clients
	if s.scheduler != nil {
		_ = s.scheduler.Reload()
	}
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("assistant_update", map[string]any{"kind": "backup_restored"})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"message":         fmt.Sprintf("备份恢复成功：已恢复 %d 个资产，%d 个任务，%d 个技能，%d 个 Bot，%d 条待办与 %d 篇文档", assetsCount, tasksCount, skillsCount, botsCount, todosCount, notesCount),
		"assets_restored": assetsCount,
		"tasks_restored":  tasksCount,
		"skills_restored": skillsCount,
		"bots_restored":   botsCount,
		"todos_restored":  todosCount,
		"notes_restored":  notesCount,
	})
}

func (s *Server) handleSaveModelPool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []store.ModelPoolEntry `json:"entries"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效模型池请求体")
		return
	}
	for _, entry := range req.Entries {
		if strings.TrimSpace(entry.BaseURL) != "" && !validateHTTPURL(entry.BaseURL) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("模型池条目 %s 的 Base URL 必须为合法的 http/https 地址", entry.Name))
			return
		}
	}
	if err := s.store.SaveModelPool(req.Entries); err != nil {
		writeError(w, http.StatusInternalServerError, "保存免费模型池失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("model_health")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"model_pool": store.MaskModelPool(s.store.GetModelPool()),
	})
}

func (s *Server) handleTestModelTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效测试请求")
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if strings.Contains(apiKey, "****") || apiKey == "" {
		if req.TargetID == "primary" {
			stored, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
			apiKey = strings.TrimSpace(stored)
		} else {
			for _, item := range s.store.GetModelPool() {
				if item.ID == req.TargetID {
					apiKey = strings.TrimSpace(item.APIKey)
					break
				}
			}
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 22*time.Second)
	defer cancel()

	latencyMs, preview, err := s.agent.ProbeModelTargetWithID(ctx, req.TargetID, req.BaseURL, apiKey, req.Model, req.Provider)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"latency_ms":   latencyMs,
		"reply":        preview,
		"model_health": s.agent.GetModelHealth(),
		"message":      fmt.Sprintf("连接成功 (%dms)", latencyMs),
	})
}

func (s *Server) handleFetchUpstreamModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Verify   *bool  `json:"verify,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效模型拉取请求")
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if strings.Contains(apiKey, "****") || apiKey == "" {
		if req.TargetID == "primary" {
			stored, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
			apiKey = strings.TrimSpace(stored)
		} else {
			for _, item := range s.store.GetModelPool() {
				if item.ID == req.TargetID {
					apiKey = strings.TrimSpace(item.APIKey)
					break
				}
			}
		}
	}

	verify := true
	if req.Verify != nil {
		verify = *req.Verify
	}

	ctx, cancel := context.WithTimeout(r.Context(), 38*time.Second)
	defer cancel()

	res, err := s.agent.FetchAndVerifyUpstreamModels(ctx, req.BaseURL, apiKey, req.Provider, verify)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                 true,
		"models":             res.Models,
		"count":              len(res.Models),
		"checked_models":     res.CheckedModels,
		"filtered_out_count": res.FilteredOutCount,
		"unverified_count":   res.UnverifiedCount,
	})
}

func (s *Server) spaFileServer() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if f, err := s.staticFS.Open(cleanPath); err == nil {
			fi, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && fi.IsDir() {
				// Prevent directory listing on /assets/ or arbitrary folders
				if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/assets") {
					http.NotFound(w, r)
					return
				}
			} else {
				if strings.HasSuffix(cleanPath, ".html") {
					w.Header().Set("Cache-Control", "no-cache")
				} else if strings.HasPrefix(cleanPath, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "public, max-age=86400")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Never masquerade missing API routes, hashed asset bundles, or non-HTML asset files as 200 index.html
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/assets") {
			http.NotFound(w, r)
			return
		}
		if ext := strings.ToLower(path.Ext(cleanPath)); ext != "" && ext != ".html" && ext != ".htm" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
