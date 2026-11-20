package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

const (
	MaxReadURLChars  = 10000
	desktopBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	mobileBrowserUA  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Mobile/15E148 Safari/604.1"
)

var (
	reHTMLComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reNoiseBlocks  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|iframe|nav|header|footer|aside|form|button)[^>]*>.*?</(script|style|noscript|svg|iframe|nav|header|footer|aside|form|button)>`)
	reHTMLTags     = regexp.MustCompile(`(?s)<[^>]+>`)
	reMultiSpace   = regexp.MustCompile(`[ \t\r\f]+`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
	reMetaDesc1    = regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["'](?:description|og:description|twitter:description)["'][^>]+content=["']([^"']{1,800})["']`)
	reMetaDesc2    = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']{1,800})["'][^>]+(?:name|property)=["'](?:description|og:description|twitter:description)["']`)
	reMetaOGTitle1 = regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["'](?:og:title|twitter:title)["'][^>]+content=["']([^"']{1,400})["']`)
	reMetaOGTitle2 = regexp.MustCompile(`(?is)<meta[^>]+content=["']([^"']{1,400})["'][^>]+(?:name|property)=["'](?:og:title|twitter:title)["']`)
	reMetaKeywords = regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["']keywords["'][^>]+content=["']([^"']{1,400})["']`)
	reMetaAuthor   = regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)=["']author["'][^>]+content=["']([^"']{1,120})["']`)
	reHTMLTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reDDGResult    = regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reDDGSnippet   = regexp.MustCompile(`(?is)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)
	reBingAlgoLi   = regexp.MustCompile(`(?is)<li[^>]+class="[^"]*\bb_algo\b[^"]*"[^>]*>(.*?)</li>`)
	reBingTitleA   = regexp.MustCompile(`(?is)<h2[^>]*>\s*<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reBingSnippetP = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)

	tavilySearchEndpoint = "https://api.tavily.com/search"

	htmlBlockBreakReplacer = strings.NewReplacer(
		"</p>", "\n",
		"</div>", "\n",
		"</li>", "\n",
		"</h1>", "\n",
		"</h2>", "\n",
		"</h3>", "\n",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
	)

	sharedSafeHTTPClient = newSafeHTTPClient()
)

func readURLToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_url",
				Description: "抓取指定网页 URL 的正文内容并转换为纯文本摘要，用于阅读文章、文档或监控页面。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url": map[string]any{
							"type":        "string",
							"description": "要抓取的完整 HTTP/HTTPS 网页地址",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		Timeout:        25 * time.Second,
		ReadOnly:       true,
		StatusLabel:    "正在精读网页正文...",
		DomainKeywords: []string{"网页", "url", "链接", "http", "文章"},
		ErrorHint:      "该目标地址不可访问或受策略拦截，请勿编造正文，如实向用户说明或更换公开来源",
		RetryErrorHint: "目标站点访问超时，可更换关键词或改用另一参考源",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolReadURL(ctx, cleanArgs)
		},
	}
}

func webSearchToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "web_search",
				Description: "通过互联网搜索关键词获取最新资讯、技术方案或公开信息。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回结果条数（默认 5，最大 10）",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		Timeout:        25 * time.Second,
		ReadOnly:       true,
		StatusLabel:    "正在检索全网资料...",
		DomainKeywords: []string{"github", "开源", "趋势", "雷达", "新闻", "前沿", "hn", "hacker", "搜索", "推荐", "查一下"},
		ErrorHint:      "该目标地址不可访问或受策略拦截，请勿编造正文，如实向用户说明或更换公开来源",
		RetryErrorHint: "目标站点访问超时，可更换关键词或改用另一参考源",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolWebSearch(ctx, cleanArgs)
		},
	}
}

func extractHTMLOGTitle(rawHTML string) string {
	for _, rx := range []*regexp.Regexp{reMetaOGTitle1, reMetaOGTitle2} {
		if m := rx.FindStringSubmatch(rawHTML); len(m) > 1 {
			clean := strings.TrimSpace(html.UnescapeString(m[1]))
			if clean != "" {
				return notifier.StripEmoji(clean)
			}
		}
	}
	return ""
}

func extractHTMLKeywords(rawHTML string) string {
	if m := reMetaKeywords.FindStringSubmatch(rawHTML); len(m) > 1 {
		return notifier.StripEmoji(strings.TrimSpace(html.UnescapeString(m[1])))
	}
	return ""
}

func extractHTMLAuthor(rawHTML string) string {
	if m := reMetaAuthor.FindStringSubmatch(rawHTML); len(m) > 1 {
		return notifier.StripEmoji(strings.TrimSpace(html.UnescapeString(m[1])))
	}
	return ""
}

func extractHTMLTitle(rawHTML string) string {
	if og := extractHTMLOGTitle(rawHTML); og != "" {
		return og
	}
	m := reHTMLTitle.FindStringSubmatch(rawHTML)
	if len(m) < 2 {
		return ""
	}
	return notifier.StripEmoji(strings.TrimSpace(reMultiSpace.ReplaceAllString(html.UnescapeString(reHTMLTags.ReplaceAllString(m[1], " ")), " ")))
}

func extractHTMLMetaDesc(rawHTML string) string {
	for _, rx := range []*regexp.Regexp{reMetaDesc1, reMetaDesc2} {
		if m := rx.FindStringSubmatch(rawHTML); len(m) > 1 {
			clean := strings.TrimSpace(html.UnescapeString(m[1]))
			if clean != "" {
				return notifier.StripEmoji(clean)
			}
		}
	}
	return ""
}

func stripHTMLToText(rawHTML string) string {
	cleaned := reHTMLComment.ReplaceAllString(rawHTML, " ")
	cleaned = reNoiseBlocks.ReplaceAllString(cleaned, " ")
	cleaned = htmlBlockBreakReplacer.Replace(cleaned)
	cleaned = reHTMLTags.ReplaceAllString(cleaned, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = reMultiSpace.ReplaceAllString(cleaned, " ")
	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	res := strings.Join(out, "\n")
	res = reMultiNewline.ReplaceAllString(res, "\n\n")
	return notifier.StripEmoji(strings.TrimSpace(res))
}

func isPrivateOrLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// Block RFC 6598 Carrier-Grade NAT / Tailscale mesh (100.64.0.0/10)
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		// Block RFC 2544 benchmark testing range (198.18.0.0/15)
		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return true
		}
		// Block "this network" 0.0.0.0/8
		if ip4[0] == 0 {
			return true
		}
	}
	return false
}

func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 15 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("无效目标地址: %w", err)
			}
			ip := net.ParseIP(host)
			if isPrivateOrLoopbackIP(ip) {
				return fmt.Errorf("安全拦截：禁止访问内网或本机回环地址 (%s)", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   6,
			IdleConnTimeout:       45 * time.Second,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("安全拦截：不支持的重定向协议 %s", req.URL.Scheme)
			}
			host := strings.ToLower(req.URL.Hostname())
			if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".ts.net") {
				return fmt.Errorf("安全拦截：禁止重定向到内网主机 %s", host)
			}
			if len(via) > 0 && !strings.EqualFold(via[len(via)-1].URL.Host, req.URL.Host) {
				req.Header.Del("Authorization")
				req.Header.Del("Cookie")
			}
			return nil
		},
	}
}

func (e *Engine) safeFetchClient() *http.Client {
	if e.allowLocalhostForTest {
		return e.httpClient
	}
	return sharedSafeHTTPClient
}

// fetchWebResource performs SSRF-protected GET fetching shared across read_url and Smart Stash extractors.
func (e *Engine) fetchWebResource(ctx context.Context, targetURL, accept string, headers map[string]string) (int, string, string, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, "", "", "", err
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if accept == "" {
		accept = "text/html,application/xhtml+xml,application/json,text/plain;q=0.9,*/*;q=0.8"
	}
	req.Header.Set("Accept", accept)
	for k, v := range headers {
		if strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := e.safeFetchClient().Do(req)
	if err != nil {
		return 0, "", "", "", err
	}
	defer resp.Body.Close()

	finalURL := targetURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1536*1024))
	if err != nil {
		return resp.StatusCode, finalURL, "", "", err
	}
	return resp.StatusCode, finalURL, strings.ToLower(resp.Header.Get("Content-Type")), string(bodyBytes), nil
}

func parseGitHubRepo(parsed *url.URL) (owner, repo string, ok bool) {
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}

func (e *Engine) toolReadURL(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 read_url 参数失败: %w", err)
	}
	target := strings.TrimSpace(args.URL)
	if target == "" {
		return "", fmt.Errorf("url 不能为空")
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("仅支持 http 或 https 协议的 URL")
	}

	maxChars := args.MaxChars
	if maxChars <= 0 || maxChars > MaxReadURLChars {
		maxChars = MaxReadURLChars
	}

	var statusCode int
	var text string
	var pageTitle string
	var pageDesc string
	sourceMode := "direct"

	// Fast-path: GitHub repository root URL -> fetch raw README.md directly
	if owner, repo, ok := parseGitHubRepo(parsed); ok && !e.allowLocalhostForTest {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/README.md", owner, repo)
		if sc, _, _, body, gErr := e.fetchWebResource(ctx, rawURL, "text/plain", nil); gErr == nil && sc >= 200 && sc < 300 && strings.TrimSpace(body) != "" {
			statusCode = sc
			pageTitle = fmt.Sprintf("%s/%s", owner, repo)
			text = notifier.StripEmoji(strings.TrimSpace(body))
			sourceMode = "github_raw_readme"
		}
	}

	if text == "" {
		sc, _, contentType, rawBody, fErr := e.fetchWebResource(ctx, target, "", nil)
		if fErr != nil {
			return "", fmt.Errorf("请求网页失败: %w", fErr)
		}
		statusCode = sc
		if strings.Contains(contentType, "json") || strings.Contains(contentType, "plain") {
			text = notifier.StripEmoji(strings.TrimSpace(rawBody))
		} else {
			pageTitle = extractHTMLTitle(rawBody)
			pageDesc = extractHTMLMetaDesc(rawBody)
			text = stripHTMLToText(rawBody)
		}

		// Fallback: if CSR/SPA page yields very little text, try Jina Reader Markdown extraction
		if len([]rune(text)) < 200 && !e.allowLocalhostForTest && !strings.Contains(strings.ToLower(parsed.Hostname()), "r.jina.ai") {
			jinaURL := "https://r.jina.ai/" + target
			if jsc, _, _, jBody, jErr := e.fetchWebResource(ctx, jinaURL, "text/plain", nil); jErr == nil && jsc >= 200 && jsc < 300 {
				jClean := notifier.StripEmoji(strings.TrimSpace(jBody))
				if len([]rune(jClean)) > len([]rune(text)) {
					text = jClean
					statusCode = jsc
					sourceMode = "jina_reader"
				}
			}
		}
	}

	runes := []rune(text)
	truncated := false
	if len(runes) > maxChars {
		text = string(runes[:maxChars])
		truncated = true
	}

	resultMap := map[string]any{
		"url":         target,
		"status_code": statusCode,
		"source_mode": sourceMode,
		"char_count":  len([]rune(text)),
		"truncated":   truncated,
		"content":     text,
	}
	if pageTitle != "" {
		resultMap["title"] = pageTitle
	}
	if pageDesc != "" {
		resultMap["description"] = pageDesc
	}

	// If any lightweight/fallback model is available and the article is long (> 3000 chars),
	// pre-distill a summary via CallLightModel to save tokens on the primary reasoning model.
	if e.hasLightTarget() && len([]rune(text)) > 3000 {
		if lightSum, lErr := e.CallLightModel(ctx, "你是正文提炼助手。请将网页正文提炼为高信息密度的核心要点摘要，严禁使用 Emoji。", text); lErr == nil && lightSum != "" {
			resultMap["light_summary"] = lightSum
		}
	}

	payload, _ := json.Marshal(resultMap)
	return string(payload), nil
}

func unwrapDDGRedirectURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if parsed, err := url.Parse(raw); err == nil {
		if uddg := parsed.Query().Get("uddg"); uddg != "" {
			return uddg
		}
	}
	return raw
}

type webSearchItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Excerpt string `json:"excerpt,omitempty"`
	Source  string `json:"source"`
}

func unwrapBingRedirectURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, "bing.com") && strings.HasPrefix(parsed.Path, "/ck/a") {
		if encoded := strings.TrimSpace(parsed.Query().Get("u")); encoded != "" {
			payload := strings.TrimPrefix(encoded, "a1")
			if decoded, dErr := base64.RawURLEncoding.DecodeString(payload); dErr == nil {
				s := strings.TrimSpace(string(decoded))
				if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
					return s
				}
			}
			if decoded, dErr := base64.URLEncoding.DecodeString(payload); dErr == nil {
				s := strings.TrimSpace(string(decoded))
				if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
					return s
				}
			}
		}
	}
	return raw
}

func parseBingHTML(rawHTML string, limit int) []webSearchItem {
	if limit <= 0 || strings.TrimSpace(rawHTML) == "" {
		return nil
	}
	blocks := reBingAlgoLi.FindAllStringSubmatch(rawHTML, limit*2)
	out := make([]webSearchItem, 0, min(limit, len(blocks)))
	for _, blk := range blocks {
		if len(out) >= limit {
			break
		}
		if len(blk) < 2 {
			continue
		}
		m := reBingTitleA.FindStringSubmatch(blk[1])
		if len(m) < 3 {
			continue
		}
		targetURL := unwrapBingRedirectURL(m[1])
		if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
			continue
		}
		title := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(m[2])))
		if title == "" {
			continue
		}
		snippet := ""
		if pm := reBingSnippetP.FindStringSubmatch(blk[1]); len(pm) >= 2 {
			snippet = notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(pm[1])))
		}
		if len([]rune(snippet)) > 260 {
			snippet = string([]rune(snippet)[:260]) + "..."
		}
		out = append(out, webSearchItem{
			Title:   title,
			URL:     targetURL,
			Snippet: snippet,
			Source:  "Bing",
		})
	}
	return out
}

func (e *Engine) searchBingHTML(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	bingURL := fmt.Sprintf("https://www.bing.com/search?q=%s&setlang=zh-Hans&cc=CN&count=%d", url.QueryEscape(query), limit+2)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bingURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil
	}
	return parseBingHTML(string(bodyBytes), limit)
}

type googleNewsRSSDoc struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			Description string `xml:"description"`
			Source      string `xml:"source"`
		} `xml:"item"`
	} `xml:"channel"`
}

func parseGoogleNewsRSS(rawXML string, limit int) []webSearchItem {
	if limit <= 0 || strings.TrimSpace(rawXML) == "" {
		return nil
	}
	var doc googleNewsRSSDoc
	if err := xml.Unmarshal([]byte(rawXML), &doc); err != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(limit, len(doc.Channel.Items)))
	for _, it := range doc.Channel.Items {
		if len(out) >= limit {
			break
		}
		title := notifier.StripEmoji(strings.TrimSpace(it.Title))
		link := strings.TrimSpace(it.Link)
		if title == "" || link == "" {
			continue
		}
		var parts []string
		if src := notifier.StripEmoji(strings.TrimSpace(it.Source)); src != "" {
			parts = append(parts, "来源: "+src)
		}
		if pub := strings.TrimSpace(it.PubDate); pub != "" {
			if parsedTime, err := time.Parse(time.RFC1123, pub); err == nil {
				parts = append(parts, parsedTime.In(store.ShanghaiLocation()).Format("2006-01-02 15:04"))
			} else if parsedTime, err := time.Parse(time.RFC1123Z, pub); err == nil {
				parts = append(parts, parsedTime.In(store.ShanghaiLocation()).Format("2006-01-02 15:04"))
			} else {
				parts = append(parts, pub)
			}
		}
		desc := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(it.Description)))
		if desc != "" && !strings.EqualFold(desc, title) {
			if len([]rune(desc)) > 200 {
				desc = string([]rune(desc)[:200]) + "..."
			}
			parts = append(parts, desc)
		}
		out = append(out, webSearchItem{
			Title:   title,
			URL:     link,
			Snippet: strings.Join(parts, " | "),
			Source:  "GoogleNews",
		})
	}
	return out
}

func (e *Engine) searchGoogleNewsRSS(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	rssURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=zh-CN&gl=CN&ceid=CN:zh-Hans", url.QueryEscape(query))
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

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil
	}
	return parseGoogleNewsRSS(string(bodyBytes), limit)
}

func (e *Engine) searchTavilyAPI(ctx context.Context, apiKey, query string, limit int) []webSearchItem {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || limit <= 0 {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{
		"api_key":        apiKey,
		"query":          query,
		"search_depth":   "basic",
		"max_results":    limit,
		"include_answer": false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilySearchEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := e.safeFetchClient()
	if e.allowLocalhostForTest {
		client = e.httpClient
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(limit, len(parsed.Results)))
	for _, r := range parsed.Results {
		if len(out) >= limit {
			break
		}
		title := notifier.StripEmoji(strings.TrimSpace(r.Title))
		targetURL := strings.TrimSpace(r.URL)
		if title == "" || targetURL == "" {
			continue
		}
		content := notifier.StripEmoji(strings.TrimSpace(r.Content))
		snippet := content
		excerpt := ""
		if runes := []rune(content); len(runes) > 260 {
			snippet = string(runes[:260]) + "..."
			if len(runes) > 900 {
				excerpt = string(runes[:900]) + "..."
			} else {
				excerpt = content
			}
		}
		out = append(out, webSearchItem{
			Title:   title,
			URL:     targetURL,
			Snippet: snippet,
			Excerpt: excerpt,
			Source:  "Tavily",
		})
	}
	return out
}

func (e *Engine) searchSearXNG(ctx context.Context, baseURL, query string, limit int) []webSearchItem {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || limit <= 0 {
		return nil
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") {
		return nil
	}
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&language=zh-CN", baseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/json")

	// searxng_url is an admin-configured endpoint, so allow local/Docker self-hosted instances via e.httpClient.
	resp, err := e.httpClient.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(limit, len(parsed.Results)))
	for _, r := range parsed.Results {
		if len(out) >= limit {
			break
		}
		title := notifier.StripEmoji(strings.TrimSpace(r.Title))
		targetURL := strings.TrimSpace(r.URL)
		if title == "" || targetURL == "" {
			continue
		}
		snippet := notifier.StripEmoji(strings.TrimSpace(r.Content))
		if len([]rune(snippet)) > 280 {
			snippet = string([]rune(snippet)[:280]) + "..."
		}
		out = append(out, webSearchItem{
			Title:   title,
			URL:     targetURL,
			Snippet: snippet,
			Source:  "SearXNG",
		})
	}
	return out
}

func isLikelyTechOrASCIIQuery(query string) bool {
	var asciiLetters, cjkCount int
	for _, r := range query {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			asciiLetters++
		case unicode.Is(unicode.Han, r):
			cjkCount++
		}
	}
	if cjkCount == 0 && asciiLetters >= 3 {
		return true
	}
	lower := strings.ToLower(query)
	for _, kw := range []string{"github", "golang", "rust", "python", "docker", "k8s", "kubernetes", "llm", "openai", "claude", "gemini", "api", "sdk", "开源"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (e *Engine) searchDuckDuckGoHTML(ctx context.Context, query string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	ddgURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ddgURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.safeFetchClient().Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil
	}
	raw := string(bodyBytes)
	titleMatches := reDDGResult.FindAllStringSubmatch(raw, limit)
	snippetMatches := reDDGSnippet.FindAllStringSubmatch(raw, limit)

	out := make([]webSearchItem, 0, len(titleMatches))
	for i, m := range titleMatches {
		targetURL := unwrapDDGRedirectURL(m[1])
		title := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(m[2])))
		if targetURL == "" || title == "" {
			continue
		}
		snippet := ""
		if i < len(snippetMatches) {
			snippet = notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(snippetMatches[i][1])))
		}
		out = append(out, webSearchItem{
			Title:   title,
			URL:     targetURL,
			Snippet: snippet,
			Source:  "DuckDuckGo",
		})
	}
	return out
}

func shouldAutoDeepReadResult(item webSearchItem) bool {
	if len([]rune(strings.TrimSpace(item.Excerpt))) >= 240 {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(item.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// Google News RSS article links are JS-redirect stubs; skip direct HTML fetch.
	if host == "news.google.com" {
		return false
	}
	return true
}

func (e *Engine) enrichTopSearchResults(ctx context.Context, results []webSearchItem, maxReads int) {
	if e.allowLocalhostForTest || len(results) == 0 || maxReads <= 0 {
		return
	}
	var candidateIdxs []int
	for i, item := range results {
		if shouldAutoDeepReadResult(item) {
			candidateIdxs = append(candidateIdxs, i)
			if len(candidateIdxs) >= maxReads {
				break
			}
		}
	}
	if len(candidateIdxs) == 0 {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, idx := range candidateIdxs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src, err := e.extractGenericWebSource(readCtx, results[i].URL)
			if err != nil || src == nil {
				return
			}
			body := notifier.StripEmoji(strings.TrimSpace(src.Excerpt))
			if len([]rune(body)) < 80 {
				return
			}
			if runes := []rune(body); len(runes) > 900 {
				body = string(runes[:900]) + "..."
			}
			results[i].Excerpt = body
		}(idx)
	}
	wg.Wait()
}

func canonicalSearchURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimRight(strings.TrimSpace(raw), "/")
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	p := strings.TrimRight(u.Path, "/")
	if u.RawQuery != "" {
		return host + p + "?" + u.RawQuery
	}
	return host + p
}

func (e *Engine) toolWebSearch(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 web_search 参数失败: %w", err)
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	limit := args.Limit
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	var tavilyKey, searxngURL string
	if e.store != nil {
		tavilyKey, _ = e.store.GetConfig(store.KeyTavilyAPIKey)
		searxngURL, _ = e.store.GetConfig(store.KeySearXNGURL)
	}
	tavilyKey = strings.TrimSpace(tavilyKey)
	searxngURL = strings.TrimSpace(searxngURL)

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Priority buckets:
	// 0: Tavily API (if configured)
	// 1: SearXNG (if configured)
	// 2: Bing Web Search HTML
	// 3: DuckDuckGo HTML (with desktop browser UA)
	// 4: Google News RSS (fresh news/events)
	// 5: Hacker News Algolia (boosted before news for English/tech queries)
	// 6: DuckDuckGo Instant Answer JSON
	// 7: Chinese Wikipedia OpenSearch (demoted fallback)
	buckets := make([][]webSearchItem, 8)
	var wg sync.WaitGroup

	if tavilyKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			buckets[0] = e.searchTavilyAPI(pCtx, tavilyKey, query, limit)
		}()
	}

	if searxngURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			buckets[1] = e.searchSearXNG(pCtx, searxngURL, query, limit)
		}()
	}

	if !e.allowLocalhostForTest {
		client := e.safeFetchClient()

		// Provider 2: Bing Web Search HTML
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			buckets[2] = e.searchBingHTML(pCtx, query, limit)
		}()

		// Provider 3: DuckDuckGo HTML
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			buckets[3] = e.searchDuckDuckGoHTML(pCtx, query, limit)
		}()

		// Provider 4: Google News RSS
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
			defer pCancel()
			buckets[4] = e.searchGoogleNewsRSS(pCtx, query, limit)
		}()

		// Provider 5: Hacker News Algolia Search
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
			defer pCancel()
			hnURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&hitsPerPage=%d", url.QueryEscape(query), limit)
			req, err := http.NewRequestWithContext(pCtx, http.MethodGet, hnURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", desktopBrowserUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			var hnResp struct {
				Hits []struct {
					Title     string `json:"title"`
					URL       string `json:"url"`
					StoryText string `json:"story_text"`
					ObjectID  string `json:"objectID"`
				} `json:"hits"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&hnResp) != nil {
				return
			}
			for _, h := range hnResp.Hits {
				if h.Title == "" {
					continue
				}
				itemURL := h.URL
				if itemURL == "" {
					itemURL = "https://news.ycombinator.com/item?id=" + h.ObjectID
				}
				snippet := stripHTMLToText(h.StoryText)
				if len([]rune(snippet)) > 200 {
					snippet = string([]rune(snippet)[:200]) + "..."
				}
				buckets[5] = append(buckets[5], webSearchItem{
					Title:   notifier.StripEmoji(h.Title),
					URL:     itemURL,
					Snippet: snippet,
					Source:  "HackerNews",
				})
			}
		}()

		// Provider 6: DuckDuckGo Instant Answer JSON API
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
			defer pCancel()
			ddgAPI := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))
			req, err := http.NewRequestWithContext(pCtx, http.MethodGet, ddgAPI, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", desktopBrowserUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			var ddg struct {
				Heading       string `json:"Heading"`
				AbstractText  string `json:"AbstractText"`
				AbstractURL   string `json:"AbstractURL"`
				RelatedTopics []struct {
					Text     string `json:"Text"`
					FirstURL string `json:"FirstURL"`
					Topics   []struct {
						Text     string `json:"Text"`
						FirstURL string `json:"FirstURL"`
					} `json:"Topics"`
				} `json:"RelatedTopics"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&ddg) != nil {
				return
			}
			if ddg.AbstractText != "" && ddg.AbstractURL != "" {
				buckets[6] = append(buckets[6], webSearchItem{
					Title:   notifier.StripEmoji(ddg.Heading),
					URL:     ddg.AbstractURL,
					Snippet: notifier.StripEmoji(ddg.AbstractText),
					Source:  "DuckDuckGo",
				})
			}
			for _, rt := range ddg.RelatedTopics {
				if len(buckets[6]) >= limit {
					break
				}
				if rt.Text != "" && rt.FirstURL != "" {
					title := rt.Text
					if idx := strings.Index(title, " - "); idx > 0 {
						title = title[:idx]
					}
					buckets[6] = append(buckets[6], webSearchItem{
						Title:   notifier.StripEmoji(truncateTitle(title)),
						URL:     rt.FirstURL,
						Snippet: notifier.StripEmoji(rt.Text),
						Source:  "DuckDuckGo",
					})
				}
				for _, sub := range rt.Topics {
					if len(buckets[6]) >= limit {
						break
					}
					if sub.Text != "" && sub.FirstURL != "" {
						buckets[6] = append(buckets[6], webSearchItem{
							Title:   notifier.StripEmoji(truncateTitle(sub.Text)),
							URL:     sub.FirstURL,
							Snippet: notifier.StripEmoji(sub.Text),
							Source:  "DuckDuckGo",
						})
					}
				}
			}
		}()

		// Provider 7: Chinese Wikipedia OpenSearch (demoted fallback)
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
			defer pCancel()
			wikiURL := fmt.Sprintf("https://zh.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&utf8=1&format=json&srlimit=%d", url.QueryEscape(query), limit)
			req, err := http.NewRequestWithContext(pCtx, http.MethodGet, wikiURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", desktopBrowserUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			var wikiResp struct {
				Query struct {
					Search []struct {
						Title   string `json:"title"`
						Snippet string `json:"snippet"`
					} `json:"search"`
				} `json:"query"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&wikiResp) != nil {
				return
			}
			for _, s := range wikiResp.Query.Search {
				buckets[7] = append(buckets[7], webSearchItem{
					Title:   notifier.StripEmoji(s.Title),
					URL:     "https://zh.wikipedia.org/wiki/" + url.PathEscape(s.Title),
					Snippet: stripHTMLToText(s.Snippet),
					Source:  "Wikipedia",
				})
			}
		}()
	}

	wg.Wait()

	order := []int{0, 1, 2, 3, 4, 5, 6, 7}
	if isLikelyTechOrASCIIQuery(query) {
		order = []int{0, 1, 2, 3, 5, 4, 6, 7}
	}

	results := make([]webSearchItem, 0, limit)
	seenCanonical := make(map[string]bool)
	providersSeen := make(map[string]bool)
	providersUsed := make([]string, 0, 4)

	for _, bIdx := range order {
		for _, item := range buckets[bIdx] {
			u := strings.TrimSpace(item.URL)
			canon := canonicalSearchURL(u)
			if u == "" || strings.TrimSpace(item.Title) == "" || seenCanonical[canon] {
				continue
			}
			seenCanonical[canon] = true
			if item.Source != "" && !providersSeen[item.Source] {
				providersSeen[item.Source] = true
				providersUsed = append(providersUsed, item.Source)
			}
			results = append(results, item)
			if len(results) >= limit {
				break
			}
		}
		if len(results) >= limit {
			break
		}
	}

	// Automatically deep-read top 2 web hits concurrently so weaker models receive real page content in one call.
	e.enrichTopSearchResults(ctx, results, 2)

	payload, _ := json.Marshal(map[string]any{
		"query":          query,
		"count":          len(results),
		"providers_used": providersUsed,
		"results":        results,
	})
	return string(payload), nil
}
