package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestParseXHandles(t *testing.T) {
	raw := "@Alice, https://x.com/Bob_Dev/status/123?s=20; twitter.com/carol_ai @alice   https://twitter.com/Dave123\n" +
		"invalid-handle-with-dash u1 u2 u3 u4 u5 u6 u7 u8 u9 u10 u11 u12 u13 u14 u15 u16 u17 u18 u19 u20"
	got := parseXHandles(raw)
	if len(got) != 20 {
		t.Fatalf("expected capped 20 handles, got %d (%v)", len(got), got)
	}
	if got[0] != "Alice" || got[1] != "Bob_Dev" || got[2] != "carol_ai" {
		t.Fatalf("unexpected parsed handles order/values: %v", got[:5])
	}
}

func TestCallXAIXSearchAndCredentialFallback(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "xai_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	var (
		gotAuth    string
		gotPayload map[string]any
	)
	xaiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output": [
				{
					"type": "message",
					"content": [
						{
							"type": "output_text",
							"text": "@AI_Deals_Pro (2026-11-27): AWS Quick Suite 限时黑五活动开放免费体验 Opus，需在控制台绑定活动区。",
							"annotations": [
								{"type": "url_citation", "url": "https://x.com/AI_Deals_Pro/status/999001", "title": "1"}
							]
						}
					]
				}
			],
			"citations": [
				"https://x.com/AI_Deals_Pro/status/999001",
				"https://x.com/BuilderOne/status/999002"
			]
		}`))
	}))
	defer xaiSrv.Close()

	oldEndpoint := xaiResponsesEndpoint
	xaiResponsesEndpoint = xaiSrv.URL
	defer func() { xaiResponsesEndpoint = oldEndpoint }()

	// Verify automatic fallback to enabled ModelProvider whose BaseURL contains api.x.ai
	_ = st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov_xai",
			Name:     "xAI",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  "https://api.x.ai/v1",
			APIKey:   "xai-pool-secret",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov_xai",
			ModelID:    "grok-4.7",
			Enabled:    true,
			Priority:   1,
		}},
		store.PoolStrategyPriority,
	)

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	res, err := eng.callXAIXSearch(context.Background(), "黑五 AI 限时免费羊毛", []string{"@AI_Deals_Pro", "BuilderOne"}, 2, "deals")
	if err != nil {
		t.Fatalf("callXAIXSearch failed: %v", err)
	}
	if gotAuth != "Bearer xai-pool-secret" {
		t.Fatalf("expected pool fallback Bearer key, got %q", gotAuth)
	}
	toolsRaw, _ := gotPayload["tools"].([]any)
	if len(toolsRaw) != 1 {
		t.Fatalf("expected 1 x_search tool, got %+v", gotPayload["tools"])
	}
	toolObj, _ := toolsRaw[0].(map[string]any)
	if toolObj["type"] != "x_search" {
		t.Fatalf("expected type x_search, got %+v", toolObj)
	}
	if fromStr, _ := toolObj["from_date"].(string); fromStr == "" || len(fromStr) != 10 {
		t.Fatalf("expected YYYY-MM-DD from_date, got %+v", toolObj["from_date"])
	}
	handlesRaw, _ := toolObj["allowed_x_handles"].([]any)
	if len(handlesRaw) != 2 || handlesRaw[0] != "AI_Deals_Pro" {
		t.Fatalf("expected cleaned allowed_x_handles, got %+v", handlesRaw)
	}
	if !res.OK || !strings.Contains(res.Summary, "AWS Quick Suite") || len(res.Citations) != 2 {
		t.Fatalf("unexpected xSearchResult: %+v", res)
	}
}

func TestDealsRoutineAntiEvergreenAndSilentSkip(t *testing.T) {
	evergreenSamples := []string{
		"1. Google Pro 会员有 10 刀赠金可领取",
		"1. Google Cloud (GCP) 新用户送 300 美元赠金试用",
		"1. Oracle Cloud 甲骨文永久免费 ARM 服务器",
		"1. GitHub 学生包继续免费白嫖 Copilot",
	}
	for _, s := range evergreenSamples {
		if reason := detectEvergreenPromoFiller(s); reason == "" {
			t.Fatalf("expected evergreen promo filler detected for %q", s)
		}
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "deals_routine_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	xaiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output_text": "@deal_scout: 今天暂无新黑五 AI 限时活动。",
			"citations": ["https://x.com/deal_scout/status/12345"]
		}`))
	}))
	defer xaiSrv.Close()

	oldEndpoint := xaiResponsesEndpoint
	xaiResponsesEndpoint = xaiSrv.URL
	defer func() { xaiResponsesEndpoint = oldEndpoint }()
	_ = st.SetConfig(store.KeyXAIAPIKey, "xai-direct-key")

	var (
		mu       sync.Mutex
		reqCount int
		seenMsgs [][]apiMessage
	)
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []apiMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		reqCount++
		cur := reqCount
		seenMsgs = append(seenMsgs, body.Messages)
		mu.Unlock()

		replyText := "[SILENT_OK]"
		if cur == 1 {
			// Draft 1 lazily falls back to evergreen Google Pro $10 credit filler
			replyText = "今日羊毛推荐：Google Pro 会员有 10 刀赠金可以领取。"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"role": "assistant", "content": replyText}},
			},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", string(chunk))
	}))
	defer llmSrv.Close()

	_ = st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov_llm",
			Name:     "Mock",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  llmSrv.URL + "/v1",
			APIKey:   "sk-deals-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov_llm",
			ModelID:    "mock-chat",
			Enabled:    true,
			Priority:   1,
		}},
		store.PoolStrategyPriority,
	)
	_ = st.SaveBot(store.Bot{
		ID:      "bot_deals",
		Name:    "DealsScout",
		BotMode: store.BotModeWorker,
		Model:   "auto",
	})

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	ctx := store.WithTaskID(context.Background(), "task_daily_deals")
	ctx = store.WithTaskXHandles(ctx, []string{"deal_scout"})

	reply, err := eng.RunBotRoutine(ctx, "bot_deals", "每天推送最新黑五与 AI 羊毛", "每日羊毛推送")
	if err != nil {
		t.Fatalf("RunBotRoutine failed: %v", err)
	}
	if strings.TrimSpace(reply) != "" {
		t.Fatalf("expected silent skip empty reply when rewrite outputs [SILENT_OK], got %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	if reqCount != 2 {
		t.Fatalf("expected 2 LLM calls (evergreen filler intercepted -> silent skip), got %d", reqCount)
	}
	var firstAll, secondAll strings.Builder
	for _, m := range seenMsgs[0] {
		firstAll.WriteString(m.Content)
	}
	for _, m := range seenMsgs[1] {
		secondAll.WriteString(m.Content)
	}
	if !strings.Contains(firstAll.String(), "[X (Twitter) 近 48 小时一手实时动态]") || !strings.Contains(firstAll.String(), "@deal_scout") {
		t.Fatalf("expected X pre-pass context and handle injected in routine prompt")
	}
	if !strings.Contains(firstAll.String(), "推特羊毛博主主编铁律") {
		t.Fatalf("expected strict deals curation rules injected")
	}
	if !strings.Contains(secondAll.String(), "常驻老羊毛") {
		t.Fatalf("expected evergreen filler rewrite reason in second turn")
	}
}
