package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestFreeModelPoolRoutingFailoverAndLiveFetchModels(t *testing.T) {
	if got := NormalizeModelID("deepseek-v4-pro"); got != "" {
		t.Fatalf("expected legacy deepseek-v4-pro cleared, got %q", got)
	}
	if got := NormalizeModelID("deepseek-chat"); got != "deepseek-chat" {
		t.Fatalf("expected deepseek-chat preserved verbatim, got %q", got)
	}
	if got := NormalizeModelID("deepseek-reasoner"); got != "deepseek-reasoner" {
		t.Fatalf("expected deepseek-reasoner preserved verbatim, got %q", got)
	}

	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "model_pool_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Fresh DB pool must start blank (no pre-seeded hardcoded providers)
	if initial := st.GetModelPool(); len(initial) != 0 {
		t.Fatalf("expected initial free_model_pool empty, got %d items", len(initial))
	}

	// Create user bot explicitly
	_ = st.SaveBot(store.Bot{
		ID:           "assistant_one",
		Name:         "个人助理",
		RoleTitle:    "测试 Bot",
		SystemPrompt: "简洁回答",
	})

	// Mock upstream /models discovery + selective usability verification
	googleRateLimited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"},{"id":"models/gemini-2.5-pro"},{"id":"models/text-embedding-004"},{"id":"models/imagen-3.0-generate-002"},{"id":"models/gemini-2.5-flash-preview-tts"}]}`))
			return
		}
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}))
	defer googleRateLimited.Close()

	// Dedicated server returning 200 for gemini-2.5-flash and 429 quota=0 for gemini-2.5-pro
	selectiveVerifySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"},{"id":"models/gemini-2.5-pro"},{"id":"models/gemini-embedding-001"},{"id":"models/veo-2.0-generate-001"}]}`))
			return
		}
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Model == "gemini-2.5-pro" {
			http.Error(w, `{"error":{"message":"Quota exceeded for metric: limit: 0"}}`, http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer selectiveVerifySrv.Close()

	var sfPayload string
	sfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"Qwen/Qwen2.5-7B-Instruct"},{"id":"deepseek-ai/DeepSeek-V3"}]}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		sfPayload = string(raw)
		if strings.Contains(r.URL.Path, "chat/completions") && !strings.Contains(r.URL.Path, "chat/completions/chat") {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			for _, c := range []string{
				`data: {"choices":[{"delta":{"role":"assistant","content":"硅基流动免费模型已自动接管并回复。"}}]}`,
				`data: [DONE]`,
			} {
				fmt.Fprintln(w, c)
				fmt.Fprintln(w)
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer sfServer.Close()

	eng := NewEngine(st, &fakeTaskController{st: st})

	// Test live /models discovery filters non-chat modalities cleanly
	fetchedGoogleModels, err := eng.FetchUpstreamModels(context.Background(), googleRateLimited.URL, "AIzaTestGoogleKey123456", "google")
	if err != nil || len(fetchedGoogleModels) != 2 || fetchedGoogleModels[0] != "gemini-2.5-flash" {
		t.Fatalf("unexpected live Google models fetch: models=%v err=%v", fetchedGoogleModels, err)
	}

	// Test FetchAndVerifyUpstreamModels verifies usability and keeps only usable chat models
	verifiedRes, err := eng.FetchAndVerifyUpstreamModels(context.Background(), selectiveVerifySrv.URL, "AIzaTestGoogleKey123456", "google", true)
	if err != nil {
		t.Fatalf("FetchAndVerifyUpstreamModels failed: %v", err)
	}
	if verifiedRes.FilteredOutCount != 2 {
		t.Fatalf("expected 2 non-chat models filtered out, got %d", verifiedRes.FilteredOutCount)
	}
	if len(verifiedRes.Models) != 1 || verifiedRes.Models[0] != "gemini-2.5-flash" {
		t.Fatalf("expected only gemini-2.5-flash usable, got %+v", verifiedRes)
	}

	// User creates two free pool items from templates using dynamically discovered models
	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	userPool := []store.ModelPoolEntry{
		{
			ID:       "google_user",
			Name:     "Google AI Studio",
			Provider: "google",
			BaseURL:  googleRateLimited.URL,
			APIKey:   "AIzaTestGoogleKey123456",
			Model:    fetchedGoogleModels[0],
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:       "sf_user",
			Name:     "硅基流动",
			Provider: "siliconflow",
			BaseURL:  sfServer.URL,
			APIKey:   "sk-sf-testkey123456",
			Model:    "Qwen/Qwen2.5-7B-Instruct",
			Enabled:  true,
			Priority: 20,
		},
	}
	if err := st.SaveModelPool(userPool); err != nil {
		t.Fatalf("SaveModelPool: %v", err)
	}

	// Verify SaveModelPool preserves existing key when masked placeholder is submitted back
	masked := store.MaskModelPool(st.GetModelPool())
	if err := st.SaveModelPool(masked); err != nil {
		t.Fatalf("SaveModelPool masked roundtrip: %v", err)
	}
	reloaded := st.GetModelPool()
	if reloaded[0].APIKey != "AIzaTestGoogleKey123456" {
		t.Fatalf("expected Google API key preserved across **** mask roundtrip, got %q", reloaded[0].APIKey)
	}

	// Test ChatStream on Bot with model="free-pool": hits Google (429) -> failover cleanly to SiliconFlow
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "assistant_one",
		Message: "测试免费模型池自动容灾切换",
		Model:   "free-pool",
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream free-pool failed: %v", err)
	}
	if resp.Model != "Qwen/Qwen2.5-7B-Instruct" || !strings.Contains(resp.Content, "硅基流动免费模型") {
		t.Fatalf("unexpected free-pool failover response: %+v", resp)
	}
	if strings.Contains(sfPayload, `"thinking"`) {
		t.Fatalf("non-DeepSeek pool provider must not receive proprietary DeepSeek thinking field: %s", sfPayload)
	}

	// Test Primary Relay Station URL + custom relay model name probe
	var relayUsedModel, relayAuth string
	relaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relayAuth = r.Header.Get("Authorization")
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		relayUsedModel, _ = reqBody["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer relaySrv.Close()

	_, preview, probeErr := eng.ProbeModelTarget(context.Background(), relaySrv.URL+"/v1/chat/completions", "sk-relay-token", "deepseek-v3-flash-relay", "deepseek")
	if probeErr != nil || preview != "OK" {
		t.Fatalf("ProbeModelTarget relay failed: err=%v preview=%q", probeErr, preview)
	}
	if relayUsedModel != "deepseek-v3-flash-relay" || relayAuth != "Bearer sk-relay-token" {
		t.Fatalf("unexpected relay probe request: model=%q auth=%q", relayUsedModel, relayAuth)
	}
}

func TestModelHealthCooldownAndModelUsedPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "health_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SaveBot(store.Bot{
		ID:           "assistant_one",
		Name:         "个人助理",
		RoleTitle:    "测试 Bot",
		SystemPrompt: "简洁回答",
	})

	hitCounts := map[string]int{}
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCounts["google"]++
		http.Error(w, `{"error":"rate limit"}`, http.StatusTooManyRequests)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCounts["sf"]++
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, c := range []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"节点二正常响应。"}}]}`,
			`data: [DONE]`,
		} {
			fmt.Fprintln(w, c)
			fmt.Fprintln(w)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer s2.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	_ = st.SaveModelPool([]store.ModelPoolEntry{
		{
			ID:       "google_ai_studio",
			Name:     "Google AI Studio",
			Provider: "google",
			BaseURL:  s1.URL,
			APIKey:   "AIzaTestKey",
			Model:    "gemini-2.5-flash",
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:       "siliconflow_free",
			Name:     "硅基流动",
			Provider: "siliconflow",
			BaseURL:  s2.URL,
			APIKey:   "sk-sf-key",
			Model:    "Qwen/Qwen2.5-7B-Instruct",
			Enabled:  true,
			Priority: 20,
		},
	})

	eng := NewEngine(st, &fakeTaskController{st: st})

	var events []string
	_, err = eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "assistant_one",
		Message: "第一次提问",
		Model:   "free-pool",
	}, func(event string, _ any) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("first turn failed: %v", err)
	}
	if !strings.Contains(strings.Join(events, ","), "model_switch") {
		t.Fatalf("expected model_switch SSE event during failover, got: %v", events)
	}

	// Second turn should skip cooled google_ai_studio and hit s2 directly!
	_, err = eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "assistant_one",
		Message: "第二次提问",
		Model:   "free-pool",
	}, nil)
	if err != nil {
		t.Fatalf("second turn failed: %v", err)
	}
	if hitCounts["google"] != 1 || hitCounts["sf"] != 2 {
		t.Fatalf("expected google called once (then cooled) and sf called twice, got %+v", hitCounts)
	}

	snap := eng.GetModelHealth()
	if snap["pool:google_ai_studio"].CooldownSec <= 0 || snap["pool:google_ai_studio"].Status != "cooling_down" || snap["pool:siliconflow_free"].SuccessCount != 2 || snap["pool:siliconflow_free"].Status != "healthy" {
		t.Fatalf("unexpected health snapshot: %+v", snap)
	}

	// Verify CallLightModel also skips cooled google_ai_studio and records health telemetry
	s2Light := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCounts["sf_light"]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"轻量摘要结果"}}]}`))
	}))
	defer s2Light.Close()
	_ = st.SaveModelPool([]store.ModelPoolEntry{
		{ID: "google_ai_studio", Name: "Google AI Studio", Provider: "google", BaseURL: s1.URL, APIKey: "AIzaTestKey", Model: "gemini-2.5-flash", Enabled: true, Priority: 10},
		{ID: "siliconflow_free", Name: "硅基流动", Provider: "siliconflow", BaseURL: s2Light.URL, APIKey: "sk-sf-key", Model: "Qwen/Qwen2.5-7B-Instruct", Enabled: true, Priority: 20},
	})
	lightReply, lightErr := eng.CallLightModel(context.Background(), "摘要助手", "测试输入")
	if lightErr != nil || lightReply != "轻量摘要结果" {
		t.Fatalf("CallLightModel failed: err=%v reply=%q", lightErr, lightReply)
	}
	if hitCounts["google"] != 1 || hitCounts["sf_light"] != 1 {
		t.Fatalf("expected CallLightModel to bypass cooled google_ai_studio and hit sf_light once, got %+v", hitCounts)
	}

	// Verify primary cooldown deprioritizes primary behind healthy pool fallback while retaining primary at tail
	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-primary-key")
	eng.health.recordFailure("primary", "HTTP 429 rate limited", true)
	targets, rErr := eng.ResolveChatTargets("primary")
	if rErr != nil || len(targets) < 2 {
		t.Fatalf("ResolveChatTargets primary fallback failed: err=%v targets=%+v", rErr, targets)
	}
	if targets[0].ID != "pool:siliconflow_free" || targets[1].ID != "primary" {
		t.Fatalf("expected healthy pool target first and cooled primary moved into cooling tail, got %+v", targets)
	}

	msgs, _ := st.ListBotMessages("assistant_one", 10)
	foundModelUsed := false
	for _, m := range msgs {
		if m.Role == "assistant" && m.ModelUsed == "Qwen/Qwen2.5-7B-Instruct" {
			foundModelUsed = true
		}
	}
	if !foundModelUsed {
		t.Fatalf("expected persisted model_used=Qwen/Qwen2.5-7B-Instruct in agent_messages, got %+v", msgs)
	}
}

func TestDualPoolMultiModelAndPerTaskPaidGuard(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "dual_pool.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := st.SaveModelPool([]store.ModelPoolEntry{
		{
			ID:       "free_vendor",
			Name:     "免费聚合",
			Tier:     store.ModelTierFree,
			Provider: "openai",
			BaseURL:  "https://free.example.com/v1",
			APIKey:   "sk-free-key",
			Models:   []string{"free-m1", "free-m2"},
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:       "paid_vendor",
			Name:     "付费主力",
			Tier:     store.ModelTierPaid,
			Provider: "deepseek",
			BaseURL:  "https://paid.example.com/v1",
			APIKey:   "sk-paid-key",
			Models:   []string{"paid-m1", "paid-m2"},
			Enabled:  true,
			Priority: 10,
		},
	}); err != nil {
		t.Fatalf("SaveModelPool failed: %v", err)
	}

	eng := NewEngine(st, &fakeTaskController{st: st})

	opts := eng.ModelOptionsPayload()
	optIDs := make(map[string]store.ModelPoolEntry)
	seenIDs := make(map[string]string)
	for _, o := range opts {
		seenIDs[o.ID] = o.Tier
	}
	_ = optIDs
	for _, want := range []string{"auto:free", "auto:paid", "pool:free_vendor", "pool:free_vendor::free-m2", "pool:paid_vendor", "pool:paid_vendor::paid-m2"} {
		if _, ok := seenIDs[want]; !ok {
			t.Fatalf("expected ModelOptionsPayload to contain %q, got %+v", want, opts)
		}
	}

	// 1. Per-task guard OFF (false): auto:free MUST NOT include any Paid Pool target
	ctxBlocked := store.WithPaidFallback(context.Background(), false)
	targets, err := eng.ResolveChatTargetsWithContext(ctxBlocked, "auto:free", false)
	if err != nil || len(targets) != 2 {
		t.Fatalf("expected strictly 2 free targets when allow_paid_fallback=false, got err=%v targets=%+v", err, targets)
	}
	for _, tg := range targets {
		if tg.Model == "paid-m1" || tg.Model == "paid-m2" {
			t.Fatalf("paid model leaked into auto:free with guard off: %+v", tg)
		}
	}

	// 2. Task guard OFF also blocks auto:paid / pinned paid model if task explicitly forbids paid
	if _, err := eng.ResolveChatTargetsWithContext(ctxBlocked, "auto:paid", false); err == nil {
		t.Fatalf("expected task allow_paid_fallback=false to block auto:paid")
	}
	if _, err := eng.ResolveChatTargetsWithContext(ctxBlocked, "pool:paid_vendor::paid-m2", false); err == nil {
		t.Fatalf("expected task allow_paid_fallback=false to block pinned paid node")
	}

	// 3. Task guard ON (true): auto:free includes Free models first, Paid models as tail fallback
	ctxAllowed := store.WithPaidFallback(context.Background(), true)
	targetsWithFallback, err := eng.ResolveChatTargetsWithContext(ctxAllowed, "auto:free", true)
	if err != nil || len(targetsWithFallback) != 4 {
		t.Fatalf("expected 4 targets (2 free + 2 paid fallback), got err=%v targets=%+v", err, targetsWithFallback)
	}
	if targetsWithFallback[0].Model != "free-m1" || targetsWithFallback[2].Model != "paid-m1" {
		t.Fatalf("unexpected priority ordering in dual pool fallback: %+v", targetsWithFallback)
	}

	// 4. Disable free vendor & verify allow_paid_fallback=false fails loudly instead of silently billing
	_ = st.SaveModelPool([]store.ModelPoolEntry{
		{
			ID:       "free_vendor",
			Name:     "免费聚合",
			Tier:     store.ModelTierFree,
			Provider: "openai",
			BaseURL:  "https://free.example.com/v1",
			APIKey:   "sk-free-key",
			Models:   []string{"free-m1"},
			Enabled:  false,
			Priority: 10,
		},
		{
			ID:       "paid_vendor",
			Name:     "付费主力",
			Tier:     store.ModelTierPaid,
			Provider: "deepseek",
			BaseURL:  "https://paid.example.com/v1",
			APIKey:   "sk-paid-key",
			Models:   []string{"paid-m1"},
			Enabled:  true,
			Priority: 10,
		},
	})
	if _, err := eng.ResolveChatTargetsWithContext(ctxBlocked, "auto:free", false); err == nil {
		t.Fatalf("expected error when Free Pool is empty and allow_paid_fallback=false")
	}
	fallbackOnlyPaid, err := eng.ResolveChatTargetsWithContext(ctxAllowed, "auto:free", true)
	if err != nil || len(fallbackOnlyPaid) != 1 || fallbackOnlyPaid[0].Model != "paid-m1" {
		t.Fatalf("expected fallback to paid-m1 when allow_paid_fallback=true, got err=%v targets=%+v", err, fallbackOnlyPaid)
	}
}
