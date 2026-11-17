package server

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

type mockOIDCIdP struct {
	mu            sync.Mutex
	server        *httptest.Server
	rsaKey        *rsa.PrivateKey
	activeKid     string
	activeAlg     string // "RS256" | "ES256" | "EdDSA"
	ecKey         *ecdsa.PrivateKey
	edPriv        ed25519.PrivateKey
	edPub         ed25519.PublicKey
	clientID      string
	clientSecret  string
	issueSub      string
	jwksHits      int
	issuedCodes   map[string]mockAuthCode
}

type mockAuthCode struct {
	clientID      string
	redirectURI   string
	nonce         string
	codeChallenge string
	sub           string
}

func newMockOIDCIdP(t *testing.T, clientID, clientSecret, defaultSub string) *mockOIDCIdP {
	t.Helper()
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key: %v", err)
	}
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	idp := &mockOIDCIdP{
		rsaKey:       rsaPriv,
		ecKey:        ecPriv,
		edPriv:       edPriv,
		edPub:        edPub,
		activeKid:    "vesper-rsa-v1",
		activeAlg:    "RS256",
		clientID:     clientID,
		clientSecret: clientSecret,
		issueSub:     defaultSub,
		issuedCodes:  make(map[string]mockAuthCode),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := idp.server.URL
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                                base,
			"authorization_endpoint":                base + "/oauth2/authorize",
			"token_endpoint":                        base + "/oauth2/token",
			"jwks_uri":                              base + "/.well-known/jwks.json",
			"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		})
	})

	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		idp.mu.Lock()
		idp.jwksHits++
		kid := idp.activeKid
		alg := idp.activeAlg
		rsaPub := idp.rsaKey.PublicKey
		ecPub := idp.ecKey.PublicKey
		edPubCopy := idp.edPub
		idp.mu.Unlock()

		var keyObj map[string]any
		switch alg {
		case "ES256":
			xBuf := make([]byte, 32)
			yBuf := make([]byte, 32)
			ecPub.X.FillBytes(xBuf)
			ecPub.Y.FillBytes(yBuf)
			keyObj = map[string]any{
				"kty": "EC",
				"use": "sig",
				"alg": "ES256",
				"kid": kid,
				"crv": "P-256",
				"x":   base64.RawURLEncoding.EncodeToString(xBuf),
				"y":   base64.RawURLEncoding.EncodeToString(yBuf),
			}
		case "EdDSA":
			keyObj = map[string]any{
				"kty": "OKP",
				"use": "sig",
				"alg": "EdDSA",
				"kid": kid,
				"crv": "Ed25519",
				"x":   base64.RawURLEncoding.EncodeToString(edPubCopy),
			}
		default:
			eBytes := big.NewInt(int64(rsaPub.E)).Bytes()
			keyObj = map[string]any{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   base64.RawURLEncoding.EncodeToString(rsaPub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"keys": []any{keyObj}})
	})

	mux.HandleFunc("POST /oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		if r.FormValue("grant_type") != "authorization_code" {
			writeError(w, http.StatusBadRequest, "unsupported_grant_type")
			return
		}
		if r.FormValue("client_id") != idp.clientID || r.FormValue("client_secret") != idp.clientSecret {
			writeError(w, http.StatusUnauthorized, "invalid_client")
			return
		}
		code := r.FormValue("code")
		verifier := r.FormValue("code_verifier")

		idp.mu.Lock()
		ac, ok := idp.issuedCodes[code]
		delete(idp.issuedCodes, code)
		idp.mu.Unlock()
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
		if r.FormValue("redirect_uri") != ac.redirectURI {
			writeError(w, http.StatusBadRequest, "redirect_uri_mismatch")
			return
		}
		if computePKCES256Challenge(verifier) != ac.codeChallenge {
			writeError(w, http.StatusBadRequest, "pkce_verification_failed")
			return
		}

		idToken := idp.signIDToken(t, ac.sub, ac.nonce, time.Now().Add(5*time.Minute))
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": "mock_access_token",
			"token_type":   "Bearer",
			"expires_in":   300,
			"id_token":     idToken,
		})
	})

	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

func (idp *mockOIDCIdP) issueCodeFromAuthRedirect(t *testing.T, authLocation string) (code string, state string) {
	t.Helper()
	parsed, err := url.Parse(authLocation)
	if err != nil {
		t.Fatalf("parse authorization redirect URL %q: %v", authLocation, err)
	}
	q := parsed.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("expected PKCE S256 challenge in auth URL, got %v", q)
	}
	if q.Get("scope") != "openid profile email" {
		t.Fatalf("unexpected OIDC scope %q", q.Get("scope"))
	}

	buf, _ := randomBase64URL(16)
	code = "code_" + buf
	state = q.Get("state")

	idp.mu.Lock()
	idp.issuedCodes[code] = mockAuthCode{
		clientID:      q.Get("client_id"),
		redirectURI:   q.Get("redirect_uri"),
		nonce:         q.Get("nonce"),
		codeChallenge: q.Get("code_challenge"),
		sub:           idp.issueSub,
	}
	idp.mu.Unlock()
	return code, state
}

func (idp *mockOIDCIdP) signIDToken(t *testing.T, sub, nonce string, exp time.Time) string {
	t.Helper()
	idp.mu.Lock()
	kid := idp.activeKid
	alg := idp.activeAlg
	rsaPriv := idp.rsaKey
	ecPriv := idp.ecKey
	edPriv := idp.edPriv
	issuer := idp.server.URL
	clientID := idp.clientID
	idp.mu.Unlock()

	hdrBytes, _ := json.Marshal(map[string]any{
		"alg": alg,
		"kid": kid,
		"typ": "JWT",
	})
	claimsBytes, _ := json.Marshal(map[string]any{
		"iss":   issuer,
		"sub":   sub,
		"aud":   clientID,
		"exp":   exp.Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
		"email": "owner@starryheavens.uk",
	})

	signingInput := base64.RawURLEncoding.EncodeToString(hdrBytes) + "." + base64.RawURLEncoding.EncodeToString(claimsBytes)

	var sig []byte
	switch alg {
	case "ES256":
		digest := sha256.Sum256([]byte(signingInput))
		rInt, sInt, err := ecdsa.Sign(rand.Reader, ecPriv, digest[:])
		if err != nil {
			t.Fatalf("sign ES256: %v", err)
		}
		sig = make([]byte, 64)
		rInt.FillBytes(sig[:32])
		sInt.FillBytes(sig[32:64])
	case "EdDSA":
		sig = ed25519.Sign(edPriv, []byte(signingInput))
	default:
		digest := sha256.Sum256([]byte(signingInput))
		var err error
		sig, err = rsa.SignPKCS1v15(rand.Reader, rsaPriv, crypto.SHA256, digest[:])
		if err != nil {
			t.Fatalf("sign RS256: %v", err)
		}
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestOIDCWebAndAndroidFlowsSecurityAndRotation(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	s.staticFS = fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>BotHub</title>")},
	}

	idp := newMockOIDCIdP(t, "vesper-bothub-client", "super-secret-oidc-9999", "owner-sub-001")

	// Test environment variable fallback + sensitive secret redaction in /api/dashboard and /api/config
	t.Setenv("BOTHUB_OIDC_ENABLED", "true")
	t.Setenv("BOTHUB_OIDC_ISSUER", idp.server.URL)
	t.Setenv("BOTHUB_OIDC_CLIENT_ID", "vesper-bothub-client")
	t.Setenv("BOTHUB_OIDC_CLIENT_SECRET", "super-secret-oidc-9999")
	t.Setenv("BOTHUB_OIDC_ALLOWED_SUB", "other-admin, owner-sub-001")

	appSrv := httptest.NewServer(s.Handler())
	defer appSrv.Close()

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Verify /api/auth/status and /api/v2/auth/oidc/status advertise Vesper Auth
	for _, statusPath := range []string{"/api/auth/status", "/api/v2/auth/oidc/status"} {
		resp, err := noRedirectClient.Get(appSrv.URL + statusPath)
		if err != nil {
			t.Fatalf("GET %s: %v", statusPath, err)
		}
		var st map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()
		if st["oidc_enabled"] != true || st["oidc_provider_name"] != "Vesper Auth" {
			t.Fatalf("unexpected %s response: %+v", statusPath, st)
		}
	}

	// 2. Open redirect protection on return_to
	for _, badReturn := range []string{
		"https://evil.example/phish",
		"//evil.example/phish",
		`/\evil.example`,
		"/%5cevil.example",
		"javascript:alert(1)",
	} {
		resp, err := noRedirectClient.Get(appSrv.URL + "/api/auth/oidc/login?return_to=" + url.QueryEscape(badReturn))
		if err != nil {
			t.Fatalf("open redirect test %q: %v", badReturn, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 BadRequest for return_to=%q, got %d", badReturn, resp.StatusCode)
		}
	}

	// 3. Complete Web OIDC + PKCE S256 login flow -> 30-day session cookie + safe return_to redirect
	loginResp, err := noRedirectClient.Get(appSrv.URL + "/api/auth/oidc/login?platform=web&return_to=%2Fops")
	if err != nil {
		t.Fatalf("web oidc login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302 from /api/auth/oidc/login, got %d", loginResp.StatusCode)
	}
	var bindCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == oidcBindCookieName {
			bindCookie = c
		}
	}
	if bindCookie == nil || !bindCookie.HttpOnly || bindCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected HttpOnly SameSite=Lax bind cookie, got %+v", bindCookie)
	}

	code, state := idp.issueCodeFromAuthRedirect(t, loginResp.Header.Get("Location"))

	cbReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/auth/oidc/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(bindCookie)
	cbResp, err := noRedirectClient.Do(cbReq)
	if err != nil {
		t.Fatalf("web oidc callback: %v", err)
	}
	cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusFound || cbResp.Header.Get("Location") != "/ops" {
		t.Fatalf("expected 302 redirect to /ops, got %d (%q)", cbResp.StatusCode, cbResp.Header.Get("Location"))
	}

	var sessionCookie *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == sessionCookieName && c.Value != "" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || sessionCookie.MaxAge != 30*24*3600 {
		t.Fatalf("expected 30-day bothub_session cookie, got %+v", sessionCookie)
	}

	// Verify authenticated dashboard masks oidc_client_secret and saving masked secret preserves real secret
	dashReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/dashboard", nil)
	dashReq.AddCookie(sessionCookie)
	dashResp, err := noRedirectClient.Do(dashReq)
	if err != nil {
		t.Fatalf("GET /api/dashboard: %v", err)
	}
	var dashData struct {
		Configs map[string]string `json:"configs"`
	}
	_ = json.NewDecoder(dashResp.Body).Decode(&dashData)
	dashResp.Body.Close()
	if dashResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /api/dashboard with OIDC session cookie, got %d", dashResp.StatusCode)
	}
	maskedSecret := dashData.Configs[store.KeyOIDCClientSecret]
	if maskedSecret == "super-secret-oidc-9999" || !strings.Contains(maskedSecret, "****") {
		t.Fatalf("expected oidc_client_secret masked in /api/dashboard, got %q", maskedSecret)
	}

	saveCfgReq, _ := http.NewRequest(http.MethodPost, appSrv.URL+"/api/config", bytes.NewBufferString(`{"oidc_client_secret":"`+maskedSecret+`"}`))
	saveCfgReq.AddCookie(sessionCookie)
	saveCfgResp, err := noRedirectClient.Do(saveCfgReq)
	if err != nil {
		t.Fatalf("POST /api/config: %v", err)
	}
	saveCfgResp.Body.Close()
	realSecretAfterSave, _ := s.store.GetConfig(store.KeyOIDCClientSecret)
	if realSecretAfterSave != "super-secret-oidc-9999" {
		t.Fatalf("expected masked save not to overwrite real oidc_client_secret, got %q", realSecretAfterSave)
	}

	// 4. CSRF protection: missing or forged bind cookie on callback must fail with 403, and replaying consumed state must fail with 400
	csrfLoginResp, _ := noRedirectClient.Get(appSrv.URL + "/api/auth/oidc/login?platform=web")
	csrfLoginResp.Body.Close()
	csrfCode, csrfState := idp.issueCodeFromAuthRedirect(t, csrfLoginResp.Header.Get("Location"))

	forgedCbReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/auth/oidc/callback?code="+url.QueryEscape(csrfCode)+"&state="+url.QueryEscape(csrfState), nil)
	forgedCbReq.AddCookie(&http.Cookie{Name: oidcBindCookieName, Value: "forged-bind-token"})
	forgedCbResp, err := noRedirectClient.Do(forgedCbReq)
	if err != nil {
		t.Fatalf("forged callback: %v", err)
	}
	forgedCbResp.Body.Close()
	if forgedCbResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden on CSRF bind cookie mismatch, got %d", forgedCbResp.StatusCode)
	}

	replayCbReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/auth/oidc/callback?code="+url.QueryEscape(csrfCode)+"&state="+url.QueryEscape(csrfState), nil)
	for _, c := range csrfLoginResp.Cookies() {
		if c.Name == oidcBindCookieName {
			replayCbReq.AddCookie(c)
		}
	}
	replayCbResp, _ := noRedirectClient.Do(replayCbReq)
	replayCbResp.Body.Close()
	if replayCbResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 BadRequest on replayed state, got %d", replayCbResp.StatusCode)
	}

	// 5. Unauthorized subject interception: sub outside oidc_allowed_sub must be rejected with 403
	idp.mu.Lock()
	idp.issueSub = "intruder-sub-666"
	idp.mu.Unlock()

	unauthLogin, _ := noRedirectClient.Get(appSrv.URL + "/api/auth/oidc/login?platform=web")
	unauthLogin.Body.Close()
	unauthCode, unauthState := idp.issueCodeFromAuthRedirect(t, unauthLogin.Header.Get("Location"))
	unauthCbReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/auth/oidc/callback?code="+url.QueryEscape(unauthCode)+"&state="+url.QueryEscape(unauthState), nil)
	for _, c := range unauthLogin.Cookies() {
		if c.Name == oidcBindCookieName {
			unauthCbReq.AddCookie(c)
		}
	}
	unauthCbResp, _ := noRedirectClient.Do(unauthCbReq)
	unauthCbResp.Body.Close()
	if unauthCbResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unauthorized sub, got %d", unauthCbResp.StatusCode)
	}

	// 6. JWKS key rotation auto-refresh (switching kid + ES256 algorithm) + Android platform=android pairing bridge
	idp.mu.Lock()
	idp.issueSub = "owner-sub-001"
	idp.activeKid = "vesper-ec-rotated-v2"
	idp.activeAlg = "ES256"
	jwksBefore := idp.jwksHits
	idp.mu.Unlock()

	androidLogin, err := noRedirectClient.Get(appSrv.URL + "/api/v2/auth/oidc/login?platform=android")
	if err != nil {
		t.Fatalf("android oidc login: %v", err)
	}
	androidLogin.Body.Close()
	andCode, andState := idp.issueCodeFromAuthRedirect(t, androidLogin.Header.Get("Location"))

	andCbReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/auth/callback?code="+url.QueryEscape(andCode)+"&state="+url.QueryEscape(andState), nil)
	for _, c := range androidLogin.Cookies() {
		if c.Name == oidcBindCookieName {
			andCbReq.AddCookie(c)
		}
	}
	andCbResp, err := noRedirectClient.Do(andCbReq)
	if err != nil {
		t.Fatalf("android oidc callback: %v", err)
	}
	htmlBytes := new(bytes.Buffer)
	_, _ = htmlBytes.ReadFrom(andCbResp.Body)
	andCbResp.Body.Close()

	if andCbResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 HTML bridge page on platform=android, got %d: %s", andCbResp.StatusCode, htmlBytes.String())
	}

	idp.mu.Lock()
	jwksAfter := idp.jwksHits
	idp.mu.Unlock()
	if jwksAfter <= jwksBefore {
		t.Fatalf("expected JWKS auto-refresh upon rotated kid miss (before=%d, after=%d)", jwksBefore, jwksAfter)
	}

	// Verify downstream nonce CSP was preserved by withSecurityHeaders and matches script nonce
	cspHeader := andCbResp.Header.Get("Content-Security-Policy")
	nonceRe := regexp.MustCompile(`'nonce-([A-Za-z0-9+/=]+)'`)
	nonceMatch := nonceRe.FindStringSubmatch(cspHeader)
	if len(nonceMatch) != 2 {
		t.Fatalf("expected per-request CSP nonce in Content-Security-Policy header, got %q", cspHeader)
	}
	htmlStr := htmlBytes.String()
	if !strings.Contains(htmlStr, `<script nonce="`+nonceMatch[1]+`">`) {
		t.Fatalf("expected matching <script nonce=%q> in HTML bridge page", nonceMatch[1])
	}
	if !strings.Contains(htmlStr, "打开 BotHub 完成绑定") {
		t.Fatalf("expected fallback CTA button in HTML bridge page, got %s", htmlStr)
	}

	hrefRe := regexp.MustCompile(`href="(bothub://pair\?[^"]+)"`)
	hrefMatch := hrefRe.FindStringSubmatch(htmlStr)
	if len(hrefMatch) != 2 {
		t.Fatalf("expected bothub://pair deep link in HTML bridge page, got %s", htmlStr)
	}
	deepURI, err := url.Parse(strings.ReplaceAll(hrefMatch[1], "&amp;", "&"))
	if err != nil {
		t.Fatalf("parse deep link: %v", err)
	}
	pairCode := deepURI.Query().Get("code")
	if !strings.HasPrefix(pairCode, "pair_") {
		t.Fatalf("expected pair_ code in deep link, got %q", pairCode)
	}

	// Exchange single-use pair code via /api/v2/auth/pair/exchange -> receive App Bearer Token
	exPayload, _ := json.Marshal(map[string]string{"code": pairCode})
	exResp, err := noRedirectClient.Post(appSrv.URL+"/api/v2/auth/pair/exchange", "application/json", bytes.NewReader(exPayload))
	if err != nil {
		t.Fatalf("pair exchange: %v", err)
	}
	var exData struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	_ = json.NewDecoder(exResp.Body).Decode(&exData)
	exResp.Body.Close()
	if exResp.StatusCode != http.StatusOK || !exData.OK || !strings.HasPrefix(exData.Token, "bh_") {
		t.Fatalf("expected valid bh_ App Bearer Token from pair exchange, got status=%d data=%+v", exResp.StatusCode, exData)
	}

	// Verify exchanged Bearer token authenticates v2 API
	botsReq, _ := http.NewRequest(http.MethodGet, appSrv.URL+"/api/v2/bots", nil)
	botsReq.Header.Set("Authorization", "Bearer "+exData.Token)
	botsResp, err := noRedirectClient.Do(botsReq)
	if err != nil {
		t.Fatalf("GET /api/v2/bots with exchanged bearer token: %v", err)
	}
	botsResp.Body.Close()
	if botsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK with exchanged App Bearer Token, got %d", botsResp.StatusCode)
	}

	// Replay of single-use pair code must fail
	exReplayResp, _ := noRedirectClient.Post(appSrv.URL+"/api/v2/auth/pair/exchange", "application/json", bytes.NewReader(exPayload))
	exReplayResp.Body.Close()
	if exReplayResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on replayed pair code, got %d", exReplayResp.StatusCode)
	}
}
