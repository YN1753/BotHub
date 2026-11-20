package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	reBiliBVID      = regexp.MustCompile(`(?i)(BV[0-9A-Za-z]{10})`)
	reBiliAID       = regexp.MustCompile(`(?i)(?:/video/av|(?:\?|&)aid=)(\d+)`)
	reDouyinVideoID = regexp.MustCompile(`(?i)(?:/video/|/share/video/|/note/)(\d{10,25})`)
	reJSUndefined   = regexp.MustCompile(`:\s*undefined\b`)
	reWeChatNick    = regexp.MustCompile(`var\s+nickname\s*=\s*htmlDecode\("([^"]+)"\)|"nick_name"\s*:\s*"([^"]+)"`)
)

func extractAssignedJSONObject(rawHTML string, markers ...string) string {
	for _, marker := range markers {
		idx := strings.Index(rawHTML, marker)
		if idx < 0 {
			continue
		}
		start := strings.IndexByte(rawHTML[idx+len(marker):], '{')
		if start < 0 {
			continue
		}
		pos := idx + len(marker) + start
		depth := 0
		inStr := false
		escaped := false
		for i := pos; i < len(rawHTML) && i-pos < 512*1024; i++ {
			ch := rawHTML[i]
			if inStr {
				if escaped {
					escaped = false
				} else if ch == '\\' {
					escaped = true
				} else if ch == '"' {
					inStr = false
				}
				continue
			}
			switch ch {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return rawHTML[pos : i+1]
				}
			}
		}
	}
	return ""
}

func isBlockedOrLoginWall(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	lower := strings.ToLower(trimmed)
	runes := len([]rune(trimmed))
	if runes < 220 {
		for _, kw := range []string{
			"小红书 - 你的生活指南",
			"登录后查看",
			"请先登录",
			"扫码登录",
			"安全验证",
			"滑动验证",
			"验证码",
			"访问受限",
			"just a moment",
			"access denied",
			"412 precondition failed",
			"403 forbidden",
			"enable javascript",
			"captcha",
		} {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	return false
}

func (e *Engine) fetchJinaReaderFallback(ctx context.Context, targetURL string) string {
	if e == nil || e.allowLocalhostForTest || strings.TrimSpace(targetURL) == "" {
		return ""
	}
	if strings.Contains(strings.ToLower(targetURL), "r.jina.ai") {
		return ""
	}
	jinaCtx, jinaCancel := context.WithTimeout(ctx, 10*time.Second)
	defer jinaCancel()
	jsc, _, _, jBody, jErr := e.fetchWebResource(jinaCtx, "https://r.jina.ai/"+targetURL, "text/plain", nil)
	if jErr != nil || jsc < 200 || jsc >= 300 {
		return ""
	}
	jClean := notifier.StripEmoji(strings.TrimSpace(jBody))
	if len([]rune(jClean)) < 40 || isBlockedOrLoginWall(jClean) {
		return ""
	}
	return jClean
}

type extractedStashSource struct {
	CanonicalURL string
	Domain       string
	Kind         string
	Title        string
	Description  string
	Excerpt      string
	SuggestedTag []string
}

// ConfigureTestEndpoints enables localhost HTTP targets in unit tests without weakening production SSRF protection.
func (e *Engine) ConfigureTestEndpoints(allowLocalhost bool, biliAPIBase, githubAPIBase, githubRawBase string) {
	if e == nil {
		return
	}
	e.allowLocalhostForTest = allowLocalhost
	e.biliAPIBase = strings.TrimRight(strings.TrimSpace(biliAPIBase), "/")
	e.githubAPIBase = strings.TrimRight(strings.TrimSpace(githubAPIBase), "/")
	e.githubRawBase = strings.TrimRight(strings.TrimSpace(githubRawBase), "/")
}

func (e *Engine) getBiliAPIBase() string {
	if e != nil && e.biliAPIBase != "" {
		return e.biliAPIBase
	}
	return "https://api.bilibili.com"
}

func (e *Engine) getGitHubAPIBase() string {
	if e != nil && e.githubAPIBase != "" {
		return e.githubAPIBase
	}
	return "https://api.github.com"
}

func (e *Engine) getGitHubRawBase() string {
	if e != nil && e.githubRawBase != "" {
		return e.githubRawBase
	}
	return "https://raw.githubusercontent.com"
}

func sanitizeBiliSessdata(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, "\r\n;") {
		return ""
	}
	return s
}

func isAllowedBiliSubtitleHost(host string, allowLocalhost bool) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if allowLocalhost && (h == "127.0.0.1" || h == "localhost") {
		return true
	}
	return h == "bilibili.com" || strings.HasSuffix(h, ".bilibili.com") || h == "hdslb.com" || strings.HasSuffix(h, ".hdslb.com")
}

func extractBiliVideoIDs(targetURL string) (bvid, aid string) {
	if m := reBiliBVID.FindStringSubmatch(targetURL); len(m) > 1 {
		raw := m[1]
		if len(raw) > 2 {
			bvid = "BV" + raw[2:]
		} else {
			bvid = raw
		}
	}
	if bvid == "" {
		if m := reBiliAID.FindStringSubmatch(targetURL); len(m) > 1 {
			aid = m[1]
		}
	}
	return bvid, aid
}

func parseBilibiliHTMLFallback(rawHTML, normURL, dom, bvid string) *extractedStashSource {
	if strings.TrimSpace(rawHTML) == "" {
		return nil
	}
	var title, desc, upName, tname string
	tags := []string{"Bilibili"}
	var pages []string

	if stateJSON := extractAssignedJSONObject(rawHTML, "window.__INITIAL_STATE__=", "window.__INITIAL_STATE__ ="); stateJSON != "" {
		var st struct {
			VideoData struct {
				BVID  string `json:"bvid"`
				Title string `json:"title"`
				Desc  string `json:"desc"`
				TName string `json:"tname"`
				Owner struct {
					Name string `json:"name"`
				} `json:"owner"`
				Pages []struct {
					Page int    `json:"page"`
					Part string `json:"part"`
				} `json:"pages"`
			} `json:"videoData"`
			Tags []struct {
				TagName string `json:"tag_name"`
			} `json:"tags"`
		}
		if json.Unmarshal([]byte(stateJSON), &st) == nil && strings.TrimSpace(st.VideoData.Title) != "" {
			title = notifier.StripEmoji(strings.TrimSpace(st.VideoData.Title))
			desc = notifier.StripEmoji(strings.TrimSpace(st.VideoData.Desc))
			upName = notifier.StripEmoji(strings.TrimSpace(st.VideoData.Owner.Name))
			tname = notifier.StripEmoji(strings.TrimSpace(st.VideoData.TName))
			if st.VideoData.BVID != "" && strings.Contains(strings.ToLower(normURL), "b23.tv") {
				normURL = "https://www.bilibili.com/video/" + st.VideoData.BVID
				dom = "bilibili.com"
			}
			for i, p := range st.VideoData.Pages {
				if i >= 20 {
					break
				}
				part := notifier.StripEmoji(strings.TrimSpace(p.Part))
				if part != "" {
					pages = append(pages, fmt.Sprintf("- P%d: %s", p.Page, part))
				}
			}
			for i, tg := range st.Tags {
				if i >= 4 {
					break
				}
				ct := notifier.StripEmoji(strings.TrimSpace(tg.TagName))
				if ct != "" {
					tags = append(tags, ct)
				}
			}
		}
	}

	if title == "" {
		rawTitle := extractHTMLTitle(rawHTML)
		rawTitle = strings.TrimSuffix(rawTitle, "_哔哩哔哩_bilibili")
		rawTitle = strings.TrimSuffix(rawTitle, "-哔哩哔哩")
		rawTitle = strings.TrimSpace(rawTitle)
		if rawTitle != "" && !isBlockedOrLoginWall(rawTitle) && !strings.EqualFold(rawTitle, "bilibili") {
			title = rawTitle
		}
	}
	if desc == "" {
		metaDesc := extractHTMLMetaDesc(rawHTML)
		if metaDesc != "" && !isBlockedOrLoginWall(metaDesc) {
			desc = metaDesc
		}
	}
	if upName == "" {
		upName = extractHTMLAuthor(rawHTML)
	}
	if len(tags) == 1 {
		if kw := extractHTMLKeywords(rawHTML); kw != "" {
			for i, part := range strings.Split(kw, ",") {
				if i >= 4 {
					break
				}
				ct := strings.TrimSpace(part)
				if ct != "" && !strings.EqualFold(ct, "bilibili") && ct != "哔哩哔哩" && ct != "弹幕" && ct != title {
					tags = append(tags, ct)
				}
			}
		}
	}

	if title == "" && desc == "" {
		return nil
	}
	if bvid != "" && strings.Contains(strings.ToLower(normURL), "b23.tv") {
		normURL = "https://www.bilibili.com/video/" + bvid
		dom = "bilibili.com"
	}

	var sb strings.Builder
	if upName != "" {
		sb.WriteString(fmt.Sprintf("UP主：%s\n", upName))
	}
	if tname != "" {
		sb.WriteString(fmt.Sprintf("分区频道：%s\n", tname))
		tags = append(tags, tname)
	}
	if desc != "" && desc != "-" {
		sb.WriteString("视频简介：\n")
		sb.WriteString(desc)
		sb.WriteString("\n")
	}
	if len(pages) > 1 {
		sb.WriteString("\n分P目录：\n")
		sb.WriteString(strings.Join(pages, "\n"))
	}
	if sb.Len() == 0 && title != "" {
		sb.WriteString(title)
	}
	displayTitle := title
	if upName != "" && title != "" && !strings.HasPrefix(title, "【") && !strings.Contains(title, upName) {
		displayTitle = fmt.Sprintf("[%s] %s", upName, title)
	}
	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         store.StashKindBilibili,
		Title:        displayTitle,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
		SuggestedTag: tags,
	}
}

func (e *Engine) extractBilibiliSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	resolvedURL := targetURL
	landingHTML := ""
	bvid, aid := extractBiliVideoIDs(resolvedURL)

	// Follow b23.tv or short-link redirects first when BV/av ID is not yet visible
	if bvid == "" && aid == "" {
		rsc, finalURL, _, htmlBody, err := e.fetchWebResource(ctx, targetURL, "", map[string]string{
			"Referer": "https://www.bilibili.com/",
		})
		if err != nil {
			return nil, err
		}
		landingHTML = htmlBody
		if finalURL != "" {
			resolvedURL = finalURL
		}
		bvid, aid = extractBiliVideoIDs(resolvedURL)
		if bvid == "" && aid == "" {
			if rsc >= 400 || strings.HasPrefix(strings.TrimSpace(htmlBody), "{") {
				return nil, fmt.Errorf("Bilibili 短链解析受限 (HTTP %d)", rsc)
			}
			// Non-video Bilibili page (e.g. bilibili.com/read/cv...): parse HTML directly
			normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
			if !ok {
				normURL = targetURL
				dom = "bilibili.com"
			}
			title := extractHTMLTitle(htmlBody)
			desc := extractHTMLMetaDesc(htmlBody)
			text := stripHTMLToText(htmlBody)
			if isBlockedOrLoginWall(text) && title == "" && desc == "" {
				return nil, fmt.Errorf("Bilibili 页面访问受限")
			}
			return &extractedStashSource{
				CanonicalURL: normURL,
				Domain:       dom,
				Kind:         store.StashKindBilibili,
				Title:        title,
				Description:  desc,
				Excerpt:      store.TruncateRunes(text, store.MaxStashExcerptRunes),
				SuggestedTag: []string{"Bilibili"},
			}, nil
		}
	}

	normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
	if !ok {
		normURL = targetURL
		dom = "bilibili.com"
	}
	if bvid != "" && strings.Contains(strings.ToLower(normURL), "b23.tv") {
		normURL = "https://www.bilibili.com/video/" + bvid
		dom = "bilibili.com"
	}

	sessCfg, _ := e.store.GetConfig(store.KeyBiliSessdata)
	sessdata := sanitizeBiliSessdata(sessCfg)
	biliReferer := "https://www.bilibili.com/"
	if bvid != "" {
		biliReferer = "https://www.bilibili.com/video/" + bvid
	}
	biliHeaders := map[string]string{
		"Referer": biliReferer,
		"Origin":  "https://www.bilibili.com",
	}
	if sessdata != "" {
		biliHeaders["Cookie"] = "SESSDATA=" + sessdata
	}

	viewQuery := ""
	if bvid != "" {
		viewQuery = "bvid=" + url.QueryEscape(bvid)
	} else {
		viewQuery = "aid=" + url.QueryEscape(aid)
	}
	viewAPI := fmt.Sprintf("%s/x/web-interface/view?%s", e.getBiliAPIBase(), viewQuery)
	sc, _, _, viewRaw, err := e.fetchWebResource(ctx, viewAPI, "application/json", biliHeaders)
	apiFailed := err != nil || sc < 200 || sc >= 300
	var apiErr error
	if err != nil {
		apiErr = err
	} else if sc < 200 || sc >= 300 {
		apiErr = fmt.Errorf("Bilibili 接口响应 HTTP %d", sc)
	}

	var viewResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			BVID  string `json:"bvid"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			TName string `json:"tname"`
			CID   int64  `json:"cid"`
			Owner struct {
				Name string `json:"name"`
			} `json:"owner"`
			Pages []struct {
				Page int    `json:"page"`
				Part string `json:"part"`
				CID  int64  `json:"cid"`
			} `json:"pages"`
			Subtitle struct {
				List []struct {
					LanDoc      string `json:"lan_doc"`
					SubtitleURL string `json:"subtitle_url"`
				} `json:"list"`
			} `json:"subtitle"`
		} `json:"data"`
	}
	if !apiFailed {
		if uErr := json.Unmarshal([]byte(viewRaw), &viewResp); uErr != nil || viewResp.Code != 0 {
			apiFailed = true
			if viewResp.Message != "" {
				apiErr = fmt.Errorf("Bilibili 视频元数据返回异常: %s", viewResp.Message)
			} else {
				apiErr = fmt.Errorf("解析 Bilibili 视频元数据失败")
			}
		}
	}

	if apiFailed {
		// Fallback A: parse landing HTML from b23.tv redirect (if already fetched)
		if fb := parseBilibiliHTMLFallback(landingHTML, normURL, dom, bvid); fb != nil {
			return fb, nil
		}
		// Fallback B: fetch desktop/mobile video HTML page directly (only when not targeting localhost test stub)
		if !e.allowLocalhostForTest {
			pageURL := normURL
			if bvid != "" {
				pageURL = "https://www.bilibili.com/video/" + bvid
			}
			for _, attempt := range []struct {
				u  string
				ua string
			}{
				{u: pageURL, ua: desktopBrowserUA},
				{u: strings.Replace(pageURL, "https://www.bilibili.com/", "https://m.bilibili.com/", 1), ua: mobileBrowserUA},
			} {
				pCtx, pCancel := context.WithTimeout(ctx, 8*time.Second)
				psc, _, _, pHTML, pErr := e.fetchWebResource(pCtx, attempt.u, "", map[string]string{
					"User-Agent": attempt.ua,
					"Referer":    "https://www.bilibili.com/",
				})
				pCancel()
				if pErr == nil && psc >= 200 && psc < 300 {
					if fb := parseBilibiliHTMLFallback(pHTML, normURL, dom, bvid); fb != nil {
						return fb, nil
					}
				}
			}
			// Fallback C: Jina Reader
			if jText := e.fetchJinaReaderFallback(ctx, pageURL); jText != "" {
				return &extractedStashSource{
					CanonicalURL: normURL,
					Domain:       dom,
					Kind:         store.StashKindBilibili,
					Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
					SuggestedTag: []string{"Bilibili"},
				}, nil
			}
		}
		return nil, apiErr
	}

	d := viewResp.Data
	if d.BVID != "" && strings.Contains(strings.ToLower(normURL), "b23.tv") {
		normURL = "https://www.bilibili.com/video/" + d.BVID
		dom = "bilibili.com"
	}

	var sb strings.Builder
	title := notifier.StripEmoji(strings.TrimSpace(d.Title))
	upName := notifier.StripEmoji(strings.TrimSpace(d.Owner.Name))
	tname := notifier.StripEmoji(strings.TrimSpace(d.TName))
	desc := notifier.StripEmoji(strings.TrimSpace(d.Desc))

	if upName != "" && title != "" && !strings.Contains(title, upName) {
		sb.WriteString(fmt.Sprintf("UP主：%s\n", upName))
	}
	if tname != "" {
		sb.WriteString(fmt.Sprintf("分区频道：%s\n", tname))
	}
	if desc != "" && desc != "-" {
		sb.WriteString("视频简介：\n")
		sb.WriteString(desc)
		sb.WriteString("\n")
	}
	if len(d.Pages) > 1 {
		sb.WriteString("\n分P目录：\n")
		maxPages := len(d.Pages)
		if maxPages > 25 {
			maxPages = 25
		}
		for i := 0; i < maxPages; i++ {
			part := notifier.StripEmoji(strings.TrimSpace(d.Pages[i].Part))
			if part != "" {
				sb.WriteString(fmt.Sprintf("- P%d: %s\n", d.Pages[i].Page, part))
			}
		}
	}

	tags := []string{"Bilibili"}
	if tname != "" {
		tags = append(tags, tname)
	}
	// Best-effort fetch official archive tags
	tagCtx, tagCancel := context.WithTimeout(ctx, 4*time.Second)
	tagAPI := fmt.Sprintf("%s/x/tag/archive/tags?%s", e.getBiliAPIBase(), viewQuery)
	if tsc, _, _, tagBody, tErr := e.fetchWebResource(tagCtx, tagAPI, "application/json", biliHeaders); tErr == nil && tsc == 200 {
		var tagResp struct {
			Code int `json:"code"`
			Data []struct {
				TagName string `json:"tag_name"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(tagBody), &tagResp) == nil && tagResp.Code == 0 {
			for i, tg := range tagResp.Data {
				if i >= 4 {
					break
				}
				cleanTag := notifier.StripEmoji(strings.TrimSpace(tg.TagName))
				if cleanTag != "" {
					tags = append(tags, cleanTag)
				}
			}
		}
	}
	tagCancel()

	// Optional subtitle extraction when bili_sessdata is configured (or view payload already exposes subtitle URL)
	subURL := ""
	if len(d.Subtitle.List) > 0 {
		subURL = strings.TrimSpace(d.Subtitle.List[0].SubtitleURL)
	}
	cid := d.CID
	if cid == 0 && len(d.Pages) > 0 {
		cid = d.Pages[0].CID
	}
	if subURL == "" && sessdata != "" && cid > 0 && (d.BVID != "" || bvid != "") {
		useBV := d.BVID
		if useBV == "" {
			useBV = bvid
		}
		pCtx, pCancel := context.WithTimeout(ctx, 5*time.Second)
		playerAPI := fmt.Sprintf("%s/x/player/v2?bvid=%s&cid=%d", e.getBiliAPIBase(), url.QueryEscape(useBV), cid)
		if psc, _, _, pBody, pErr := e.fetchWebResource(pCtx, playerAPI, "application/json", biliHeaders); pErr == nil && psc == 200 {
			var pResp struct {
				Code int `json:"code"`
				Data struct {
					Subtitle struct {
						Subtitles []struct {
							SubtitleURL string `json:"subtitle_url"`
						} `json:"subtitles"`
					} `json:"subtitle"`
				} `json:"data"`
			}
			if json.Unmarshal([]byte(pBody), &pResp) == nil && pResp.Code == 0 && len(pResp.Data.Subtitle.Subtitles) > 0 {
				subURL = strings.TrimSpace(pResp.Data.Subtitle.Subtitles[0].SubtitleURL)
			}
		}
		pCancel()
	}

	if subURL != "" {
		if strings.HasPrefix(subURL, "//") {
			subURL = "https:" + subURL
		}
		if parsedSub, sErr := url.Parse(subURL); sErr == nil && isAllowedBiliSubtitleHost(parsedSub.Hostname(), e.allowLocalhostForTest) {
			sCtx, sCancel := context.WithTimeout(ctx, 6*time.Second)
			if ssc, _, _, sBody, fErr := e.fetchWebResource(sCtx, subURL, "application/json", nil); fErr == nil && ssc == 200 {
				var subDoc struct {
					Body []struct {
						Content string `json:"content"`
					} `json:"body"`
				}
				if json.Unmarshal([]byte(sBody), &subDoc) == nil && len(subDoc.Body) > 0 {
					var subBuilder strings.Builder
					lastLine := ""
					for _, line := range subDoc.Body {
						txt := notifier.StripEmoji(strings.TrimSpace(line.Content))
						if txt == "" || txt == lastLine {
							continue
						}
						lastLine = txt
						subBuilder.WriteString(txt)
						subBuilder.WriteString(" ")
						if len([]rune(subBuilder.String())) >= 4800 {
							break
						}
					}
					if subText := strings.TrimSpace(subBuilder.String()); subText != "" {
						sb.WriteString("\n字幕内容摘录：\n")
						sb.WriteString(store.TruncateRunes(subText, 5000))
					}
				}
			}
			sCancel()
		}
	}

	displayTitle := title
	if upName != "" && title != "" && !strings.HasPrefix(title, "【") {
		displayTitle = fmt.Sprintf("[%s] %s", upName, title)
	}

	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         store.StashKindBilibili,
		Title:        displayTitle,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
		SuggestedTag: tags,
	}, nil
}

func (e *Engine) extractGitHubSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return e.extractGenericWebSource(ctx, targetURL)
	}
	// If pointing to an issue, pull request, or discussion subpage, extract HTML directly
	if len(parts) >= 3 && parts[2] != "tree" && parts[2] != "blob" {
		src, gErr := e.extractGenericWebSource(ctx, targetURL)
		if gErr == nil && src != nil {
			src.Kind = store.StashKindGitHub
			src.SuggestedTag = append([]string{"GitHub"}, src.SuggestedTag...)
		}
		return src, gErr
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")
	repoFull := owner + "/" + repo

	var sb strings.Builder
	var desc string
	tags := []string{"GitHub", "开源"}

	metaCtx, metaCancel := context.WithTimeout(ctx, 6*time.Second)
	metaURL := fmt.Sprintf("%s/repos/%s/%s", e.getGitHubAPIBase(), url.PathEscape(owner), url.PathEscape(repo))
	if sc, _, _, metaRaw, mErr := e.fetchWebResource(metaCtx, metaURL, "application/vnd.github+json", nil); mErr == nil && sc == 200 {
		var repoMeta struct {
			FullName    string   `json:"full_name"`
			Description string   `json:"description"`
			Language    string   `json:"language"`
			Stars       int      `json:"stargazers_count"`
			Topics      []string `json:"topics"`
		}
		if json.Unmarshal([]byte(metaRaw), &repoMeta) == nil {
			if repoMeta.FullName != "" {
				repoFull = repoMeta.FullName
			}
			desc = notifier.StripEmoji(strings.TrimSpace(repoMeta.Description))
			sb.WriteString(fmt.Sprintf("仓库：%s\n", repoFull))
			if repoMeta.Language != "" {
				sb.WriteString(fmt.Sprintf("主要语言：%s · Stars：%d\n", repoMeta.Language, repoMeta.Stars))
				tags = append(tags, repoMeta.Language)
			}
			if desc != "" {
				sb.WriteString(fmt.Sprintf("简介：%s\n\n", desc))
			}
			for i, tp := range repoMeta.Topics {
				if i >= 2 {
					break
				}
				tags = append(tags, notifier.StripEmoji(tp))
			}
		}
	}
	metaCancel()

	readmeURL := fmt.Sprintf("%s/%s/%s/HEAD/README.md", e.getGitHubRawBase(), url.PathEscape(owner), url.PathEscape(repo))
	rsc, _, _, readmeBody, rErr := e.fetchWebResource(ctx, readmeURL, "text/plain", nil)
	if rErr == nil && rsc >= 200 && rsc < 300 && strings.TrimSpace(readmeBody) != "" {
		cleanReadme := notifier.StripEmoji(strings.TrimSpace(readmeBody))
		sb.WriteString("README 核心内容：\n")
		sb.WriteString(cleanReadme)
	} else if sb.Len() == 0 {
		return e.extractGenericWebSource(ctx, targetURL)
	}

	title := repoFull
	if desc != "" {
		title = fmt.Sprintf("%s — %s", repoFull, store.TruncateRunes(desc, 60))
	}
	normURL, dom, _, _ := store.NormalizeStashURL(targetURL)
	if dom == "" {
		dom = "github.com"
	}

	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         store.StashKindGitHub,
		Title:        title,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
		SuggestedTag: tags,
	}, nil
}

func parseXiaohongshuHTML(rawHTML, normURL, dom string) *extractedStashSource {
	if strings.TrimSpace(rawHTML) == "" {
		return nil
	}
	var title, desc, author string
	tags := []string{"小红书"}

	if stateJSON := extractAssignedJSONObject(rawHTML, "window.__INITIAL_STATE__=", "window.__INITIAL_STATE__ ="); stateJSON != "" {
		cleanJSON := reJSUndefined.ReplaceAllString(stateJSON, ":null")
		var st struct {
			Note struct {
				NoteDetailMap map[string]struct {
					Note struct {
						Title string `json:"title"`
						Desc  string `json:"desc"`
						User  struct {
							Nickname string `json:"nickname"`
						} `json:"user"`
						TagList []struct {
							Name string `json:"name"`
						} `json:"tagList"`
					} `json:"note"`
				} `json:"noteDetailMap"`
			} `json:"note"`
		}
		if json.Unmarshal([]byte(cleanJSON), &st) == nil {
			for _, entry := range st.Note.NoteDetailMap {
				n := entry.Note
				if strings.TrimSpace(n.Title) != "" || strings.TrimSpace(n.Desc) != "" {
					title = notifier.StripEmoji(strings.TrimSpace(n.Title))
					desc = notifier.StripEmoji(strings.TrimSpace(n.Desc))
					author = notifier.StripEmoji(strings.TrimSpace(n.User.Nickname))
					for i, tg := range n.TagList {
						if i >= 4 {
							break
						}
						ct := notifier.StripEmoji(strings.TrimSpace(tg.Name))
						if ct != "" {
							tags = append(tags, ct)
						}
					}
					break
				}
			}
		}
	}

	if title == "" {
		t := strings.TrimSpace(strings.TrimSuffix(extractHTMLTitle(rawHTML), " - 小红书"))
		if t != "" && !isBlockedOrLoginWall(t) && t != "小红书" {
			title = t
		}
	}
	if desc == "" {
		d := extractHTMLMetaDesc(rawHTML)
		if d != "" && !isBlockedOrLoginWall(d) {
			desc = d
		}
	}

	if title == "" && desc == "" {
		return nil
	}
	var sb strings.Builder
	if author != "" {
		sb.WriteString(fmt.Sprintf("作者：%s\n", author))
	}
	if title != "" {
		sb.WriteString(fmt.Sprintf("标题：%s\n", title))
	}
	if desc != "" {
		sb.WriteString("正文：\n")
		sb.WriteString(desc)
	}
	displayTitle := title
	if displayTitle == "" && desc != "" {
		displayTitle = store.TruncateRunes(desc, 48)
	}
	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         store.StashKindXiaohongshu,
		Title:        displayTitle,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
		SuggestedTag: tags,
	}
}

func (e *Engine) extractXiaohongshuSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	resolvedURL := targetURL
	var lastErr error
	for _, ua := range []string{desktopBrowserUA, mobileBrowserUA} {
		sc, finalURL, _, body, err := e.fetchWebResource(ctx, targetURL, "", map[string]string{
			"User-Agent": ua,
			"Referer":    "https://www.xiaohongshu.com/",
		})
		if err != nil {
			lastErr = err
			continue
		}
		if finalURL != "" {
			resolvedURL = finalURL
		}
		if sc >= 200 && sc < 300 {
			normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
			if !ok {
				normURL = targetURL
				dom = "xiaohongshu.com"
			}
			if parsed := parseXiaohongshuHTML(body, normURL, dom); parsed != nil {
				return parsed, nil
			}
		} else {
			lastErr = fmt.Errorf("小红书页面返回 HTTP %d", sc)
		}
	}

	normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
	if !ok {
		normURL = targetURL
		dom = "xiaohongshu.com"
	}
	if jText := e.fetchJinaReaderFallback(ctx, resolvedURL); jText != "" {
		return &extractedStashSource{
			CanonicalURL: normURL,
			Domain:       dom,
			Kind:         store.StashKindXiaohongshu,
			Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
			SuggestedTag: []string{"小红书"},
		}, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("小红书需要登录态或访问受限")
}

func parseDouyinHTML(rawHTML, normURL, dom string) *extractedStashSource {
	if strings.TrimSpace(rawHTML) == "" {
		return nil
	}
	var desc, author, musicTitle string
	tags := []string{"抖音"}

	if routerJSON := extractAssignedJSONObject(rawHTML, "window._ROUTER_DATA =", "window._ROUTER_DATA="); routerJSON != "" {
		var root struct {
			LoaderData map[string]json.RawMessage `json:"loaderData"`
		}
		if json.Unmarshal([]byte(routerJSON), &root) == nil {
			for _, rawVal := range root.LoaderData {
				var page struct {
					VideoInfoRes struct {
						ItemList []struct {
							Desc   string `json:"desc"`
							Author struct {
								Nickname string `json:"nickname"`
							} `json:"author"`
							Music struct {
								Title string `json:"title"`
							} `json:"music"`
							TextExtra []struct {
								HashtagName string `json:"hashtag_name"`
							} `json:"text_extra"`
						} `json:"item_list"`
					} `json:"videoInfoRes"`
				}
				if json.Unmarshal(rawVal, &page) == nil && len(page.VideoInfoRes.ItemList) > 0 {
					it := page.VideoInfoRes.ItemList[0]
					desc = notifier.StripEmoji(strings.TrimSpace(it.Desc))
					author = notifier.StripEmoji(strings.TrimSpace(it.Author.Nickname))
					musicTitle = notifier.StripEmoji(strings.TrimSpace(it.Music.Title))
					for i, te := range it.TextExtra {
						if i >= 4 {
							break
						}
						ht := notifier.StripEmoji(strings.TrimSpace(te.HashtagName))
						if ht != "" {
							tags = append(tags, ht)
						}
					}
					break
				}
			}
		}
	}

	title := ""
	if desc != "" {
		title = store.TruncateRunes(desc, 64)
	}
	if title == "" {
		t := strings.TrimSpace(strings.TrimSuffix(extractHTMLTitle(rawHTML), " - 抖音"))
		if t != "" && !isBlockedOrLoginWall(t) && t != "抖音" && t != "抖音-记录美好生活" {
			title = t
		}
	}
	if desc == "" {
		d := extractHTMLMetaDesc(rawHTML)
		if d != "" && !isBlockedOrLoginWall(d) && !strings.Contains(d, "抖音为您提供短视频") {
			desc = d
		}
	}
	if title == "" && desc == "" {
		return nil
	}

	var sb strings.Builder
	if author != "" {
		sb.WriteString(fmt.Sprintf("作者：%s\n", author))
	}
	if desc != "" {
		sb.WriteString("视频文案：\n")
		sb.WriteString(desc)
		sb.WriteString("\n")
	}
	if musicTitle != "" {
		sb.WriteString(fmt.Sprintf("配乐：%s\n", musicTitle))
	}
	displayTitle := title
	if author != "" && displayTitle != "" && !strings.Contains(displayTitle, author) {
		displayTitle = fmt.Sprintf("[%s] %s", author, displayTitle)
	}
	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         store.StashKindDouyin,
		Title:        displayTitle,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
		SuggestedTag: tags,
	}
}

func (e *Engine) extractDouyinSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	resolvedURL := targetURL
	sc, finalURL, _, body, err := e.fetchWebResource(ctx, targetURL, "", map[string]string{
		"User-Agent": mobileBrowserUA,
		"Referer":    "https://www.douyin.com/",
	})
	if err == nil {
		if finalURL != "" {
			resolvedURL = finalURL
		}
		normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
		if !ok {
			normURL = targetURL
			dom = "douyin.com"
		}
		if sc >= 200 && sc < 300 {
			if parsed := parseDouyinHTML(body, normURL, dom); parsed != nil {
				return parsed, nil
			}
		}
		// If redirected to desktop douyin.com/video/<id>, try public mobile share endpoint iesdouyin.com/share/video/<id>/
		if !e.allowLocalhostForTest {
			if m := reDouyinVideoID.FindStringSubmatch(resolvedURL); len(m) > 1 {
				shareURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s/", m[1])
				if !strings.EqualFold(shareURL, resolvedURL) {
					sCtx, sCancel := context.WithTimeout(ctx, 8*time.Second)
					ssc, _, _, sBody, sErr := e.fetchWebResource(sCtx, shareURL, "", map[string]string{
						"User-Agent": mobileBrowserUA,
						"Referer":    "https://www.iesdouyin.com/",
					})
					sCancel()
					if sErr == nil && ssc >= 200 && ssc < 300 {
						if parsed := parseDouyinHTML(sBody, normURL, dom); parsed != nil {
							return parsed, nil
						}
					}
				}
			}
		}
	}

	normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
	if !ok {
		normURL = targetURL
		dom = "douyin.com"
	}
	if jText := e.fetchJinaReaderFallback(ctx, resolvedURL); jText != "" {
		return &extractedStashSource{
			CanonicalURL: normURL,
			Domain:       dom,
			Kind:         store.StashKindDouyin,
			Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
			SuggestedTag: []string{"抖音"},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("抖音分享页需要验证或访问受限")
}

func (e *Engine) extractWeChatSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	sc, finalURL, _, rawBody, err := e.fetchWebResource(ctx, targetURL, "", map[string]string{
		"User-Agent": mobileBrowserUA,
		"Referer":    "https://mp.weixin.qq.com/",
	})
	useURL := targetURL
	if finalURL != "" {
		useURL = finalURL
	}
	normURL, dom, _, ok := store.NormalizeStashURL(useURL)
	if !ok {
		normURL = targetURL
		dom = "mp.weixin.qq.com"
	}

	if err == nil && sc >= 200 && sc < 300 {
		title := extractHTMLTitle(rawBody)
		desc := extractHTMLMetaDesc(rawBody)
		author := ""
		if m := reWeChatNick.FindStringSubmatch(rawBody); len(m) > 1 {
			author = notifier.StripEmoji(strings.TrimSpace(m[1]))
			if author == "" && len(m) > 2 {
				author = notifier.StripEmoji(strings.TrimSpace(m[2]))
			}
		}
		text := stripHTMLToText(rawBody)
		if !isBlockedOrLoginWall(text) && (len([]rune(text)) >= 100 || title != "" || desc != "") {
			var sb strings.Builder
			if author != "" {
				sb.WriteString(fmt.Sprintf("公众号：%s\n", author))
			}
			if desc != "" && !strings.Contains(text, desc) {
				sb.WriteString(desc + "\n\n")
			}
			sb.WriteString(text)
			return &extractedStashSource{
				CanonicalURL: normURL,
				Domain:       dom,
				Kind:         store.StashKindWeChat,
				Title:        title,
				Description:  desc,
				Excerpt:      store.TruncateRunes(strings.TrimSpace(sb.String()), store.MaxStashExcerptRunes),
				SuggestedTag: []string{"公众号"},
			}, nil
		}
	}

	if jText := e.fetchJinaReaderFallback(ctx, useURL); jText != "" {
		return &extractedStashSource{
			CanonicalURL: normURL,
			Domain:       dom,
			Kind:         store.StashKindWeChat,
			Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
			SuggestedTag: []string{"公众号"},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("公众号文章返回 HTTP %d 或需要验证", sc)
}

func (e *Engine) extractGenericWebSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	sc, finalURL, contentType, rawBody, err := e.fetchWebResource(ctx, targetURL, "", nil)
	if err != nil {
		if jText := e.fetchJinaReaderFallback(ctx, targetURL); jText != "" {
			normURL, dom, kind, ok := store.NormalizeStashURL(targetURL)
			if !ok {
				normURL = targetURL
			}
			return &extractedStashSource{
				CanonicalURL: normURL,
				Domain:       dom,
				Kind:         kind,
				Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
			}, nil
		}
		return nil, err
	}
	if finalURL != "" && finalURL != targetURL {
		if bvid, aid := extractBiliVideoIDs(finalURL); bvid != "" || aid != "" {
			if biliSrc, bErr := e.extractBilibiliSource(ctx, finalURL); bErr == nil && biliSrc != nil {
				return biliSrc, nil
			}
		}
		if _, _, redirectedKind, ok := store.NormalizeStashURL(finalURL); ok {
			switch redirectedKind {
			case store.StashKindBilibili:
				if biliSrc, bErr := e.extractBilibiliSource(ctx, finalURL); bErr == nil && biliSrc != nil {
					return biliSrc, nil
				}
			case store.StashKindXiaohongshu:
				if xhsSrc, xErr := e.extractXiaohongshuSource(ctx, finalURL); xErr == nil && xhsSrc != nil {
					return xhsSrc, nil
				}
			case store.StashKindDouyin:
				if dySrc, dErr := e.extractDouyinSource(ctx, finalURL); dErr == nil && dySrc != nil {
					return dySrc, nil
				}
			case store.StashKindWeChat:
				if wxSrc, wErr := e.extractWeChatSource(ctx, finalURL); wErr == nil && wxSrc != nil {
					return wxSrc, nil
				}
			}
		}
	}

	useURL := targetURL
	if finalURL != "" {
		useURL = finalURL
	}
	normURL, dom, kind, ok := store.NormalizeStashURL(useURL)
	if !ok {
		normURL = targetURL
	}

	if sc >= 400 {
		// Rescue 403/412/429/5xx pages via Jina Reader before failing
		if jText := e.fetchJinaReaderFallback(ctx, useURL); jText != "" {
			return &extractedStashSource{
				CanonicalURL: normURL,
				Domain:       dom,
				Kind:         kind,
				Excerpt:      store.TruncateRunes(jText, store.MaxStashExcerptRunes),
			}, nil
		}
		return nil, fmt.Errorf("目标页面返回 HTTP %d", sc)
	}

	var title, desc, text string
	if strings.Contains(contentType, "json") || strings.Contains(contentType, "plain") {
		text = notifier.StripEmoji(strings.TrimSpace(rawBody))
	} else {
		title = extractHTMLTitle(rawBody)
		desc = extractHTMLMetaDesc(rawBody)
		text = stripHTMLToText(rawBody)
	}

	// CSR/SPA or anti-bot fallback via Jina Reader in production when HTML body has minimal text
	if len([]rune(text)) < 160 || isBlockedOrLoginWall(text) {
		if jClean := e.fetchJinaReaderFallback(ctx, useURL); len([]rune(jClean)) > len([]rune(text)) {
			text = jClean
		}
	}

	combined := text
	if desc != "" && !strings.Contains(text, desc) {
		combined = desc + "\n\n" + text
	}

	return &extractedStashSource{
		CanonicalURL: normURL,
		Domain:       dom,
		Kind:         kind,
		Title:        title,
		Description:  desc,
		Excerpt:      store.TruncateRunes(strings.TrimSpace(combined), store.MaxStashExcerptRunes),
	}, nil
}
