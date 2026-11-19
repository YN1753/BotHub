package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type Server struct {
	store              *store.Store
	scheduler          *scheduler.Scheduler
	push               *notifier.Client
	agent              *agent.Engine
	staticFS           fs.FS
	startedAt          time.Time
	version            string
	commit             string
	buildTime          string
	limiter            *rateLimiter
	passkeys           *passkeyStateStore
	oidc               *oidcStateManager
	nezhaAlertMu       sync.Mutex
	nezhaAlertCooldown map[string]time.Time
}

func New(
	st *store.Store,
	sched *scheduler.Scheduler,
	push *notifier.Client,
	staticRoot fs.FS,
	version, commit, buildTime string,
) (*Server, error) {
	sub, err := fs.Sub(staticRoot, "static")
	if err != nil {
		return nil, fmt.Errorf("mount static fs: %w", err)
	}

	eng := agent.NewEngine(st, sched)
	if push != nil {
		eng.SetHub(push.Hub())
	}
	if sched != nil {
		sched.SetBotRoutineExecutor(eng)
		sched.SetLightSummarizer(modules.LightSummarizerFunc(eng.CallLightModel))
	}

	return &Server{
		store:     st,
		scheduler: sched,
		push:      push,
		agent:     eng,
		staticFS:  sub,
		startedAt: time.Now(),
		version:   version,
		commit:    commit,
		buildTime: buildTime,
		limiter:            newRateLimiter(),
		passkeys:           newPasskeyStateStore(),
		oidc:               newOIDCStateManager(),
		nezhaAlertCooldown: make(map[string]time.Time),
	}, nil
}

func (s *Server) Agent() *agent.Engine {
	return s.agent
}

func (s *Server) ClosePushStream() {
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().CloseAll()
	}
}

func (s *Server) KickPushSubscribers() {
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().KickSubscribers()
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public Health, Digital Asset Links & Auth (OIDC + Passkeys + Password + One-Tap App Pairing)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /.well-known/assetlinks.json", s.handleAssetLinks)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("GET /api/v2/auth/oidc/status", s.handleOIDCStatus)
	mux.HandleFunc("GET /api/auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/v2/auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /api/auth/oidc/callback", s.handleOIDCCallback)
	mux.HandleFunc("GET /api/v2/auth/oidc/callback", s.handleOIDCCallback)
	mux.HandleFunc("GET /auth/callback", s.handleOIDCCallback)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /api/auth/logout-all", s.requireAuth(s.handleLogoutAllDevices))
	mux.HandleFunc("GET /api/auth/passkey/status", s.handlePasskeyStatus)
	mux.HandleFunc("POST /api/auth/passkey/login/options", s.handlePasskeyLoginOptions)
	mux.HandleFunc("POST /api/auth/passkey/login/verify", s.handlePasskeyLoginVerify)
	mux.HandleFunc("POST /api/auth/passkey/register/options", s.requireAuth(s.handlePasskeyRegisterOptions))
	mux.HandleFunc("POST /api/auth/passkey/register/verify", s.requireAuth(s.handlePasskeyRegisterVerify))
	mux.HandleFunc("GET /api/v2/auth/passkey/status", s.handlePasskeyStatus)
	mux.HandleFunc("POST /api/v2/auth/passkey/login/options", s.handlePasskeyLoginOptions)
	mux.HandleFunc("POST /api/v2/auth/passkey/login/verify", s.handlePasskeyLoginVerify)
	mux.HandleFunc("POST /api/v2/auth/passkey/register/options", s.requireAuth(s.handlePasskeyRegisterOptions))
	mux.HandleFunc("POST /api/v2/auth/passkey/register/verify", s.requireAuth(s.handlePasskeyRegisterVerify))
	mux.HandleFunc("GET /api/auth/passkeys", s.requireAuth(s.handleListPasskeys))
	mux.HandleFunc("PUT /api/auth/passkeys/{id}", s.requireAuth(s.handleRenamePasskey))
	mux.HandleFunc("DELETE /api/auth/passkeys/{id}", s.requireAuth(s.handleDeletePasskey))
	mux.HandleFunc("POST /api/v2/auth/pair/create", s.requireAuth(s.handleCreatePairCode))
	mux.HandleFunc("POST /api/v2/auth/pair/exchange", s.handleExchangePairCode)

	// Public Sanitized Read-only Assets Overview (CORS for main dashboard)
	mux.HandleFunc("OPTIONS /api/v1/assets/overview", s.handleCORSPreflight)
	mux.HandleFunc("GET /api/v1/assets/overview", s.handlePublicAssetsOverview)

	// Authenticated Management Endpoints
	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("GET /api/v2/dashboard/slice", s.requireAuth(s.handleDashboardSlice))
	mux.HandleFunc("GET /api/v2/dashboard/{slice}", s.requireAuth(s.handleDashboardSlice))
	mux.HandleFunc("POST /api/config", s.requireAuth(s.handleUpdateConfig))
	mux.HandleFunc("GET /api/v2/model-config", s.requireAuth(s.handleGetModelConfig))
	mux.HandleFunc("POST /api/v2/model-config", s.requireAuth(s.handleSaveModelConfig))
	mux.HandleFunc("POST /api/v2/model-pool", s.requireAuth(s.handleSaveModelPool))
	mux.HandleFunc("POST /api/v2/model-pool/test", s.requireAuth(s.handleTestModelTarget))
	mux.HandleFunc("POST /api/v2/model-pool/models", s.requireAuth(s.handleFetchUpstreamModels))
	mux.HandleFunc("POST /api/v2/model-pool/cooldown/clear", s.requireAuth(s.handleClearModelCooldown))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleUpdatePassword))
	mux.HandleFunc("POST /api/modules/{id}/run", s.requireAuth(s.handleRunTask))
	mux.HandleFunc("POST /api/test-push", s.requireAuth(s.handleTestPush))

	// Tasks Management
	mux.HandleFunc("GET /api/task-templates", s.requireAuth(s.handleListTaskTemplates))
	mux.HandleFunc("GET /api/tasks", s.requireAuth(s.handleListTasks))
	mux.HandleFunc("POST /api/tasks", s.requireAuth(s.handleCreateTask))
	mux.HandleFunc("POST /api/tasks/{id}", s.requireAuth(s.handleUpdateTask))
	mux.HandleFunc("DELETE /api/tasks/{id}", s.requireAuth(s.handleDeleteTask))
	mux.HandleFunc("POST /api/tasks/{id}/run", s.requireAuth(s.handleRunTask))

	// Assets Management
	mux.HandleFunc("GET /api/asset-templates", s.requireAuth(s.handleListAssetTemplates))
	mux.HandleFunc("GET /api/v2/asset-categories", s.requireAuth(s.handleListAssetCategories))
	mux.HandleFunc("PUT /api/v2/asset-categories", s.requireAuth(s.handleSaveAssetCategories))
	mux.HandleFunc("GET /api/assets", s.requireAuth(s.handleListAssets))
	mux.HandleFunc("POST /api/assets", s.requireAuth(s.handleCreateAsset))
	mux.HandleFunc("POST /api/assets/{id}", s.requireAuth(s.handleUpdateAsset))
	mux.HandleFunc("DELETE /api/assets/{id}", s.requireAuth(s.handleDeleteAsset))

	// DeepSeek Agent Hub (SSE Chat, Sessions, Memories)
	mux.HandleFunc("POST /api/agent/chat", s.requireAuth(s.handleAgentChat))
	mux.HandleFunc("GET /api/agent/sessions", s.requireAuth(s.handleListAgentSessions))
	mux.HandleFunc("GET /api/agent/sessions/{id}", s.requireAuth(s.handleGetAgentSession))
	mux.HandleFunc("DELETE /api/agent/sessions/{id}", s.requireAuth(s.handleDeleteAgentSession))
	mux.HandleFunc("GET /api/agent/memories", s.requireAuth(s.handleListAgentMemories))
	mux.HandleFunc("POST /api/agent/memories", s.requireAuth(s.handleSaveAgentMemory))
	mux.HandleFunc("POST /api/agent/memories/prune", s.requireAuth(s.handlePruneAgentMemories))
	mux.HandleFunc("POST /api/agent/memories/consolidate", s.requireAuth(s.handleConsolidateAgentMemories))
	mux.HandleFunc("POST /api/agent/memories/{id}", s.requireAuth(s.handleUpdateAgentMemory))
	mux.HandleFunc("DELETE /api/agent/memories/{id}", s.requireAuth(s.handleDeleteAgentMemory))

	// BotHub v2 Custom Skills
	mux.HandleFunc("GET /api/v2/skills", s.requireAuth(s.handleListSkills))
	mux.HandleFunc("POST /api/v2/skills", s.requireAuth(s.handleCreateSkill))
	mux.HandleFunc("POST /api/v2/skills/{id}", s.requireAuth(s.handleUpdateSkill))
	mux.HandleFunc("DELETE /api/v2/skills/{id}", s.requireAuth(s.handleDeleteSkill))
	mux.HandleFunc("POST /api/v2/skills/{id}/test", s.requireAuth(s.handleTestSkill))

	// BotHub v2 Persistent Bots (GrokBot Model) & Isolated Multi-Topic Threads
	mux.HandleFunc("GET /api/v2/bots", s.requireAuth(s.handleListBots))
	mux.HandleFunc("POST /api/v2/bots", s.requireAuth(s.handleCreateBot))
	mux.HandleFunc("POST /api/v2/bots/adhoc-group", s.requireAuth(s.handleCreateAdhocGroup))
	mux.HandleFunc("POST /api/v2/bots/{id}", s.requireAuth(s.handleUpdateBot))
	mux.HandleFunc("POST /api/v2/bots/{id}/members", s.requireAuth(s.handleUpdateGroupMembers))
	mux.HandleFunc("POST /api/v2/bots/{id}/forward", s.requireAuth(s.handleForwardBotMessage))
	mux.HandleFunc("DELETE /api/v2/bots/{id}", s.requireAuth(s.handleDeleteBot))
	mux.HandleFunc("GET /api/v2/bots/{id}/messages", s.requireAuth(s.handleGetBotMessages))
	mux.HandleFunc("DELETE /api/v2/bots/{id}/messages", s.requireAuth(s.handleClearBotMessages))
	mux.HandleFunc("DELETE /api/v2/bots/{id}/messages/{msgId}", s.requireAuth(s.handleDeleteBotMessage))
	mux.HandleFunc("POST /api/v2/bots/{id}/threads", s.requireAuth(s.handleCreateBotThread))
	mux.HandleFunc("POST /api/v2/bots/{id}/threads/fork", s.requireAuth(s.handleForkBotThread))
	mux.HandleFunc("POST /api/v2/bots/{id}/threads/{threadId}", s.requireAuth(s.handleUpdateBotThread))
	mux.HandleFunc("POST /api/v2/bots/{id}/threads/{threadId}/summarize", s.requireAuth(s.handleSummarizeBotThread))
	mux.HandleFunc("DELETE /api/v2/bots/{id}/threads/{threadId}", s.requireAuth(s.handleDeleteBotThread))

	// BotHub v2 Native Push Stream, Inbox & Webhooks
	mux.HandleFunc("GET /api/v2/app/version", s.handleV2AppVersion)
	mux.HandleFunc("GET /api/v2/app/apk", s.handleV2AppApk)
	mux.HandleFunc("GET /api/v2/app/bootstrap", s.requireAuth(s.handleV2AppBootstrap))
	mux.HandleFunc("POST /api/v2/auth/token", s.handleV2AuthToken)
	mux.HandleFunc("POST /api/v2/auth/token/rotate", s.requireAuth(s.handleV2RotateAppToken))
	mux.HandleFunc("POST /api/v2/auth/web-session", s.requireAuth(s.handleV2WebSession))
	mux.HandleFunc("GET /api/v2/events", s.requireAuth(s.handleV2Events))
	mux.HandleFunc("GET /api/v2/notifications", s.requireAuth(s.handleV2ListNotifications))
	mux.HandleFunc("POST /api/v2/notifications/{id}/read", s.requireAuth(s.handleV2MarkNotificationRead))
	mux.HandleFunc("POST /api/v2/notifications/read-all", s.requireAuth(s.handleV2MarkAllNotificationsRead))
	mux.HandleFunc("POST /api/v2/notifications/clear-read", s.requireAuth(s.handleV2ClearReadNotifications))
	mux.HandleFunc("POST /api/v1/webhook/nezha", s.handleNezhaWebhook)
	mux.HandleFunc("POST /api/v2/webhook/nezha", s.handleNezhaWebhook)

	// Personal Assistant Workbench (Todos & Workspace Notes)
	mux.HandleFunc("GET /api/v2/briefing", s.requireAuth(s.handleGetBriefing))
	mux.HandleFunc("GET /api/v2/todos", s.requireAuth(s.handleListTodos))
	mux.HandleFunc("POST /api/v2/todos", s.requireAuth(s.handleCreateTodo))
	mux.HandleFunc("POST /api/v2/todos/clear-done", s.requireAuth(s.handleClearDoneTodos))
	mux.HandleFunc("POST /api/v2/todos/{id}", s.requireAuth(s.handleUpdateTodo))
	mux.HandleFunc("DELETE /api/v2/todos/{id}", s.requireAuth(s.handleDeleteTodo))
	mux.HandleFunc("GET /api/v2/notes", s.requireAuth(s.handleListNotes))
	mux.HandleFunc("POST /api/v2/notes", s.requireAuth(s.handleCreateNote))
	mux.HandleFunc("POST /api/v2/notes/{id}", s.requireAuth(s.handleUpdateNote))
	mux.HandleFunc("DELETE /api/v2/notes/{id}", s.requireAuth(s.handleDeleteNote))

	// Smart Stash / Capture (Links, Bilibili, GitHub & Flash Notes)
	mux.HandleFunc("GET /api/v2/stash", s.requireAuth(s.handleListStash))
	mux.HandleFunc("POST /api/v2/stash", s.requireAuth(s.handleCreateStash))
	mux.HandleFunc("POST /api/v2/stash/{id}", s.requireAuth(s.handleUpdateStash))
	mux.HandleFunc("POST /api/v2/stash/{id}/reprocess", s.requireAuth(s.handleReprocessStash))
	mux.HandleFunc("DELETE /api/v2/stash/{id}", s.requireAuth(s.handleDeleteStash))

	// System Backup & Portability
	mux.HandleFunc("GET /api/system/export", s.requireAuth(s.handleExportBackup))
	mux.HandleFunc("POST /api/system/import", s.requireAuth(s.handleImportBackup))

	// Embedded Static Single Binary SPA Router
	mux.Handle("GET /", s.spaFileServer())

	return withSecurityHeaders(withGzip(mux))
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gw          *gzip.Writer
	wroteHeader bool
	useGzip     bool
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	if code >= 200 && code != http.StatusNoContent && code != http.StatusNotModified && (code < 300 || code >= 400) {
		g.useGzip = true
		g.Header().Del("Content-Length")
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Add("Vary", "Accept-Encoding")
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.useGzip {
		return g.gw.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipResponseWriter) Flush() {
	if g.useGzip {
		_ = g.gw.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func shouldSkipGzip(r *http.Request) bool {
	if r.Method == http.MethodHead {
		return true
	}
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return true
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return true
	}
	p := r.URL.Path
	return p == "/api/v2/events" ||
		p == "/api/events" ||
		p == "/api/agent/chat" ||
		p == "/api/v2/app/apk" ||
		strings.HasSuffix(p, ".apk") ||
		strings.HasSuffix(p, ".png") ||
		strings.HasSuffix(p, ".webp")
}

func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkipGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gw := gzipWriterPool.Get().(*gzip.Writer)
		gw.Reset(w)
		grw := &gzipResponseWriter{ResponseWriter: w, gw: gw}
		defer func() {
			if grw.useGzip {
				_ = gw.Close()
			}
			gzipWriterPool.Put(gw)
		}()

		next.ServeHTTP(grw, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if w.Header().Get("Content-Security-Policy") == "" {
			w.Header().Set(
				"Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; form-action 'self'; base-uri 'self'; frame-ancestors 'self'",
			)
		}
		next.ServeHTTP(w, r)
	})
}
