package server

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	defaultAndroidPackageName = "uk.starryheavens.vesper.bothub"
	defaultAndroidCertSHA256  = "14:5C:48:AE:54:4F:9B:41:6D:50:BA:E9:1C:F1:F0:9A:D0:CC:B9:F2:51:AD:C8:C7:EE:D2:10:62:E6:F9:68:65"
)

var knownAAGUIDNames = map[string]string{
	"ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4": "Google 通行密钥",
	"fbfc3007-154e-4ecc-8c0b-6e020557d7bd": "iCloud 钥匙串",
	"adce0002-35bc-c60a-648b-0b25f1f05503": "Chrome 平台密钥",
	"08987058-cadc-4b81-b6e1-30de50dcbe96": "Windows Hello",
	"9ddd1817-af5a-4672-a2b9-3e3dd95000a9": "Windows Hello",
	"bada5566-a7aa-401f-bd96-45619a55120d": "1Password",
	"d548826e-79b4-db40-a3d8-11116f7e8349": "Bitwarden",
	"53414d53-554e-4700-0000-000000000000": "Samsung Pass",
}

type passkeyCeremonyEntry struct {
	session   webauthn.SessionData
	rpID      string
	origin    string
	expiresAt time.Time
}

type pairCodeEntry struct {
	expiresAt time.Time
}

type passkeyStateStore struct {
	mu         sync.Mutex
	ceremonies map[string]passkeyCeremonyEntry
	pairCodes  map[string]pairCodeEntry
}

func newPasskeyStateStore() *passkeyStateStore {
	return &passkeyStateStore{
		ceremonies: make(map[string]passkeyCeremonyEntry),
		pairCodes:  make(map[string]pairCodeEntry),
	}
}

func (ps *passkeyStateStore) saveCeremony(sd webauthn.SessionData, rpID, origin string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := "pkc_" + hex.EncodeToString(buf)
	now := time.Now()

	ps.mu.Lock()
	defer ps.mu.Unlock()
	for k, v := range ps.ceremonies {
		if now.After(v.expiresAt) {
			delete(ps.ceremonies, k)
		}
	}
	ps.ceremonies[id] = passkeyCeremonyEntry{
		session:   sd,
		rpID:      rpID,
		origin:    origin,
		expiresAt: now.Add(3 * time.Minute),
	}
	return id, nil
}

func (ps *passkeyStateStore) takeCeremony(id string) (passkeyCeremonyEntry, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	entry, ok := ps.ceremonies[strings.TrimSpace(id)]
	if !ok {
		return passkeyCeremonyEntry{}, false
	}
	delete(ps.ceremonies, strings.TrimSpace(id))
	if time.Now().After(entry.expiresAt) {
		return passkeyCeremonyEntry{}, false
	}
	return entry, true
}

func (ps *passkeyStateStore) createPairCode(ttl time.Duration) (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := "pair_" + base64.RawURLEncoding.EncodeToString(buf)
	now := time.Now()

	ps.mu.Lock()
	defer ps.mu.Unlock()
	for k, v := range ps.pairCodes {
		if now.After(v.expiresAt) {
			delete(ps.pairCodes, k)
		}
	}
	ps.pairCodes[code] = pairCodeEntry{
		expiresAt: now.Add(ttl),
	}
	return code, nil
}

func (ps *passkeyStateStore) consumePairCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	entry, ok := ps.pairCodes[code]
	if !ok {
		return false
	}
	delete(ps.pairCodes, code)
	return time.Now().Before(entry.expiresAt)
}

type bothubWebAuthnUser struct {
	id          []byte
	name        string
	displayName string
	creds       []webauthn.Credential
}

func (u *bothubWebAuthnUser) WebAuthnID() []byte                         { return u.id }
func (u *bothubWebAuthnUser) WebAuthnName() string                       { return u.name }
func (u *bothubWebAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *bothubWebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func (s *Server) loadWebAuthnUser() (*bothubWebAuthnUser, string, error) {
	rawID, b64ID, err := s.store.EnsurePasskeyUserID()
	if err != nil {
		return nil, "", err
	}
	records, err := s.store.ListPasskeys()
	if err != nil {
		return nil, "", err
	}
	creds := make([]webauthn.Credential, 0, len(records))
	for _, r := range records {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(r.CredentialJSON), &c); err == nil {
			creds = append(creds, c)
		}
	}
	return &bothubWebAuthnUser{
		id:          rawID,
		name:        "admin",
		displayName: "Vesper BotHub 主理人",
		creds:       creds,
	}, b64ID, nil
}

func resolveRequestRPAndOrigin(r *http.Request) (rpID string, origin string) {
	if envRP := strings.TrimSpace(os.Getenv("BOTHUB_RP_ID")); envRP != "" {
		rpID = envRP
	}
	hostHeader := strings.TrimSpace(r.Host)
	if fwdHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwdHost != "" {
		if idx := strings.Index(fwdHost, ","); idx > 0 {
			fwdHost = strings.TrimSpace(fwdHost[:idx])
		}
		hostHeader = fwdHost
	}
	hostOnly := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		hostOnly = h
	}
	if rpID == "" {
		if hostOnly == "" || hostOnly == "127.0.0.1" || hostOnly == "::1" {
			rpID = "localhost"
		} else {
			rpID = strings.ToLower(hostOnly)
		}
	}
	scheme := "http"
	if isSecureRequest(r) || (rpID != "localhost" && net.ParseIP(rpID) == nil) {
		scheme = "https"
	}
	if hostHeader == "" {
		origin = scheme + "://" + rpID
	} else if hostOnly == "127.0.0.1" && rpID == "localhost" {
		if _, port, err := net.SplitHostPort(hostHeader); err == nil && port != "" {
			origin = scheme + "://localhost:" + port
		} else {
			origin = scheme + "://localhost"
		}
	} else {
		origin = scheme + "://" + hostHeader
	}
	return rpID, origin
}

func (s *Server) configuredAndroidCertFingerprints() []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 2)
	add := func(fp string) {
		clean := strings.ToUpper(strings.TrimSpace(fp))
		if clean != "" && !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	add(defaultAndroidCertSHA256)
	if custom, err := s.store.GetConfig(store.KeyAndroidCertSHA256); err == nil {
		for _, part := range strings.Split(custom, ",") {
			add(part)
		}
	}
	return out
}

func certHexToAPKKeyHashOrigin(colonHex string) (string, bool) {
	cleanHex := strings.ReplaceAll(strings.TrimSpace(colonHex), ":", "")
	raw, err := hex.DecodeString(cleanHex)
	if err != nil || len(raw) != 32 {
		return "", false
	}
	return "android:apk-key-hash:" + base64.RawURLEncoding.EncodeToString(raw), true
}

func (s *Server) newWebAuthnInstance(rpID, origin string) (*webauthn.WebAuthn, error) {
	opaque := make([]string, 0, 2)
	for _, fp := range s.configuredAndroidCertFingerprints() {
		if apkOrigin, ok := certHexToAPKKeyHashOrigin(fp); ok {
			opaque = append(opaque, apkOrigin)
		}
	}
	cfg := &webauthn.Config{
		RPID:                              rpID,
		RPDisplayName:                     "Vesper BotHub",
		RPOrigins:                         []string{origin},
		RPOpaqueOrigins:                   opaque,
		AttestationPreference:             protocol.PreferNoAttestation,
		ExtensionsUnsolicitedOutputPolicy: protocol.UnsolicitedOutputPolicyIgnore,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationPreferred,
		},
	}
	return webauthn.New(cfg)
}

// handleAssetLinks serves /.well-known/assetlinks.json so Android CredentialManager Passkeys
// and App Links trust uk.starryheavens.vesper.bothub without external configuration.
func (s *Server) handleAssetLinks(w http.ResponseWriter, r *http.Request) {
	fps := s.configuredAndroidCertFingerprints()
	payload := []map[string]any{
		{
			"relation": []string{
				"delegate_permission/common.handle_all_urls",
				"delegate_permission/common.get_login_creds",
			},
			"target": map[string]any{
				"namespace":                "android_app",
				"package_name":             defaultAndroidPackageName,
				"sha256_cert_fingerprints": fps,
			},
		},
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handlePasskeyStatus(w http.ResponseWriter, r *http.Request) {
	rpID, _ := resolveRequestRPAndOrigin(r)
	count, _ := s.store.CountPasskeys()
	writeJSON(w, http.StatusOK, map[string]any{
		"has_passkeys": count > 0,
		"count":        count,
		"rp_id":        rpID,
	})
}

func (s *Server) handlePasskeyRegisterOptions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Promotion bool `json:"promotion"`
	}
	_ = decodeOptionalJSON(w, r, 4096, &body)

	rpID, origin := resolveRequestRPAndOrigin(r)
	wa, err := s.newWebAuthnInstance(rpID, origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "初始化通行密钥引擎失败: "+err.Error())
		return
	}
	user, _, err := s.loadWebAuthnUser()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账户身份失败")
		return
	}

	exclusions := make([]protocol.CredentialDescriptor, 0, len(user.creds))
	for _, c := range user.creds {
		exclusions = append(exclusions, c.Descriptor())
	}

	sel := protocol.AuthenticatorSelection{
		ResidentKey:        protocol.ResidentKeyRequirementRequired,
		RequireResidentKey: protocol.ResidentKeyRequired(),
		UserVerification:   protocol.VerificationPreferred,
	}
	if body.Promotion {
		sel.AuthenticatorAttachment = protocol.Platform
	}

	creation, sessionData, err := wa.BeginRegistration(
		user,
		webauthn.WithAuthenticatorSelection(sel),
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成通行密钥注册挑战失败: "+err.Error())
		return
	}

	ceremonyID, err := s.passkeys.saveCeremony(*sessionData, rpID, origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存挑战状态失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ceremony_id": ceremonyID,
		"publicKey":   creation.Response,
	})
}

func formatAAGUID(raw []byte) string {
	if len(raw) != 16 {
		return "00000000-0000-0000-0000-000000000000"
	}
	h := hex.EncodeToString(raw)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

func inferPasskeyProviderName(aaguid, userAgent string) string {
	if aaguid != "" && aaguid != "00000000-0000-0000-0000-000000000000" {
		if name, ok := knownAAGUIDNames[strings.ToLower(aaguid)]; ok {
			return name
		}
	}
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "okhttp") || strings.Contains(ua, "android"):
		return "Android 面容/指纹密钥"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iPhone / iPad 通行密钥"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		return "Mac 触控 ID 通行密钥"
	case strings.Contains(ua, "windows"):
		return "Windows Hello 通行密钥"
	default:
		return "设备生物识别密钥"
	}
}

func (s *Server) handlePasskeyRegisterVerify(w http.ResponseWriter, r *http.Request) {
	ceremonyID := strings.TrimSpace(r.URL.Query().Get("ceremony_id"))
	entry, ok := s.passkeys.takeCeremony(ceremonyID)
	if !ok {
		writeError(w, http.StatusBadRequest, "注册会话已过期，请重试")
		return
	}

	wa, err := s.newWebAuthnInstance(entry.rpID, entry.origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "通行密钥校验器异常")
		return
	}
	user, _, err := s.loadWebAuthnUser()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账户失败")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	cred, err := wa.FinishRegistration(user, entry.session, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "通行密钥验证失败: "+err.Error())
		return
	}

	credBytes, err := json.Marshal(cred)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "序列化公钥失败")
		return
	}

	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	aaguid := formatAAGUID(cred.Authenticator.AAGUID)
	customName := notifier.StripEmoji(strings.TrimSpace(r.URL.Query().Get("name")))
	if customName == "" {
		customName = inferPasskeyProviderName(aaguid, r.UserAgent())
	}

	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		if s := strings.TrimSpace(string(t)); s != "" {
			transports = append(transports, s)
		}
	}
	deviceType := "singleDevice"
	if cred.Flags.BackupEligible {
		deviceType = "multiDevice"
	}

	rec := store.PasskeyRecord{
		ID:             credID,
		Name:           customName,
		AAGUID:         aaguid,
		CredentialJSON: string(credBytes),
		DeviceType:     deviceType,
		BackedUp:       cred.Flags.BackupState,
		Transports:     transports,
		RegisteredAt:   time.Now().Unix(),
	}
	if err := s.store.SavePasskey(rec); err != nil {
		writeError(w, http.StatusInternalServerError, "保存通行密钥失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"rpId":       entry.rpID,
		"credential": rec,
	})
}

func (s *Server) handlePasskeyLoginOptions(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter != nil && s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}

	rpID, origin := resolveRequestRPAndOrigin(r)
	wa, err := s.newWebAuthnInstance(rpID, origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "初始化通行密钥引擎失败")
		return
	}

	assertion, sessionData, err := wa.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationPreferred),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成登录挑战失败")
		return
	}

	ceremonyID, err := s.passkeys.saveCeremony(*sessionData, rpID, origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存挑战失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ceremony_id": ceremonyID,
		"publicKey":   assertion.Response,
	})
}

func (s *Server) handlePasskeyLoginVerify(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter != nil && s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}

	ceremonyID := strings.TrimSpace(r.URL.Query().Get("ceremony_id"))
	entry, ok := s.passkeys.takeCeremony(ceremonyID)
	if !ok {
		writeError(w, http.StatusBadRequest, "登录挑战已过期，请重新触发验证")
		return
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取验证载荷失败")
		return
	}

	var peek struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rawBody, &peek); err != nil || strings.TrimSpace(peek.ID) == "" {
		writeError(w, http.StatusBadRequest, "无效通行密钥响应格式")
		return
	}
	if _, err := s.store.GetPasskey(peek.ID); errors.Is(err, sql.ErrNoRows) {
		// Return explicit HTTP 404 so browser WebAuthn Signal API cleans up orphaned passkeys
		writeError(w, http.StatusNotFound, "未找到对应的通行密钥记录")
		return
	}

	wa, err := s.newWebAuthnInstance(entry.rpID, entry.origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "通行密钥引擎异常")
		return
	}

	user, _, err := s.loadWebAuthnUser()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "加载账户失败")
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(rawBody))
	_, cred, err := wa.FinishPasskeyLogin(
		func(rawID, userHandle []byte) (webauthn.User, error) {
			if !bytes.Equal(userHandle, user.id) {
				return nil, errors.New("user handle mismatch")
			}
			return user, nil
		},
		entry.session,
		r,
	)
	if err != nil {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusUnauthorized, "生物识别签名校验未通过")
		return
	}
	if s.limiter != nil {
		s.limiter.recordSuccess(ip)
	}

	if updatedBytes, mErr := json.Marshal(cred); mErr == nil {
		credID := base64.RawURLEncoding.EncodeToString(cred.ID)
		_ = s.store.TouchPasskeyUsage(credID, string(updatedBytes), time.Now().Unix())
	}

	// Issue 30-day Web session cookie AND return App Bearer Token for Android CredentialManager clients
	cookieTok, err := s.issueSessionCookieWithTTL(30 * 24 * time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发会话失败")
		return
	}
	writeSessionCookie(w, r, cookieTok, 30*24*3600)

	appToken, err := s.ensureAppBearerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发客户端凭证失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"authenticated": true,
		"token":         appToken,
		"version":       s.version,
		"rpId":          entry.rpID,
	})
}

func (s *Server) handleListPasskeys(w http.ResponseWriter, r *http.Request) {
	rpID, _ := resolveRequestRPAndOrigin(r)
	_, b64UserID, err := s.store.EnsurePasskeyUserID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取通行密钥用户标识失败")
		return
	}
	list, err := s.store.ListPasskeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取通行密钥列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rpId":        rpID,
		"userId":      b64UserID,
		"credentials": list,
	})
}

func (s *Server) handleRenamePasskey(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, 4096, &body) {
		return
	}
	cleanName := notifier.StripEmoji(strings.TrimSpace(body.Name))
	if cleanName == "" {
		writeError(w, http.StatusBadRequest, "名称不能为空")
		return
	}
	if err := s.store.RenamePasskey(id, cleanName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "通行密钥不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "更新名称失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": cleanName})
}

func (s *Server) handleDeletePasskey(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if err := s.store.DeletePasskey(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "通行密钥不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "删除通行密钥失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCreatePairCode issues a 90-second single-use pairing code so a mobile browser
// can wake and bind the native Android app via bothub://pair without typing any password or token.
func (s *Server) handleCreatePairCode(w http.ResponseWriter, r *http.Request) {
	code, err := s.passkeys.createPairCode(90 * time.Second)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成配对码失败")
		return
	}
	_, origin := resolveRequestRPAndOrigin(r)
	deepLink := fmt.Sprintf("bothub://pair?url=%s&code=%s", url.QueryEscape(origin), url.QueryEscape(code))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"code":       code,
		"expires_in": 90,
		"deep_link":  deepLink,
	})
}

// handleExchangePairCode atomically consumes a single-use pairing code and returns the App Bearer Token.
func (s *Server) handleExchangePairCode(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limiter != nil && s.limiter.isLocked(ip) {
		writeError(w, http.StatusTooManyRequests, "尝试过于频繁，请稍后再试")
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, 4096, &body) {
		return
	}
	if !s.passkeys.consumePairCode(body.Code) {
		if s.limiter != nil {
			s.limiter.recordFail(ip)
		}
		writeError(w, http.StatusUnauthorized, "配对码无效或已过期，请在网页端重新发起一键绑定")
		return
	}
	if s.limiter != nil {
		s.limiter.recordSuccess(ip)
	}
	appToken, err := s.ensureAppBearerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发客户端凭证失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"token":   appToken,
		"version": s.version,
	})
}
