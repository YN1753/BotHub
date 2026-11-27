package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestParseTGChannels(t *testing.T) {
	raw := "@Channel_One, https://t.me/s/channel_two?q=1; t.me/channel_three\n" +
		"https://telegram.me/s/Channel_Four/99 @channel_one bad"
	got := parseTGChannels(raw)
	if len(got) != 4 {
		t.Fatalf("expected 4 unique channels, got %d (%v)", len(got), got)
	}
	if got[0] != "Channel_One" || got[1] != "channel_two" || got[2] != "channel_three" || got[3] != "Channel_Four" {
		t.Fatalf("unexpected channels: %v", got)
	}
	if ch, ok := extractTGChannelFromURL("https://t.me/s/ai_deals_channel"); !ok || ch != "ai_deals_channel" {
		t.Fatalf("expected channel extraction from t.me/s URL, got ch=%q ok=%v", ch, ok)
	}
}

func TestParseTelegramChannelHTMLAndTimeWindow(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	freshTime := now.Add(-4 * time.Hour).Format(time.RFC3339)
	staleTime := now.Add(-120 * time.Hour).Format(time.RFC3339)

	sampleHTML := fmt.Sprintf(`<!DOCTYPE html><html><body>
<div class="tgme_widget_message_wrap">
  <div class="tgme_widget_message" data-post="deals_feed/101">
    <div class="tgme_widget_message_text">Old deal that already expired<br/>Skip this one.</div>
    <time datetime="%s"></time>
    <span class="tgme_widget_message_views">1.2K</span>
  </div>
</div>
<div class="tgme_widget_message_wrap">
  <div class="tgme_widget_message" data-post="deals_feed/102">
    <div class="tgme_widget_message_text">AWS Quick Suite 限时白嫖 Opus<br/>领取地址：<a href="https://aws.amazon.com/promo-test">直达页面</a></div>
    <div class="tgme_widget_message_link_preview"><a href="https://example.com/guide">教程</a></div>
    <time datetime="%s"></time>
    <span class="tgme_widget_message_views">18.5K</span>
  </div>
</div>
</body></html>`, staleTime, freshTime)

	recent := parseTelegramChannelHTML(sampleHTML, "deals_feed", now, 48, 10)
	if len(recent) != 1 {
		t.Fatalf("expected only 1 fresh message within 48h, got %d (%+v)", len(recent), recent)
	}
	if recent[0].URL != "https://t.me/deals_feed/102" || recent[0].Views != "18.5K" {
		t.Fatalf("unexpected parsed metadata: %+v", recent[0])
	}
	if !strings.Contains(recent[0].Text, "AWS Quick Suite") || len(recent[0].Links) != 2 {
		t.Fatalf("unexpected parsed text/links: %+v", recent[0])
	}
}

func TestToolReadURLTelegramChannel(t *testing.T) {
	freshTime := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	tgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Telegram @ai_news</title></head><body>
<div class="tgme_widget_message_wrap"><div class="tgme_widget_message" data-post="ai_news/55">
  <div class="tgme_widget_message_text">Go 1.26 released with new profiler <a href="https://go.dev/blog">Go Blog</a></div>
  <time datetime="%s"></time>
</div></div></body></html>`, freshTime)
	}))
	defer tgSrv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "tg_readurl_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	raw, err := eng.toolReadURL(context.Background(), `{"url":"`+tgSrv.URL+`"}`)
	if err != nil {
		t.Fatalf("toolReadURL failed on TG HTML: %v", err)
	}
	var out struct {
		SourceMode string `json:"source_mode"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal read_url output: %v", err)
	}
	if out.SourceMode != "telegram_channel" || !strings.Contains(out.Content, "Go 1.26") {
		t.Fatalf("unexpected telegram read_url output: %+v", out)
	}
}

func TestWebSearchDomainsForwarding(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "domains_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	var (
		gotTavilyBody map[string]any
		gotSerperBody map[string]any
	)
	tavilySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotTavilyBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"X Promo Post","url":"https://x.com/user/status/1","content":"Fresh black friday promo."}]}`))
	}))
	defer tavilySrv.Close()

	serperSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotSerperBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organic":[{"title":"Linux.do Thread","link":"https://linux.do/t/topic/1","snippet":"New free tier discovered."}]}`))
	}))
	defer serperSrv.Close()

	oldTavily := tavilySearchEndpoint
	oldSerper := serperSearchEndpoint
	tavilySearchEndpoint = tavilySrv.URL
	serperSearchEndpoint = serperSrv.URL
	defer func() {
		tavilySearchEndpoint = oldTavily
		serperSearchEndpoint = oldSerper
	}()

	_ = st.SetConfig(store.KeyTavilyAPIKey, "tvly-key")
	_ = st.SetConfig(store.KeySerperAPIKey, "serper-key")

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	_, err = eng.toolWebSearch(context.Background(), `{"query":"Black Friday AI","time_range":"day","topic":"deals","domains":["https://x.com/","linux.do"]}`)
	if err != nil {
		t.Fatalf("toolWebSearch failed: %v", err)
	}
	inc, _ := gotTavilyBody["include_domains"].([]any)
	if len(inc) != 2 || inc[0] != "x.com" || inc[1] != "linux.do" {
		t.Fatalf("expected include_domains normalized in Tavily request, got %+v", gotTavilyBody["include_domains"])
	}
	sq, _ := gotSerperBody["q"].(string)
	if !strings.Contains(sq, "site:x.com OR site:linux.do") {
		t.Fatalf("expected site filter in Serper query, got %q", sq)
	}
}

func TestRoutineTelegramPrepassInjection(t *testing.T) {
	freshTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	tgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<div class="tgme_widget_message_wrap"><div class="tgme_widget_message" data-post="xianbao_test/888">
  <div class="tgme_widget_message_text">突发限时口子：CloudCode 新活动送 50 刀限时体验金 <a href="https://cloudcode.example.com">入口</a></div>
  <time datetime="%s"></time>
</div></div></body></html>`, freshTime)
	}))
	defer tgSrv.Close()

	oldTG := telegramPublicBaseURL
	telegramPublicBaseURL = tgSrv.URL
	defer func() { telegramPublicBaseURL = oldTG }()

	st, err := store.Open(filepath.Join(t.TempDir(), "tg_routine_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	var seenPrompt string
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []apiMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			seenPrompt += m.Content + "\n"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"role": "assistant", "content": "1. CloudCode 限时新活动送 50 刀体验金，24小时有效：https://cloudcode.example.com"}},
			},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", string(chunk))
	}))
	defer llmSrv.Close()

	_ = st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov_tg",
			Name:     "Mock",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  llmSrv.URL + "/v1",
			APIKey:   "sk-tg-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov_tg",
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

	ctx := store.WithTaskID(context.Background(), "task_tg_deals")
	ctx = store.WithTaskTGChannels(ctx, []string{"xianbao_test"})

	reply, err := eng.RunBotRoutine(ctx, "bot_deals", "每日推送最新羊毛", "每日羊毛推送")
	if err != nil {
		t.Fatalf("RunBotRoutine failed: %v", err)
	}
	if !strings.Contains(reply, "CloudCode") {
		t.Fatalf("expected CloudCode deal in reply, got %q", reply)
	}
	if !strings.Contains(seenPrompt, "[Telegram 公开频道近 48 小时一手实时动态]") || !strings.Contains(seenPrompt, "@xianbao_test") {
		t.Fatalf("expected Telegram channel digest pre-injected into routine prompt, got:\n%s", seenPrompt)
	}
}
