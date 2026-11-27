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
	"strconv"
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
	reBingAlgoLi   = regexp.MustCompile(`(?is)<(?:li|div)[^>]+class="[^"]*\bb_algo\b[^"]*"[^>]*>(.*?)</(?:li|div)>`)
	reBingTitleA   = regexp.MustCompile(`(?is)<h2[^>]*>\s*<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reBingSnippetP = regexp.MustCompile(`(?is)<(?:p|div)[^>]+class="[^"]*(?:b_lineclamp|b_algoSlug|b_caption)[^"]*"[^>]*>(.*?)</(?:p|div)>|<p[^>]*>(.*?)</p>`)

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
				Description: "通过多源搜索引擎与实时社区/开源榜单检索公开信息；支持按时间范围(time_range)、赛道(topic)和双语(lang)检索。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "精准搜索关键词，建议结合中英文分别调用",
						},
						"time_range": map[string]any{
							"type":        "string",
							"enum":        []string{"day", "week", "month"},
							"description": "可选时效过滤：day(24小时内)、week(近7天)、month(近30天)；每日推荐与羊毛任务务必传 day 或 week",
						},
						"topic": map[string]any{
							"type":        "string",
							"enum":        []string{"general", "news", "tech", "deals"},
							"description": "可选垂直赛道：tech(AI/Go/GitHub/HN/Lobsters/论文)、deals(黑五/免费额度/羊毛/V2EX/NodeSeek)、news(突发新闻)、general",
						},
						"lang": map[string]any{
							"type":        "string",
							"enum":        []string{"auto", "zh", "en", "all"},
							"description": "可选语区偏好：all(中英双源并发)、en(海外一手源)、zh(中文源)、auto",
						},
						"domains": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "可选限定检索域名列表，例如 [\"x.com\", \"linux.do\", \"v2ex.com\", \"nodeseek.com\"]，配合 Tavily/Serper 抓取推特与一线社区一手帖",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回结果条数（默认 8，最大 15）",
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

	// Fast-path 1: GitHub repository root URL -> fetch raw README.md directly
	if owner, repo, ok := parseGitHubRepo(parsed); ok && !e.allowLocalhostForTest {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/README.md", owner, repo)
		if sc, _, _, body, gErr := e.fetchWebResource(ctx, rawURL, "text/plain", nil); gErr == nil && sc >= 200 && sc < 300 && strings.TrimSpace(body) != "" {
			statusCode = sc
			pageTitle = fmt.Sprintf("%s/%s", owner, repo)
			text = notifier.StripEmoji(strings.TrimSpace(body))
			sourceMode = "github_raw_readme"
		}
	}

	// Fast-path 2: Telegram public channel URL (t.me/<channel> or t.me/s/<channel>)
	if text == "" && !e.allowLocalhostForTest {
		if tgCh, ok := extractTGChannelFromURL(target); ok {
			if msgs, tErr := e.fetchTelegramChannel(ctx, tgCh, 168, 15); tErr == nil {
				if len(msgs) == 0 {
					msgs, _ = e.fetchTelegramChannel(ctx, tgCh, 0, 10)
				}
				if len(msgs) > 0 {
					statusCode = 200
					pageTitle = "Telegram @" + tgCh
					text = formatTelegramMessagesForPrompt(msgs, []string{tgCh}, 15)
					sourceMode = "telegram_channel"
				}
			}
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
		} else if strings.Contains(rawBody, "tgme_widget_message_wrap") {
			msgs := parseTelegramChannelHTML(rawBody, "", time.Now(), 0, 15)
			if len(msgs) > 0 {
				pageTitle = extractHTMLTitle(rawBody)
				text = formatTelegramMessagesForPrompt(msgs, nil, 15)
				sourceMode = "telegram_channel"
			} else {
				pageTitle = extractHTMLTitle(rawBody)
				pageDesc = extractHTMLMetaDesc(rawBody)
				text = stripHTMLToText(rawBody)
			}
		} else if strings.Contains(contentType, "xml") || strings.Contains(contentType, "rss") || strings.Contains(contentType, "atom") ||
			strings.HasPrefix(strings.TrimSpace(rawBody), "<?xml") || strings.HasPrefix(strings.TrimSpace(rawBody), "<rss") || strings.HasPrefix(strings.TrimSpace(rawBody), "<feed") {
			if feedTitle, feedBody, ok := parseFeedToReadableText(rawBody, 15); ok {
				if feedTitle != "" {
					pageTitle = feedTitle
				}
				text = feedBody
				sourceMode = "feed"
			} else {
				pageTitle = extractHTMLTitle(rawBody)
				pageDesc = extractHTMLMetaDesc(rawBody)
				text = stripHTMLToText(rawBody)
			}
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
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Source      string `json:"source"`
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
			rawSnip := pm[1]
			if rawSnip == "" && len(pm) >= 3 {
				rawSnip = pm[2]
			}
			snippet = notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(rawSnip)))
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

func (e *Engine) searchBingHTML(ctx context.Context, opts searchOptions) []webSearchItem {
	if e.allowLocalhostForTest || opts.Limit <= 0 {
		return nil
	}
	u, err := url.Parse("https://www.bing.com/search")
	if err != nil {
		return nil
	}
	q := u.Query()
	q.Set("q", appendSiteFilter(opts.Query, opts.Domains))
	q.Set("count", fmt.Sprintf("%d", opts.Limit+2))
	if opts.Lang == "en" || (opts.Lang != "zh" && isASCIIOnlyQuery(opts.Query)) {
		q.Set("setlang", "en")
	} else {
		q.Set("setlang", "zh-Hans")
		q.Set("cc", "CN")
	}
	switch opts.TimeRange {
	case "day":
		q.Set("filters", `ex1:"ez1"`)
	case "week":
		q.Set("filters", `ex1:"ez2"`)
	case "month":
		q.Set("filters", `ex1:"ez3"`)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if opts.Lang == "en" || isASCIIOnlyQuery(opts.Query) {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.7")
	} else {
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	}

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
	return parseBingHTML(string(bodyBytes), opts.Limit)
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

func parseGoogleNewsRSS(rawXML string, limit int, sourceLabel string) []webSearchItem {
	if limit <= 0 || strings.TrimSpace(rawXML) == "" {
		return nil
	}
	if sourceLabel == "" {
		sourceLabel = "GoogleNews"
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
		pubStr := formatSearchDate(it.PubDate)
		desc := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(it.Description)))
		if desc != "" && !strings.EqualFold(desc, title) {
			if len([]rune(desc)) > 200 {
				desc = string([]rune(desc)[:200]) + "..."
			}
			parts = append(parts, desc)
		}
		out = append(out, webSearchItem{
			Title:       title,
			URL:         link,
			Snippet:     strings.Join(parts, " | "),
			PublishedAt: pubStr,
			Source:      sourceLabel,
		})
	}
	return out
}

func (e *Engine) searchGoogleNewsRSS(ctx context.Context, query, timeRange, locale string, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	q := strings.TrimSpace(query)
	if timeRange != "" && !strings.Contains(strings.ToLower(q), "when:") {
		switch timeRange {
		case "day":
			q += " when:1d"
		case "week":
			q += " when:7d"
		case "month":
			q += " when:30d"
		}
	}
	rssParams := "hl=zh-CN&gl=CN&ceid=CN:zh-Hans"
	sourceLabel := "GoogleNews-ZH"
	if strings.EqualFold(locale, "en") {
		rssParams = "hl=en-US&gl=US&ceid=US:en"
		sourceLabel = "GoogleNews-EN"
	}
	rssURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&%s", url.QueryEscape(q), rssParams)
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
	return parseGoogleNewsRSS(string(bodyBytes), limit, sourceLabel)
}

func (e *Engine) searchTavilyAPI(ctx context.Context, apiKey string, opts searchOptions) []webSearchItem {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || opts.Limit <= 0 {
		return nil
	}
	reqMap := map[string]any{
		"api_key":        apiKey,
		"query":          opts.Query,
		"search_depth":   "basic",
		"max_results":    opts.Limit,
		"include_answer": false,
	}
	if opts.TimeRange != "" {
		reqMap["time_range"] = opts.TimeRange
		reqMap["search_depth"] = "advanced"
	}
	if opts.Topic == "news" || opts.Topic == "deals" {
		reqMap["topic"] = "news"
		reqMap["search_depth"] = "advanced"
	}
	if len(opts.Domains) > 0 {
		reqMap["include_domains"] = opts.Domains
	}
	payload, _ := json.Marshal(reqMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tavilySearchEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil
	}
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
		Results []struct {
			Title         string `json:"title"`
			URL           string `json:"url"`
			Content       string `json:"content"`
			PublishedDate string `json:"published_date"`
		} `json:"results"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(opts.Limit, len(parsed.Results)))
	for _, r := range parsed.Results {
		if len(out) >= opts.Limit {
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
			Title:       title,
			URL:         targetURL,
			Snippet:     snippet,
			Excerpt:     excerpt,
			PublishedAt: formatSearchDate(r.PublishedDate),
			Source:      "Tavily",
		})
	}
	return out
}

func (e *Engine) searchSearXNG(ctx context.Context, baseURL string, opts searchOptions) []webSearchItem {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || opts.Limit <= 0 {
		return nil
	}
	parsedBase, err := url.Parse(baseURL)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") {
		return nil
	}
	langParam := "zh-CN"
	if opts.Lang == "en" || opts.Lang == "all" || isASCIIOnlyQuery(opts.Query) {
		langParam = "all"
	}
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&language=%s", baseURL, url.QueryEscape(opts.Query), url.QueryEscape(langParam))
	if opts.TimeRange != "" {
		searchURL += "&time_range=" + url.QueryEscape(opts.TimeRange)
	}
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
			Title         string `json:"title"`
			URL           string `json:"url"`
			Content       string `json:"content"`
			PublishedDate string `json:"publishedDate"`
		} `json:"results"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(opts.Limit, len(parsed.Results)))
	for _, r := range parsed.Results {
		if len(out) >= opts.Limit {
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
			Title:       title,
			URL:         targetURL,
			Snippet:     snippet,
			PublishedAt: formatSearchDate(r.PublishedDate),
			Source:      "SearXNG",
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
	// Structured API/deal community results already carry crisp descriptions/metadata.
	if host == "news.google.com" || host == "api.github.com" || strings.HasSuffix(host, "v2ex.com") || strings.HasSuffix(host, "slickdeals.net") {
		return false
	}
	if host == "huggingface.co" && strings.HasPrefix(u.Path, "/papers/") {
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
		Query     string   `json:"query"`
		TimeRange string   `json:"time_range"`
		Topic     string   `json:"topic"`
		Lang      string   `json:"lang"`
		Domains   []string `json:"domains"`
		Limit     int      `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 web_search 参数失败: %w", err)
	}
	rawQuery := strings.TrimSpace(args.Query)
	if rawQuery == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	query, fallbackQuery := distillSearchKeywords(rawQuery)
	if query == "" {
		query = rawQuery
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 8
	} else if limit > 15 {
		limit = 15
	}

	opts := searchOptions{
		Query:     query,
		TimeRange: normalizeSearchTimeRange(args.TimeRange),
		Topic:     normalizeSearchTopic(args.Topic, query),
		Lang:      normalizeSearchLang(args.Lang),
		Domains:   normalizeSearchDomains(args.Domains),
		Limit:     limit,
	}

	var tavilyKey, serperKey, braveKey, searxngURL string
	if e.store != nil {
		tavilyKey, _ = e.store.GetConfig(store.KeyTavilyAPIKey)
		serperKey, _ = e.store.GetConfig(store.KeySerperAPIKey)
		braveKey, _ = e.store.GetConfig(store.KeyBraveAPIKey)
		searxngURL, _ = e.store.GetConfig(store.KeySearXNGURL)
	}
	tavilyKey = strings.TrimSpace(tavilyKey)
	serperKey = strings.TrimSpace(serperKey)
	braveKey = strings.TrimSpace(braveKey)
	searxngURL = strings.TrimSpace(searxngURL)

	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	// Priority buckets:
	// 0: Tavily API
	// 1: Google (Serper API)
	// 2: Brave Search API
	// 3: Self-hosted SearXNG
	priorityBuckets := make([][]webSearchItem, 4)

	// Live web + real-time community/vertical buckets (merged via round-robin diversity):
	// 0: Bing Web Search HTML (locale + freshness aware)
	// 1: Google News RSS EN (fresh overseas/tech/deals news)
	// 2: Google News RSS ZH (fresh Chinese news)
	// 3: DuckDuckGo HTML
	// 4: Vertical 1 (Tech: GitHub Trending-last-Nd; Deals: NodeSeek RSS)
	// 5: Vertical 2 (Tech: Hacker News search_by_date; Deals: V2EX deals/share)
	// 6: Vertical 3 (Tech: Lobsters; Deals: Slickdeals Frontpage RSS)
	// 7: Vertical 4 (Tech: HF Daily Papers; General/Other: nil)
	// 8: Vertical 5 (Tech: V2EX go/programmer)
	liveBuckets := make([][]webSearchItem, 9)

	// Static encyclopedic fallback buckets:
	// 0: DuckDuckGo Instant Answer JSON
	// 1: Chinese Wikipedia OpenSearch
	fallbackBuckets := make([][]webSearchItem, 2)

	var wg sync.WaitGroup

	if tavilyKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			priorityBuckets[0] = e.searchTavilyAPI(pCtx, tavilyKey, opts)
		}()
	}
	if serperKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			priorityBuckets[1] = e.searchSerperAPI(pCtx, serperKey, opts)
		}()
	}
	if braveKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			priorityBuckets[2] = e.searchBraveAPI(pCtx, braveKey, opts)
		}()
	}
	if searxngURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			priorityBuckets[3] = e.searchSearXNG(pCtx, searxngURL, opts)
		}()
	}

	if !e.allowLocalhostForTest {
		client := e.safeFetchClient()

		// Live 0: Bing Web Search HTML
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 7*time.Second)
			defer pCancel()
			liveBuckets[0] = e.searchBingHTML(pCtx, opts)
		}()

		// Live 1: Google News RSS EN
		if opts.Lang == "en" || opts.Lang == "all" || isASCIIOnlyQuery(opts.Query) || opts.Topic == "tech" || opts.Topic == "deals" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[1] = e.searchGoogleNewsRSS(pCtx, opts.Query, opts.TimeRange, "en", min(limit, 6))
			}()
		}

		// Live 2: Google News RSS ZH
		if opts.Lang != "en" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[2] = e.searchGoogleNewsRSS(pCtx, opts.Query, opts.TimeRange, "zh", min(limit, 6))
			}()
		}

		// Live 3: DuckDuckGo HTML
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
			defer pCancel()
			liveBuckets[3] = e.searchDuckDuckGoHTML(pCtx, opts.Query, min(limit, 6))
		}()

		switch opts.Topic {
		case "deals":
			// Vertical 1: NodeSeek RSS (VPS / cloud / AI promos)
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[4] = e.searchNodeSeekRSS(pCtx, opts.Query, 6)
			}()
			// Vertical 2: V2EX deals & share nodes
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[5] = e.searchV2EXNodes(pCtx, []string{"deals", "share"}, "V2EX", opts.Query, 6)
			}()
			// Vertical 3: Slickdeals Frontpage RSS
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[6] = e.searchSlickdealsRSS(pCtx, opts.Query, 5)
			}()

		case "tech":
			// Vertical 1: GitHub Search API (fresh repos in last 7-14d)
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[4] = e.searchGitHubRepos(pCtx, opts)
			}()
			// Vertical 2: Hacker News search_by_date
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[5] = e.searchHackerNewsAlgolia(pCtx, client, opts, 6)
			}()
			// Vertical 3: Lobsters (/t/go.json, /t/ai.json)
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[6] = e.searchLobsters(pCtx, opts.Query, 5)
			}()
			// Vertical 4: HuggingFace Daily Papers
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[7] = e.searchHuggingFacePapers(pCtx, opts.Query, 5)
			}()
			// Vertical 5: V2EX go & programmer nodes
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 6*time.Second)
				defer pCancel()
				liveBuckets[8] = e.searchV2EXNodes(pCtx, []string{"go", "programmer"}, "V2EX", opts.Query, 5)
			}()

		default:
			// Keep Hacker News search active for general/news queries too
			wg.Add(1)
			go func() {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
				defer pCancel()
				liveBuckets[5] = e.searchHackerNewsAlgolia(pCtx, client, opts, min(limit, 5))
			}()
		}

		// Fallback 0: DuckDuckGo Instant Answer JSON API
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
			defer pCancel()
			ddgAPI := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(opts.Query))
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
				fallbackBuckets[0] = append(fallbackBuckets[0], webSearchItem{
					Title:   notifier.StripEmoji(ddg.Heading),
					URL:     ddg.AbstractURL,
					Snippet: notifier.StripEmoji(ddg.AbstractText),
					Source:  "DuckDuckGo",
				})
			}
			for _, rt := range ddg.RelatedTopics {
				if len(fallbackBuckets[0]) >= limit {
					break
				}
				if rt.Text != "" && rt.FirstURL != "" {
					title := rt.Text
					if idx := strings.Index(title, " - "); idx > 0 {
						title = title[:idx]
					}
					fallbackBuckets[0] = append(fallbackBuckets[0], webSearchItem{
						Title:   notifier.StripEmoji(truncateTitle(title)),
						URL:     rt.FirstURL,
						Snippet: notifier.StripEmoji(rt.Text),
						Source:  "DuckDuckGo",
					})
				}
				for _, sub := range rt.Topics {
					if len(fallbackBuckets[0]) >= limit {
						break
					}
					if sub.Text != "" && sub.FirstURL != "" {
						fallbackBuckets[0] = append(fallbackBuckets[0], webSearchItem{
							Title:   notifier.StripEmoji(truncateTitle(sub.Text)),
							URL:     sub.FirstURL,
							Snippet: notifier.StripEmoji(sub.Text),
							Source:  "DuckDuckGo",
						})
					}
				}
			}
		}()

		// Fallback 1: Chinese Wikipedia OpenSearch
		wg.Add(1)
		go func() {
			defer wg.Done()
			pCtx, pCancel := context.WithTimeout(reqCtx, 5*time.Second)
			defer pCancel()
			wikiURL := fmt.Sprintf("https://zh.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&utf8=1&format=json&srlimit=%d", url.QueryEscape(opts.Query), limit)
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
				fallbackBuckets[1] = append(fallbackBuckets[1], webSearchItem{
					Title:   notifier.StripEmoji(s.Title),
					URL:     "https://zh.wikipedia.org/wiki/" + url.PathEscape(s.Title),
					Snippet: stripHTMLToText(s.Snippet),
					Source:  "Wikipedia",
				})
			}
		}()
	}

	wg.Wait()

	// Reorder Google News buckets slightly ahead of HTML scrapes on pure news/deals or ASCII queries
	if opts.Lang == "en" || isASCIIOnlyQuery(opts.Query) {
		liveBuckets[0], liveBuckets[1] = liveBuckets[1], liveBuckets[0]
	}

	results, providersUsed := mergeSearchBuckets(priorityBuckets, liveBuckets, fallbackBuckets, limit)

	// If conversational query returned <2 results and a broader keyword fallback exists, run a quick second pass.
	if len(results) < 2 && fallbackQuery != "" && fallbackQuery != opts.Query && !e.allowLocalhostForTest {
		fbOpts := opts
		fbOpts.Query = fallbackQuery
		fbCtx, fbCancel := context.WithTimeout(ctx, 5*time.Second)
		fbItems := e.searchBingHTML(fbCtx, fbOpts)
		if len(fbItems) == 0 {
			fbItems = e.searchGoogleNewsRSS(fbCtx, fbOpts.Query, fbOpts.TimeRange, "zh", limit)
		}
		fbCancel()
		extra, extraProviders := mergeSearchBuckets(nil, [][]webSearchItem{results, fbItems}, nil, limit)
		if len(extra) > len(results) {
			results = extra
			providersUsed = extraProviders
		}
	}

	// Automatically deep-read top 3 article hits concurrently so weaker models receive real page content in one call.
	e.enrichTopSearchResults(ctx, results, 3)

	payload, _ := json.Marshal(map[string]any{
		"query":          opts.Query,
		"time_range":     opts.TimeRange,
		"topic":          opts.Topic,
		"count":          len(results),
		"providers_used": providersUsed,
		"results":        results,
	})
	return string(payload), nil
}

func (e *Engine) searchHackerNewsAlgolia(ctx context.Context, client *http.Client, opts searchOptions, limit int) []webSearchItem {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	useByDate := opts.TimeRange != "" || opts.Topic == "tech" || opts.Topic == "news"
	u, _ := url.Parse("https://hn.algolia.com/api/v1/search")
	if useByDate {
		u, _ = url.Parse("https://hn.algolia.com/api/v1/search_by_date")
	}
	q := u.Query()
	q.Set("tags", "story")
	q.Set("hitsPerPage", strconv.Itoa(limit))

	var asciiParts []string
	for _, tok := range extractSearchTerms(opts.Query) {
		if isASCIIOnlyQuery(tok) {
			asciiParts = append(asciiParts, tok)
		}
	}
	if len(asciiParts) > 0 {
		q.Set("query", strings.Join(asciiParts, " "))
	} else if !useByDate {
		q.Set("query", opts.Query)
	}

	if useByDate {
		window := 7 * 24 * time.Hour
		switch opts.TimeRange {
		case "day":
			window = 48 * time.Hour
		case "week":
			window = 7 * 24 * time.Hour
		case "month":
			window = 30 * 24 * time.Hour
		}
		cutoff := time.Now().Add(-window).Unix()
		filters := fmt.Sprintf("created_at_i>%d", cutoff)
		if len(asciiParts) == 0 {
			filters += ",points>35"
		} else {
			filters += ",points>10"
		}
		q.Set("numericFilters", filters)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var hnResp struct {
		Hits []struct {
			Title     string `json:"title"`
			URL       string `json:"url"`
			StoryText string `json:"story_text"`
			ObjectID  string `json:"objectID"`
			Points    int    `json:"points"`
			CreatedAt string `json:"created_at"`
		} `json:"hits"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&hnResp) != nil {
		return nil
	}
	out := make([]webSearchItem, 0, min(limit, len(hnResp.Hits)))
	for _, h := range hnResp.Hits {
		if len(out) >= limit {
			break
		}
		if strings.TrimSpace(h.Title) == "" {
			continue
		}
		itemURL := strings.TrimSpace(h.URL)
		if itemURL == "" {
			itemURL = "https://news.ycombinator.com/item?id=" + h.ObjectID
		}
		snippet := stripHTMLToText(h.StoryText)
		if len([]rune(snippet)) > 180 {
			snippet = string([]rune(snippet)[:180]) + "..."
		}
		if h.Points > 0 {
			if snippet != "" {
				snippet = fmt.Sprintf("[%d points] %s", h.Points, snippet)
			} else {
				snippet = fmt.Sprintf("%d points on Hacker News", h.Points)
			}
		}
		out = append(out, webSearchItem{
			Title:       notifier.StripEmoji(h.Title),
			URL:         itemURL,
			Snippet:     snippet,
			PublishedAt: formatSearchDate(h.CreatedAt),
			Source:      "HackerNews",
		})
	}
	return out
}
