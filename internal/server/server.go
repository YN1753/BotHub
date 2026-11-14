package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
)

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

func New(
	st *store.Store,
	sched *scheduler.Scheduler,
	push *pushplus.Client,
	staticRoot fs.FS,
	version, commit, buildTime string,
) (*Server, error) {
	sub, err := fs.Sub(staticRoot, "static")
	if err != nil {
		return nil, fmt.Errorf("mount static fs: %w", err)
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

	// Public Health & Auth
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Public Sanitized Read-only Assets Overview (CORS for main dashboard)
	mux.HandleFunc("OPTIONS /api/v1/assets/overview", s.handleCORSPreflight)
	mux.HandleFunc("GET /api/v1/assets/overview", s.handlePublicAssetsOverview)

	// Authenticated Management Endpoints
	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("POST /api/config", s.requireAuth(s.handleUpdateConfig))
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
	mux.HandleFunc("GET /api/assets", s.requireAuth(s.handleListAssets))
	mux.HandleFunc("POST /api/assets", s.requireAuth(s.handleCreateAsset))
	mux.HandleFunc("POST /api/assets/{id}", s.requireAuth(s.handleUpdateAsset))
	mux.HandleFunc("DELETE /api/assets/{id}", s.requireAuth(s.handleDeleteAsset))

	// Embedded Static Single Binary SPA Router
	mux.Handle("GET /", s.spaFileServer())

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
