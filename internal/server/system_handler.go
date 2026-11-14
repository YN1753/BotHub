package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.scheduler.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取任务状态失败")
		return
	}
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)

	logs, _ := s.store.ListPushLogs(40)
	assets, _ := s.store.ListAssets()
	stats, _ := s.store.CountStats()

	writeJSON(w, http.StatusOK, map[string]any{
		"version":         s.version,
		"commit":          s.commit,
		"build_time":      s.buildTime,
		"modules":         tasks, // 兼容前端历史模块字段
		"tasks":           tasks,
		"templates":       modules.BuiltinTemplates(),
		"asset_templates": modules.BuiltinAssetTemplates(),
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
		"pushplus_token": true,
		"cf_api_token":   true,
	}

	for k, v := range updates {
		if !allowedKeys[k] {
			continue
		}
		val := strings.TrimSpace(v)
		_ = s.store.SetConfig(k, val)
	}

	if err := s.scheduler.Reload(); err != nil {
		writeError(w, http.StatusBadRequest, "重载调度失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestPush(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	now := store.NowShanghai()
	content := fmt.Sprintf("### Vesper BotHub 通道自检\n\n- **服务状态**：运行正常\n- **推送通道**：PushPlus Markdown\n- **触发时间**：`%s`\n\n---\n*来自 Vesper BotHub · bot.starryheavens.uk*", now)
	if err := s.push.SendMarkdown(ctx, "system_test", "Vesper BotHub 链路连通测试", content); err != nil {
		writeError(w, http.StatusBadGateway, "推送失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "测试消息已成功投递至 PushPlus"})
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
