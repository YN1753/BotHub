package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestAssetLinksPairCodeAndPasskeyFlow(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Digital Asset Links endpoint
	reqAL := httptest.NewRequest(http.MethodGet, "/.well-known/assetlinks.json", nil)
	recAL := httptest.NewRecorder()
	s.handleAssetLinks(recAL, reqAL)
	if recAL.Code != http.StatusOK {
		t.Fatalf("assetlinks status = %d, want 200", recAL.Code)
	}
	if !strings.Contains(recAL.Body.String(), "delegate_permission/common.get_login_creds") ||
		!strings.Contains(recAL.Body.String(), defaultAndroidPackageName) {
		t.Fatalf("unexpected assetlinks payload: %s", recAL.Body.String())
	}

	// 2. Single-use One-Tap Pair Code create & atomic exchange
	reqCreatePair := httptest.NewRequest(http.MethodPost, "/api/v2/auth/pair/create", nil)
	reqCreatePair.Host = "bot.starryheavens.uk"
	recCreatePair := httptest.NewRecorder()
	s.handleCreatePairCode(recCreatePair, reqCreatePair)
	if recCreatePair.Code != http.StatusOK {
		t.Fatalf("pair create status = %d, body = %s", recCreatePair.Code, recCreatePair.Body.String())
	}
	var pairCreated struct {
		Code     string `json:"code"`
		DeepLink string `json:"deep_link"`
	}
	if err := json.Unmarshal(recCreatePair.Body.Bytes(), &pairCreated); err != nil {
		t.Fatalf("decode pair create: %v", err)
	}
	if !strings.HasPrefix(pairCreated.Code, "pair_") || !strings.HasPrefix(pairCreated.DeepLink, "bothub://pair?") {
		t.Fatalf("invalid pair response: %+v", pairCreated)
	}

	exBody, _ := json.Marshal(map[string]string{"code": pairCreated.Code})
	reqEx1 := httptest.NewRequest(http.MethodPost, "/api/v2/auth/pair/exchange", bytes.NewReader(exBody))
	recEx1 := httptest.NewRecorder()
	s.handleExchangePairCode(recEx1, reqEx1)
	if recEx1.Code != http.StatusOK {
		t.Fatalf("first pair exchange failed: %d %s", recEx1.Code, recEx1.Body.String())
	}
	var exResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(recEx1.Body.Bytes(), &exResp)
	if exResp.Token == "" {
		t.Fatalf("expected non-empty app bearer token on pair exchange")
	}

	// Second exchange of the same single-use code MUST fail with 401
	reqEx2 := httptest.NewRequest(http.MethodPost, "/api/v2/auth/pair/exchange", bytes.NewReader(exBody))
	recEx2 := httptest.NewRecorder()
	s.handleExchangePairCode(recEx2, reqEx2)
	if recEx2.Code != http.StatusUnauthorized {
		t.Fatalf("replayed pair code got %d, want 401", recEx2.Code)
	}

	// 3. Passkey options generation & 404 Signal API contract for unknown credential
	reqLoginOpt := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/options", nil)
	reqLoginOpt.Host = "bot.starryheavens.uk"
	recLoginOpt := httptest.NewRecorder()
	s.handlePasskeyLoginOptions(recLoginOpt, reqLoginOpt)
	if recLoginOpt.Code != http.StatusOK {
		t.Fatalf("login options status = %d, body = %s", recLoginOpt.Code, recLoginOpt.Body.String())
	}
	var loginOptResp struct {
		CeremonyID string         `json:"ceremony_id"`
		PublicKey  map[string]any `json:"publicKey"`
	}
	if err := json.Unmarshal(recLoginOpt.Body.Bytes(), &loginOptResp); err != nil || loginOptResp.CeremonyID == "" {
		t.Fatalf("invalid login options: %v %s", err, recLoginOpt.Body.String())
	}

	unknownVerify := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/passkey/login/verify?ceremony_id="+loginOptResp.CeremonyID,
		strings.NewReader(`{"id":"dGVzdC11bmtub3duLWNyZWQ","type":"public-key"}`),
	)
	recUnknown := httptest.NewRecorder()
	s.handlePasskeyLoginVerify(recUnknown, unknownVerify)
	if recUnknown.Code != http.StatusNotFound {
		t.Fatalf("unknown credential verify got %d, want 404 for WebAuthn Signal API", recUnknown.Code)
	}

	// 4. Passkey CRUD & status check
	if err := s.store.SavePasskey(store.PasskeyRecord{
		ID:             "dGVzdC1jcmVkLTE",
		Name:           "Pixel 面容密钥",
		AAGUID:         "ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4",
		CredentialJSON: `{"id":"dGVzdC1jcmVkLTE"}`,
		DeviceType:     "multiDevice",
		BackedUp:       true,
		Transports:     []string{"internal", "hybrid"},
		RegisteredAt:   1700000000,
	}); err != nil {
		t.Fatalf("save passkey: %v", err)
	}

	reqStatus := httptest.NewRequest(http.MethodGet, "/api/auth/passkey/status", nil)
	reqStatus.Host = "bot.starryheavens.uk"
	recStatus := httptest.NewRecorder()
	s.handlePasskeyStatus(recStatus, reqStatus)
	if !strings.Contains(recStatus.Body.String(), `"has_passkeys":true`) {
		t.Fatalf("expected has_passkeys=true, got %s", recStatus.Body.String())
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/auth/passkeys", nil)
	reqList.Host = "bot.starryheavens.uk"
	recList := httptest.NewRecorder()
	s.handleListPasskeys(recList, reqList)
	if recList.Code != http.StatusOK || !strings.Contains(recList.Body.String(), "dGVzdC1jcmVkLTE") {
		t.Fatalf("unexpected passkey list: %d %s", recList.Code, recList.Body.String())
	}
}
