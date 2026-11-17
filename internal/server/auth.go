package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

const sessionCookieName = "bothub_session"

// RateLimiter tracks login failures to prevent brute force attacks.
type rateLimiter struct {
	mu       sync.Mutex
	failures map[string]int
	lastFail map[string]time.Time
	lockedAt map[string]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		failures: make(map[string]int),
		lastFail: make(map[string]time.Time),
		lockedAt: make(map[string]time.Time),
	}
}

func (rl *rateLimiter) isLocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if t, ok := rl.lockedAt[ip]; ok {
		if time.Since(t) < 3*time.Minute {
			return true
		}
		delete(rl.lockedAt, ip)
		delete(rl.failures, ip)
		delete(rl.lastFail, ip)
	}
	return false
}

func (rl *rateLimiter) recordFail(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if len(rl.failures) > 128 {
		for k, ts := range rl.lastFail {
			if now.Sub(ts) >= 15*time.Minute {
				delete(rl.lastFail, k)
				delete(rl.failures, k)
				delete(rl.lockedAt, k)
			}
		}
	}
	rl.lastFail[ip] = now
	rl.failures[ip]++
	if rl.failures[ip] >= 5 {
		rl.lockedAt[ip] = now
	}
}

func (rl *rateLimiter) recordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.failures, ip)
	delete(rl.lastFail, ip)
	delete(rl.lockedAt, ip)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	s.handleOIDCStatus(w, r)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "密码尝试过于频繁，已锁定 3 分钟")
		return
	}

	var body struct {
		Password       string `json:"password"`
		RememberDevice bool   `json:"remember_device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil || !s.verifyPassword(body.Password) {
		s.limiter.recordFail(ip)
		writeError(w, http.StatusUnauthorized, "访问密钥不正确")
		return
	}
	s.limiter.recordSuccess(ip)

	ttl := 7 * 24 * time.Hour
	cookieMaxAge := 7 * 24 * 3600
	if body.RememberDevice {
		ttl = 30 * 24 * time.Hour
		cookieMaxAge = 30 * 24 * 3600
	}
	token, err := s.issueSessionCookieWithTTL(ttl)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "会话签发失败")
		return
	}
	writeSessionCookie(w, r, token, cookieMaxAge)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	writeSessionCookie(w, r, "", -1)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (s *Server) rotateSessionSecretAndReissue(w http.ResponseWriter, r *http.Request, keepCurrent bool) error {
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return err
	}
	if err := s.store.SetConfig(store.KeySessionSecret, hex.EncodeToString(secretBytes)); err != nil {
		return err
	}
	s.KickPushSubscribers()

	if !keepCurrent {
		writeSessionCookie(w, r, "", -1)
		return nil
	}
	token, err := s.issueSessionCookie()
	if err != nil {
		return err
	}
	writeSessionCookie(w, r, token, 7*24*3600)
	return nil
}

func (s *Server) handleLogoutAllDevices(w http.ResponseWriter, r *http.Request) {
	var body struct {
		KeepCurrent *bool `json:"keep_current"`
	}
	_ = decodeOptionalJSON(w, r, 4096, &body)
	keepCurrent := true
	if body.KeepCurrent != nil {
		keepCurrent = *body.KeepCurrent
	}
	if err := s.rotateSessionSecretAndReissue(w, r, keepCurrent); err != nil {
		writeError(w, http.StatusInternalServerError, "吊销全部设备会话失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"authenticated": keepCurrent,
	})
}

func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, 4096, &body) {
		return
	}
	if len(strings.TrimSpace(body.NewPassword)) < 6 {
		writeError(w, http.StatusBadRequest, "新密钥至少 6 位")
		return
	}
	_ = s.store.SetConfig(store.KeyAdminPassHash, store.HashPassword(strings.TrimSpace(body.NewPassword)))
	_ = s.rotateSessionSecretAndReissue(w, r, true)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
	if cfEmail := strings.TrimSpace(r.Header.Get("Cf-Access-Authenticated-User-Email")); cfEmail != "" {
		if r.Header.Get("Cf-Ray") != "" && strings.EqualFold(cfEmail, "15347290648@163.com") {
			return true
		}
	}
	// 2. Bearer token (supports admin password OR dedicated App Bearer Token)
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tok := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if s.verifyPassword(tok) || s.verifyAppToken(tok) {
			return true
		}
	}
	// 3. Query token fallback for SSE EventSource connections
	if qTok := strings.TrimSpace(r.URL.Query().Get("token")); qTok != "" {
		if s.verifyPassword(qTok) || s.verifyAppToken(qTok) {
			return true
		}
	}
	// 4. Cookie session
	c, err := r.Cookie(sessionCookieName)
	return err == nil && c.Value != "" && s.verifySessionCookie(c.Value)
}

func (s *Server) verifyAppToken(raw string) bool {
	if raw == "" {
		return false
	}
	stored, err := s.store.GetConfig(store.KeyAppBearerToken)
	return err == nil && stored != "" && subtle.ConstantTimeCompare([]byte(raw), []byte(stored)) == 1
}

func (s *Server) verifyPassword(raw string) bool {
	stored, err := s.store.GetConfig(store.KeyAdminPassHash)
	return err == nil && stored != "" && store.VerifyPassword(raw, stored)
}

func (s *Server) issueSessionCookie() (string, error) {
	return s.issueSessionCookieWithTTL(7 * 24 * time.Hour)
}

func (s *Server) issueSessionCookieWithTTL(ttl time.Duration) (string, error) {
	secret, err := s.store.GetConfig(store.KeySessionSecret)
	if err != nil || secret == "" {
		return "", fmt.Errorf("missing session secret")
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
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
