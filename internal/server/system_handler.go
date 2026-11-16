package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"service":    "vesper-bothub",
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"time":       store.NowShanghai(),
	})
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
	v := strings.TrimSpace(val)
	if v == "" {
		return ""
	}
	runes := []rune(v)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	var tasks []any
	if s.scheduler != nil {
		tList, err := s.scheduler.Status()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取任务状态失败")
			return
		}
		for _, t := range tList {
			tasks = append(tasks, t)
		}
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
	modelPool := store.MaskModelPool(s.store.GetModelPool())
	var modelHealth any
	if s.agent != nil {
		modelHealth = s.agent.GetModelHealth()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":                  s.version,
		"commit":                   s.commit,
		"build_time":               s.buildTime,
		"modules":                  tasks, // 兼容前端历史模块字段
		"tasks":                    tasks,
		"bots":                     bots,
		"bot_templates":            store.BuiltinBotTemplates(),
		"model_pool":               modelPool,
		"model_provider_templates": store.ModelProviderTemplates(),
		"model_health":             modelHealth,
		"available_tools":          agent.AvailableToolsPayload(),
		"templates":                modules.BuiltinTemplates(),
		"asset_templates":          modules.BuiltinAssetTemplates(),
		"personas":                 agent.BuiltinPersonas(),
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
	for k, v := range updates {
		if !allowedKeys[k] {
			continue
		}
		val := strings.TrimSpace(v)
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
	AgentMemories []store.AgentMemory `json:"agent_memories,omitempty"`
}

func (s *Server) handleExportBackup(w http.ResponseWriter, r *http.Request) {
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
	memories, _ := s.store.ListAgentMemories("", 500)
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)

	backup := BackupData{
		Version:       s.version,
		ExportedAt:    store.NowShanghai(),
		Configs:       cfgs,
		Assets:        assets,
		Tasks:         tasks,
		Bots:          bots,
		AgentMemories: memories,
	}

	filename := fmt.Sprintf("bothub_backup_%s.json", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_ = json.NewEncoder(w).Encode(backup)
}

func (s *Server) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	var backup BackupData
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 5*1024*1024)).Decode(&backup); err != nil {
		writeError(w, http.StatusBadRequest, "无效备份文件: "+err.Error())
		return
	}

	// 1. Restore Configs
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
	for k, v := range backup.Configs {
		if allowedKeys[k] {
			_ = s.store.SetConfig(k, strings.TrimSpace(v))
		}
	}

	// 2. Restore Assets
	assetsCount := 0
	for _, a := range backup.Assets {
		if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.Name) == "" {
			continue
		}
		if err := s.store.SaveAsset(a); err == nil {
			assetsCount++
		}
	}

	// 3. Restore Tasks
	tasksCount := 0
	for _, t := range backup.Tasks {
		if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.Name) == "" {
			continue
		}
		if err := s.store.SaveTask(t); err == nil {
			tasksCount++
		}
	}

	// 4. Restore Agent Memories (preserve scope)
	for _, m := range backup.AgentMemories {
		if strings.TrimSpace(m.FactKey) == "" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		_ = s.store.SaveAgentMemoryWithScope(m.Scope, m.Category, m.FactKey, m.Content)
	}

	// 5. Restore Custom Bots & Group Configurations
	botsCount := 0
	for _, b := range backup.Bots {
		if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.Name) == "" {
			continue
		}
		if err := s.store.SaveBot(b); err == nil {
			botsCount++
		}
	}

	// 6. Reload scheduler
	if s.scheduler != nil {
		_ = s.scheduler.Reload()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"message":         fmt.Sprintf("备份恢复成功：已恢复 %d 个资产，%d 个任务，%d 个 Bot 与全局配置", assetsCount, tasksCount, botsCount),
		"assets_restored": assetsCount,
		"tasks_restored":  tasksCount,
		"bots_restored":   botsCount,
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
	if err := s.store.SaveModelPool(req.Entries); err != nil {
		writeError(w, http.StatusInternalServerError, "保存免费模型池失败: "+err.Error())
		return
	}
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

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	models, err := s.agent.FetchUpstreamModels(ctx, req.BaseURL, apiKey, req.Provider)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"models": models,
		"count":  len(models),
	})
}

func (s *Server) spaFileServer() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if f, err := s.staticFS.Open(path); err == nil {
			_ = f.Close()
			if strings.HasSuffix(path, ".html") {
				w.Header().Set("Cache-Control", "no-cache")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=86400")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-cache")
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
