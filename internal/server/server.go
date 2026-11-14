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
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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
	limiter   *rateLimiter
}

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]int
	locked   map[string]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string]int),
		locked:   make(map[string]time.Time),
	}
}

func (rl *rateLimiter) isLocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if t, ok := rl.locked[ip]; ok {
		if time.Now().Before(t) {
			return true
		}
		delete(rl.locked, ip)
		delete(rl.attempts, ip)
	}
	return false
}

func (rl *rateLimiter) recordFail(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[ip]++
	if rl.attempts[ip] >= 5 {
		rl.locked[ip] = time.Now().Add(3 * time.Minute)
	}
}

func (rl *rateLimiter) recordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
	delete(rl.locked, ip)
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
		limiter:   newRateLimiter(),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Public Sanitized Read-only Assets Overview (CORS)
	mux.HandleFunc("OPTIONS /api/v1/assets/overview", s.handleCORSPreflight)
	mux.HandleFunc("GET /api/v1/assets/overview", s.handlePublicAssetsOverview)

	// Authenticated Management Endpoints
	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("POST /api/config", s.requireAuth(s.handleUpdateConfig))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleUpdatePassword))
	mux.HandleFunc("POST /api/modules/{id}/run", s.requireAuth(s.handleRunModule))
	mux.HandleFunc("POST /api/test-push", s.requireAuth(s.handleTestPush))
	mux.HandleFunc("GET /api/assets", s.requireAuth(s.handleListAssets))
	mux.HandleFunc("POST /api/assets", s.requireAuth(s.handleCreateAsset))
	mux.HandleFunc("POST /api/assets/{id}", s.requireAuth(s.handleUpdateAsset))
	mux.HandleFunc("DELETE /api/assets/{id}", s.requireAuth(s.handleDeleteAsset))

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
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePublicAssetsOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	list, err := s.store.GetPublicAssets()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": 1, "message": err.Error(), "data": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": list, "updated_at": store.NowShanghai()})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": s.isAuthenticated(r)})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "密码尝试过于频繁，已锁定 3 分钟")
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || !s.verifyPassword(body.Password) {
		s.limiter.recordFail(ip)
		writeError(w, http.StatusUnauthorized, "访问密钥不正确")
		return
	}
	s.limiter.recordSuccess(ip)

	token, err := s.issueSessionCookie()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "会话签发失败")
		return
	}
	isSecure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
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
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)

	logs, _ := s.store.ListPushLogs(40)
	assets, _ := s.store.ListAssets()
	stats, _ := s.store.CountStats()

	writeJSON(w, http.StatusOK, map[string]any{
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
		"modules":    mods,
		"configs":    cfgs,
		"assets":     assets,
		"push_logs":  logs,
		"stats":      stats,
		"uptime_sec": int64(time.Since(s.startedAt).Seconds()),
		"server_now": store.NowShanghai(),
	})
}

func (s *Server) handleListAssets(w http.ResponseWriter, r *http.Request) {
	assets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetAsset(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "资产不存在")
		return
	}

	var req store.Asset
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	if req.Name != "" { existing.Name = req.Name }
	if req.Category != "" { existing.Category = req.Category }
	if req.Spec != "" { existing.Spec = req.Spec }
	if req.Price != "" { existing.Price = req.Price }
	if req.Status != "" { existing.Status = req.Status }
	if req.DueDate != "" { existing.DueDate = req.DueDate }
	existing.AutoRenew = req.AutoRenew
	if req.WarnDays != "" { existing.WarnDays = req.WarnDays }
	if req.ExtraConfig != "" {
		var js map[string]any
		if json.Unmarshal([]byte(req.ExtraConfig), &js) != nil {
			writeError(w, http.StatusBadRequest, "额外配置需为合法 JSON")
			return
		}
		existing.ExtraConfig = req.ExtraConfig
	}
	existing.LastCheckedAt = store.NowShanghai()

	if err := s.store.SaveAsset(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "保存资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var req store.Asset
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "资产 ID 与名称为必填项")
		return
	}
	existing, _ := s.store.GetAsset(req.ID)
	if existing != nil {
		writeError(w, http.StatusConflict, "资产 ID 已存在")
		return
	}
	if req.Category == "" {
		req.Category = "compute"
	}
	if req.WarnDays == "" {
		req.WarnDays = "[7, 3, 1]"
	}
	if req.ExtraConfig == "" {
		req.ExtraConfig = "{}"
	} else {
		var js map[string]any
		if json.Unmarshal([]byte(req.ExtraConfig), &js) != nil {
			writeError(w, http.StatusBadRequest, "额外配置需为合法 JSON")
			return
		}
	}
	req.LastCheckedAt = store.NowShanghai()
	if err := s.store.SaveAsset(req); err != nil {
		writeError(w, http.StatusInternalServerError, "创建资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少资产 ID")
		return
	}
	if err := s.store.DeleteAsset(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除资产失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "无效 JSON")
		return
	}

	for k, v := range updates {
		val := strings.TrimSpace(v)
		if strings.HasSuffix(k, "_cron") {
			if _, err := cron.ParseStandard(val); err != nil {
				writeError(w, http.StatusBadRequest, "Cron 表达式无效: "+val)
				return
			}
		}
		_ = s.store.SetConfig(k, val)
	}

	if err := s.scheduler.Reload(); err != nil {
		writeError(w, http.StatusBadRequest, "重载调度失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || len(strings.TrimSpace(body.NewPassword)) < 6 {
		writeError(w, http.StatusBadRequest, "新密钥至少 6 位")
		return
	}
	_ = s.store.SetConfig(store.KeyAdminPassHash, store.HashPassword(strings.TrimSpace(body.NewPassword)))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRunModule(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	res, err := s.scheduler.RunNow(ctx, r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "summary": res.Summary, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": res.Summary, "items_pushed": res.ItemsPushed})
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

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.isAuthenticated(r) {
			writeError(w, http.StatusUnauthorized, "未授权访问")
			return
		}
		next(w, r)
	}
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	// 1. Cloudflare Access Authenticated Email (Zero Trust Passkey / TouchID / FaceID)
	// Guard against forged header from raw IP attacks: require Cf-Ray presence
	if cfEmail := strings.TrimSpace(r.Header.Get("Cf-Access-Authenticated-User-Email")); cfEmail != "" {
		if r.Header.Get("Cf-Ray") != "" && strings.EqualFold(cfEmail, "15347290648@163.com") {
			return true
		}
	}
	// 2. Bearer token
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if s.verifyPassword(strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))) {
			return true
		}
	}
	// 3. Cookie session
	c, err := r.Cookie(sessionCookieName)
	return err == nil && c.Value != "" && s.verifySessionCookie(c.Value)
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) verifyPassword(raw string) bool {
	stored, err := s.store.GetConfig(store.KeyAdminPassHash)
	return err == nil && stored != "" && subtle.ConstantTimeCompare([]byte(store.HashPassword(raw)), []byte(stored)) == 1
}

func (s *Server) issueSessionCookie() (string, error) {
	secret, err := s.store.GetConfig(store.KeySessionSecret)
	if err != nil || secret == "" {
		return "", fmt.Errorf("missing session secret")
	}
	exp := strconv.FormatInt(time.Now().Add(7*24*time.Hour).Unix(), 10)
	return exp + "." + signHMAC(secret, exp), nil
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
	return err == nil && secret != "" && hmac.Equal([]byte(parts[1]), []byte(signHMAC(secret, parts[0])))
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
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}
