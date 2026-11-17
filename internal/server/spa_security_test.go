package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestSPAFallback404AndCacheHeadersAndCSP(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	s.staticFS = fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html><title>Vesper BotHub</title>")},
		"favicon.svg":           {Data: []byte("<svg></svg>")},
		"assets/app.1234567.js": {Data: []byte("console.log('ok');")},
	}

	handler := withSecurityHeaders(s.spaFileServer())

	// 1. Missing assets, directory paths (/assets, /assets/), & extensions must return 404 instead of 200 HTML/dir listing
	for _, p := range []string{"/assets/", "/assets", "/assets/nonexist.js", "/favicon.ico", "/apple-touch-icon.png", "/robots.txt"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, p, nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s, got %d (%s)", p, rec.Code, rec.Header().Get("Content-Type"))
		}
	}

	// 2. SPA client routes still resolve to index.html with no-cache + CSP
	recSpa := httptest.NewRecorder()
	handler.ServeHTTP(recSpa, httptest.NewRequest(http.MethodGet, "/ops", nil))
	if recSpa.Code != http.StatusOK {
		t.Fatalf("expected 200 for /ops SPA route, got %d", recSpa.Code)
	}
	if cc := recSpa.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("expected no-cache for SPA HTML, got %q", cc)
	}
	csp := recSpa.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "object-src 'none'") || !strings.Contains(csp, "form-action 'self'") {
		t.Fatalf("expected tightened CSP header, got %q", csp)
	}
	if pp := recSpa.Header().Get("Permissions-Policy"); !strings.Contains(pp, "camera=()") {
		t.Fatalf("expected Permissions-Policy header, got %q", pp)
	}

	// 3. Hashed assets receive 1-year immutable caching
	recAsset := httptest.NewRecorder()
	handler.ServeHTTP(recAsset, httptest.NewRequest(http.MethodGet, "/assets/app.1234567.js", nil))
	if recAsset.Code != http.StatusOK {
		t.Fatalf("expected 200 for existing asset, got %d", recAsset.Code)
	}
	if cc := recAsset.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") || !strings.Contains(cc, "31536000") {
		t.Fatalf("expected immutable 1-year cache header for hashed asset, got %q", cc)
	}
}

func TestLoginRememberDeviceCookieTTL(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	s.limiter = newRateLimiter()
	_ = s.store.SetConfig(store.KeyAdminPassHash, store.HashPassword("test-pass-123"))
	_ = s.store.SetConfig(store.KeySessionSecret, "test-secret-key")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"password":"test-pass-123","remember_device":true}`))
	s.handleLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 login OK, got %d: %s", rec.Code, rec.Body.String())
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "Max-Age=2592000") {
		t.Fatalf("expected 30-day Max-Age=2592000 in Set-Cookie, got %q", setCookie)
	}

	// Verify POST /api/auth/logout-all revokes existing 30-day device cookies while keeping current session valid
	oldCookies := rec.Result().Cookies()
	if len(oldCookies) == 0 {
		t.Fatal("expected session cookie issued on login")
	}
	oldCheckReq := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	oldCheckReq.AddCookie(oldCookies[0])
	if !s.isAuthenticated(oldCheckReq) {
		t.Fatal("expected newly issued 30-day cookie to authenticate before revocation")
	}

	revRec := httptest.NewRecorder()
	revReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout-all", bytes.NewBufferString(`{"keep_current":true}`))
	revReq.AddCookie(oldCookies[0])
	s.requireAuth(s.handleLogoutAllDevices)(revRec, revReq)
	if revRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout-all, got %d: %s", revRec.Code, revRec.Body.String())
	}
	if s.isAuthenticated(oldCheckReq) {
		t.Fatal("expected old 30-day cookie to be revoked immediately after logout-all")
	}
	newCookies := revRec.Result().Cookies()
	if len(newCookies) == 0 {
		t.Fatal("expected fresh session cookie reissued when keep_current=true")
	}
	newCheckReq := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	newCheckReq.AddCookie(newCookies[0])
	if !s.isAuthenticated(newCheckReq) {
		t.Fatal("expected reissued cookie to remain authenticated")
	}

	// Verify fine-grained dashboard slice endpoint returns only requested slices
	_, _ = s.store.SaveTodo(store.TodoItem{Title: "检查证书续期", Priority: store.TodoPriorityHigh})
	sliceRec := httptest.NewRecorder()
	sliceReq := httptest.NewRequest(http.MethodGet, "/api/v2/dashboard/slice?kinds=todos,memories", nil)
	sliceReq.AddCookie(newCookies[0])
	s.Handler().ServeHTTP(sliceRec, sliceReq)
	if sliceRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for dashboard slice, got %d: %s", sliceRec.Code, sliceRec.Body.String())
	}
	bodyStr := sliceRec.Body.String()
	if !strings.Contains(bodyStr, `"todos"`) || !strings.Contains(bodyStr, `"agent_memories"`) || strings.Contains(bodyStr, `"configs"`) {
		t.Fatalf("expected only todos and agent_memories in slice payload, got %s", bodyStr)
	}

	// Verify redacted backup export (?mask_secrets=1) and masked-secret preservation on backup import
	_ = s.store.SetConfig(store.KeyDeepSeekAPIKey, "sk-real-secret-key-9876")
	_ = s.store.SaveModelPool([]store.ModelPoolEntry{
		{ID: "pool_1", Name: "Gemini", Provider: "openai", BaseURL: "https://api.example.com/v1", APIKey: "sk-pool-real-1111", Model: "gemini-2.5-flash", Enabled: true},
	})
	expRec := httptest.NewRecorder()
	expReq := httptest.NewRequest(http.MethodGet, "/api/system/export?mask_secrets=1", nil)
	expReq.AddCookie(newCookies[0])
	s.Handler().ServeHTTP(expRec, expReq)
	if expRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup export, got %d", expRec.Code)
	}
	expBody := expRec.Body.String()
	if strings.Contains(expBody, "sk-real-secret-key-9876") || strings.Contains(expBody, "sk-pool-real-1111") {
		t.Fatalf("expected secrets masked when mask_secrets=1, got %s", expBody)
	}

	// Importing that redacted backup must preserve stored real keys instead of overwriting with "****"
	impRec := httptest.NewRecorder()
	impReq := httptest.NewRequest(http.MethodPost, "/api/system/import", bytes.NewBufferString(expBody))
	impReq.AddCookie(newCookies[0])
	s.Handler().ServeHTTP(impRec, impReq)
	if impRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for backup import, got %d: %s", impRec.Code, impRec.Body.String())
	}
	storedKey, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
	if storedKey != "sk-real-secret-key-9876" {
		t.Fatalf("expected stored deepseek_api_key preserved on masked import, got %q", storedKey)
	}
	poolAfter := s.store.GetModelPool()
	if len(poolAfter) != 1 || poolAfter[0].APIKey != "sk-pool-real-1111" {
		t.Fatalf("expected model pool API key preserved on masked import, got %+v", poolAfter)
	}
}
