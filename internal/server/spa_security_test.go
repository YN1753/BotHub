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
}
