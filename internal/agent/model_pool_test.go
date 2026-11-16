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
	if got := NormalizeModelID("deepseek-v4-pro"); got != "deepseek-flash" {
		t.Fatalf("expected deepseek-v4-pro normalized to deepseek-flash, got %q", got)
	}
	if got := NormalizeModelID("deepseek-reasoner"); got != "deepseek-flash" {
		t.Fatalf("expected deepseek-reasoner normalized to deepseek-flash, got %q", got)
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
	// Creation templates must still include Google AI Studio & SiliconFlow
	tpls := store.ModelProviderTemplates()
	var hasGoogleTpl, hasSFTpl bool
	for _, tpl := range tpls {
		if tpl.ID == "google_ai_studio" {
			hasGoogleTpl = true
		}
		if tpl.ID == "siliconflow" {
			hasSFTpl = true
		}
	}
	if !hasGoogleTpl || !hasSFTpl {
		t.Fatalf("expected model provider creation templates for Google & SiliconFlow, got %+v", tpls)
	}

	// Create user bot explicitly
	_ = st.SaveBot(store.Bot{
		ID:           "assistant_one",
		Name:         "个人助理",
		RoleTitle:    "测试 Bot",
		SystemPrompt: "简洁回答",
	})

	// Mock upstream /models discovery + rate-limited node + healthy fallback node
	googleRateLimited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"},{"id":"models/gemini-2.5-pro"}]}`))
			return
		}
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}))
	defer googleRateLimited.Close()

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

	// Test live /models discovery from upstream
	fetchedGoogleModels, err := eng.FetchUpstreamModels(context.Background(), googleRateLimited.URL, "AIzaTestGoogleKey123456", "google")
	if err != nil || len(fetchedGoogleModels) != 2 || fetchedGoogleModels[0] != "gemini-2.5-flash" {
		t.Fatalf("unexpected live Google models fetch: models=%v err=%v", fetchedGoogleModels, err)
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
	if snap["pool:google_ai_studio"].CooldownSec <= 0 || snap["pool:siliconflow_free"].SuccessCount != 2 {
		t.Fatalf("unexpected health snapshot: %+v", snap)
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
