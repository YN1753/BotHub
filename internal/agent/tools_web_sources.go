package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	serperSearchEndpoint = "https://google.serper.dev/search"
	braveSearchEndpoint  = "https://api.search.brave.com/res/v1/web/search"
)

type searchOptions struct {
	Query     string
	TimeRange string // "", "day", "week", "month"
	Topic     string // "general", "news", "tech", "deals"
	Lang      string // "auto", "zh", "en", "all"
	Domains   []string
	Limit     int
}

func normalizeSearchDomains(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, min(len(raw), 8))
	for _, r := range raw {
		d := strings.TrimSpace(strings.ToLower(r))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		if slash := strings.IndexByte(d, '/'); slash >= 0 {
			d = d[:slash]
		}
		d = strings.TrimPrefix(d, "www.")
		if d == "" || !strings.Contains(d, ".") || strings.ContainsAny(d, " \t\n") || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func appendSiteFilter(query string, domains []string) string {
	cleanQuery := strings.TrimSpace(query)
	if len(domains) == 0 || strings.Contains(strings.ToLower(cleanQuery), "site:") {
		return cleanQuery
	}
	if len(domains) == 1 {
		return cleanQuery + " site:" + domains[0]
	}
	parts := make([]string, 0, len(domains))
	for _, d := range domains {
		parts = append(parts, "site:"+d)
	}
	return cleanQuery + " (" + strings.Join(parts, " OR ") + ")"
}

func normalizeSearchTimeRange(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "day", "1d", "24h", "today", "pd", "qdr:d":
		return "day"
	case "week", "7d", "pw", "qdr:w":
		return "week"
	case "month", "30d", "pm", "qdr:m":
		return "month"
	default:
		return ""
	}
}

func normalizeSearchTopic(raw, query string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "general", "news", "tech", "deals":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	return detectSearchTopic(query)
}

func normalizeSearchLang(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "zh", "cn", "zh-cn":
		return "zh"
	case "en", "en-us":
		return "en"
	case "all", "both":
		return "all"
	default:
		return "auto"
	}
}

func detectSearchTopic(query string) string {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return "general"
	}
	dealsKeywords := []string{
		"羊毛", "黑五", "白嫖", "免费额度", "免费用", "限时免费", "免费试用", "优惠",
		"折扣", "促销", "神车", "立减", "福利", "coupon", "promo", "deal", "deals",
		"black friday", "freebie", "free credit", "free tier", "voucher",
	}
	for _, kw := range dealsKeywords {
		if strings.Contains(lower, kw) {
			return "deals"
		}
	}
	techKeywords := []string{
		"ai", "llm", "agent", "mcp", "claude", "opus", "openai", "gpt", "gemini",
		"deepseek", "qwen", "golang", "go ", " go", "go语言", "rust", "github",
		"开源", "论文", "arxiv", "框架", "架构", "大模型", "编程", "开发", "云原生",
		"docker", "kubernetes",
	}
	for _, kw := range techKeywords {
		if strings.Contains(lower, kw) {
			return "tech"
		}
	}
	newsKeywords := []string{
		"新闻", "资讯", "最新", "今日", "动态", "热点", "快讯", "发布", "更新",
		"日报", "早报", "breaking", "latest", "news",
	}
	for _, kw := range newsKeywords {
		if strings.Contains(lower, kw) {
			return "news"
		}
	}
	return "general"
}

func isASCIIOnlyQuery(query string) bool {
	var asciiLetters, cjkCount int
	for _, r := range query {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			asciiLetters++
		case unicode.Is(unicode.Han, r):
			cjkCount++
		}
	}
	return cjkCount == 0 && asciiLetters >= 2
}

func extractSearchTerms(query string) []string {
	stop := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"推荐": true, "最新": true, "今日": true, "每日": true, "相关": true, "内容": true,
	}
	rawTokens := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	seen := make(map[string]bool, len(rawTokens))
	terms := make([]string, 0, len(rawTokens))
	for _, tok := range rawTokens {
		tok = strings.TrimSpace(tok)
		if len([]rune(tok)) < 2 || stop[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		terms = append(terms, tok)
	}
	return terms
}

func scoreFeedRelevance(title, snippet string, terms []string, boostKeywords []string) int {
	hay := strings.ToLower(title + " " + snippet)
	score := 0
	for _, t := range terms {
		if strings.Contains(hay, t) {
			score += 10
		}
	}
	for _, b := range boostKeywords {
		if strings.Contains(hay, b) {
			score += 2
		}
	}
	return score
}

func rankFeedItems(items []webSearchItem, terms []string, boostKeywords []string, limit int) []webSearchItem {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	type rankedItem struct {
		idx   int
		score int
		item  webSearchItem
	}
	ranked := make([]rankedItem, 0, len(items))
	for i, it := range items {
		ranked = append(ranked, rankedItem{
			idx:   i,
			score: scoreFeedRelevance(it.Title, it.Snippet, terms, boostKeywords),
			item:  it,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].idx < ranked[j].idx
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]webSearchItem, len(ranked))
	for i, r := range ranked {
		out[i] = r.item
	}
	return out
}

func formatSearchDate(raw string) string {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, clean); err == nil {
			return t.In(store.ShanghaiLocation()).Format("2006-01-02 15:04")
		}
	}
	return clean
}

func (e *Engine) apiHTTPClient() *http.Client {
	if e.allowLocalhostForTest {
		return e.httpClient
	}
	return e.safeFetchClient()
}

func (e *Engine) searchSerperAPI(ctx context.Context, apiKey string, opts searchOptions) []webSearchItem {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || opts.Limit <= 0 {
		return nil
	}
	effectiveQ := appendSiteFilter(opts.Query, opts.Domains)
	bodyMap := map[string]any{
		"q":   effectiveQ,
		"num": opts.Limit,
	}
	switch opts.TimeRange {
	case "day":
		bodyMap["tbs"] = "qdr:d"
	case "week":
		bodyMap["tbs"] = "qdr:w"
	case "month":
		bodyMap["tbs"] = "qdr:m"
	}
	if !isASCIIOnlyQuery(opts.Query) && opts.Lang != "en" {
		bodyMap["hl"] = "zh-cn"
	}
	payload, _ := json.Marshal(bodyMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serperSearchEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := e.apiHTTPClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var parsed struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Date    string `json:"date"`
		} `json:"organic"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(opts.Limit, len(parsed.Organic)))
	for _, r := range parsed.Organic {
		if len(out) >= opts.Limit {
			break
		}
		title := notifier.StripEmoji(strings.TrimSpace(r.Title))
		targetURL := strings.TrimSpace(r.Link)
		if title == "" || targetURL == "" {
			continue
		}
		snippet := notifier.StripEmoji(strings.TrimSpace(r.Snippet))
		if runes := []rune(snippet); len(runes) > 260 {
			snippet = string(runes[:260]) + "..."
		}
		out = append(out, webSearchItem{
			Title:       title,
			URL:         targetURL,
			Snippet:     snippet,
			PublishedAt: strings.TrimSpace(r.Date),
			Source:      "Google(Serper)",
		})
	}
	return out
}

func (e *Engine) searchBraveAPI(ctx context.Context, apiKey string, opts searchOptions) []webSearchItem {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || opts.Limit <= 0 {
		return nil
	}
	u, err := url.Parse(braveSearchEndpoint)
	if err != nil {
		return nil
	}
	q := u.Query()
	q.Set("q", appendSiteFilter(opts.Query, opts.Domains))
	q.Set("count", strconv.Itoa(min(opts.Limit, 20)))
	switch opts.TimeRange {
	case "day":
		q.Set("freshness", "pd")
	case "week":
		q.Set("freshness", "pw")
	case "month":
		q.Set("freshness", "pm")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := e.apiHTTPClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var parsed struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				PageAge     string `json:"page_age"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(opts.Limit, len(parsed.Web.Results)))
	for _, r := range parsed.Web.Results {
		if len(out) >= opts.Limit {
			break
		}
		title := notifier.StripEmoji(strings.TrimSpace(r.Title))
		targetURL := strings.TrimSpace(r.URL)
		if title == "" || targetURL == "" {
			continue
		}
		snippet := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(r.Description)))
		if runes := []rune(snippet); len(runes) > 260 {
			snippet = string(runes[:260]) + "..."
		}
		pub := strings.TrimSpace(r.PageAge)
		if pub == "" {
			pub = strings.TrimSpace(r.Age)
		} else {
			pub = formatSearchDate(pub)
		}
		out = append(out, webSearchItem{
			Title:       title,
			URL:         targetURL,
			Snippet:     snippet,
			PublishedAt: pub,
			Source:      "Brave",
		})
	}
	return out
}

type genericRSSDoc struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			DCDate      string `xml:"http://purl.org/dc/elements/1.1/ date"`
			Description string `xml:"description"`
			Encoded     string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
		} `xml:"item"`
	} `xml:"channel"`
}

type genericAtomDoc struct {
	Title   string `xml:"title"`
	Entries []struct {
		Title     string `xml:"title"`
		Updated   string `xml:"updated"`
		Published string `xml:"published"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
		Links     []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

func parseRSSOrAtomItems(rawXML string, sourceLabel string, limit int) (string, []webSearchItem) {
	trimmed := strings.TrimSpace(rawXML)
	if trimmed == "" || limit <= 0 {
		return "", nil
	}

	var rss genericRSSDoc
	if xml.Unmarshal([]byte(trimmed), &rss) == nil && len(rss.Channel.Items) > 0 {
		out := make([]webSearchItem, 0, min(limit, len(rss.Channel.Items)))
		for _, it := range rss.Channel.Items {
			if len(out) >= limit {
				break
			}
			title := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(it.Title)))
			link := strings.TrimSpace(it.Link)
			if title == "" || link == "" {
				continue
			}
			body := strings.TrimSpace(it.Description)
			if body == "" {
				body = strings.TrimSpace(it.Encoded)
			}
			snippet := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(body)))
			if runes := []rune(snippet); len(runes) > 240 {
				snippet = string(runes[:240]) + "..."
			}
			pub := formatSearchDate(it.PubDate)
			if pub == "" {
				pub = formatSearchDate(it.DCDate)
			}
			out = append(out, webSearchItem{
				Title:       title,
				URL:         link,
				Snippet:     snippet,
				PublishedAt: pub,
				Source:      sourceLabel,
			})
		}
		return notifier.StripEmoji(strings.TrimSpace(rss.Channel.Title)), out
	}

	var atom genericAtomDoc
	if xml.Unmarshal([]byte(trimmed), &atom) == nil && len(atom.Entries) > 0 {
		out := make([]webSearchItem, 0, min(limit, len(atom.Entries)))
		for _, ent := range atom.Entries {
			if len(out) >= limit {
				break
			}
			title := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(ent.Title)))
			link := ""
			for _, lk := range ent.Links {
				href := strings.TrimSpace(lk.Href)
				if href == "" {
					continue
				}
				if lk.Rel == "" || lk.Rel == "alternate" {
					link = href
					break
				}
				if link == "" {
					link = href
				}
			}
			if title == "" || link == "" {
				continue
			}
			body := strings.TrimSpace(ent.Summary)
			if body == "" {
				body = strings.TrimSpace(ent.Content)
			}
			snippet := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(body)))
			if runes := []rune(snippet); len(runes) > 240 {
				snippet = string(runes[:240]) + "..."
			}
			pub := formatSearchDate(ent.Published)
			if pub == "" {
				pub = formatSearchDate(ent.Updated)
			}
			out = append(out, webSearchItem{
				Title:       title,
				URL:         link,
				Snippet:     snippet,
				PublishedAt: pub,
				Source:      sourceLabel,
			})
		}
		return notifier.StripEmoji(strings.TrimSpace(atom.Title)), out
	}

	return "", nil
}

func parseFeedToReadableText(rawXML string, maxItems int) (string, string, bool) {
	if maxItems <= 0 {
		maxItems = 15
	}
	title, items := parseRSSOrAtomItems(rawXML, "Feed", maxItems)
	if len(items) == 0 {
		return "", "", false
	}
	var sb strings.Builder
	for _, it := range items {
		sb.WriteString("- [")
		sb.WriteString(it.Title)
		sb.WriteString("](")
		sb.WriteString(it.URL)
		sb.WriteString(")")
		if it.PublishedAt != "" {
			sb.WriteString(" (")
			sb.WriteString(it.PublishedAt)
			sb.WriteString(")")
		}
		sb.WriteByte('\n')
		if it.Snippet != "" {
			sb.WriteString("  ")
			sb.WriteString(it.Snippet)
			sb.WriteByte('\n')
		}
	}
	return title, strings.TrimSpace(sb.String()), true
}

func parseV2EXTopicsJSON(rawJSON, sourceLabel string, terms, boostKeywords []string, limit int) []webSearchItem {
	var topics []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Replies int    `json:"replies"`
		Created int64  `json:"created"`
		Node    struct {
			Title string `json:"title"`
			Name  string `json:"name"`
		} `json:"node"`
	}
	if json.Unmarshal([]byte(rawJSON), &topics) != nil || len(topics) == 0 {
		return nil
	}
	items := make([]webSearchItem, 0, len(topics))
	for _, tp := range topics {
		title := notifier.StripEmoji(strings.TrimSpace(tp.Title))
		targetURL := strings.TrimSpace(tp.URL)
		if title == "" || targetURL == "" {
			continue
		}
		nodeLabel := strings.TrimSpace(tp.Node.Title)
		if nodeLabel == "" {
			nodeLabel = strings.TrimSpace(tp.Node.Name)
		}
		excerpt := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(tp.Content)))
		if runes := []rune(excerpt); len(runes) > 200 {
			excerpt = string(runes[:200]) + "..."
		}
		var parts []string
		if nodeLabel != "" {
			parts = append(parts, "节点: "+nodeLabel)
		}
		parts = append(parts, fmt.Sprintf("%d回复", tp.Replies))
		if excerpt != "" {
			parts = append(parts, excerpt)
		}
		pub := ""
		if tp.Created > 0 {
			pub = time.Unix(tp.Created, 0).In(store.ShanghaiLocation()).Format("2006-01-02 15:04")
		}
		items = append(items, webSearchItem{
			Title:       title,
			URL:         targetURL,
			Snippet:     strings.Join(parts, " | "),
			PublishedAt: pub,
			Source:      sourceLabel,
		})
	}
	return rankFeedItems(items, terms, boostKeywords, limit)
}

func (e *Engine) searchV2EXNodes(ctx context.Context, nodes []string, sourceLabel, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 || len(nodes) == 0 {
		return nil
	}
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		allRaw  []webSearchItem
	)
	terms := extractSearchTerms(query)
	boost := []string{"黑五", "免费", "白嫖", "优惠", "折扣", "立减", "羊毛", "ai", "claude", "opus", "aws", "google", "go", "golang"}
	client := e.safeFetchClient()

	for _, node := range nodes {
		nodeName := strings.TrimSpace(node)
		if nodeName == "" {
			continue
		}
		wg.Add(1)
		go func(nn string) {
			defer wg.Done()
			apiURL := "https://www.v2ex.com/api/topics/show.json?node_name=" + url.QueryEscape(nn)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", desktopBrowserUA)
			req.Header.Set("Accept", "application/json")
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if resp != nil {
					_ = resp.Body.Close()
				}
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
			if err != nil {
				return
			}
			parsed := parseV2EXTopicsJSON(string(body), sourceLabel, terms, boost, limit*2)
			if len(parsed) > 0 {
				mu.Lock()
				allRaw = append(allRaw, parsed...)
				mu.Unlock()
			}
		}(nodeName)
	}
	wg.Wait()
	return rankFeedItems(allRaw, terms, boost, limit)
}

func (e *Engine) searchNodeSeekRSS(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://rss.nodeseek.com/", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/rss+xml,application/xml,text/xml;q=0.9,*/*;q=0.8")
	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil
	}
	_, items := parseRSSOrAtomItems(string(body), "NodeSeek", 25)
	boost := []string{"黑五", "免费", "白嫖", "优惠", "折", "羊毛", "ai", "claude", "aws", "google", "vps", "额度", "年付"}
	return rankFeedItems(items, extractSearchTerms(query), boost, limit)
}

func (e *Engine) searchSlickdealsRSS(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	rssURL := "https://slickdeals.net/newsearch.php?mode=frontpage&searcharea=deals&searchin=first&rss=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/rss+xml,application/xml,text/xml;q=0.9,*/*;q=0.8")
	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil
	}
	_, items := parseRSSOrAtomItems(string(body), "Slickdeals", 25)
	boost := []string{"free", "ai", "cloud", "vpn", "domain", "hosting", "google", "aws", "black friday", "credit", "subscription", "software"}
	return rankFeedItems(items, extractSearchTerms(query), boost, limit)
}

func parseGitHubReposJSON(rawJSON string, limit int) []webSearchItem {
	if limit <= 0 || strings.TrimSpace(rawJSON) == "" {
		return nil
	}
	var parsed struct {
		Items []struct {
			FullName        string `json:"full_name"`
			HTMLURL         string `json:"html_url"`
			Description     string `json:"description"`
			Language        string `json:"language"`
			StargazersCount int    `json:"stargazers_count"`
			CreatedAt       string `json:"created_at"`
		} `json:"items"`
	}
	if json.Unmarshal([]byte(rawJSON), &parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(limit, len(parsed.Items)))
	for _, repo := range parsed.Items {
		if len(out) >= limit {
			break
		}
		name := notifier.StripEmoji(strings.TrimSpace(repo.FullName))
		u := strings.TrimSpace(repo.HTMLURL)
		if name == "" || u == "" {
			continue
		}
		lang := strings.TrimSpace(repo.Language)
		if lang == "" {
			lang = "Repo"
		}
		desc := notifier.StripEmoji(strings.TrimSpace(repo.Description))
		if runes := []rune(desc); len(runes) > 200 {
			desc = string(runes[:200]) + "..."
		}
		pub := strings.TrimSpace(repo.CreatedAt)
		if len(pub) >= 10 {
			pub = pub[:10]
		}
		out = append(out, webSearchItem{
			Title:       name,
			URL:         u,
			Snippet:     fmt.Sprintf("[%s | %d stars] %s", lang, repo.StargazersCount, desc),
			PublishedAt: pub,
			Source:      "GitHub",
		})
	}
	return out
}

func (e *Engine) searchGitHubRepos(ctx context.Context, opts searchOptions) []webSearchItem {
	if e.allowLocalhostForTest || opts.Limit <= 0 {
		return nil
	}
	days := 14
	switch opts.TimeRange {
	case "day", "week":
		days = 7
	case "month":
		days = 30
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	lower := strings.ToLower(opts.Query)

	parts := []string{"created:>" + cutoff}
	if strings.Contains(lower, "golang") || strings.Contains(lower, "go语言") || strings.Contains(lower, " go") || strings.HasPrefix(lower, "go ") || lower == "go" {
		parts = append(parts, "language:go")
	} else if strings.Contains(lower, "rust") {
		parts = append(parts, "language:rust")
	} else if strings.Contains(lower, "python") {
		parts = append(parts, "language:python")
	}

	var topicWords []string
	for _, tok := range extractSearchTerms(opts.Query) {
		if isASCIIOnlyQuery(tok) && tok != "go" && tok != "golang" && tok != "rust" && tok != "python" && tok != "github" {
			topicWords = append(topicWords, tok)
			if len(topicWords) >= 3 {
				break
			}
		}
	}
	if len(topicWords) > 0 {
		parts = append(parts, strings.Join(topicWords, " "))
	} else if strings.Contains(lower, "大模型") || strings.Contains(lower, "智能体") || strings.Contains(lower, "人工智能") || strings.Contains(lower, "ai") {
		parts = append(parts, "(ai OR llm OR agent OR mcp)")
	}

	ghURL := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s&sort=stars&order=desc&per_page=%d",
		url.QueryEscape(strings.Join(parts, " ")),
		min(opts.Limit, 10),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ghURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil
	}
	return parseGitHubReposJSON(string(body), opts.Limit)
}

func (e *Engine) searchLobsters(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	lower := strings.ToLower(query)
	hasGo := strings.Contains(lower, "golang") || strings.Contains(lower, "go语言") || strings.Contains(lower, " go") || strings.HasPrefix(lower, "go ") || lower == "go"
	hasAI := strings.Contains(lower, "ai") || strings.Contains(lower, "llm") || strings.Contains(lower, "agent") || strings.Contains(lower, "大模型")

	var endpoints []string
	if hasGo {
		endpoints = append(endpoints, "https://lobste.rs/t/go.json")
	}
	if hasAI {
		endpoints = append(endpoints, "https://lobste.rs/t/ai.json")
	}
	if len(endpoints) == 0 {
		endpoints = append(endpoints, "https://lobste.rs/hottest.json")
	}

	client := e.safeFetchClient()
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		all     []webSearchItem
	)
	for _, ep := range endpoints {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", desktopBrowserUA)
			req.Header.Set("Accept", "application/json")
			resp, err := client.Do(req)
			if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
				if resp != nil {
					_ = resp.Body.Close()
				}
				return
			}
			defer resp.Body.Close()
			var entries []struct {
				Title            string   `json:"title"`
				URL              string   `json:"url"`
				ShortIDURL       string   `json:"short_id_url"`
				CreatedAt        string   `json:"created_at"`
				Score            int      `json:"score"`
				Tags             []string `json:"tags"`
				DescriptionPlain string   `json:"description_plain"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&entries) != nil {
				return
			}
			mu.Lock()
			for _, ent := range entries {
				if len(all) >= limit*2 {
					break
				}
				title := notifier.StripEmoji(strings.TrimSpace(ent.Title))
				u := strings.TrimSpace(ent.URL)
				if u == "" {
					u = strings.TrimSpace(ent.ShortIDURL)
				}
				if title == "" || u == "" {
					continue
				}
				desc := notifier.StripEmoji(strings.TrimSpace(ent.DescriptionPlain))
				if runes := []rune(desc); len(runes) > 180 {
					desc = string(runes[:180]) + "..."
				}
				snip := fmt.Sprintf("[score:%d | tags:%s]", ent.Score, strings.Join(ent.Tags, ","))
				if desc != "" {
					snip += " " + desc
				}
				all = append(all, webSearchItem{
					Title:       title,
					URL:         u,
					Snippet:     snip,
					PublishedAt: formatSearchDate(ent.CreatedAt),
					Source:      "Lobsters",
				})
			}
			mu.Unlock()
		}(ep)
	}
	wg.Wait()
	return rankFeedItems(all, extractSearchTerms(query), nil, limit)
}

func (e *Engine) searchHuggingFacePapers(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	lower := strings.ToLower(query)
	relevant := false
	for _, kw := range []string{"ai", "llm", "agent", "paper", "model", "大模型", "论文", "前沿", "多模态"} {
		if strings.Contains(lower, kw) {
			relevant = true
			break
		}
	}
	if !relevant {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://huggingface.co/api/daily_papers?limit=12", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/json")
	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()
	var rows []struct {
		PublishedAt string `json:"publishedAt"`
		Paper       struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Upvotes int    `json:"upvotes"`
		} `json:"paper"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&rows) != nil {
		return nil
	}
	items := make([]webSearchItem, 0, len(rows))
	for _, r := range rows {
		title := notifier.StripEmoji(strings.TrimSpace(r.Paper.Title))
		id := strings.TrimSpace(r.Paper.ID)
		if title == "" || id == "" {
			continue
		}
		summary := notifier.StripEmoji(strings.TrimSpace(r.Paper.Summary))
		if runes := []rune(summary); len(runes) > 220 {
			summary = string(runes[:220]) + "..."
		}
		items = append(items, webSearchItem{
			Title:       title,
			URL:         "https://huggingface.co/papers/" + id,
			Snippet:     fmt.Sprintf("[%d upvotes] %s", r.Paper.Upvotes, summary),
			PublishedAt: formatSearchDate(r.PublishedAt),
			Source:      "HF-Papers",
		})
	}
	return rankFeedItems(items, extractSearchTerms(query), nil, limit)
}

func mergeSearchBuckets(priorityBuckets [][]webSearchItem, liveBuckets [][]webSearchItem, fallbackBuckets [][]webSearchItem, limit int) ([]webSearchItem, []string) {
	if limit <= 0 {
		return nil, nil
	}
	results := make([]webSearchItem, 0, limit)
	seenCanonical := make(map[string]bool)
	providersSeen := make(map[string]bool)
	providersUsed := make([]string, 0, 8)

	tryAppend := func(item webSearchItem) bool {
		if len(results) >= limit {
			return false
		}
		u := strings.TrimSpace(item.URL)
		title := strings.TrimSpace(item.Title)
		if u == "" || title == "" {
			return false
		}
		canon := canonicalSearchURL(u)
		if seenCanonical[canon] {
			return false
		}
		seenCanonical[canon] = true
		if item.Source != "" && !providersSeen[item.Source] {
			providersSeen[item.Source] = true
			providersUsed = append(providersUsed, item.Source)
		}
		results = append(results, item)
		return true
	}

	roundRobinMerge := func(buckets [][]webSearchItem, maxPerBucket int) {
		if len(buckets) == 0 || len(results) >= limit {
			return
		}
		taken := make([]int, len(buckets))
		cursors := make([]int, len(buckets))
		for len(results) < limit {
			progressed := false
			for bIdx, b := range buckets {
				if len(results) >= limit {
					break
				}
				if maxPerBucket > 0 && taken[bIdx] >= maxPerBucket {
					continue
				}
				for cursors[bIdx] < len(b) {
					item := b[cursors[bIdx]]
					cursors[bIdx]++
					if tryAppend(item) {
						taken[bIdx]++
						progressed = true
						break
					}
				}
			}
			if !progressed {
				break
			}
		}
	}

	// 1. Premium / Admin-configured search APIs interleaved fairly.
	roundRobinMerge(priorityBuckets, 0)
	// 2. Built-in live web + vertical community/tech/deals sources:
	// Pass A caps each source at 2 items so a noisy HTML scrape never crowds out fresher feeds.
	roundRobinMerge(liveBuckets, 2)
	// Pass B drains any remaining items round-robin if slots remain.
	roundRobinMerge(liveBuckets, 0)
	// 3. Static encyclopedic fallback buckets only when live slots are still unfilled.
	roundRobinMerge(fallbackBuckets, 0)

	return results, providersUsed
}
