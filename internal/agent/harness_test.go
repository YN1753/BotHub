package agent

import (
	"context"
	"encoding/base64"
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

func TestParseBingHTMLAndRedirectUnwrap(t *testing.T) {
	realURL := "https://go.dev/blog/go1.24"
	encoded := "a1" + base64.RawURLEncoding.EncodeToString([]byte(realURL))
	bingClickURL := "https://www.bing.com/ck/a?u=" + encoded + "&ntb=1"

	if got := unwrapBingRedirectURL(bingClickURL); got != realURL {
		t.Fatalf("expected unwrapped URL %q, got %q", realURL, got)
	}

	sampleHTML := fmt.Sprintf(`
<html><body><ol id="b_results">
  <li class="b_algo">
    <h2><a href="%s">Go 1.24 发布说明 - Go 官方博客</a></h2>
    <div class="b_caption"><p>Go 1.24 带来了泛型类型别名与性能改进。</p></div>
  </li>
  <li class="b_algo">
    <h2><a href="https://example.com/article">第二条普通链接</a></h2>
    <div class="b_caption"><p>这是一段简短摘要。</p></div>
  </li>
</ol></body></html>`, bingClickURL)

	items := parseBingHTML(sampleHTML, 5)
	if len(items) != 2 {
		t.Fatalf("expected 2 Bing results, got %d: %+v", len(items), items)
	}
	if items[0].URL != realURL || !strings.Contains(items[0].Title, "Go 1.24") || !strings.Contains(items[0].Snippet, "泛型类型别名") {
		t.Fatalf("unexpected first Bing item: %+v", items[0])
	}
	if items[0].Source != "Bing" {
		t.Fatalf("expected source Bing, got %q", items[0].Source)
	}
}

func TestParseGoogleNewsRSS(t *testing.T) {
	sampleXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>Google News</title>
  <item>
    <title>AI 模型推理成本继续下降 - 科技日报</title>
    <link>https://news.google.com/rss/articles/CBMiTest1</link>
    <pubDate>Thu, 19 Nov 2026 08:30:00 GMT</pubDate>
    <description>&lt;a href="https://example.com"&gt;详细报道&lt;/a&gt; 行业最新动态。</description>
    <source url="https://tech.example.com">科技日报</source>
  </item>
</channel></rss>`

	items := parseGoogleNewsRSS(sampleXML, 3)
	if len(items) != 1 {
		t.Fatalf("expected 1 RSS item, got %d", len(items))
	}
	if items[0].Source != "GoogleNews" || !strings.Contains(items[0].Title, "AI 模型推理成本") || !strings.Contains(items[0].Snippet, "科技日报") {
		t.Fatalf("unexpected RSS item: %+v", items[0])
	}
}

func TestWebSearchTavilyAndSearXNGPriority(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "search_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	tavilySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["api_key"] != "tvly-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Tavily 首条结果","url":"https://example.com/shared","content":"Tavily 提供的深度摘要正文，包含完整事实与参数细节。"}]}`))
	}))
	defer tavilySrv.Close()

	searxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"重复链接应去重","url":"https://www.example.com/shared/","content":"重复"},{"title":"SearXNG 补充结果","url":"https://docs.example.org/guide","content":"SearXNG 返回的技术文档摘要"}]}`))
	}))
	defer searxSrv.Close()

	oldEndpoint := tavilySearchEndpoint
	tavilySearchEndpoint = tavilySrv.URL
	defer func() { tavilySearchEndpoint = oldEndpoint }()

	_ = st.SetConfig(store.KeyTavilyAPIKey, "tvly-test-key")
	_ = st.SetConfig(store.KeySearXNGURL, searxSrv.URL)

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	raw, err := eng.toolWebSearch(context.Background(), `{"query":"DeepSeek R1","limit":5}`)
	if err != nil {
		t.Fatalf("toolWebSearch failed: %v", err)
	}
	var out struct {
		Count         int             `json:"count"`
		ProvidersUsed []string        `json:"providers_used"`
		Results       []webSearchItem `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out.Count != 2 {
		t.Fatalf("expected 2 deduplicated results, got %d: %+v", out.Count, out.Results)
	}
	if out.Results[0].Source != "Tavily" || out.Results[1].Source != "SearXNG" {
		t.Fatalf("expected Tavily first then SearXNG, got %+v", out.Results)
	}
}

func TestPruneToolsForTurn(t *testing.T) {
	allTools := BuiltinTools()
	if len(allTools) <= 7 {
		t.Fatalf("expected >7 builtin tools, got %d", len(allTools))
	}

	// 1. Web search prompt should prune to <=7 tools and include web_search & read_url
	webPruned := pruneToolsForTurn(allTools, "帮我联网搜一下 2026 年最新的开源大模型评测", nil)
	if len(webPruned) < 4 || len(webPruned) > 7 {
		t.Fatalf("expected 4..7 pruned tools, got %d", len(webPruned))
	}
	if !hasToolNamed(webPruned, "web_search") || !hasToolNamed(webPruned, "read_url") {
		t.Fatalf("expected web_search and read_url in pruned set, got %+v", webPruned)
	}

	// 2. Server monitoring prompt should include query_nezha_nodes
	vpsPruned := pruneToolsForTurn(allTools, "看看我的服务器和探针节点现在负载高不高", nil)
	if !hasToolNamed(vpsPruned, "query_nezha_nodes") {
		t.Fatalf("expected query_nezha_nodes in pruned set")
	}

	// 3. Asset renewal prompt should include manage_assets
	assetPruned := pruneToolsForTurn(allTools, "帮我算下本月到期订阅和资产续费总额", nil)
	if !hasToolNamed(assetPruned, "manage_assets") {
		t.Fatalf("expected manage_assets in pruned set")
	}
}

func TestDetectPreflightGrounding(t *testing.T) {
	allTools := BuiltinTools()

	// Explicit web search -> web_search
	tool, args, ok := detectPreflightGrounding("帮我联网搜一下 Claude Code 最新功能", allTools)
	if !ok || tool != "web_search" || !strings.Contains(args, "Claude Code") {
		t.Fatalf("expected web_search preflight, got ok=%v tool=%q args=%s", ok, tool, args)
	}

	// Inline URL -> read_url
	tool, args, ok = detectPreflightGrounding("总结一下这篇文章 https://example.com/post/123，列出 3 个重点", allTools)
	if !ok || tool != "read_url" || !strings.Contains(args, "https://example.com/post/123") {
		t.Fatalf("expected read_url preflight, got ok=%v tool=%q args=%s", ok, tool, args)
	}

	// Bookmarking intent -> should NOT hijack with read_url
	if tool, _, ok := detectPreflightGrounding("帮我收藏这个链接 https://example.com/save-me", allTools); ok {
		t.Fatalf("expected bookmarking request not to preflight %q", tool)
	}

	// Local workspace query -> should NOT hijack with web_search
	if tool, _, ok := detectPreflightGrounding("查一下我的待办清单和最近到期资产", allTools); ok {
		t.Fatalf("expected local workspace query not to preflight %q", tool)
	}
}

func TestCorrectionGuardrailCompactsOldTurnsAndInjectsDirective(t *testing.T) {
	history := []store.AgentMessage{
		{Role: "user", Content: "帮我查下某框架优缺点"},
		{Role: "assistant", Content: strings.Repeat("旧的模板回答段落。", 30)},
		{Role: "user", Content: "请给具体数据"},
		{Role: "assistant", Content: "上一轮仍然比较泛泛的回答。"},
	}
	if !detectCorrectionTurn("不对，你说了三次都是一样的答案，没按我要求列出对比表格", history) {
		t.Fatalf("expected correction turn to be detected")
	}
	if detectCorrectionTurn("不对，重来", nil) {
		t.Fatalf("expected correction turn to be false without prior assistant message")
	}

	msgs := []apiMessage{
		{Role: "system", Content: "base system prompt"},
		{Role: "user", Content: history[0].Content},
		{Role: "assistant", Content: history[1].Content},
		{Role: "user", Content: history[2].Content},
		{Role: "assistant", Content: history[3].Content},
		{Role: "user", Content: "不对，没按我要求来，重新回答"},
	}
	guarded := applyCorrectionGuardrail(msgs)
	if len(guarded) != len(msgs)+1 {
		t.Fatalf("expected 1 appended correction system message, got %d vs %d", len(guarded), len(msgs))
	}
	if !strings.Contains(guarded[2].Content, "此前已被用户否定的旧回复已折叠") {
		t.Fatalf("expected older rejected assistant message to be compacted, got %q", guarded[2].Content)
	}
	if guarded[4].Content != history[3].Content {
		t.Fatalf("expected immediately preceding assistant message to remain intact")
	}
	if guarded[len(guarded)-1].Role != "system" || !strings.Contains(guarded[len(guarded)-1].Content, "纠偏执行指令") {
		t.Fatalf("expected final correction system directive, got %+v", guarded[len(guarded)-1])
	}
}

func TestEndToEndBotChatPreflightAndCorrection(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "harness_e2e.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SaveBot(store.Bot{
		ID:           "butler_test",
		Name:         "测试管家",
		RoleTitle:    "全能助手",
		SystemPrompt: "简洁客观回答",
		AllowedTools: `["web_search","read_url","manage_todos"]`,
		Model:        "primary",
	})

	// Mock SearXNG so preflight web_search succeeds offline in unit test
	searxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Go 1.24 Release Notes","url":"https://go.dev/doc/go1.24","content":"Go 1.24 正式发布，引入弱指针与 SwissTable map。"}]}`))
	}))
	defer searxSrv.Close()
	_ = st.SetConfig(store.KeySearXNGURL, searxSrv.URL)

	var capturedBody string
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capturedBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, line := range []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"根据检索结果，Go 1.24 引入了弱指针与 SwissTable map。"}}]}`,
			`data: [DONE]`,
		} {
			fmt.Fprintln(w, line)
			fmt.Fprintln(w)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer llmSrv.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-test-key")
	_ = st.SetConfig(store.KeyDeepSeekBaseURL, llmSrv.URL)

	eng := NewEngine(st, &fakeTaskController{st: st})
	eng.allowLocalhostForTest = true

	var sseEvents []string
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "butler_test",
		Message: "帮我联网搜一下 Go 1.24 新特性",
	}, func(event string, _ any) {
		sseEvents = append(sseEvents, event)
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if !strings.Contains(strings.Join(sseEvents, ","), "tool_start") || !strings.Contains(strings.Join(sseEvents, ","), "tool_result") {
		t.Fatalf("expected preflight tool_start and tool_result SSE events, got %v", sseEvents)
	}
	if !strings.Contains(capturedBody, "SwissTable map") {
		t.Fatalf("expected preflight search result injected into LLM request body, got: %s", capturedBody)
	}
	if len(resp.ToolTraces) == 0 || resp.ToolTraces[0].Name != "web_search" {
		t.Fatalf("expected preflight web_search trace in response, got %+v", resp.ToolTraces)
	}
}
