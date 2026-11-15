package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestV2NotificationsAndEventsCatchup(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	s.push = notifier.NewClient(s.store)
	_ = s.store.SetConfig(store.KeyPushChannel, "native")

	// 1. Send 3 notifications via notifier.Client
	ctx := context.Background()
	if err := s.push.SendMarkdown(ctx, "morning", "早报 1", "内容 1"); err != nil {
		t.Fatalf("send 1 failed: %v", err)
	}
	if err := s.push.SendMarkdown(ctx, "radar", "雷达 2", "内容 2"); err != nil {
		t.Fatalf("send 2 failed: %v", err)
	}
	if err := s.push.SendMarkdown(ctx, "asset_watcher", "预警 3", "内容 3"); err != nil {
		t.Fatalf("send 3 failed: %v", err)
	}

	// 2. Test incremental fetch with after_id=1
	reqList := httptest.NewRequest(http.MethodGet, "/api/v2/notifications?after_id=1", nil)
	recList := httptest.NewRecorder()
	s.handleV2ListNotifications(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recList.Code)
	}

	var listResp struct {
		Notifications []store.Notification `json:"notifications"`
		UnreadCount   int                  `json:"unread_count"`
		LatestID      int64                `json:"latest_id"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal listResp failed: %v", err)
	}
	if len(listResp.Notifications) != 2 {
		t.Fatalf("expected 2 incremental notifications after ID 1, got %d", len(listResp.Notifications))
	}
	if listResp.Notifications[0].ID != 2 || listResp.Notifications[1].ID != 3 {
		t.Fatalf("expected ascending IDs [2, 3], got [%d, %d]", listResp.Notifications[0].ID, listResp.Notifications[1].ID)
	}
	if listResp.UnreadCount != 3 {
		t.Fatalf("expected 3 unread, got %d", listResp.UnreadCount)
	}

	// 3. Test SSE offline catch-up stream (/api/v2/events?last_event_id=1&oneshot=1)
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/v2/events?last_event_id=1&oneshot=1", nil)
	recSSE := httptest.NewRecorder()
	s.handleV2Events(recSSE, reqSSE)
	sseBody := recSSE.Body.String()
	if !strings.Contains(sseBody, "id: 2") || !strings.Contains(sseBody, "id: 3") || !strings.Contains(sseBody, "预警 3") {
		t.Fatalf("SSE catchup missing expected events 2 and 3, got:\n%s", sseBody)
	}

	// 4. Test Nezha Webhook endpoint
	hookBody := []byte(`{"server_name":"CloudCone-LA","message":"CPU 负载超过 90%"}`)
	reqHook := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/nezha", bytes.NewReader(hookBody))
	reqHook.RemoteAddr = "127.0.0.1:54321"
	recHook := httptest.NewRecorder()
	s.handleNezhaWebhook(recHook, reqHook)
	if recHook.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from nezha webhook, got %d: %s", recHook.Code, recHook.Body.String())
	}

	if s.store.LatestNotificationID() != 4 {
		t.Fatalf("expected latest notification ID 4 after webhook, got %d", s.store.LatestNotificationID())
	}

	// 5. Test Mark All Read
	reqReadAll := httptest.NewRequest(http.MethodPost, "/api/v2/notifications/read-all", nil)
	recReadAll := httptest.NewRecorder()
	s.handleV2MarkAllNotificationsRead(recReadAll, reqReadAll)
	if s.store.CountUnreadNotifications() != 0 {
		t.Fatalf("expected 0 unread after read-all, got %d", s.store.CountUnreadNotifications())
	}
}

func TestLightModelRoutingInModules(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	var usedModel string
	mockLightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		usedModel = req.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"轻量模型提炼结论：一切正常。"}}]}`))
	}))
	defer mockLightSrv.Close()

	_ = s.store.SetConfig(store.KeyLightBaseURL, mockLightSrv.URL)
	_ = s.store.SetConfig(store.KeyLightAPIKey, "sk-light-free")
	_ = s.store.SetConfig(store.KeyLightModel, "deepseek-flash")

	out, err := modules.SummarizeWithDeepSeek(context.Background(), s.store, "sys", "user")
	if err != nil {
		t.Fatalf("SummarizeWithDeepSeek failed: %v", err)
	}
	if usedModel != "deepseek-flash" || !strings.Contains(out, "轻量模型提炼结论") {
		t.Fatalf("unexpected model=%q or out=%q", usedModel, out)
	}
}

func TestSecurityMaskingAndWebhookDeduplication(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Test secret masking in /api/dashboard and preservation on /api/config
	_ = s.store.SetConfig(store.KeyDeepSeekAPIKey, "sk-1234567890abcdef")
	reqDash := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	recDash := httptest.NewRecorder()
	s.handleDashboard(recDash, reqDash)
	if recDash.Code != http.StatusOK {
		t.Fatalf("dashboard failed: %d", recDash.Code)
	}
	var dashResp struct {
		Configs map[string]string `json:"configs"`
	}
	if err := json.Unmarshal(recDash.Body.Bytes(), &dashResp); err != nil {
		t.Fatalf("unmarshal dashboard: %v", err)
	}
	masked := dashResp.Configs[store.KeyDeepSeekAPIKey]
	if masked == "sk-1234567890abcdef" || !strings.Contains(masked, "****") {
		t.Fatalf("expected masked deepseek_api_key, got %q", masked)
	}

	// Submitting the masked placeholder back to /api/config must not overwrite the real secret
	cfgPayload, _ := json.Marshal(map[string]string{
		"deepseek_api_key": masked,
		"ai_model":         "deepseek-v4-pro",
	})
	reqCfg := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(string(cfgPayload)))
	recCfg := httptest.NewRecorder()
	s.handleUpdateConfig(recCfg, reqCfg)
	if recCfg.Code != http.StatusOK {
		t.Fatalf("update config failed: %d", recCfg.Code)
	}
	storedKey, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
	if storedKey != "sk-1234567890abcdef" {
		t.Fatalf("expected real key preserved, got %q", storedKey)
	}

	// 2. Test Nezha webhook 5-minute cooldown deduplication
	_ = s.store.SetConfig(store.KeyNezhaWebhookToken, "hook-tok")
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/nezha?token=hook-tok", strings.NewReader(`{"title":"Node-Alpha 离线告警","content":"ping timeout"}`))
		rec := httptest.NewRecorder()
		s.handleNezhaWebhook(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("webhook call %d failed: %d", i, rec.Code)
		}
		if i == 1 && !strings.Contains(rec.Body.String(), `"deduplicated":true`) {
			t.Fatalf("expected second rapid identical webhook to be deduplicated, got: %s", rec.Body.String())
		}
	}
	if s.store.LatestNotificationID() != 1 {
		t.Fatalf("expected only 1 notification created due to deduplication, got %d", s.store.LatestNotificationID())
	}
}
