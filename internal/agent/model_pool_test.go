package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestUnifiedModelPoolRoutingFailoverAndLiveFetchModels(t *testing.T) {
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
	provs, mods, strat := st.GetModelConfig()
	if len(provs) != 0 || len(mods) != 0 || strat != store.PoolStrategyPriority {
		t.Fatalf("expected blank initial model config, got provs=%d mods=%d strat=%q", len(provs), len(mods), strat)
	}

	_ = st.SaveBot(store.Bot{
		ID:           "assistant_one",
		Name:         "个人助理",
		RoleTitle:    "测试 Bot",
		SystemPrompt: "简洁回答",
		Model:        "auto",
	})

	googleRateLimited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"models/gemini-2.5-flash"},{"id":"models/gemini-2.5-pro"},{"id":"models/text-embedding-004"},{"id":"models/imagen-3.0-generate-002"},{"id":"models/gemini-2.5-flash-preview-tts"}]}`))
			return
		}
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}))
	defer googleRateLimited.Close()

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
		if strings.Contains(r.URL.Path, "chat/completions") {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			for _, c := range []string{
				`data: {"choices":[{"delta":{"role":"assistant","content":"硅基流动模型已自动接管并回复。"}}]}`,
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

	fetchedGoogleModels, err := eng.FetchUpstreamModels(context.Background(), googleRateLimited.URL, "AIzaTestGoogleKey123456", "google")
	if err != nil || len(fetchedGoogleModels) != 2 || fetchedGoogleModels[0] != "gemini-2.5-flash" {
		t.Fatalf("unexpected live Google models fetch: models=%v err=%v", fetchedGoogleModels, err)
	}

	verifiedRes, err := eng.FetchAndVerifyUpstreamModels(context.Background(), selectiveVerifySrv.URL, "AIzaTestGoogleKey123456", "google", true)
	if err != nil {
		t.Fatalf("FetchAndVerifyUpstreamModels failed: %v", err)
	}
	if verifiedRes.FilteredOutCount != 2 || len(verifiedRes.Models) != 1 || verifiedRes.Models[0] != "gemini-2.5-flash" {
		t.Fatalf("expected only gemini-2.5-flash usable, got %+v", verifiedRes)
	}

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	if err := st.SaveModelConfig(
		[]store.ModelProvider{
			{ID: "google_user", Name: "Google AI Studio", Protocol: "google", BaseURL: googleRateLimited.URL, APIKey: "AIzaTestGoogleKey123456", Enabled: true},
			{ID: "sf_user", Name: "硅基流动", Protocol: "openai", BaseURL: sfServer.URL, APIKey: "sk-sf-testkey123456", Enabled: true},
		},
		[]store.PooledModel{
			{ProviderID: "google_user", ModelID: fetchedGoogleModels[0], Enabled: true, Priority: 10},
			{ProviderID: "sf_user", ModelID: "Qwen/Qwen2.5-7B-Instruct", Enabled: true, Priority: 20},
		},
		store.PoolStrategyPriority,
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	// Verify masked roundtrip preserves API key
	pGot, mGot, sGot := st.GetModelConfig()
	if err := st.SaveModelConfig(store.MaskModelProviders(pGot), mGot, sGot); err != nil {
		t.Fatalf("SaveModelConfig masked roundtrip: %v", err)
	}
	reloaded, _, _ := st.GetModelConfig()
	if reloaded[0].APIKey != "AIzaTestGoogleKey123456" {
		t.Fatalf("expected Google API key preserved across **** mask roundtrip, got %q", reloaded[0].APIKey)
	}

	// ChatStream with model="auto": hits Google (429) -> fails over cleanly to SiliconFlow
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "assistant_one",
		Message: "测试统一模型池自动容灾切换",
		Model:   "auto",
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream auto failed: %v", err)
	}
	if resp.Model != "Qwen/Qwen2.5-7B-Instruct" || !strings.Contains(resp.Content, "硅基流动模型") {
		t.Fatalf("unexpected auto failover response: %+v", resp)
	}
	if strings.Contains(sfPayload, `"thinking"`) {
		t.Fatalf("non-DeepSeek pool provider must not receive proprietary DeepSeek thinking field: %s", sfPayload)
	}
}

func TestModelHealthCooldownStrategiesCapabilitiesAndLightTag(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "strategies_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

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

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	if err := st.SaveModelConfig(
		[]store.ModelProvider{
			{ID: "prov_a", Name: "通道A", Protocol: "openai", BaseURL: toolSrv.URL, APIKey: "sk-prov-a-secret", Enabled: true},
		},
		[]store.PooledModel{
			{ProviderID: "prov_a", ModelID: "no-tools-model", Enabled: true, Priority: 10},
			{ProviderID: "prov_a", ModelID: "full-tools-model", Enabled: true, Priority: 20},
			{ProviderID: "prov_a", ModelID: "fast-light-model", Enabled: true, Priority: 30, IsLight: true, LastLatencyMs: 90},
		},
		store.PoolStrategyPriority,
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	eng := NewEngine(st, &fakeTaskController{st: st})

	// 1. ModelOptionsPayload returns single auto + enabled models
	opts := eng.ModelOptionsPayload()
	if len(opts) != 4 || opts[0].ID != "auto" || !opts[0].IsAuto {
		t.Fatalf("expected auto + 3 pool models, got %+v", opts)
	}

	// 2. Capability probing persists supports_tools & supports_thinking and guides NeedTools routing
	p1, err := eng.ProbeModelTargetWithCapabilities(context.Background(), "prov_a", toolSrv.URL, "sk-prov-a-secret", "no-tools-model", "openai", true)
	if err != nil || p1.SupportsTools == nil || *p1.SupportsTools {
		t.Fatalf("expected no-tools-model supports_tools=false, got err=%v res=%+v", err, p1)
	}
	p2, err := eng.ProbeModelTargetWithCapabilities(context.Background(), "prov_a", toolSrv.URL, "sk-prov-a-secret", "full-tools-model", "openai", true)
	if err != nil || p2.SupportsTools == nil || !*p2.SupportsTools || p2.SupportsThinking == nil || !*p2.SupportsThinking {
		t.Fatalf("expected full-tools-model supports_tools=true & supports_thinking=true, got err=%v res=%+v", err, p2)
	}
	toolTargets, err := eng.ResolveChatTargetsWithOptions(context.Background(), "auto", ModelResolveOptions{NeedTools: true})
	if err != nil || len(toolTargets) < 2 || toolTargets[0].Model != "full-tools-model" {
		t.Fatalf("expected NeedTools to prefer full-tools-model over no-tools-model, got err=%v targets=%+v", err, toolTargets)
	}

	// 3. Lowest latency & round_robin strategies
	_ = st.SetConfig(store.KeyPoolStrategy, store.PoolStrategyLowestLatency)
	latTargets, err := eng.ResolveChatTargetsWithOptions(context.Background(), "auto", ModelResolveOptions{})
	if err != nil || latTargets[0].Model != "fast-light-model" {
		t.Fatalf("expected lowest_latency to pick fast-light-model first, got err=%v targets=%+v", err, latTargets)
	}

	_ = st.SetConfig(store.KeyPoolStrategy, store.PoolStrategyRoundRobin)
	rr1, _ := eng.ResolveChatTargetsWithOptions(context.Background(), "auto", ModelResolveOptions{})
	rr2, _ := eng.ResolveChatTargetsWithOptions(context.Background(), "auto", ModelResolveOptions{})
	if rr1[0].Model == rr2[0].Model {
		t.Fatalf("expected round_robin to alternate primary target, got %q and %q", rr1[0].Model, rr2[0].Model)
	}

	// 4. Pinned model stays index 0 even when cooling down
	_ = st.SetConfig(store.KeyPoolStrategy, store.PoolStrategyPriority)
	eng.health.recordFailure("pool:prov_a::full-tools-model", "HTTP 429", true)
	pinned, err := eng.ResolveChatTargetsWithOptions(context.Background(), "pool:prov_a::full-tools-model", ModelResolveOptions{})
	if err != nil || len(pinned) < 2 || pinned[0].ID != "pool:prov_a::full-tools-model" {
		t.Fatalf("expected pinned target at index 0 even during cooldown, got err=%v targets=%+v", err, pinned)
	}
	eng.ClearModelCooldown("pool:prov_a::full-tools-model")
	if eng.health.isCooling("pool:prov_a::full-tools-model") {
		t.Fatalf("expected cooldown cleared")
	}

	// 5. CallLightModel prefers IsLight-tagged pool model first
	var lightUsedModel string
	lightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		lightUsedModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"light-ok"}}]}`))
	}))
	defer lightSrv.Close()
	_ = st.SaveModelConfig(
		[]store.ModelProvider{{ID: "prov_a", Name: "通道A", Protocol: "openai", BaseURL: lightSrv.URL, APIKey: "sk-prov-a-secret", Enabled: true}},
		[]store.PooledModel{
			{ProviderID: "prov_a", ModelID: "claude-sonnet-4", Enabled: true, Priority: 10, IsLight: false},
			{ProviderID: "prov_a", ModelID: "gemini-2.5-flash", Enabled: true, Priority: 20, IsLight: true},
		},
		store.PoolStrategyPriority,
	)
	out, err := eng.CallLightModel(context.Background(), "sys", "hello")
	if err != nil || out != "light-ok" || lightUsedModel != "gemini-2.5-flash" {
		t.Fatalf("expected CallLightModel to prefer IsLight model gemini-2.5-flash, got model=%q out=%q err=%v", lightUsedModel, out, err)
	}
}

func TestPromptCachingCostEstimationVisionAndAutoCompact(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "v289_features.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Seed a tiny valid 1x1 PNG in <dataDir>/chat_uploads/test_img.png
	uploadDir := filepath.Join(st.DataDir(), "chat_uploads")
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		t.Fatalf("mkdir chat_uploads: %v", err)
	}
	tinyPNG, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ip1sAAAAASUVORK5CYII=")
	if err := os.WriteFile(filepath.Join(uploadDir, "test_img.png"), tinyPNG, 0o640); err != nil {
		t.Fatalf("write test_img.png: %v", err)
	}

	var lastReqBody map[string]any
	mockClaude := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &lastReqBody)
		isStream, _ := lastReqBody["stream"].(bool)
		if !isStream {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"用户希望持续优化服务器架构，已完成图片排查与第一轮评审。"}}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"total_tokens":1100,"prompt_tokens_details":{"cached_tokens":600}}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, c := range []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"我看到了这张 PNG 图片，架构非常清晰。"}}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":200,"total_tokens":2200,"prompt_tokens_details":{"cached_tokens":1500}}}`,
			`data: [DONE]`,
		} {
			fmt.Fprintln(w, c)
			fmt.Fprintln(w)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer mockClaude.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	if err := st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "or_claude",
			Name:     "OpenRouter",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  mockClaude.URL + "/v1",
			APIKey:   "sk-or-v289-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "or_claude",
			ModelID:    "anthropic/claude-sonnet-4",
			Enabled:    true,
			Priority:   10,
		}},
		store.PoolStrategyPriority,
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	_ = st.SaveBot(store.Bot{
		ID:           "bot_vision",
		Name:         "架构师",
		RoleTitle:    "架构审查与多模态诊断",
		SystemPrompt: "你是资深架构师，分析问题直接给本质结论。",
		Model:        "auto",
		AllowedTools: `["web_search"]`,
	})

	eng := NewEngine(st, &fakeTaskController{st: st})

	// 1. Send a vision chat message referencing local /api/v2/chat/uploads/test_img.png
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "bot_vision",
		Message: "请看这张架构图",
		Images:  []string{"/api/v2/chat/uploads/test_img.png"},
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream with image failed: %v", err)
	}
	if !strings.Contains(resp.Content, "PNG 图片") {
		t.Fatalf("unexpected reply: %q", resp.Content)
	}
	if resp.Usage == nil || resp.Usage.CachedTokens != 1500 || resp.Usage.CostUSD <= 0 {
		t.Fatalf("expected cached_tokens=1500 and positive cost_usd in response usage, got %+v", resp.Usage)
	}

	// Verify upstream payload carried cache_control on system static block & tool, and base64 image_url on user message
	rawReqJSON, _ := json.Marshal(lastReqBody)
	reqStr := string(rawReqJSON)
	if !strings.Contains(reqStr, `"cache_control":{"type":"ephemeral"}`) {
		t.Fatalf("expected cache_control ephemeral in Claude request payload, got: %s", reqStr)
	}
	if !strings.Contains(reqStr, `"type":"image_url"`) || !strings.Contains(reqStr, "data:image/png;base64,") {
		t.Fatalf("expected multipart image_url with base64 data URL in user message, got: %s", reqStr)
	}

	// Verify usage summary persisted cached tokens & USD/CNY cost
	sum, err := st.SummarizeModelUsage(7)
	if err != nil || sum.TotalCachedTokens < 1500 || sum.TotalCostUSD <= 0 || sum.TotalCostCNY <= 0 || sum.CacheHitRate <= 0 {
		t.Fatalf("expected usage summary with cache hit & USD/CNY cost, err=%v sum=%+v", err, sum)
	}

	// 2. Seed additional turns and test CompactBotThread + rolling summary injection
	for i := 0; i < 6; i++ {
		_, _ = st.AppendAgentMessage(store.AgentMessage{
			SessionID: "bot:bot_vision",
			BotID:     "bot_vision",
			Role:      "user",
			Content:   fmt.Sprintf("第 %d 轮架构问题讨论", i+1),
		})
		_, _ = st.AppendAgentMessage(store.AgentMessage{
			SessionID: "bot:bot_vision",
			BotID:     "bot_vision",
			Role:      "assistant",
			Content:   fmt.Sprintf("第 %d 轮架构回答结论", i+1),
		})
	}
	bot, _ := st.GetBot("bot_vision")
	summary, compactedCount, err := eng.CompactBotThread(context.Background(), bot, "")
	if err != nil || compactedCount == 0 || !strings.Contains(summary, "服务器架构") {
		t.Fatalf("CompactBotThread failed: err=%v count=%d summary=%q", err, compactedCount, summary)
	}
	compState := st.GetThreadCompactionState("bot_vision", "")
	if compState.UpToID == 0 || !strings.Contains(compState.Summary, "服务器架构") {
		t.Fatalf("expected persisted compaction state, got %+v", compState)
	}

	// Next turn must inject the rolling summary into upstream messages!
	_, err = eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "bot_vision",
		Message: "继续下一步改造",
	}, nil)
	if err != nil {
		t.Fatalf("post-compact chat failed: %v", err)
	}
	postReqBytes, _ := json.Marshal(lastReqBody)
	if !strings.Contains(string(postReqBytes), "早期对话滚动摘要") || !strings.Contains(string(postReqBytes), "服务器架构") {
		t.Fatalf("expected rolling summary injected into post-compact prompt, got: %s", string(postReqBytes))
	}
}
