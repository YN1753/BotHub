package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestV2ChatImageUploadServeAndThreadCompactEndpoints(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	appTok, err := s.ensureAppBearerToken()
	if err != nil {
		t.Fatalf("ensureAppBearerToken: %v", err)
	}
	h := s.Handler()

	// 1. Upload PNG via JSON data_url
	tinyPNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII="
	upBody, _ := json.Marshal(map[string]string{
		"data_url": "data:image/png;base64," + tinyPNG,
	})
	reqUp := httptest.NewRequest(http.MethodPost, "/api/v2/chat/upload-image", bytes.NewReader(upBody))
	reqUp.Header.Set("Authorization", "Bearer "+appTok)
	reqUp.Header.Set("Content-Type", "application/json")
	recUp := httptest.NewRecorder()
	h.ServeHTTP(recUp, reqUp)
	if recUp.Code != http.StatusOK {
		t.Fatalf("upload image failed: %d %s", recUp.Code, recUp.Body.String())
	}
	var upResp struct {
		OK   bool   `json:"ok"`
		URL  string `json:"url"`
		Mime string `json:"mime"`
	}
	if err := json.Unmarshal(recUp.Body.Bytes(), &upResp); err != nil || !upResp.OK || !strings.HasPrefix(upResp.URL, "/api/v2/chat/uploads/") {
		t.Fatalf("unexpected upload response: %s", recUp.Body.String())
	}

	// 2. Serve uploaded image with auth & reject path traversal
	reqGet := httptest.NewRequest(http.MethodGet, upResp.URL, nil)
	reqGet.Header.Set("Authorization", "Bearer "+appTok)
	recGet := httptest.NewRecorder()
	h.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK || recGet.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("serve image failed: code=%d ct=%q", recGet.Code, recGet.Header().Get("Content-Type"))
	}
	wantBytes, _ := base64.StdEncoding.DecodeString(tinyPNG)
	if !bytes.Equal(recGet.Body.Bytes(), wantBytes) {
		t.Fatalf("served image bytes mismatch")
	}

	// 3. Test POST /api/v2/bots/{id}/threads/compact endpoint
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"已压缩前 4 轮对话核心结论：确认迁移至统一模型池。"}}]}`))
	}))
	defer mockLLM.Close()

	_ = s.store.SetConfig(store.KeyDeepSeekAPIKey, "")
	_ = s.store.SaveModelConfig(
		[]store.ModelProvider{{ID: "p1", Name: "Mock", Protocol: "openai", BaseURL: mockLLM.URL + "/v1", APIKey: "sk-test", Enabled: true}},
		[]store.PooledModel{{ProviderID: "p1", ModelID: "claude-sonnet-4", Enabled: true, Priority: 10}},
		store.PoolStrategyPriority,
	)
	_ = s.store.SaveBot(store.Bot{ID: "bot_c", Name: "CompactBot", Model: "auto"})
	for i := 0; i < 6; i++ {
		_, _ = s.store.AppendAgentMessage(store.AgentMessage{
			SessionID: "bot:bot_c",
			BotID:     "bot_c",
			Role:      "user",
			Content:   fmt.Sprintf("历史消息 %d", i+1),
		})
	}

	reqCmp := httptest.NewRequest(http.MethodPost, "/api/v2/bots/bot_c/threads/compact", strings.NewReader(`{"thread_id":""}`))
	reqCmp.Header.Set("Authorization", "Bearer "+appTok)
	reqCmp.Header.Set("Content-Type", "application/json")
	recCmp := httptest.NewRecorder()
	h.ServeHTTP(recCmp, reqCmp)
	if recCmp.Code != http.StatusOK || !strings.Contains(recCmp.Body.String(), "统一模型池") {
		t.Fatalf("compact endpoint failed: %d %s", recCmp.Code, recCmp.Body.String())
	}
}
