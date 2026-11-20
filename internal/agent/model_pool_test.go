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
	googleHealth := snap["pool:google_ai_studio::gemini-2.5-flash"]
	sfHealth := snap["pool:siliconflow_free::Qwen/Qwen2.5-7B-Instruct"]
	if googleHealth.CooldownSec <= 0 || googleHealth.Status != "cooling_down" || sfHealth.SuccessCount != 2 || sfHealth.Status != "healthy" {
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
	if targets[0].ID != "pool:siliconflow_free::Qwen/Qwen2.5-7B-Instruct" || targets[1].ID != "primary" {
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
	for _, want := range []string{"auto:free", "auto:paid", "pool:free_vendor::free-m1", "pool:free_vendor::free-m2", "pool:paid_vendor::paid-m1", "pool:paid_vendor::paid-m2"} {
		if _, ok := seenIDs[want]; !ok {
			t.Fatalf("expected ModelOptionsPayload to contain %q, got %+v", want, opts)
		}
	}
	// Legacy pool:<providerID> alias without ::<model> must still resolve cleanly
	legacyPinned, lErr := eng.ResolveChatTargetsWithContext(context.Background(), "pool:free_vendor", false)
	if lErr != nil || len(legacyPinned) == 0 || legacyPinned[0].Model != "free-m1" {
		t.Fatalf("expected legacy pool:free_vendor alias to resolve to free-m1, got err=%v targets=%+v", lErr, legacyPinned)
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

func TestDecoupledProvidersMigrationCapabilitiesStrategiesAndTaskQuota(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "v259_model_config.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// 1. Seamless lossless migration & deduplication from legacy free_model_pool (same vendor in Free + Paid)
	if err := st.SaveModelPool([]store.ModelPoolEntry{
		{
			ID:       "or_free",
			Name:     "OpenRouter Free",
			Tier:     store.ModelTierFree,
			Provider: "openai",
			BaseURL:  "https://openrouter.ai/api/v1/chat/completions",
			APIKey:   "sk-or-shared-secret-1234",
			Models:   []string{"qwen/qwen3:free"},
			Enabled:  true,
			Priority: 10,
		},
		{
			ID:       "or_paid",
			Name:     "OpenRouter Paid",
			Tier:     store.ModelTierPaid,
			Provider: "openai",
			BaseURL:  "https://openrouter.ai/api/v1/",
			APIKey:   "sk-or-shared-secret-1234",
			Models:   []string{"anthropic/claude-sonnet-4"},
			Enabled:  true,
			Priority: 10,
		},
	}); err != nil {
		t.Fatalf("SaveModelPool: %v", err)
	}

	providers, pooled, _ := st.GetModelConfig()
	if len(providers) != 1 {
		t.Fatalf("expected identical OpenRouter free+paid entries merged into 1 ModelProvider, got %+v", providers)
	}
	if providers[0].BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("expected CleanProviderBaseURL to strip /chat/completions, got %q", providers[0].BaseURL)
	}
	if len(pooled) != 2 {
		t.Fatalf("expected 2 pooled models across free and paid tiers, got %+v", pooled)
	}

	eng := NewEngine(st, &fakeTaskController{st: st})

	// Legacy alias ID "pool:or_paid" must still resolve cleanly even after merge
	aliasTargets, err := eng.ResolveChatTargetsWithContext(context.Background(), "pool:or_paid", true)
	if err != nil || len(aliasTargets) == 0 || aliasTargets[0].ProviderID != providers[0].ID {
		t.Fatalf("expected merged alias pool:or_paid to resolve, err=%v targets=%+v", err, aliasTargets)
	}

	// 2. Zero-cost full model fetch (>24 models) must NOT truncate or fire inference probes
	var chatCalls int
	fullModelsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			items := make([]map[string]string, 0, 32)
			for i := 1; i <= 32; i++ {
				items = append(items, map[string]string{"id": fmt.Sprintf("vendor/model-%02d:free", i)})
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": items})
			return
		}
		chatCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer fullModelsSrv.Close()

	fullRes, err := eng.FetchAndVerifyUpstreamModels(context.Background(), fullModelsSrv.URL, "sk-test", "openai", false)
	if err != nil {
		t.Fatalf("zero-cost fetch failed: %v", err)
	}
	if len(fullRes.Models) != 32 || chatCalls != 0 {
		t.Fatalf("expected all 32 models returned with 0 chat calls, got len=%d chatCalls=%d", len(fullRes.Models), chatCalls)
	}

	// 3. Capability probing (supports_tools + supports_thinking persisted to SQLite) + NeedTools routing
	toolSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mid, _ := body["model"].(string)
		_, hasTools := body["tools"]
		if mid == "no-tools-model" && hasTools {
			http.Error(w, `{"error":{"message":"This model does not support tool calling"}}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK","reasoning_content":"step"}}]}`))
	}))
	defer toolSrv.Close()

	if err := st.SaveModelConfig(
		[]store.ModelProvider{
			{ID: "prov_a", Name: "通道A", Protocol: "openai", BaseURL: toolSrv.URL, APIKey: "sk-prov-a-secret", Enabled: true},
		},
		[]store.PooledModel{
			{ProviderID: "prov_a", ModelID: "no-tools-model", Tier: store.ModelTierFree, Enabled: true, Priority: 10},
			{ProviderID: "prov_a", ModelID: "full-tools-model", Tier: store.ModelTierFree, Enabled: true, Priority: 20},
			{ProviderID: "prov_a", ModelID: "paid-pro", Tier: store.ModelTierPaid, Enabled: true, Priority: 10},
		},
		store.PoolStrategyConfig{Free: store.PoolStrategyPriority, Paid: store.PoolStrategyPriority},
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	probe1, err := eng.ProbeModelTargetWithCapabilities(context.Background(), "prov_a", toolSrv.URL, "sk-prov-a-secret", "no-tools-model", "openai", true)
	if err != nil || probe1.SupportsTools == nil || *probe1.SupportsTools {
		t.Fatalf("expected no-tools-model supports_tools=false, got err=%v res=%+v", err, probe1)
	}
	probe2, err := eng.ProbeModelTargetWithCapabilities(context.Background(), "prov_a", toolSrv.URL, "sk-prov-a-secret", "full-tools-model", "openai", true)
	if err != nil || probe2.SupportsTools == nil || !*probe2.SupportsTools || probe2.SupportsThinking == nil || !*probe2.SupportsThinking {
		t.Fatalf("expected full-tools-model supports_tools=true & supports_thinking=true, got err=%v res=%+v", err, probe2)
	}

	toolTargets, err := eng.ResolveChatTargetsWithOptions(context.Background(), "auto:free", ModelResolveOptions{NeedTools: true})
	if err != nil || len(toolTargets) < 2 || toolTargets[0].Model != "full-tools-model" {
		t.Fatalf("expected NeedTools routing to prefer full-tools-model over higher-priority no-tools-model, got err=%v targets=%+v", err, toolTargets)
	}

	// 4. Intra-pool strategies: round_robin & lowest_latency + ClearModelCooldown
	if err := st.SaveModelConfig(
		[]store.ModelProvider{
			{ID: "prov_a", Name: "通道A", Protocol: "openai", BaseURL: toolSrv.URL, APIKey: "sk-prov-a-secret", Enabled: true},
		},
		[]store.PooledModel{
			{ProviderID: "prov_a", ModelID: "slow-m", Tier: store.ModelTierFree, Enabled: true, Priority: 10, LastLatencyMs: 900},
			{ProviderID: "prov_a", ModelID: "fast-m", Tier: store.ModelTierFree, Enabled: true, Priority: 20, LastLatencyMs: 120},
			{ProviderID: "prov_a", ModelID: "paid-pro", Tier: store.ModelTierPaid, Enabled: true, Priority: 10},
		},
		store.PoolStrategyConfig{Free: store.PoolStrategyLowestLatency, Paid: store.PoolStrategyPriority},
	); err != nil {
		t.Fatalf("SaveModelConfig lowest_latency: %v", err)
	}
	latTargets, err := eng.ResolveChatTargetsWithOptions(context.Background(), "auto:free", ModelResolveOptions{})
	if err != nil || latTargets[0].Model != "fast-m" {
		t.Fatalf("expected lowest_latency strategy to pick fast-m first, got err=%v targets=%+v", err, latTargets)
	}

	_ = st.SetConfig(store.KeyFreePoolStrategy, store.PoolStrategyRoundRobin)
	rr1, _ := eng.ResolveChatTargetsWithOptions(context.Background(), "auto:free", ModelResolveOptions{})
	rr2, _ := eng.ResolveChatTargetsWithOptions(context.Background(), "auto:free", ModelResolveOptions{})
	if rr1[0].Model == rr2[0].Model {
		t.Fatalf("expected round_robin strategy to alternate primary target across calls, got %q and %q", rr1[0].Model, rr2[0].Model)
	}

	eng.health.recordFailure("pool:prov_a::fast-m", "HTTP 429", true)
	if !eng.health.isCooling("pool:prov_a::fast-m") {
		t.Fatalf("expected fast-m cooling down")
	}
	eng.ClearModelCooldown("pool:prov_a::fast-m")
	if eng.health.isCooling("pool:prov_a::fast-m") {
		t.Fatalf("expected ClearModelCooldown to immediately unblock fast-m")
	}

	// 5. Usage telemetry & per-task daily paid call limit circuit breaker
	_ = st.RecordModelUsage(store.ModelUsageRecord{
		TargetID:         "pool:prov_a::paid-pro",
		ProviderID:       "prov_a",
		ModelID:          "paid-pro",
		Tier:             store.ModelTierPaid,
		Source:           "routine",
		TaskID:           "task_daily_guard",
		PromptTokens:     120,
		CompletionTokens: 80,
		LatencyMs:        340,
		Success:          true,
	})
	_ = st.RecordModelUsage(store.ModelUsageRecord{
		TargetID:         "pool:prov_a::fast-m",
		ProviderID:       "prov_a",
		ModelID:          "fast-m",
		Tier:             store.ModelTierFree,
		Source:           "chat",
		PromptTokens:     50,
		CompletionTokens: 50,
		LatencyMs:        110,
		Success:          true,
	})

	summary, err := st.SummarizeModelUsage(7)
	if err != nil || summary.FreeCalls != 1 || summary.PaidCalls != 1 || summary.PaidTokens != 200 || summary.FreeTokens != 100 {
		t.Fatalf("unexpected 7-day usage summary: err=%v summary=%+v", err, summary)
	}

	quotaCtx := store.WithPaidFallback(context.Background(), true)
	quotaCtx = store.WithTaskQuota(quotaCtx, "task_daily_guard", 1)
	if _, qErr := eng.ResolveChatTargetsWithOptions(quotaCtx, "auto:paid", ModelResolveOptions{AllowPaidFallback: true}); qErr == nil || !strings.Contains(qErr.Error(), "今日付费调用已达上限") {
		t.Fatalf("expected daily paid call quota limit to block task_daily_guard after 1/1 call, got err=%v", qErr)
	}
}

func TestPinnedModelNeverSilentlyDemotedByCooldown(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "pinned_model_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	_ = st.SetConfig(store.KeyPoolFallback, "true")
	if err := st.SaveModelConfig(
		[]store.ModelProvider{
			{
				ID:       "prov_free",
				Name:     "免费渠道",
				Protocol: "openai",
				BaseURL:  "https://free.example.com/v1",
				APIKey:   "sk-free",
				Enabled:  true,
			},
			{
				ID:       "prov_paid",
				Name:     "付费渠道",
				Protocol: "openai",
				BaseURL:  "https://paid.example.com/v1",
				APIKey:   "sk-paid",
				Enabled:  true,
			},
		},
		[]store.PooledModel{
			{ProviderID: "prov_free", ModelID: "model-alpha", Tier: store.ModelTierFree, Enabled: true, Priority: 10},
			{ProviderID: "prov_free", ModelID: "model-beta", Tier: store.ModelTierFree, Enabled: true, Priority: 20},
			{ProviderID: "prov_free", ModelID: "model-gamma", Tier: store.ModelTierFree, Enabled: true, Priority: 30},
			{ProviderID: "prov_paid", ModelID: "paid-one", Tier: store.ModelTierPaid, Enabled: true, Priority: 10},
			{ProviderID: "prov_paid", ModelID: "paid-two", Tier: store.ModelTierPaid, Enabled: true, Priority: 20},
		},
		store.PoolStrategyConfig{Free: store.PoolStrategyPriority, Paid: store.PoolStrategyPriority},
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	eng := NewEngine(st, &fakeTaskController{st: st})

	// Put both pinned targets and one fallback in cooldown
	eng.health.recordFailure("pool:prov_free::model-alpha", "HTTP 429", true)
	eng.health.recordFailure("pool:prov_free::model-beta", "HTTP 429", true)
	eng.health.recordFailure("pool:prov_paid::paid-one", "HTTP 503", false)

	// Free pinned model MUST remain index 0 even when cooling down, while healthy fallback (gamma) is sorted before cooling fallback (beta)
	freeResolved, err := eng.ResolveChatTargetsWithOptions(context.Background(), "pool:prov_free::model-alpha", ModelResolveOptions{
		AllowPaidFallback: true,
	})
	if err != nil {
		t.Fatalf("resolve pinned free model: %v", err)
	}
	if len(freeResolved) < 3 {
		t.Fatalf("expected pinned + fallbacks, got %d", len(freeResolved))
	}
	if freeResolved[0].ID != "pool:prov_free::model-alpha" {
		t.Fatalf("expected pinned free model at index 0, got %s", freeResolved[0].ID)
	}
	if freeResolved[1].ID != "pool:prov_free::model-gamma" {
		t.Fatalf("expected healthy fallback gamma at index 1, got %s", freeResolved[1].ID)
	}

	// Paid pinned model MUST also remain index 0 even when cooling down
	paidResolved, err := eng.ResolveChatTargetsWithOptions(context.Background(), "pool:prov_paid::paid-one", ModelResolveOptions{
		AllowPaidFallback: true,
	})
	if err != nil {
		t.Fatalf("resolve pinned paid model: %v", err)
	}
	if len(paidResolved) < 2 || paidResolved[0].ID != "pool:prov_paid::paid-one" {
		t.Fatalf("expected pinned paid model at index 0, got %+v", paidResolved)
	}
}
