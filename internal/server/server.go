package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

const sessionCookieName = "bothub_session"

type Server struct {
	store     *store.Store
	scheduler *scheduler.Scheduler
	push      *pushplus.Client
	staticFS  fs.FS
	startedAt time.Time
}

func New(st *store.Store, sched *scheduler.Scheduler, push *pushplus.Client, staticEmbed embed.FS) (*Server, error) {
	sub, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		return nil, err
	}
	return &Server{
		store:     st,
		scheduler: sched,
		push:      push,
		staticFS:  sub,
		startedAt: time.Now(),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("POST /api/config", s.requireAuth(s.handleUpdateConfig))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleUpdatePassword))
	mux.HandleFunc("POST /api/modules/{id}/run", s.requireAuth(s.handleRunModule))
	mux.HandleFunc("POST /api/test-push", s.requireAuth(s.handleTestPush))

	fileServer := http.FileServer(http.FS(s.staticFS))
	mux.Handle("GET /", fileServer)

	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"service":    "vesper-bothub",
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"time":       store.NowShanghai(),
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": s.isAuthenticated(r),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if !s.verifyPassword(body.Password) {
		writeError(w, http.StatusUnauthorized, "访问密钥不正确")
		return
	}

	token, err := s.issueSessionCookie()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "会话签发失败")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	mods, err := s.scheduler.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取模块状态失败")
		return
	}

	cfgs, err := s.store.GetAllConfigs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)

	logs, err := s.store.ListPushLogs(40)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取推送日志失败")
		return
	}

	stats, _ := s.store.CountStats()

	writeJSON(w, http.StatusOK, map[string]any{
		"modules":    mods,
		"configs":    cfgs,
		"push_logs":  logs,
		"stats":      stats,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"server_now": store.NowShanghai(),
	})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "无效的配置 JSON")
		return
	}

	allowed := map[string]bool{
		store.KeyMorningEnabled:  true,
		store.KeyMorningCron:     true,
		store.KeyMorningCity:     true,
		store.KeyNezhaURL:        true,
		store.KeyRadarEnabled:    true,
		store.KeyRadarCron:       true,
		store.KeyRadarHNMinScore: true,
		store.KeyRadarLimit:      true,
		store.KeyPushPlusToken:   true,
	}

	for k, v := range updates {
		if !allowed[k] {
			continue
		}
		val := strings.TrimSpace(v)
		if k == store.KeyMorningCron || k == store.KeyRadarCron {
			if _, err := cron.ParseStandard(val); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("Cron 表达式无效 (%s): %v", val, err))
				return
			}
		}
		if k == store.KeyMorningEnabled || k == store.KeyRadarEnabled {
			if val != "true" && val != "false" {
				writeError(w, http.StatusBadRequest, "开关字段必须为 true 或 false")
				return
			}
		}
		if k == store.KeyRadarHNMinScore || k == store.KeyRadarLimit {
			if _, err := strconv.Atoi(val); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("数值字段无效 (%s)", k))
				return
			}
		}
		if err := s.store.SetConfig(k, val); err != nil {
			writeError(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
			return
		}
	}

	if err := s.scheduler.Reload(); err != nil {
		writeError(w, http.StatusBadRequest, "重载调度器失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	np := strings.TrimSpace(body.NewPassword)
	if len(np) < 6 {
		writeError(w, http.StatusBadRequest, "新密钥长度至少需要 6 位")
		return
	}
	if err := s.store.SetConfig(store.KeyAdminPassHash, store.HashPassword(np)); err != nil {
		writeError(w, http.StatusInternalServerError, "更新密钥失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRunModule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	res, err := s.scheduler.RunNow(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"summary": res.Summary,
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"summary":      res.Summary,
		"items_pushed": res.ItemsPushed,
	})
}

func (s *Server) handleTestPush(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	now := time.Now().In(modules.ShanghaiLocation()).Format("2006-01-02 15:04:05")
	title := "Vesper BotHub 链路连通测试"
	content := fmt.Sprintf(
		"### Vesper BotHub 通道自检\n\n"+
			"- **服务状态**：运行正常\n"+
			"- **推送通道**：PushPlus Markdown\n"+
			"- **触发时间**：`%s` (Asia/Shanghai)\n\n"+
			"---\n*来自 Vesper BotHub · bot.starryheavens.uk*",
		now,
	)

	if err := s.push.SendMarkdown(ctx, "system_test", title, content); err != nil {
		writeError(w, http.StatusBadGateway, "测试推送失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "测试消息已成功投递至 PushPlus",
	})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "未授权访问，请先登录控制台")
			return
		}
		next(w, r)
	}
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		raw := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if s.verifyPassword(raw) {
			return true
		}
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return s.verifySessionCookie(c.Value)
}

func (s *Server) verifyPassword(raw string) bool {
	stored, err := s.store.GetConfig(store.KeyAdminPassHash)
	if err != nil || stored == "" {
		return false
	}
	candidate := store.HashPassword(raw)
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(stored)) == 1
}

func (s *Server) issueSessionCookie() (string, error) {
	secret, err := s.store.GetConfig(store.KeySessionSecret)
	if err != nil || secret == "" {
		return "", fmt.Errorf("missing session secret")
	}
	exp := strconv.FormatInt(time.Now().Add(7*24*time.Hour).Unix(), 10)
	sig := signHMAC(secret, exp)
	return exp + "." + sig, nil
}

func (s *Server) verifySessionCookie(val string) bool {
	parts := strings.SplitN(val, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return false
	}
	secret, err := s.store.GetConfig(store.KeySessionSecret)
	if err != nil || secret == "" {
		return false
	}
	expected := signHMAC(secret, parts[0])
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func signHMAC(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": msg,
	})
}
