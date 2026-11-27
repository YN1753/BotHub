package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestWebSearchParametersAndPremiumAPIs(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "search_premium_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	var (
		gotTavilyBody  map[string]any
		gotSerperBody  map[string]any
		gotBraveQuery  string
		gotSearXQuery  string
	)

	tavilySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotTavilyBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"AWS Black Friday Free Tier Opus","url":"https://aws.example.com/bf","content":"AWS offers Black Friday AI free tier credits.","published_date":"2026-11-26T10:00:00Z"}]}`))
	}))
	defer tavilySrv.Close()

	serperSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "serper-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotSerperBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organic":[{"title":"Google Cloud Black Friday Credits","link":"https://cloud.google.com/bf-2026","snippet":"Get new Google Cloud credits for AI.","date":"1 day ago"}]}`))
	}))
	defer serperSrv.Close()

	braveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Subscription-Token") != "brave-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		gotBraveQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Brave Deal Hit","url":"https://deals.example.com/brave","description":"Developer <b>free tier</b> promo.","page_age":"2026-11-26T08:00:00Z"}]}}`))
	}))
	defer braveSrv.Close()

	searxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSearXQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Duplicate Link Should Dedupe","url":"https://cloud.google.com/bf-2026/","content":"dup"},{"title":"SearXNG Fresh Hit","url":"https://searx.example.org/promo","content":"Fresh promo details","publishedDate":"2026-11-27T01:00:00Z"}]}`))
	}))
	defer searxSrv.Close()

	oldTavily := tavilySearchEndpoint
	oldSerper := serperSearchEndpoint
	oldBrave := braveSearchEndpoint
	tavilySearchEndpoint = tavilySrv.URL
	serperSearchEndpoint = serperSrv.URL
	braveSearchEndpoint = braveSrv.URL
	defer func() {
		tavilySearchEndpoint = oldTavily
		serperSearchEndpoint = oldSerper
		braveSearchEndpoint = oldBrave
	}()

	_ = st.SetConfig(store.KeyTavilyAPIKey, "tvly-key")
	_ = st.SetConfig(store.KeySerperAPIKey, "serper-test-key")
	_ = st.SetConfig(store.KeyBraveAPIKey, "brave-test-key")
	_ = st.SetConfig(store.KeySearXNGURL, searxSrv.URL)

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	raw, err := eng.toolWebSearch(context.Background(), `{"query":"Black Friday AI free tier","time_range":"day","topic":"deals","lang":"all","limit":8}`)
	if err != nil {
		t.Fatalf("toolWebSearch failed: %v", err)
	}

	if gotTavilyBody["time_range"] != "day" || gotTavilyBody["topic"] != "news" || gotTavilyBody["search_depth"] != "advanced" {
		t.Fatalf("unexpected tavily payload: %+v", gotTavilyBody)
	}
	if gotSerperBody["tbs"] != "qdr:d" {
		t.Fatalf("unexpected serper payload: %+v", gotSerperBody)
	}
	if !strings.Contains(gotBraveQuery, "freshness=pd") {
		t.Fatalf("expected freshness=pd in brave query, got %q", gotBraveQuery)
	}
	if !strings.Contains(gotSearXQuery, "time_range=day") || !strings.Contains(gotSearXQuery, "language=all") {
		t.Fatalf("unexpected searx query: %q", gotSearXQuery)
	}

	var out struct {
		TimeRange     string          `json:"time_range"`
		Topic         string          `json:"topic"`
		Count         int             `json:"count"`
		ProvidersUsed []string        `json:"providers_used"`
		Results       []webSearchItem `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.TimeRange != "day" || out.Topic != "deals" {
		t.Fatalf("unexpected metadata: %+v", out)
	}
	if out.Count != 4 {
		t.Fatalf("expected 4 deduplicated results across 4 providers, got %d: %+v", out.Count, out.Results)
	}
	if out.Results[0].PublishedAt == "" {
		t.Fatalf("expected published_at populated, got %+v", out.Results[0])
	}
}

func TestMergeSearchBucketsDiversity(t *testing.T) {
	var bingItems []webSearchItem
	for i := 1; i <= 10; i++ {
		bingItems = append(bingItems, webSearchItem{
			Title:  "Bing Result " + string(rune('A'+i-1)),
			URL:    "https://bing.example.com/" + string(rune('a'+i-1)),
			Source: "Bing",
		})
	}
	liveBuckets := [][]webSearchItem{
		bingItems,
		{{Title: "Google News 1", URL: "https://news.example.com/1", Source: "GoogleNews-EN"}},
		{{Title: "GitHub New Repo", URL: "https://github.com/acme/go-agent", Source: "GitHub"}},
		{{Title: "V2EX Deal", URL: "https://www.v2ex.com/t/12345", Source: "V2EX"}},
		{{Title: "NodeSeek Deal", URL: "https://www.nodeseek.com/post-1-1", Source: "NodeSeek"}},
	}

	merged, providers := mergeSearchBuckets(nil, liveBuckets, nil, 8)
	if len(merged) != 8 {
		t.Fatalf("expected 8 merged items, got %d", len(merged))
	}
	providerSet := make(map[string]bool)
	for _, p := range providers {
		providerSet[p] = true
	}
	for _, expected := range []string{"Bing", "GoogleNews-EN", "GitHub", "V2EX", "NodeSeek"} {
		if !providerSet[expected] {
			t.Fatalf("expected provider %q in merged diversity output, got %v", expected, providers)
		}
	}
}

func TestVerticalParsersAndFeedReadURL(t *testing.T) {
	v2exJSON := `[
		{"title":"常规水帖","url":"https://www.v2ex.com/t/1","content":"随便聊聊","replies":2,"created":1764200000,"node":{"title":"分享发现","name":"share"}},
		{"title":"AWS Quick Suite 免费用 Claude Opus 黑五羊毛","url":"https://www.v2ex.com/t/2","content":"实测可用","replies":42,"created":1764201000,"node":{"title":"优惠信息","name":"deals"}}
	]`
	v2exItems := parseV2EXTopicsJSON(v2exJSON, "V2EX", extractSearchTerms("羊毛 黑五 aws"), []string{"羊毛", "免费", "aws"}, 5)
	if len(v2exItems) != 2 || !strings.Contains(v2exItems[0].Title, "AWS") || v2exItems[0].PublishedAt == "" {
		t.Fatalf("expected relevant AWS deal ranked first with timestamp, got %+v", v2exItems)
	}

	ghJSON := `{"items":[{"full_name":"acme/go-agent-kit","html_url":"https://github.com/acme/go-agent-kit","description":"Lightweight Go AI agent toolkit","language":"Go","stargazers_count":320,"created_at":"2026-11-25T12:00:00Z"}]}`
	ghItems := parseGitHubReposJSON(ghJSON, 5)
	if len(ghItems) != 1 || ghItems[0].Source != "GitHub" || ghItems[0].PublishedAt != "2026-11-25" {
		t.Fatalf("unexpected github parse output: %+v", ghItems)
	}

	atomXML := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>The Go Blog</title>
  <entry>
    <title>Go 1.26 Flight Recorder</title>
    <link rel="alternate" href="https://go.dev/blog/flight-recorder"/>
    <published>2026-11-20T10:00:00Z</published>
    <summary>Low-overhead tracing for production Go services.</summary>
  </entry>
</feed>`
	feedTitle, atomItems := parseRSSOrAtomItems(atomXML, "Atom", 5)
	if feedTitle != "The Go Blog" || len(atomItems) != 1 || atomItems[0].URL != "https://go.dev/blog/flight-recorder" {
		t.Fatalf("unexpected atom parse output: title=%q items=%+v", feedTitle, atomItems)
	}

	rssSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>NodeSeek Deals</title>
  <item>
    <title>Black Friday Cloud Deal</title>
    <link>https://www.nodeseek.com/post-999-1</link>
    <pubDate>Thu, 26 Nov 2026 09:00:00 GMT</pubDate>
    <description>Annual promo with free AI credits.</description>
  </item>
</channel></rss>`))
	}))
	defer rssSrv.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "feed_read_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	readOut, err := eng.toolReadURL(context.Background(), `{"url":"`+rssSrv.URL+`"}`)
	if err != nil {
		t.Fatalf("toolReadURL failed on RSS: %v", err)
	}
	var readParsed struct {
		SourceMode string `json:"source_mode"`
		Title      string `json:"title"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal([]byte(readOut), &readParsed); err != nil {
		t.Fatalf("unmarshal read_url output: %v", err)
	}
	if readParsed.SourceMode != "feed" || readParsed.Title != "NodeSeek Deals" || !strings.Contains(readParsed.Content, "Black Friday Cloud Deal") {
		t.Fatalf("unexpected feed read_url output: %+v", readParsed)
	}
}

func TestResearchRoutineExpansionPrompt(t *testing.T) {
	if !isResearchOrDigestRoutine("每日 AI 与 Go 推荐", "每天搜集 AI 和 Go 的最新前沿动态与开源项目推荐") {
		t.Fatalf("expected AI+Go digest classified as research routine")
	}
	if !isResearchOrDigestRoutine("每日羊毛推送", "每天搜集各大云厂商、Google 黑五、AWS 免费用 Opus 等羊毛优惠") {
		t.Fatalf("expected deals routine classified as research routine")
	}
	if isResearchOrDigestRoutine("每日冷知识", "每天推送一条科普冷知识，比如猫为什么喜欢钻纸箱") {
		t.Fatalf("did not expect trivia routine classified as live research")
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "routine_research_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	_, _ = st.SaveTodo(store.TodoItem{Title: "PERSONAL_SECRET_TODO_BUY_MILK", Status: "open"})

	now := time.Date(2026, 11, 27, 9, 0, 0, 0, store.ShanghaiLocation())
	snap := buildRoutineContextSnapshotForMode(st, "task_ai_go", now, true)
	if strings.Contains(snap, "PERSONAL_SECRET_TODO_BUY_MILK") || strings.Contains(snap, "今日轮换切入维度") {
		t.Fatalf("expected research snapshot to skip personal todos and generic essay angle, got:\n%s", snap)
	}
	if !strings.Contains(snap, "时效优先窗口") {
		t.Fatalf("expected freshness window in research snapshot, got:\n%s", snap)
	}

	dealsMsg := buildRoutinePromptMessage("每日羊毛推送", "搜索黑五优惠与云厂商免费 AI 额度", now, snap, "", nil, false)
	if !strings.Contains(dealsMsg, "推特羊毛博主主编铁律") || !strings.Contains(dealsMsg, "time_range") {
		t.Fatalf("expected deals curation rules in prompt, got:\n%s", dealsMsg)
	}
	techMsg := buildRoutinePromptMessage("每日 AI 与 Go 推荐", "每天搜集 AI 和 Go 的最新前沿动态与开源项目推荐", now, snap, "", nil, false)
	if !strings.Contains(techMsg, "多维发散调研策略") || !strings.Contains(techMsg, "x_search") {
		t.Fatalf("expected research expansion instructions in prompt, got:\n%s", techMsg)
	}
}
