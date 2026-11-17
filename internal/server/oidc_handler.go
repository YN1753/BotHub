package server

import (
	"context"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

const (
	oidcProviderDisplayName = "Vesper Auth"
	oidcBindCookieName      = "bothub_oidc_bind"
	oidcStateTTL            = 10 * time.Minute
	oidcMetadataCacheTTL    = 15 * time.Minute
	maxPendingOIDCStates    = 256
)

type oidcRuntimeConfig struct {
	Enabled      bool
	Issuer       string
	ClientID     string
	ClientSecret string
	AllowedSubs  []string
}

type oidcDiscoveryDoc struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

type oidcDiscoveryCacheEntry struct {
	doc       oidcDiscoveryDoc
	expiresAt time.Time
}

type jwkKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type oidcJWKSCacheEntry struct {
	keys      []jwkKey
	expiresAt time.Time
}

type oidcPendingState struct {
	Platform     string
	ReturnTo     string
	Origin       string
	RedirectURI  string
	Nonce        string
	CodeVerifier string
	BindToken    string
	ExpiresAt    time.Time
}

type oidcStateManager struct {
	mu             sync.Mutex
	pending        map[string]oidcPendingState
	discoveryCache map[string]oidcDiscoveryCacheEntry
	jwksCache      map[string]oidcJWKSCacheEntry
	httpClient     *http.Client
}

func newOIDCStateManager() *oidcStateManager {
	return &oidcStateManager{
		pending:        make(map[string]oidcPendingState),
		discoveryCache: make(map[string]oidcDiscoveryCacheEntry),
		jwksCache:      make(map[string]oidcJWKSCacheEntry),
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func randomBase64URL(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func computePKCES256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (m *oidcStateManager) saveState(state string, entry oidcPendingState) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range m.pending {
		if now.After(v.ExpiresAt) {
			delete(m.pending, k)
		}
	}
	for len(m.pending) >= maxPendingOIDCStates {
		var oldestKey string
		var oldestExp time.Time
		for k, v := range m.pending {
			if oldestKey == "" || v.ExpiresAt.Before(oldestExp) {
				oldestKey = k
				oldestExp = v.ExpiresAt
			}
		}
		if oldestKey == "" {
			break
		}
		delete(m.pending, oldestKey)
	}
	m.pending[state] = entry
}

func (m *oidcStateManager) consumeState(state string) (oidcPendingState, bool) {
	clean := strings.TrimSpace(state)
	if clean == "" {
		return oidcPendingState{}, false
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, v := range m.pending {
		if now.After(v.ExpiresAt) {
			delete(m.pending, k)
		}
	}
	entry, ok := m.pending[clean]
	if !ok {
		return oidcPendingState{}, false
	}
	delete(m.pending, clean)
	if now.After(entry.ExpiresAt) {
		return oidcPendingState{}, false
	}
	return entry, true
}

func (m *oidcStateManager) fetchDiscovery(ctx context.Context, issuer string) (oidcDiscoveryDoc, error) {
	normIssuer := strings.TrimRight(strings.TrimSpace(issuer), "/")
	now := time.Now()

	m.mu.Lock()
	if cached, ok := m.discoveryCache[normIssuer]; ok && now.Before(cached.expiresAt) {
		doc := cached.doc
		m.mu.Unlock()
		return doc, nil
	}
	client := m.httpClient
	m.mu.Unlock()

	wellKnownURL := normIssuer + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return oidcDiscoveryDoc{}, fmt.Errorf("构造 OIDC 发现请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return oidcDiscoveryDoc{}, fmt.Errorf("拉取 OIDC 发现文档失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return oidcDiscoveryDoc{}, fmt.Errorf("OIDC 发现端点响应异常 (%d)", resp.StatusCode)
	}

	var doc oidcDiscoveryDoc
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&doc); err != nil {
		return oidcDiscoveryDoc{}, fmt.Errorf("解析 OIDC 发现文档失败: %w", err)
	}
	if !validateHTTPURL(doc.AuthorizationEndpoint) || doc.AuthorizationEndpoint == "" ||
		!validateHTTPURL(doc.TokenEndpoint) || doc.TokenEndpoint == "" ||
		!validateHTTPURL(doc.JWKSURI) || doc.JWKSURI == "" {
		return oidcDiscoveryDoc{}, errors.New("OIDC 发现文档缺少有效核心端点")
	}

	m.mu.Lock()
	m.discoveryCache[normIssuer] = oidcDiscoveryCacheEntry{
		doc:       doc,
		expiresAt: time.Now().Add(oidcMetadataCacheTTL),
	}
	m.mu.Unlock()
	return doc, nil
}

func (m *oidcStateManager) fetchJWKS(ctx context.Context, jwksURI string, forceRefresh bool) ( keys []jwkKey, fromCache bool, err error) {
	cleanURI := strings.TrimSpace(jwksURI)
	now := time.Now()

	m.mu.Lock()
	if !forceRefresh {
		if cached, ok := m.jwksCache[cleanURI]; ok && now.Before(cached.expiresAt) {
			cp := slices.Clone(cached.keys)
			m.mu.Unlock()
			return cp, true, nil
		}
	}
	client := m.httpClient
	m.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURI, nil)
	if err != nil {
		return nil, false, fmt.Errorf("构造 JWKS 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("拉取 JWKS 公钥集失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("JWKS 端点响应异常 (%d)", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&doc); err != nil {
		return nil, false, fmt.Errorf("解析 JWKS 公钥集失败: %w", err)
	}
	if len(doc.Keys) == 0 {
		return nil, false, errors.New("JWKS 公钥集为空")
	}

	m.mu.Lock()
	m.jwksCache[cleanURI] = oidcJWKSCacheEntry{
		keys:      slices.Clone(doc.Keys),
		expiresAt: time.Now().Add(oidcMetadataCacheTTL),
	}
	m.mu.Unlock()
	return doc.Keys, false, nil
}

func (s *Server) loadOIDCConfig() oidcRuntimeConfig {
	enabledRaw, _ := s.store.GetConfig(store.KeyOIDCEnabled)
	issuerRaw, _ := s.store.GetConfig(store.KeyOIDCIssuer)
	clientIDRaw, _ := s.store.GetConfig(store.KeyOIDCClientID)
	clientSecretRaw, _ := s.store.GetConfig(store.KeyOIDCClientSecret)
	allowedSubRaw, _ := s.store.GetConfig(store.KeyOIDCAllowedSub)

	issuer := strings.TrimRight(strings.TrimSpace(issuerRaw), "/")
	clientID := strings.TrimSpace(clientIDRaw)
	clientSecret := strings.TrimSpace(clientSecretRaw)

	allowedSubs := make([]string, 0, 2)
	for _, part := range strings.Split(allowedSubRaw, ",") {
		if sub := strings.TrimSpace(part); sub != "" {
			allowedSubs = append(allowedSubs, sub)
		}
	}

	enabled := strings.EqualFold(strings.TrimSpace(enabledRaw), "true") &&
		issuer != "" &&
		validateHTTPURL(issuer) &&
		clientID != ""

	return oidcRuntimeConfig{
		Enabled:      enabled,
		Issuer:       issuer,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AllowedSubs:  allowedSubs,
	}
}

func resolveExternalOrigin(r *http.Request) string {
	scheme := "http"
	if isSecureRequest(r) {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if isTrustedProxyPeer(r.RemoteAddr) {
		if fwdHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwdHost != "" {
			if idx := strings.Index(fwdHost, ","); idx > 0 {
				fwdHost = strings.TrimSpace(fwdHost[:idx])
			}
			if fwdHost != "" {
				host = fwdHost
			}
		}
	}
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

// validateSafeRelativeReturnTo enforces single-slash internal relative paths and rejects
// protocol-relative "//", backslashes, control characters, and external hosts/schemes.
func validateSafeRelativeReturnTo(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "/", true
	}
	if !strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "//") {
		return "", false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "\\") || strings.Contains(lower, "%5c") || strings.ContainsAny(trimmed, "\r\n\t") {
		return "", false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return "", false
	}
	if strings.HasPrefix(parsed.Path, "/api/auth/oidc/") || parsed.Path == "/auth/callback" {
		return "/", true
	}
	return trimmed, true
}

func writeOIDCBindCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcBindCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func (s *Server) handleOIDCStatus(w http.ResponseWriter, r *http.Request) {
	cfg := s.loadOIDCConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated":      s.isAuthenticated(r),
		"oidc_enabled":       cfg.Enabled,
		"oidc_provider_name": oidcProviderDisplayName,
	})
}

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	cfg := s.loadOIDCConfig()
	if !cfg.Enabled {
		writeError(w, http.StatusServiceUnavailable, "Vesper Auth 单点登录尚未启用")
		return
	}

	platform := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	if platform == "" {
		platform = "web"
	}
	if platform != "web" && platform != "android" {
		writeError(w, http.StatusBadRequest, "不支持的 platform 参数")
		return
	}

	returnTo, ok := validateSafeRelativeReturnTo(r.URL.Query().Get("return_to"))
	if !ok {
		writeError(w, http.StatusBadRequest, "非法的 return_to 站内跳转路径")
		return
	}

	discovery, err := s.oidc.fetchDiscovery(r.Context(), cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusBadGateway, "无法连接 Vesper Auth 认证中心: "+err.Error())
		return
	}

	state, err := randomBase64URL(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成安全状态失败")
		return
	}
	nonce, err := randomBase64URL(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成随机挑战失败")
		return
	}
	codeVerifier, err := randomBase64URL(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成 PKCE 校验器失败")
		return
	}
	bindToken, err := randomBase64URL(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成会话绑定令牌失败")
		return
	}

	origin := resolveExternalOrigin(r)
	redirectURI := origin + "/api/auth/oidc/callback"

	s.oidc.saveState(state, oidcPendingState{
		Platform:     platform,
		ReturnTo:     returnTo,
		Origin:       origin,
		RedirectURI:  redirectURI,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		BindToken:    bindToken,
		ExpiresAt:    time.Now().Add(oidcStateTTL),
	})

	writeOIDCBindCookie(w, r, bindToken, int(oidcStateTTL.Seconds()))

	authURL, err := url.Parse(discovery.AuthorizationEndpoint)
	if err != nil {
		writeError(w, http.StatusBadGateway, "认证中心授权地址无效")
		return
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid profile email")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", computePKCES256Challenge(codeVerifier))
	q.Set("code_challenge_method", "S256")
	authURL.RawQuery = q.Encode()

	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

type oidcAudienceList []string

func (a *oidcAudienceList) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if strings.TrimSpace(single) != "" {
			*a = []string{single}
		} else {
			*a = nil
		}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(data, &multi); err != nil {
		return err
	}
	*a = multi
	return nil
}

type oidcJWTHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

type oidcIDTokenClaims struct {
	Iss   string           `json:"iss"`
	Sub   string           `json:"sub"`
	Aud   oidcAudienceList `json:"aud"`
	Azp   string           `json:"azp"`
	Exp   int64            `json:"exp"`
	Nbf   int64            `json:"nbf"`
	Iat   int64            `json:"iat"`
	Nonce string           `json:"nonce"`
	Email string           `json:"email"`
}

func selectMatchingJWK(keys []jwkKey, alg, kid string) (jwkKey, bool) {
	wantKty := ""
	switch alg {
	case "RS256":
		wantKty = "RSA"
	case "ES256":
		wantKty = "EC"
	case "EdDSA":
		wantKty = "OKP"
	default:
		return jwkKey{}, false
	}

	if kid != "" {
		for _, k := range keys {
			if k.Kid != kid || k.Kty != wantKty {
				continue
			}
			if k.Use != "" && k.Use != "sig" {
				continue
			}
			if k.Alg != "" && k.Alg != alg {
				continue
			}
			return k, true
		}
		return jwkKey{}, false
	}

	var candidates []jwkKey
	for _, k := range keys {
		if k.Kty != wantKty {
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if k.Alg != "" && k.Alg != alg {
			continue
		}
		candidates = append(candidates, k)
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	return jwkKey{}, false
}

func verifyJWSSignature(jwk jwkKey, alg, signingInput string, sig []byte) error {
	switch alg {
	case "RS256":
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil || len(nBytes) == 0 {
			return errors.New("RSA 模数解码失败")
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil || len(eBytes) == 0 || len(eBytes) > 8 {
			return errors.New("RSA 指数解码失败")
		}
		n := new(big.Int).SetBytes(nBytes)
		if n.BitLen() < 2048 {
			return errors.New("RSA 密钥长度不足 2048 位")
		}
		var eInt int
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}
		if eInt < 3 || eInt%2 == 0 {
			return errors.New("RSA 指数不合法")
		}
		pub := &rsa.PublicKey{N: n, E: eInt}
		digest := sha256.Sum256([]byte(signingInput))
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
			return errors.New("RS256 签名验证未通过")
		}
		return nil

	case "ES256":
		if jwk.Crv != "P-256" {
			return errors.New("ES256 仅支持 P-256 曲线")
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil || len(xBytes) == 0 || len(xBytes) > 32 {
			return errors.New("EC 公钥 X 坐标无效")
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
		if err != nil || len(yBytes) == 0 || len(yBytes) > 32 {
			return errors.New("EC 公钥 Y 坐标无效")
		}
		uncompressed := make([]byte, 65)
		uncompressed[0] = 0x04
		copy(uncompressed[1+32-len(xBytes):33], xBytes)
		copy(uncompressed[33+32-len(yBytes):65], yBytes)
		if _, err := ecdh.P256().NewPublicKey(uncompressed); err != nil {
			return errors.New("EC 公钥不在 P-256 曲线上")
		}
		if len(sig) != 64 {
			return errors.New("ES256 签名长度无效")
		}
		pub := &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}
		digest := sha256.Sum256([]byte(signingInput))
		rInt := new(big.Int).SetBytes(sig[:32])
		sInt := new(big.Int).SetBytes(sig[32:64])
		if !ecdsa.Verify(pub, digest[:], rInt, sInt) {
			return errors.New("ES256 签名验证未通过")
		}
		return nil

	case "EdDSA":
		if jwk.Crv != "Ed25519" {
			return errors.New("EdDSA 仅支持 Ed25519 曲线")
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil || len(xBytes) != ed25519.PublicKeySize {
			return errors.New("Ed25519 公钥长度无效")
		}
		if len(sig) != ed25519.SignatureSize {
			return errors.New("Ed25519 签名长度无效")
		}
		if !ed25519.Verify(ed25519.PublicKey(xBytes), []byte(signingInput), sig) {
			return errors.New("EdDSA 签名验证未通过")
		}
		return nil

	default:
		return errors.New("不支持的 JWS 签名算法")
	}
}

func (s *Server) verifyOIDCIDToken(
	ctx context.Context,
	rawToken string,
	cfg oidcRuntimeConfig,
	discovery oidcDiscoveryDoc,
	expectedNonce string,
) (oidcIDTokenClaims, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return oidcIDTokenClaims{}, errors.New("ID Token 格式不合法")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcIDTokenClaims{}, errors.New("ID Token Header 解码失败")
	}
	var hdr oidcJWTHeader
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return oidcIDTokenClaims{}, errors.New("ID Token Header 解析失败")
	}
	if hdr.Alg != "RS256" && hdr.Alg != "ES256" && hdr.Alg != "EdDSA" {
		return oidcIDTokenClaims{}, fmt.Errorf("不支持的签名算法 %q", hdr.Alg)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return oidcIDTokenClaims{}, errors.New("ID Token 签名解码失败")
	}

	keys, fromCache, err := s.oidc.fetchJWKS(ctx, discovery.JWKSURI, false)
	if err != nil {
		return oidcIDTokenClaims{}, err
	}
	matchedJWK, found := selectMatchingJWK(keys, hdr.Alg, hdr.Kid)
	if !found && fromCache {
		// Key rotation resilience: force-refresh JWKS once when kid misses cached keys
		keys, _, err = s.oidc.fetchJWKS(ctx, discovery.JWKSURI, true)
		if err != nil {
			return oidcIDTokenClaims{}, err
		}
		matchedJWK, found = selectMatchingJWK(keys, hdr.Alg, hdr.Kid)
	}
	if !found {
		return oidcIDTokenClaims{}, errors.New("未找到匹配的 JWKS 签名公钥")
	}

	signingInput := parts[0] + "." + parts[1]
	if err := verifyJWSSignature(matchedJWK, hdr.Alg, signingInput, sigBytes); err != nil {
		return oidcIDTokenClaims{}, err
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcIDTokenClaims{}, errors.New("ID Token 载荷解码失败")
	}
	var claims oidcIDTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return oidcIDTokenClaims{}, errors.New("ID Token 载荷解析失败")
	}

	normClaimIss := strings.TrimRight(strings.TrimSpace(claims.Iss), "/")
	if normClaimIss == "" || normClaimIss != cfg.Issuer {
		return oidcIDTokenClaims{}, errors.New("ID Token 签发方 (iss) 不匹配")
	}
	if strings.TrimSpace(discovery.Issuer) != "" && claims.Iss != discovery.Issuer {
		return oidcIDTokenClaims{}, errors.New("ID Token 签发方与发现文档不一致")
	}

	audMatched := 0
	for _, aud := range claims.Aud {
		audMatched |= subtle.ConstantTimeCompare([]byte(aud), []byte(cfg.ClientID))
	}
	if audMatched != 1 {
		return oidcIDTokenClaims{}, errors.New("ID Token 受众 (aud) 不匹配")
	}
	if claims.Azp != "" && subtle.ConstantTimeCompare([]byte(claims.Azp), []byte(cfg.ClientID)) != 1 {
		return oidcIDTokenClaims{}, errors.New("ID Token 授权方 (azp) 不匹配")
	}

	nowUnix := time.Now().Unix()
	if claims.Exp <= nowUnix {
		return oidcIDTokenClaims{}, errors.New("ID Token 已过期")
	}
	if claims.Nbf > 0 && claims.Nbf > nowUnix+30 {
		return oidcIDTokenClaims{}, errors.New("ID Token 尚未生效")
	}
	if claims.Nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return oidcIDTokenClaims{}, errors.New("ID Token nonce 校验未通过")
	}
	if strings.TrimSpace(claims.Sub) == "" {
		return oidcIDTokenClaims{}, errors.New("ID Token 缺少主体标识 (sub)")
	}

	return claims, nil
}

func isAllowedOIDCSubject(sub string, allowedSubs []string) bool {
	if sub == "" || len(allowedSubs) == 0 {
		return false
	}
	matched := 0
	subBytes := []byte(sub)
	for _, candidate := range allowedSubs {
		matched |= subtle.ConstantTimeCompare(subBytes, []byte(candidate))
	}
	return matched == 1
}

func (s *Server) exchangeOIDCCodeForIDToken(
	ctx context.Context,
	cfg oidcRuntimeConfig,
	discovery oidcDiscoveryDoc,
	code string,
	entry oidcPendingState,
) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", entry.RedirectURI)
	form.Set("code_verifier", entry.CodeVerifier)

	useBasicAuth := len(discovery.TokenEndpointAuthMethodsSupported) > 0 &&
		slices.Contains(discovery.TokenEndpointAuthMethodsSupported, "client_secret_basic") &&
		!slices.Contains(discovery.TokenEndpointAuthMethodsSupported, "client_secret_post")

	if !useBasicAuth {
		form.Set("client_id", cfg.ClientID)
		if cfg.ClientSecret != "" {
			form.Set("client_secret", cfg.ClientSecret)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("构造令牌交换请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if useBasicAuth {
		req.SetBasicAuth(url.QueryEscape(cfg.ClientID), url.QueryEscape(cfg.ClientSecret))
	}

	s.oidc.mu.Lock()
	client := s.oidc.httpClient
	s.oidc.mu.Unlock()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求认证中心令牌端点失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", errors.New("读取令牌响应失败")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("授权码兑换失败 (%d)", resp.StatusCode)
	}

	var tokResp struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(bodyBytes, &tokResp); err != nil || strings.TrimSpace(tokResp.IDToken) == "" {
		return "", errors.New("认证中心未返回有效 id_token")
	}
	return strings.TrimSpace(tokResp.IDToken), nil
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter != nil && s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "验证尝试过于频繁，请稍后再试")
		return
	}

	if errCode := strings.TrimSpace(r.URL.Query().Get("error")); errCode != "" {
		writeOIDCBindCookie(w, r, "", -1)
		writeError(w, http.StatusUnauthorized, "Vesper Auth 授权未完成: "+errCode)
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if state == "" || code == "" {
		writeOIDCBindCookie(w, r, "", -1)
		writeError(w, http.StatusBadRequest, "缺少 OAuth2 授权回调参数")
		return
	}

	entry, ok := s.oidc.consumeState(state)
	bindCookie, cookieErr := r.Cookie(oidcBindCookieName)
	writeOIDCBindCookie(w, r, "", -1)
	if !ok {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusBadRequest, "登录会话状态无效或已过期，请重新发起登录")
		return
	}
	if cookieErr != nil || subtle.ConstantTimeCompare([]byte(bindCookie.Value), []byte(entry.BindToken)) != 1 {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusForbidden, "登录绑定 Cookie 校验未通过 (CSRF 防护拦截)")
		return
	}

	cfg := s.loadOIDCConfig()
	if !cfg.Enabled {
		writeError(w, http.StatusServiceUnavailable, "Vesper Auth 单点登录尚未启用")
		return
	}

	discovery, err := s.oidc.fetchDiscovery(r.Context(), cfg.Issuer)
	if err != nil {
		writeError(w, http.StatusBadGateway, "读取认证中心元数据失败: "+err.Error())
		return
	}

	rawIDToken, err := s.exchangeOIDCCodeForIDToken(r.Context(), cfg, discovery, code, entry)
	if err != nil {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	claims, err := s.verifyOIDCIDToken(r.Context(), rawIDToken, cfg, discovery, entry.Nonce)
	if err != nil {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusUnauthorized, "身份令牌校验失败: "+err.Error())
		return
	}

	if !isAllowedOIDCSubject(claims.Sub, cfg.AllowedSubs) {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusForbidden, "该 Vesper Auth 账户不在允许访问的主理人白名单内")
		return
	}

	if s.limiter != nil {
		s.limiter.recordSuccess(ip)
	}

	sessionToken, err := s.issueSessionCookieWithTTL(30 * 24 * time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发主会话失败")
		return
	}
	writeSessionCookie(w, r, sessionToken, 30*24*3600)

	if entry.Platform == "android" {
		pairCode, err := s.passkeys.createPairCode(90 * time.Second)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "签发客户端配对码失败")
			return
		}
		deepLink := fmt.Sprintf("bothub://pair?url=%s&code=%s", url.QueryEscape(entry.Origin), url.QueryEscape(pairCode))
		renderAndroidPairBridgePage(w, deepLink)
		return
	}

	safeTarget, valid := validateSafeRelativeReturnTo(entry.ReturnTo)
	if !valid {
		safeTarget = "/"
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, safeTarget, http.StatusFound)
}

func renderAndroidPairBridgePage(w http.ResponseWriter, deepLink string) {
	nonceBuf := make([]byte, 16)
	_, _ = rand.Read(nonceBuf)
	cspNonce := base64.StdEncoding.EncodeToString(nonceBuf)

	w.Header().Set(
		"Content-Security-Policy",
		fmt.Sprintf(
			"default-src 'self'; script-src 'nonce-%s'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'",
			cspNonce,
		),
	)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	escapedHref := html.EscapeString(deepLink)
	jsDeepLinkBytes, _ := json.Marshal(deepLink)

	page := fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
  <meta name="theme-color" content="#080c1c" />
  <title>Vesper Auth · 正在唤起 BotHub</title>
  <style>
    :root { color-scheme: dark; }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      min-height: 100dvh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
      background: radial-gradient(circle at 20%% 15%%, rgba(129, 140, 248, 0.16), transparent 45%%),
                  radial-gradient(circle at 80%% 85%%, rgba(242, 184, 128, 0.16), transparent 45%%),
                  #060913;
      color: #f8fafc;
      font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif;
    }
    .card {
      width: 100%%;
      max-width: 400px;
      border-radius: 22px;
      padding: 28px 24px;
      background: rgba(12, 17, 34, 0.92);
      border: 1px solid rgba(242, 184, 128, 0.28);
      box-shadow: 0 24px 60px rgba(0, 0, 0, 0.55);
      text-align: center;
    }
    .badge {
      display: inline-block;
      padding: 4px 12px;
      border-radius: 999px;
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 11px;
      letter-spacing: 0.08em;
      color: #ffe2be;
      background: rgba(242, 184, 128, 0.12);
      border: 1px solid rgba(242, 184, 128, 0.32);
      margin-bottom: 16px;
    }
    h1 {
      font-size: 20px;
      font-weight: 600;
      letter-spacing: -0.01em;
      margin-bottom: 8px;
    }
    p {
      font-size: 13px;
      line-height: 1.6;
      color: #94a3b8;
      margin-bottom: 22px;
    }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 100%%;
      min-height: 48px;
      padding: 12px 18px;
      border-radius: 14px;
      background: linear-gradient(135deg, #ffe2be 0%%, #f2b880 55%%, #dea063 100%%);
      color: #090d1a;
      font-size: 14px;
      font-weight: 600;
      text-decoration: none;
      box-shadow: 0 8px 24px rgba(242, 184, 128, 0.25);
    }
    .sub {
      display: inline-block;
      margin-top: 14px;
      font-size: 12px;
      color: #64748b;
      text-decoration: none;
    }
  </style>
</head>
<body>
  <main class="card">
    <span class="badge">VESPER AUTH VERIFIED</span>
    <h1>身份核验通过，正在拉起客户端</h1>
    <p>已签发单次加密配对凭证。若系统未自动切回客户端，请点击下方主按钮完成免密激活。</p>
    <a class="btn" href="%s">打开 BotHub 完成绑定</a>
    <a class="sub" href="/">留在网页端进入工作台</a>
  </main>
  <script nonce="%s">
    window.location.assign(%s);
  </script>
</body>
</html>`, escapedHref, cspNonce, string(jsDeepLinkBytes))

	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, page)
}
