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

	// Local workspace query -> should NEVER hijack with web_search (may preflight local read tool manage_assets instead)
	if tool, _, ok := detectPreflightGrounding("查一下我的待办清单和最近到期资产", allTools); ok && tool == "web_search" {
		t.Fatalf("expected local workspace query not to preflight web_search, got %q", tool)
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

func TestSplitInlineThinkTagsAndRescueEmptyContent(t *testing.T) {
	think, clean := splitInlineThinkTags("<think>先分析一下用户的真实需求</think>\n这是最终给用户的正式答复。")
	if think != "先分析一下用户的真实需求" || clean != "这是最终给用户的正式答复。" {
		t.Fatalf("unexpected inline think split: think=%q clean=%q", think, clean)
	}

	// Orphaned closing </think>
	think2, clean2 := splitInlineThinkTags("推理步骤一\n推理步骤二</think>最终结论在这里")
	if !strings.Contains(think2, "推理步骤一") || clean2 != "最终结论在这里" {
		t.Fatalf("unexpected orphaned closing think split: think=%q clean=%q", think2, clean2)
	}

	rescued := rescueEmptyContentFromReasoning("第一步：核对条件。\n\n综上所述，推荐使用方案 A，可降低 40% 延迟。")
	if !strings.Contains(rescued, "推荐使用方案 A") {
		t.Fatalf("unexpected rescued content: %q", rescued)
	}
}

func TestExtractTextEmbeddedToolCalls(t *testing.T) {
	tools := BuiltinTools()

	// 1. <tool_call> XML tag
	cleaned, calls := extractTextEmbeddedToolCalls(
		"我先帮你查一下。\n<tool_call>{\"name\":\"web_search\",\"arguments\":{\"q\":\"Rust 2026 路线图\"}}</tool_call>",
		tools,
	)
	if len(calls) != 1 || calls[0].Function.Name != "web_search" || !strings.Contains(calls[0].Function.Arguments, `"query":"Rust 2026 路线图"`) {
		t.Fatalf("expected rescued web_search call with repaired query arg, got calls=%+v cleaned=%q", calls, cleaned)
	}
	if strings.Contains(cleaned, "<tool_call>") {
		t.Fatalf("expected <tool_call> stripped from cleaned content: %q", cleaned)
	}

	// 2. <function=...> tag
	_, calls2 := extractTextEmbeddedToolCalls(`<function=read_url>{"link":"https://example.com/doc"}</function>`, tools)
	if len(calls2) != 1 || calls2[0].Function.Name != "read_url" || !strings.Contains(calls2[0].Function.Arguments, `"url":"https://example.com/doc"`) {
		t.Fatalf("expected rescued read_url call, got %+v", calls2)
	}

	// 3. ```json code fence with tool/parameters
	_, calls3 := extractTextEmbeddedToolCalls("```json\n{\"tool\":\"manage_todos\",\"parameters\":{\"text\":\"下午三点开会\"}}\n```", tools)
	if len(calls3) != 1 || calls3[0].Function.Name != "manage_todos" || !strings.Contains(calls3[0].Function.Arguments, `"action":"add"`) || !strings.Contains(calls3[0].Function.Arguments, `"title":"下午三点开会"`) {
		t.Fatalf("expected rescued manage_todos call with auto-inferred action=add, got %+v", calls3)
	}
}

func TestRepairToolArguments(t *testing.T) {
	// web_search alias q + string limit
	wsFixed := repairToolArguments("web_search", `Here is JSON: {"q":"Go 1.24","limit":"6",}`)
	if !strings.Contains(wsFixed, `"query":"Go 1.24"`) || !strings.Contains(wsFixed, `"limit":6`) {
		t.Fatalf("unexpected repaired web_search args: %s", wsFixed)
	}

	// manage_notes missing action with content -> upsert
	notesFixed := repairToolArguments("manage_notes", `{"name":"会议纪要","body":"讨论了 Q4 目标"}`)
	if !strings.Contains(notesFixed, `"action":"upsert"`) || !strings.Contains(notesFixed, `"title":"会议纪要"`) || !strings.Contains(notesFixed, `"content":"讨论了 Q4 目标"`) {
		t.Fatalf("unexpected repaired manage_notes args: %s", notesFixed)
	}

	// manage_stash mark_read alias
	stashFixed := repairToolArguments("manage_stash", `{"op":"mark_read","id":"st_1"}`)
	if !strings.Contains(stashFixed, `"action":"update"`) || !strings.Contains(stashFixed, `"status":"read"`) {
		t.Fatalf("unexpected repaired manage_stash args: %s", stashFixed)
	}
}

func TestDistillSearchKeywordsAndImplicitPreflight(t *testing.T) {
	primary, fallback := distillSearchKeywords("不对，请帮我联网搜一下关于 2026 年最新的 MacBook Air M4 价格是多少钱？，要求用表格列出")
	if strings.Contains(primary, "要求用表格") || strings.Contains(primary, "请帮我联网") {
		t.Fatalf("expected conversational prefix and formatting clause stripped, got primary=%q fallback=%q", primary, fallback)
	}

	allTools := BuiltinTools()
	// Implicit factual query without "搜一下"
	tool, args, ok := detectPreflightGrounding("请问 2026 年 iPhone 17 最新价格是多少钱？", allTools)
	if !ok || tool != "web_search" {
		t.Fatalf("expected implicit factual query to trigger web_search preflight, got ok=%v tool=%q args=%s", ok, tool, args)
	}

	// Local workspace read queries -> preflight corresponding read tool
	tool, args, ok = detectPreflightGrounding("看看我的服务器和哪吒探针节点现在状态正常吗？", allTools)
	if !ok || tool != "query_nezha_nodes" {
		t.Fatalf("expected server status query to preflight query_nezha_nodes, got ok=%v tool=%q args=%s", ok, tool, args)
	}

	tool, args, ok = detectPreflightGrounding("我有哪些快到期的订阅和资产？", allTools)
	if !ok || tool != "manage_assets" || !strings.Contains(args, "expiring") {
		t.Fatalf("expected expiring asset query to preflight manage_assets(expiring), got ok=%v tool=%q args=%s", ok, tool, args)
	}

	// Workspace mutation command -> should NOT preflight a list read tool
	if tool, _, ok := detectPreflightGrounding("帮我新增一个资产订阅：Netflix 月付 68 元", allTools); ok {
		t.Fatalf("expected mutation command not to preflight read tool, got %q", tool)
	}
}

func TestDuplicateToolLoopCircuitBreakerAndAntiRepetitionRewrite(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "loop_breaker.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SaveBot(store.Bot{
		ID:           "loop_bot",
		Name:         "防死循环助手",
		RoleTitle:    "测试助手",
		SystemPrompt: "简洁回答",
		AllowedTools: `["manage_todos"]`,
		Model:        "primary",
	})

	// Seed a prior assistant reply so the next turn is a correction turn
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:loop_bot",
		BotID:     "loop_bot",
		Role:      "user",
		Content:   "介绍一下方案特点",
	})
	repeatedReply := "这个方案具有高性能、低延迟、易于部署和跨平台兼容四大核心特点，非常适合生产环境使用。"
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:loop_bot",
		BotID:     "loop_bot",
		Role:      "assistant",
		Content:   repeatedReply,
	})

	callSeq := 0
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callSeq++
		raw, _ := io.ReadAll(r.Body)
		bodyStr := string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		var chunk string
		if strings.Contains(bodyStr, "系统强制重写指令") {
			chunk = `data: {"choices":[{"delta":{"role":"assistant","content":"1. 内存占用仅 18MB；2. 支持 SQLite 单文件部署；3. 内置多源并发检索。"}}]}`
		} else {
			// Model stubbornly repeats the exact old reply on initial generation
			chunk = fmt.Sprintf(`data: {"choices":[{"delta":{"role":"assistant","content":%q}}]}`, repeatedReply)
		}
		for _, line := range []string{chunk, `data: [DONE]`} {
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

	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "loop_bot",
		Message: "不对，你说了三次都是一样的答案，按要求列 3 条具体指标数据",
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if !strings.Contains(resp.Content, "18MB") || strings.Contains(resp.Content, "四大核心特点") {
		t.Fatalf("expected anti-repetition rewrite to replace repeated reply, got: %q (callSeq=%d)", resp.Content, callSeq)
	}
}
