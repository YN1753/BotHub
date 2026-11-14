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
	version   string
	commit    string
	buildTime string
}

func New(st *store.Store, sched *scheduler.Scheduler, push *pushplus.Client, staticEmbed embed.FS, version, commit, buildTime string) (*Server, error) {
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
		version:   version,
		commit:    commit,
		buildTime: buildTime,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Public Sanitized Read-only Assets Overview (with CORS support)
	mux.HandleFunc("OPTIONS /api/v1/assets/overview", s.handleCORSPreflight)
	mux.HandleFunc("GET /api/v1/assets/overview", s.handlePublicAssetsOverview)

	// Authenticated Management Endpoints
	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("POST /api/config", s.requireAuth(s.handleUpdateConfig))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleUpdatePassword))
	mux.HandleFunc("POST /api/modules/{id}/run", s.requireAuth(s.handleRunModule))
	mux.HandleFunc("POST /api/test-push", s.requireAuth(s.handleTestPush))

	// Authenticated Asset Management Endpoints
	mux.HandleFunc("GET /api/assets", s.requireAuth(s.handleListAssets))
	mux.HandleFunc("POST /api/assets/{id}", s.requireAuth(s.handleUpdateAsset))

	// Embedded Static Single Binary SPA Router
	mux.Handle("GET /", s.spaFileServer())

	return withSecurityHeaders(mux)
}

func (s *Server) spaFileServer() http.Handler {
	fileServer := http.FileServer(http.FS(s.staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try opening the file in embedded staticFS
		f, err := s.staticFS.Open(path)
		if err == nil {
			_ = f.Close()
			if strings.HasSuffix(path, ".html") {
				w.Header().Set("Cache-Control", "no-cache")
			} else {
				w.Header().Set("Cache-Control", "public, max-age=86400")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// If path is not API and not found, fallback to index.html (SPA friendly)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-cache")
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})
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
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"time":       store.NowShanghai(),
	})
}

func (s *Server) handleCORSPreflight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublicAssetsOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	list, err := s.store.GetPublicAssets()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code":    1,
			"message": "获取资产概览失败: " + err.Error(),
			"data":    []any{},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":       0,
		"data":       list,
		"updated_at": store.NowShanghai(),
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

type AdminAssetItem struct {
	store.Asset
	DaysLeft int  `json:"days_left"`
	WarnFlag bool `json:"warn_flag"`
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

	rawAssets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取资产列表失败")
		return
	}

	adminAssets := make([]AdminAssetItem, 0, len(rawAssets))
	for _, a := range rawAssets {
		days, _ := store.CalculateDaysLeft(a.DueDate)
		warnFlag := (days >= 0 && days <= 7) || (a.Status == "即将到期" || a.Status == "已到期")
		adminAssets = append(adminAssets, AdminAssetItem{
			Asset:    a,
			DaysLeft: days,
			WarnFlag: warnFlag,
		})
	}

	stats, _ := s.store.CountStats()

	writeJSON(w, http.StatusOK, map[string]any{
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"modules":    mods,
		"configs":    cfgs,
		"assets":     adminAssets,
		"push_logs":  logs,
		"stats":      stats,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"server_now": store.NowShanghai(),
	})
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	rawAssets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取资产失败: "+err.Error())
		return
	}

	adminAssets := make([]AdminAssetItem, 0, len(rawAssets))
	for _, a := range rawAssets {
		days, _ := store.CalculateDaysLeft(a.DueDate)
		warnFlag := (days >= 0 && days <= 7) || (a.Status == "即将到期" || a.Status == "已到期")
		adminAssets = append(adminAssets, AdminAssetItem{
			Asset:    a,
			DaysLeft: days,
			WarnFlag: warnFlag,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"assets": adminAssets,
	})
}

type updateAssetRequest struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Spec        string `json:"spec"`
	Price       string `json:"price"`
	Status      string `json:"status"`
	DueDate     string `json:"due_date"`
	AutoRenew   bool   `json:"auto_renew"`
	WarnDays    string `json:"warn_days"`
	ExtraConfig string `json:"extra_config"`
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "资产 ID 不能为空")
		return
	}

	existing, err := s.store.GetAsset(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取资产失败: "+err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "未找到目标资产: "+id)
		return
	}

	var req updateAssetRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求格式")
		return
	}

	if req.Name != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if req.Category != "" {
		existing.Category = strings.TrimSpace(req.Category)
	}
	if req.Spec != "" {
		existing.Spec = strings.TrimSpace(req.Spec)
	}
	if req.Price != "" {
		existing.Price = strings.TrimSpace(req.Price)
	}
	if req.Status != "" {
		existing.Status = strings.TrimSpace(req.Status)
	}
	if req.DueDate != "" {
		existing.DueDate = strings.TrimSpace(req.DueDate)
	}
	existing.AutoRenew = req.AutoRenew
	if req.WarnDays != "" {
		existing.WarnDays = strings.TrimSpace(req.WarnDays)
	}
	if req.ExtraConfig != "" {
		// Validate JSON format
		var test map[string]any
		if err := json.Unmarshal([]byte(req.ExtraConfig), &test); err != nil {
			writeError(w, http.StatusBadRequest, "额外配置需为有效 JSON")
			return
		}
		existing.ExtraConfig = req.ExtraConfig
	}

	existing.LastCheckedAt = store.NowShanghai()
	if err := s.store.SaveAsset(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "保存资产更新失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "资产更新成功"})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "无效的配置 JSON")
		return
	}

	allowed := map[string]bool{
		store.KeyMorningEnabled:      true,
		store.KeyMorningCron:         true,
		store.KeyMorningCity:         true,
		store.KeyNezhaURL:            true,
		store.KeyRadarEnabled:        true,
		store.KeyRadarCron:           true,
		store.KeyRadarHNMinScore:     true,
		store.KeyRadarLimit:          true,
		store.KeyAssetWatcherEnabled: true,
		store.KeyAssetWatcherCron:    true,
		store.KeyCFAPIToken:          true,
		store.KeyPushPlusToken:       true,
	}

	for k, v := range updates {
		if !allowed[k] {
			continue
		}
		val := strings.TrimSpace(v)
		if k == store.KeyMorningCron || k == store.KeyRadarCron || k == store.KeyAssetWatcherCron {
			if _, err := cron.ParseStandard(val); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("Cron 表达式无效 (%s): %v", val, err))
				return
			}
		}
		if k == store.KeyMorningEnabled || k == store.KeyRadarEnabled || k == store.KeyAssetWatcherEnabled {
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
	// 1. Trust Cloudflare Access Authenticated Email (Zero Trust Passkey / TouchID / FaceID)
	if cfEmail := strings.TrimSpace(r.Header.Get("Cf-Access-Authenticated-User-Email")); cfEmail != "" {
		if strings.EqualFold(cfEmail, "15347290648@163.com") {
			return true
		}
	}

	// 2. Bearer token in Authorization header
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		raw := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if s.verifyPassword(raw) {
			return true
		}
	}

	// 3. Cookie session
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
