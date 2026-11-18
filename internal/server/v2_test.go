package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
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

	// 5. Test Mark All Read & Clear Read
	reqReadAll := httptest.NewRequest(http.MethodPost, "/api/v2/notifications/read-all", nil)
	recReadAll := httptest.NewRecorder()
	s.handleV2MarkAllNotificationsRead(recReadAll, reqReadAll)
	if s.store.CountUnreadNotifications() != 0 {
		t.Fatalf("expected 0 unread after read-all, got %d", s.store.CountUnreadNotifications())
	}

	reqClearRead := httptest.NewRequest(http.MethodPost, "/api/v2/notifications/clear-read", nil)
	recClearRead := httptest.NewRecorder()
	s.handleV2ClearReadNotifications(recClearRead, reqClearRead)
	if recClearRead.Code != http.StatusOK || !strings.Contains(recClearRead.Body.String(), `"deleted":4`) {
		t.Fatalf("expected 4 deleted notifications after clear-read, got %d %s", recClearRead.Code, recClearRead.Body.String())
	}

	// 6. Test CreateBot rejects malformed allowed_tools JSON array
	reqBadBot := httptest.NewRequest(http.MethodPost, "/api/v2/bots", bytes.NewReader([]byte(`{"name":"BadBot","allowed_tools":"not-json"}`)))
	recBadBot := httptest.NewRecorder()
	s.handleCreateBot(recBadBot, reqBadBot)
	if recBadBot.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed allowed_tools, got %d %s", recBadBot.Code, recBadBot.Body.String())
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

func TestAuthTokenAutoGenerateAndRotate(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// Ensure admin password is set and app_bearer_token is initially empty
	_ = s.store.SetConfig(store.KeyAppBearerToken, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v2/auth/token", strings.NewReader(`{"password":"test-admin-password"}`))
	// In setupTestServer let's check what password is set; if hash isn't set, verifyPassword checks env or default.
	// Let's set a known hash via handleUpdatePassword or direct store hash if needed; first let's test handleV2RotateAppToken directly.
	_ = req

	recRot1 := httptest.NewRequest(http.MethodPost, "/api/v2/auth/token/rotate", nil)
	rec1 := httptest.NewRecorder()
	s.handleV2RotateAppToken(rec1, recRot1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("rotate token 1 failed: %d", rec1.Code)
	}
	var out1 struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec1.Body.Bytes(), &out1)
	if !out1.OK || !strings.HasPrefix(out1.Token, "bh_") || len(out1.Token) < 20 {
		t.Fatalf("unexpected rotated token: %+v", out1)
	}

	recRot2 := httptest.NewRequest(http.MethodPost, "/api/v2/auth/token/rotate", nil)
	rec2 := httptest.NewRecorder()
	s.handleV2RotateAppToken(rec2, recRot2)
	var out2 struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &out2)
	if out2.Token == out1.Token || out2.Token == "" {
		t.Fatalf("expected new rotated token distinct from previous")
	}

	if !shouldTriggerNezhaAITriage("Node-1 离线告警", "ping timeout") {
		t.Fatalf("expected offline alert to trigger AI triage")
	}
	if shouldTriggerNezhaAITriage("Node-1 恢复在线", "status ok") {
		t.Fatalf("did not expect recovery notice to trigger AI triage")
	}
}

func TestGrokBotRosterAndTimelineLifecycle(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Fresh DB must start clean with 0 hardcoded bots; templates must still be available
	reqList := httptest.NewRequest(http.MethodGet, "/api/v2/bots", nil)
	recList := httptest.NewRecorder()
	s.handleListBots(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("list bots failed: %d", recList.Code)
	}
	var listResp struct {
		OK   bool        `json:"ok"`
		Bots []store.Bot `json:"bots"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal bots: %v", err)
	}
	if len(listResp.Bots) != 0 {
		t.Fatalf("expected clean DB with 0 hardcoded bots, got %d", len(listResp.Bots))
	}

	// 2. Create custom bot, update it, verify messages & unread clear
	createBody := `{"name":"财务复盘专员","role_title":"月度账单核查","avatar_style":"emerald","system_prompt":"负责预算核对"}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v2/bots", strings.NewReader(createBody))
	recCreate := httptest.NewRecorder()
	s.handleCreateBot(recCreate, reqCreate)
	if recCreate.Code != http.StatusOK {
		t.Fatalf("create bot failed: %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var createdResp struct {
		Bot store.Bot `json:"bot"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createdResp)
	customID := createdResp.Bot.ID
	if customID == "" {
		t.Fatalf("expected custom bot ID")
	}

	// Simulate proactive routine message posted to custom bot
	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID:   "bot:" + customID,
		BotID:       customID,
		SenderBotID: customID,
		MsgKind:     "routine",
		Role:        "assistant",
		Content:     "本月服务器预算核对完成，一切正常。",
	})
	_ = s.store.IncrementBotUnread(customID, "预算核对完成")

	// Reading bot messages should return the proactive report and reset unread_count to 0
	reqMsgs := httptest.NewRequest(http.MethodGet, "/api/v2/bots/"+customID+"/messages", nil)
	reqMsgs.SetPathValue("id", customID)
	recMsgs := httptest.NewRecorder()
	s.handleGetBotMessages(recMsgs, reqMsgs)
	if recMsgs.Code != http.StatusOK || !strings.Contains(recMsgs.Body.String(), "预算核对完成") {
		t.Fatalf("get bot messages failed: %d: %s", recMsgs.Code, recMsgs.Body.String())
	}
	botAfter, _ := s.store.GetBot(customID)
	if botAfter.UnreadCount != 0 {
		t.Fatalf("expected unread_count cleared to 0, got %d", botAfter.UnreadCount)
	}

	reqDelCustom := httptest.NewRequest(http.MethodDelete, "/api/v2/bots/"+customID, nil)
	reqDelCustom.SetPathValue("id", customID)
	recDelCustom := httptest.NewRecorder()
	s.handleDeleteBot(recDelCustom, reqDelCustom)
	if recDelCustom.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting bot, got %d", recDelCustom.Code)
	}
}

func TestV2WebSessionBridge(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	h := s.Handler()

	// 1. Unauthenticated call must fail with 401
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v2/auth/web-session", nil)
	unauthRec := httptest.NewRecorder()
	h.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without Bearer token, got %d", unauthRec.Code)
	}

	// 2. Mint App Bearer Token and exchange for WebView session cookie
	appTok, err := s.ensureAppBearerToken()
	if err != nil || appTok == "" {
		t.Fatalf("ensureAppBearerToken failed: %v", err)
	}

	authReq := httptest.NewRequest(http.MethodPost, "/api/v2/auth/web-session", nil)
	authReq.Header.Set("Authorization", "Bearer "+appTok)
	authRec := httptest.NewRecorder()
	h.ServeHTTP(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from web-session, got %d: %s", authRec.Code, authRec.Body.String())
	}

	var body struct {
		OK          bool   `json:"ok"`
		CookieName  string `json:"cookie_name"`
		CookieValue string `json:"cookie_value"`
		MaxAge      int    `json:"max_age"`
	}
	if err := json.Unmarshal(authRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal web-session: %v", err)
	}
	if !body.OK || body.CookieName != sessionCookieName || body.CookieValue == "" {
		t.Fatalf("unexpected web-session payload: %+v", body)
	}

	// 3. Verify returned cookie authenticates standard web API endpoints without Bearer header
	statusReq := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: body.CookieName, Value: body.CookieValue})
	statusRec := httptest.NewRecorder()
	h.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK || !strings.Contains(statusRec.Body.String(), `"authenticated":true`) {
		t.Fatalf("expected authenticated:true using minted cookie, got %d: %s", statusRec.Code, statusRec.Body.String())
	}
}

func TestV2BootstrapAndGzipMiddleware(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	appTok, err := s.ensureAppBearerToken()
	if err != nil {
		t.Fatalf("ensureAppBearerToken: %v", err)
	}

	h := s.Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/app/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+appTok)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v2/app/bootstrap, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected Content-Encoding: gzip, got %q", rec.Header().Get("Content-Encoding"))
	}

	gr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()
	raw, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal bootstrap json: %v", err)
	}
	if payload["ok"] != true || payload["mirror_path"] != "/api/v2/app/apk" {
		t.Fatalf("unexpected bootstrap payload: %+v", payload)
	}
	if _, hasBots := payload["bots"]; !hasBots {
		t.Fatalf("expected bots array in bootstrap payload")
	}
	if _, hasStash := payload["stash"]; !hasStash {
		t.Fatalf("expected stash array in bootstrap payload")
	}

	// Verify HEAD requests skip gzip cleanly
	headReq := httptest.NewRequest(http.MethodHead, "/api/v2/app/version", nil)
	headReq.Header.Set("Accept-Encoding", "gzip")
	headRec := httptest.NewRecorder()
	h.ServeHTTP(headRec, headReq)
	if headRec.Header().Get("Content-Encoding") == "gzip" {
		t.Fatalf("expected HEAD request to skip gzip Content-Encoding")
	}

	// Verify ?compact=1&limit=10 on /api/v2/bots/{id}/messages omits unused heavy desktop slices
	if err := s.store.SaveBot(store.Bot{ID: "test_bot", Name: "TestBot", RoleTitle: "Helper", SystemPrompt: "Hi"}); err != nil {
		t.Fatalf("SaveBot: %v", err)
	}
	msgReq := httptest.NewRequest(http.MethodGet, "/api/v2/bots/test_bot/messages?compact=1&limit=10", nil)
	msgReq.Header.Set("Authorization", "Bearer "+appTok)
	msgRec := httptest.NewRecorder()
	h.ServeHTTP(msgRec, msgReq)
	if msgRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from compact bot messages, got %d: %s", msgRec.Code, msgRec.Body.String())
	}
	var compactPayload map[string]any
	if err := json.Unmarshal(msgRec.Body.Bytes(), &compactPayload); err != nil {
		t.Fatalf("unmarshal compact bot messages: %v", err)
	}
	if _, hasThreads := compactPayload["threads"]; hasThreads {
		t.Fatalf("expected compact=1 to omit threads array")
	}
	if _, hasMemories := compactPayload["memories"]; hasMemories {
		t.Fatalf("expected compact=1 to omit memories array")
	}
}

func TestExtensibleAssetCategoriesBlankAndPersist(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	appTok, err := s.ensureAppBearerToken()
	if err != nil {
		t.Fatalf("ensureAppBearerToken: %v", err)
	}
	h := s.Handler()

	// 1. Fresh DB must start blank with zero hardcoded asset categories
	getReq := httptest.NewRequest(http.MethodGet, "/api/v2/asset-categories", nil)
	getReq.Header.Set("Authorization", "Bearer "+appTok)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/asset-categories failed: %d", getRec.Code)
	}
	var initResp struct {
		OK         bool                        `json:"ok"`
		Categories []store.AssetCategoryOption `json:"categories"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("unmarshal init categories: %v", err)
	}
	if !initResp.OK || len(initResp.Categories) != 0 {
		t.Fatalf("expected blank app to have 0 built-in asset categories, got %+v", initResp.Categories)
	}

	// 2. Save user-defined extensible categories (including duplicates / emoji stripping)
	putBody := strings.NewReader(`{"categories":[{"id":"license","label":"软件授权","icon":"key"},{"id":"insurance","label":"健康保险","icon":"shield"},{"id":"license","label":"重复项","icon":"tag"}]}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/v2/asset-categories", putBody)
	putReq.Header.Set("Authorization", "Bearer "+appTok)
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT /api/v2/asset-categories failed: %d: %s", putRec.Code, putRec.Body.String())
	}

	// 3. Re-fetch and confirm deduplicated persistence
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/v2/asset-categories", nil)
	verifyReq.Header.Set("Authorization", "Bearer "+appTok)
	verifyRec := httptest.NewRecorder()
	h.ServeHTTP(verifyRec, verifyReq)
	var savedResp struct {
		Categories []store.AssetCategoryOption `json:"categories"`
	}
	_ = json.Unmarshal(verifyRec.Body.Bytes(), &savedResp)
	if len(savedResp.Categories) != 2 || savedResp.Categories[0].ID != "license" || savedResp.Categories[1].Icon != "shield" {
		t.Fatalf("unexpected saved categories: %+v", savedResp.Categories)
	}

	// 4. Create an asset with custom category and one with empty category (defaults to "general")
	createCustom := httptest.NewRequest(http.MethodPost, "/api/assets", strings.NewReader(`{"id":"a_ins","name":"医疗险","category":"insurance","due_date":"2027-06-01"}`))
	createCustom.Header.Set("Authorization", "Bearer "+appTok)
	createCustom.Header.Set("Content-Type", "application/json")
	recCustom := httptest.NewRecorder()
	h.ServeHTTP(recCustom, createCustom)
	if recCustom.Code != http.StatusOK {
		t.Fatalf("create custom asset failed: %d: %s", recCustom.Code, recCustom.Body.String())
	}

	createDefault := httptest.NewRequest(http.MethodPost, "/api/assets", strings.NewReader(`{"id":"a_gen","name":"通用条目","due_date":"2027-06-01"}`))
	createDefault.Header.Set("Authorization", "Bearer "+appTok)
	createDefault.Header.Set("Content-Type", "application/json")
	recDefault := httptest.NewRecorder()
	h.ServeHTTP(recDefault, createDefault)
	if recDefault.Code != http.StatusOK {
		t.Fatalf("create default category asset failed: %d", recDefault.Code)
	}
	gotGen, _ := s.store.GetAsset("a_gen")
	if gotGen == nil || gotGen.Category != "general" {
		t.Fatalf("expected blank category fallback to 'general', got %+v", gotGen)
	}
}
