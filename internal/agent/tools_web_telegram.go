package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
	"golang.org/x/net/html"
)

var (
	telegramPublicBaseURL = "https://t.me/s"
	reValidTGChannel      = regexp.MustCompile(`^[A-Za-z0-9_]{4,40}$`)
)

type tgChannelMessage struct {
	Channel     string    `json:"channel"`
	PostID      string    `json:"post_id"`
	URL         string    `json:"url"`
	PublishedAt string    `json:"published_at"`
	PublishedTS time.Time `json:"-"`
	Views       string    `json:"views,omitempty"`
	Text        string    `json:"text"`
	Links       []string  `json:"links,omitempty"`
}

func parseTGChannels(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, min(len(parts), 12))
	for _, p := range parts {
		ch := strings.TrimSpace(p)
		for _, prefix := range []string{
			"https://t.me/s/", "http://t.me/s/", "t.me/s/",
			"https://t.me/", "http://t.me/", "t.me/",
			"https://telegram.me/s/", "http://telegram.me/s/", "telegram.me/s/",
			"https://telegram.me/", "http://telegram.me/", "telegram.me/",
		} {
			if len(ch) >= len(prefix) && strings.EqualFold(ch[:len(prefix)], prefix) {
				ch = ch[len(prefix):]
				break
			}
		}
		if idx := strings.IndexAny(ch, "?#/"); idx >= 0 {
			ch = ch[:idx]
		}
		ch = strings.TrimPrefix(strings.TrimSpace(ch), "@")
		if !reValidTGChannel.MatchString(ch) {
			continue
		}
		low := strings.ToLower(ch)
		if seen[low] {
			continue
		}
		seen[low] = true
		out = append(out, ch)
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func extractTGChannelFromURL(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "t.me" && host != "telegram.me" {
		return "", false
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	var ch string
	if len(segs) == 1 && segs[0] != "" {
		ch = segs[0]
	} else if len(segs) == 2 && strings.EqualFold(segs[0], "s") && segs[1] != "" {
		ch = segs[1]
	}
	ch = strings.TrimPrefix(strings.TrimSpace(ch), "@")
	if !reValidTGChannel.MatchString(ch) {
		return "", false
	}
	return ch, true
}

func htmlAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func hasHTMLClass(n *html.Node, targetClass string) bool {
	cls := htmlAttr(n, "class")
	if cls == "" {
		return false
	}
	for _, field := range strings.Fields(cls) {
		if field == targetClass {
			return true
		}
	}
	return false
}

func renderTGMessageNodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(cur *html.Node)
	walk = func(cur *html.Node) {
		switch cur.Type {
		case html.TextNode:
			sb.WriteString(cur.Data)
		case html.ElementNode:
			if cur.Data == "br" {
				sb.WriteByte('\n')
				return
			}
			for c := cur.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if cur.Data == "p" || cur.Data == "div" || cur.Data == "li" {
				sb.WriteByte('\n')
			}
		default:
			for c := cur.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(n)

	rawLines := strings.Split(sb.String(), "\n")
	cleanLines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	joined := notifier.StripEmoji(strings.TrimSpace(strings.Join(cleanLines, "\n")))
	if runes := []rune(joined); len(runes) > 600 {
		joined = string(runes[:600]) + "..."
	}
	return joined
}

func parseTelegramChannelHTML(rawHTML, fallbackChannel string, now time.Time, maxAgeHours int, limit int) []tgChannelMessage {
	if strings.TrimSpace(rawHTML) == "" || limit <= 0 {
		return nil
	}
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}

	var rawMsgs []tgChannelMessage
	var walkMsg func(n *html.Node)
	walkMsg = func(n *html.Node) {
		if n.Type == html.ElementNode && hasHTMLClass(n, "tgme_widget_message") && htmlAttr(n, "data-post") != "" {
			dataPost := strings.TrimSpace(htmlAttr(n, "data-post"))
			msgChannel := strings.TrimSpace(fallbackChannel)
			if slash := strings.IndexByte(dataPost, '/'); slash > 0 {
				msgChannel = dataPost[:slash]
			}
			msg := tgChannelMessage{
				Channel: msgChannel,
				PostID:  dataPost,
				URL:     "https://t.me/" + dataPost,
			}

			var seenLinks = make(map[string]bool)
			var inspectInner func(cur *html.Node, collectLinks bool)
			inspectInner = func(cur *html.Node, collectLinks bool) {
				if cur.Type == html.ElementNode {
					if cur.Data == "time" && msg.PublishedTS.IsZero() {
						dt := strings.TrimSpace(htmlAttr(cur, "datetime"))
						if parsedTS, pErr := time.Parse(time.RFC3339, dt); pErr == nil {
							msg.PublishedTS = parsedTS
							msg.PublishedAt = parsedTS.In(store.ShanghaiLocation()).Format("2006-01-02 15:04")
						} else if dt != "" {
							msg.PublishedAt = dt
						}
					}
					if hasHTMLClass(cur, "tgme_widget_message_views") && msg.Views == "" {
						msg.Views = strings.TrimSpace(renderTGMessageNodeText(cur))
					}
					if hasHTMLClass(cur, "tgme_widget_message_text") {
						if txt := renderTGMessageNodeText(cur); txt != "" {
							if msg.Text != "" {
								msg.Text += "\n" + txt
							} else {
								msg.Text = txt
							}
						}
						collectLinks = true
					}
					if hasHTMLClass(cur, "tgme_widget_message_link_preview") {
						collectLinks = true
					}
					if collectLinks && cur.Data == "a" {
						href := strings.TrimSpace(htmlAttr(cur, "href"))
						if (strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://")) && len(msg.Links) < 5 {
							selfPrefix := "https://t.me/" + strings.ToLower(msg.Channel)
							if !strings.HasPrefix(strings.ToLower(href), selfPrefix) && !seenLinks[href] {
								seenLinks[href] = true
								msg.Links = append(msg.Links, href)
							}
						}
					}
				}
				for c := cur.FirstChild; c != nil; c = c.NextSibling {
					inspectInner(c, collectLinks)
				}
			}
			inspectInner(n, false)

			if msg.Text != "" || len(msg.Links) > 0 {
				if maxAgeHours > 0 {
					if !msg.PublishedTS.IsZero() && now.Sub(msg.PublishedTS) <= time.Duration(maxAgeHours)*time.Hour {
						rawMsgs = append(rawMsgs, msg)
					}
				} else {
					rawMsgs = append(rawMsgs, msg)
				}
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkMsg(c)
		}
	}
	walkMsg(doc)

	// Telegram renders oldest-to-newest top-to-bottom; reverse so newest is first.
	for i, j := 0, len(rawMsgs)-1; i < j; i, j = i+1, j-1 {
		rawMsgs[i], rawMsgs[j] = rawMsgs[j], rawMsgs[i]
	}
	if len(rawMsgs) > limit {
		rawMsgs = rawMsgs[:limit]
	}
	return rawMsgs
}

func (e *Engine) fetchTelegramChannel(ctx context.Context, channel string, maxAgeHours int, limit int) ([]tgChannelMessage, error) {
	chList := parseTGChannels(channel)
	if len(chList) == 0 {
		return nil, fmt.Errorf("无效 Telegram 频道: %s", channel)
	}
	cleanCh := chList[0]
	targetURL := fmt.Sprintf("%s/%s", strings.TrimRight(telegramPublicBaseURL, "/"), url.PathEscape(cleanCh))

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.apiHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Telegram 频道 HTTP %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	return parseTelegramChannelHTML(string(bodyBytes), cleanCh, time.Now(), maxAgeHours, limit), nil
}

func (e *Engine) fetchTelegramChannelsDigest(ctx context.Context, channels []string, maxAgeHours int, perChannelLimit int) ([]tgChannelMessage, []string) {
	if len(channels) == 0 || perChannelLimit <= 0 {
		return nil, nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		combined []tgChannelMessage
		emptyChs []string
	)
	for _, ch := range channels {
		chCopy := strings.TrimSpace(ch)
		if chCopy == "" {
			continue
		}
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			msgs, err := e.fetchTelegramChannel(reqCtx, c, maxAgeHours, perChannelLimit)
			mu.Lock()
			if err != nil || len(msgs) == 0 {
				emptyChs = append(emptyChs, c)
			} else {
				combined = append(combined, msgs...)
			}
			mu.Unlock()
		}(chCopy)
	}
	wg.Wait()

	sort.SliceStable(combined, func(i, j int) bool {
		return combined[i].PublishedTS.After(combined[j].PublishedTS)
	})
	return combined, emptyChs
}

func formatTelegramMessagesForPrompt(msgs []tgChannelMessage, channels []string, maxItems int) string {
	if len(msgs) == 0 {
		return ""
	}
	if maxItems <= 0 {
		maxItems = 24
	}
	if len(msgs) > maxItems {
		msgs = msgs[:maxItems]
	}
	var sb strings.Builder
	if len(channels) > 0 {
		sb.WriteString("- 监控频道: @" + strings.Join(channels, ", @") + "\n")
	}
	for _, m := range msgs {
		meta := "@" + m.Channel
		if m.PublishedAt != "" {
			meta += " | " + m.PublishedAt
		}
		if m.Views != "" {
			meta += " | views:" + m.Views
		}
		textSingleLine := strings.Join(strings.Fields(m.Text), " ")
		if runes := []rune(textSingleLine); len(runes) > 320 {
			textSingleLine = string(runes[:320]) + "..."
		}
		sb.WriteString("- [")
		sb.WriteString(meta)
		sb.WriteString("] ")
		sb.WriteString(textSingleLine)
		sb.WriteString(" | 原帖: ")
		sb.WriteString(m.URL)
		if len(m.Links) > 0 {
			sb.WriteString(" | 外链: ")
			sb.WriteString(strings.Join(m.Links, " , "))
		}
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}
