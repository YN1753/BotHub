package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	reBiliBVID = regexp.MustCompile(`(?i)(BV[0-9A-Za-z]{10})`)
	reBiliAID  = regexp.MustCompile(`(?i)(?:/video/av|(?:\?|&)aid=)(\d+)`)
	reBullet   = regexp.MustCompile(`^\s*(?:[-*•]+|\d+[.)、])\s*`)
)

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

func (e *Engine) stashSemaphore() chan struct{} {
	if e.stashSem != nil {
		return e.stashSem
	}
	return make(chan struct{}, 3)
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

// CaptureAndDistillStash persists a newly captured link/share/thought immediately (<50ms) and schedules background distillation.
func (e *Engine) CaptureAndDistillStash(rawInput, userNote, tags string, forceReprocess bool) (store.StashItem, bool, error) {
	cleanRaw := notifier.StripEmoji(strings.TrimSpace(rawInput))
	cleanNote := notifier.StripEmoji(strings.TrimSpace(userNote))
	cleanTags := notifier.StripEmoji(strings.TrimSpace(tags))
	if cleanRaw == "" {
		return store.StashItem{}, false, fmt.Errorf("收藏或闪念内容不能为空")
	}

	item, deduplicated, shouldDistill, err := e.store.CaptureStashItem(cleanRaw, cleanNote, cleanTags, forceReprocess)
	if err != nil {
		return store.StashItem{}, false, err
	}
	e.publishAssistantUpdate("stash")
	if shouldDistill {
		e.DistillStashAsync(item.ID)
	}
	return item, deduplicated, nil
}

// ReprocessStashItem resets an existing stash item to processing and launches background source extraction + AI distillation.
func (e *Engine) ReprocessStashItem(id string) (*store.StashItem, error) {
	existing, err := e.store.GetStashItem(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	existing.Status = store.StashStatusProcessing
	existing.ErrorMsg = ""
	existing.UpdatedAt = store.NowShanghai()
	saved, err := e.store.UpsertStashItemFull(*existing)
	if err != nil {
		return nil, err
	}
	e.publishAssistantUpdate("stash")
	e.DistillStashAsync(saved.ID)
	return &saved, nil
}

// DistillStashAsync runs source extraction and AI summarization on a bounded background worker pool.
func (e *Engine) DistillStashAsync(id string) {
	if e == nil || e.store == nil || strings.TrimSpace(id) == "" {
		return
	}
	sem := e.stashSemaphore()
	go func(stashID string) {
		sem <- struct{}{}
		defer func() { <-sem }()

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = e.DistillStashNow(ctx, stashID)
	}(strings.TrimSpace(id))
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

func (e *Engine) fetchStashHTTP(ctx context.Context, targetURL, accept string, headers map[string]string) (int, string, string, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 14*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, "", "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VesperBotHub/2.32)")
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
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return resp.StatusCode, finalURL, "", "", err
	}
	return resp.StatusCode, finalURL, strings.ToLower(resp.Header.Get("Content-Type")), string(bodyBytes), nil
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

func (e *Engine) extractBilibiliSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	resolvedURL := targetURL
	bvid, aid := extractBiliVideoIDs(resolvedURL)

	// Follow b23.tv or short-link redirects first when BV/av ID is not yet visible
	if bvid == "" && aid == "" {
		_, finalURL, _, htmlBody, err := e.fetchStashHTTP(ctx, targetURL, "", map[string]string{
			"Referer": "https://www.bilibili.com/",
		})
		if err != nil {
			return nil, err
		}
		if finalURL != "" {
			resolvedURL = finalURL
		}
		bvid, aid = extractBiliVideoIDs(resolvedURL)
		if bvid == "" && aid == "" {
			// Non-video Bilibili page (e.g. bilibili.com/read/cv...): parse HTML directly
			normURL, dom, _, ok := store.NormalizeStashURL(resolvedURL)
			if !ok {
				normURL = targetURL
				dom = "bilibili.com"
			}
			title := extractHTMLTitle(htmlBody)
			desc := extractHTMLMetaDesc(htmlBody)
			text := stripHTMLToText(htmlBody)
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

	sessCfg, _ := e.store.GetConfig(store.KeyBiliSessdata)
	sessdata := sanitizeBiliSessdata(sessCfg)
	biliHeaders := map[string]string{
		"Referer": "https://www.bilibili.com/",
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
	sc, _, _, viewRaw, err := e.fetchStashHTTP(ctx, viewAPI, "application/json", biliHeaders)
	if err != nil || sc < 200 || sc >= 300 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Bilibili 接口响应 HTTP %d", sc)
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
	if err := json.Unmarshal([]byte(viewRaw), &viewResp); err != nil || viewResp.Code != 0 {
		if viewResp.Message != "" {
			return nil, fmt.Errorf("Bilibili 视频元数据返回异常: %s", viewResp.Message)
		}
		return nil, fmt.Errorf("解析 Bilibili 视频元数据失败")
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
	if tsc, _, _, tagBody, tErr := e.fetchStashHTTP(tagCtx, tagAPI, "application/json", biliHeaders); tErr == nil && tsc == 200 {
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
		if psc, _, _, pBody, pErr := e.fetchStashHTTP(pCtx, playerAPI, "application/json", biliHeaders); pErr == nil && psc == 200 {
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
			if ssc, _, _, sBody, fErr := e.fetchStashHTTP(sCtx, subURL, "application/json", nil); fErr == nil && ssc == 200 {
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
	if sc, _, _, metaRaw, mErr := e.fetchStashHTTP(metaCtx, metaURL, "application/vnd.github+json", nil); mErr == nil && sc == 200 {
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
	rsc, _, _, readmeBody, rErr := e.fetchStashHTTP(ctx, readmeURL, "text/plain", nil)
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

func (e *Engine) extractGenericWebSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	sc, finalURL, contentType, rawBody, err := e.fetchStashHTTP(ctx, targetURL, "", nil)
	if err != nil {
		return nil, err
	}
	if finalURL != "" && finalURL != targetURL {
		if bvid, aid := extractBiliVideoIDs(finalURL); bvid != "" || aid != "" {
			if biliSrc, bErr := e.extractBilibiliSource(ctx, finalURL); bErr == nil && biliSrc != nil {
				return biliSrc, nil
			}
		}
	}
	if sc >= 400 {
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

	// CSR/SPA fallback via Jina Reader in production when HTML body has minimal text
	if len([]rune(text)) < 160 && !e.allowLocalhostForTest && !strings.Contains(strings.ToLower(targetURL), "r.jina.ai") {
		jinaCtx, jinaCancel := context.WithTimeout(ctx, 10*time.Second)
		if jsc, _, _, jBody, jErr := e.fetchStashHTTP(jinaCtx, "https://r.jina.ai/"+targetURL, "text/plain", nil); jErr == nil && jsc >= 200 && jsc < 300 {
			jClean := notifier.StripEmoji(strings.TrimSpace(jBody))
			if len([]rune(jClean)) > len([]rune(text)) {
				text = jClean
			}
		}
		jinaCancel()
	}

	useURL := targetURL
	if finalURL != "" {
		useURL = finalURL
	}
	normURL, dom, kind, ok := store.NormalizeStashURL(useURL)
	if !ok {
		normURL = targetURL
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

type aiDistillPayload struct {
	Title  string   `json:"title"`
	TLDR   string   `json:"tldr"`
	Points []string `json:"points"`
	Tags   []string `json:"tags"`
}

func parseAIDistillOutput(raw string) (*aiDistillPayload, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("AI 返回为空")
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	if start := strings.IndexByte(s, '{'); start >= 0 {
		if end := strings.LastIndexByte(s, '}'); end > start {
			s = s[start : end+1]
		}
	}
	s = reTrailingJSONComma.ReplaceAllString(s, "$1")

	var out aiDistillPayload
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}

	out.Title = store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(out.Title)), 120)
	out.TLDR = store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(out.TLDR)), 70)

	cleanPts := make([]string, 0, len(out.Points))
	for _, p := range out.Points {
		cp := notifier.StripEmoji(strings.TrimSpace(reBullet.ReplaceAllString(p, "")))
		if cp != "" {
			cleanPts = append(cleanPts, store.TruncateRunes(cp, 180))
		}
		if len(cleanPts) >= 5 {
			break
		}
	}
	out.Points = cleanPts

	cleanTags := make([]string, 0, len(out.Tags))
	for _, t := range out.Tags {
		ct := notifier.StripEmoji(strings.TrimSpace(strings.TrimPrefix(t, "#")))
		if ct != "" {
			cleanTags = append(cleanTags, store.TruncateRunes(ct, 16))
		}
		if len(cleanTags) >= 4 {
			break
		}
	}
	out.Tags = cleanTags
	return &out, nil
}

func buildLocalFallbackSummary(title, desc, excerpt string) (string, []string) {
	base := strings.TrimSpace(desc)
	if base == "" {
		base = strings.TrimSpace(excerpt)
	}
	lines := strings.Split(base, "\n")
	points := make([]string, 0, 4)
	for _, line := range lines {
		clean := notifier.StripEmoji(strings.TrimSpace(reBullet.ReplaceAllString(line, "")))
		if len([]rune(clean)) < 6 {
			continue
		}
		points = append(points, store.TruncateRunes(clean, 140))
		if len(points) >= 4 {
			break
		}
	}
	tldr := ""
	if len(points) > 0 {
		tldr = store.TruncateRunes(points[0], 70)
	} else if title != "" {
		tldr = store.TruncateRunes(title, 70)
	}
	return tldr, points
}

// DistillStashNow executes synchronous source extraction + lightweight AI summarization for a stash item.
func (e *Engine) DistillStashNow(ctx context.Context, id string) error {
	item, err := e.store.GetStashItem(id)
	if err != nil || item == nil {
		return err
	}

	var src *extractedStashSource
	var fetchErr error

	if item.Kind == store.StashKindNote || strings.TrimSpace(item.URL) == "" {
		src = &extractedStashSource{
			Kind:         store.StashKindNote,
			Title:        item.Title,
			Excerpt:      store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(item.RawInput)), store.MaxStashExcerptRunes),
			SuggestedTag: []string{"闪念"},
		}
	} else {
		switch item.Kind {
		case store.StashKindBilibili:
			src, fetchErr = e.extractBilibiliSource(ctx, item.URL)
		case store.StashKindGitHub:
			src, fetchErr = e.extractGitHubSource(ctx, item.URL)
		default:
			src, fetchErr = e.extractGenericWebSource(ctx, item.URL)
		}
	}

	if src != nil {
		if src.CanonicalURL != "" {
			item.URL = src.CanonicalURL
		}
		if src.Domain != "" {
			item.Domain = src.Domain
		}
		if src.Kind != "" {
			item.Kind = src.Kind
		}
		if src.Title != "" {
			item.Title = notifier.StripEmoji(strings.TrimSpace(src.Title))
		}
		if src.Excerpt != "" {
			item.ContentExcerpt = store.TruncateRunes(notifier.StripEmoji(src.Excerpt), store.MaxStashExcerptRunes)
		}
		if len(src.SuggestedTag) > 0 {
			item.Tags = store.MergeStashTags(item.Tags, strings.Join(src.SuggestedTag, ","))
		}
	}

	// Build context for AI distillation (even if fetch partially failed, check if raw_input/title/note has enough substance)
	material := strings.TrimSpace(item.ContentExcerpt)
	if material == "" && item.Kind == store.StashKindNote {
		material = strings.TrimSpace(item.RawInput)
	}

	if fetchErr != nil && material == "" {
		// External crawl blocked or failed: preserve original candidate title, URL, and user_note cleanly without losing data
		if item.Title == "" {
			if item.Domain != "" {
				item.Title = item.Domain
			} else {
				item.Title = store.TruncateRunes(item.RawInput, 60)
			}
		}
		item.Status = store.StashStatusFailed
		item.ErrorMsg = notifier.StripEmoji(fmt.Sprintf("抓取受限，已保留原始链接与批注 (%v)", fetchErr))
		item.UpdatedAt = store.NowShanghai()
		_, uErr := e.store.UpsertStashItemFull(*item)
		e.publishAssistantUpdate("stash")
		return uErr
	}

	if e.hasLightTarget() {
		sysPrompt := `你是 Vesper BotHub 智能收藏与闪念蒸馏器。请阅读输入材料并输出严格合法的 JSON 对象（不要使用 Markdown 代码块包裹，不要输出任何额外说明），格式如下：
{"title":"简明准确的标题（不超过48字）","tldr":"一句话核心结论（不超过70字）","points":["核心干货要点1","核心干货要点2","核心干货要点3"],"tags":["标签1","标签2"]}
严格规范：
1. tldr 必须控制在 70 字以内，直击本质结论、核心方案或关键洞察，拒绝空话。
2. points 数组提炼 3 到 5 条高信息密度干货要点（若输入是很短的随手闪念，可提炼 2 到 3 条关键行动或延伸要点）。
3. tags 数组给出 2 到 4 个简短中文或技术分类标签（不含井号）。
4. 绝对禁止在任何字段中使用 Emoji 表情或装饰性符号。`

		var userPrompt strings.Builder
		userPrompt.WriteString(fmt.Sprintf("来源类型：%s\n", item.Kind))
		if item.URL != "" {
			userPrompt.WriteString(fmt.Sprintf("原始链接：%s\n", item.URL))
		}
		if item.Title != "" {
			userPrompt.WriteString(fmt.Sprintf("候选标题：%s\n", item.Title))
		}
		if item.UserNote != "" {
			userPrompt.WriteString(fmt.Sprintf("用户批注：%s\n", item.UserNote))
		}
		userPrompt.WriteString("\n正文材料：\n")
		userPrompt.WriteString(store.TruncateRunes(material, 7500))

		rawAI, aiErr := e.CallLightModel(ctx, sysPrompt, userPrompt.String())
		if aiErr == nil {
			if parsed, pErr := parseAIDistillOutput(rawAI); pErr == nil {
				if parsed.Title != "" && (item.Kind == store.StashKindNote || item.Title == "" || item.Title == item.Domain) {
					item.Title = parsed.Title
				} else if item.Title == "" && parsed.Title != "" {
					item.Title = parsed.Title
				}
				if parsed.TLDR != "" {
					item.AITldr = parsed.TLDR
				}
				if len(parsed.Points) > 0 {
					item.Points = parsed.Points
				}
				if len(parsed.Tags) > 0 {
					item.Tags = store.MergeStashTags(item.Tags, strings.Join(parsed.Tags, ","))
				}
				item.Status = store.StashStatusReady
				item.ErrorMsg = ""
				item.UpdatedAt = store.NowShanghai()
				_, uErr := e.store.UpsertStashItemFull(*item)
				e.publishAssistantUpdate("stash")
				return uErr
			}
		}
	}

	// Graceful fallback when AI key is unconfigured or model call failed: synthesize local excerpt summary so item remains usable
	desc := ""
	if src != nil {
		desc = src.Description
	}
	fallbackTLDR, fallbackPts := buildLocalFallbackSummary(item.Title, desc, material)
	if item.Title == "" {
		item.Title = store.TruncateRunes(material, 48)
	}
	if item.AITldr == "" {
		item.AITldr = fallbackTLDR
	}
	if len(item.Points) == 0 && len(fallbackPts) > 0 {
		item.Points = fallbackPts
	}
	item.Status = store.StashStatusReady
	if !e.hasLightTarget() {
		item.ErrorMsg = "未配置 AI 模型密钥，已基于抓取快照生成本地摘要"
	} else {
		item.ErrorMsg = "AI 模型暂时繁忙，已保留抓取快照与本地摘要，可稍后重提炼"
	}
	item.UpdatedAt = store.NowShanghai()
	_, uErr := e.store.UpsertStashItemFull(*item)
	e.publishAssistantUpdate("stash")
	return uErr
}

// toolManageStash implements the built-in manage_stash agent tool (capture | search | get | update | delete).
func (e *Engine) toolManageStash(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action         string `json:"action"`
		ID             string `json:"id"`
		RawInput       string `json:"raw_input"`
		URL            string `json:"url"`
		Title          string `json:"title"`
		UserNote       string `json:"user_note"`
		Query          string `json:"query"`
		Filter         string `json:"filter"`
		Tag            string `json:"tag"`
		Kind           string `json:"kind"`
		Tags           string `json:"tags"`
		Status         string `json:"status"`
		Pinned         *bool  `json:"pinned"`
		MarkRead       *bool  `json:"mark_read"`
		ForceReprocess bool   `json:"force_reprocess"`
		Limit          int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_stash 参数失败: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "search"
	}

	switch action {
	case "capture", "add":
		rawInput := strings.TrimSpace(args.RawInput)
		if rawInput == "" {
			if strings.TrimSpace(args.URL) != "" {
				if strings.TrimSpace(args.Title) != "" {
					rawInput = strings.TrimSpace(args.Title) + " " + strings.TrimSpace(args.URL)
				} else {
					rawInput = strings.TrimSpace(args.URL)
				}
			} else if strings.TrimSpace(args.Title) != "" {
				rawInput = strings.TrimSpace(args.Title)
			}
		}
		if rawInput == "" {
			return "", fmt.Errorf("capture 收藏或闪念时 raw_input（或 url）不能为空")
		}
		saved, dedup, err := e.CaptureAndDistillStash(rawInput, args.UserNote, args.Tags, args.ForceReprocess)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"deduplicated": dedup,
			"item":         saved,
			"note":         "已收入智能收藏与闪念匣，后台正异步抓取并提炼核心要点",
		})
		return string(out), nil

	case "search", "list":
		limit := args.Limit
		if limit <= 0 || limit > 50 {
			limit = 20
		}
		filter := strings.TrimSpace(args.Filter)
		if filter == "" && strings.TrimSpace(args.Status) != "" {
			filter = strings.TrimSpace(args.Status)
		}
		items, err := e.store.SearchStashItemsWithKind(args.Query, filter, args.Tag, args.Kind, limit)
		if err != nil {
			return "", err
		}
		type compactItem struct {
			ID        string   `json:"id"`
			Kind      string   `json:"kind"`
			Title     string   `json:"title"`
			URL       string   `json:"url,omitempty"`
			AITldr    string   `json:"ai_tldr,omitempty"`
			Points    []string `json:"points,omitempty"`
			Tags      string   `json:"tags,omitempty"`
			UserNote  string   `json:"user_note,omitempty"`
			Status    string   `json:"status"`
			Pinned    bool     `json:"pinned"`
			ReadCount int      `json:"read_count"`
			CreatedAt string   `json:"created_at"`
		}
		summaries := make([]compactItem, 0, len(items))
		for _, it := range items {
			summaries = append(summaries, compactItem{
				ID:        it.ID,
				Kind:      it.Kind,
				Title:     it.Title,
				URL:       it.URL,
				AITldr:    it.AITldr,
				Points:    it.Points,
				Tags:      it.Tags,
				UserNote:  it.UserNote,
				Status:    it.Status,
				Pinned:    it.Pinned,
				ReadCount: it.ReadCount,
				CreatedAt: it.CreatedAt,
			})
		}
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"count": len(summaries),
			"items": summaries,
		})
		return string(out), nil

	case "get":
		var target *store.StashItem
		var err error
		if id := strings.TrimSpace(args.ID); id != "" {
			target, err = e.store.GetStashItem(id)
			if err != nil {
				return "", err
			}
		} else if q := strings.TrimSpace(args.Query); q != "" {
			list, _ := e.store.SearchStashItems(q, "everything", "", 5)
			if len(list) > 0 {
				target = &list[0]
			}
		}
		if target == nil {
			return "", fmt.Errorf("未找到目标收藏条目（请提供准确 id 或通过 search 检索）")
		}
		out, _ := json.Marshal(map[string]any{"ok": true, "item": target})
		return string(out), nil

	case "update":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("update 收藏条目时 id 必填")
		}
		existing, err := e.store.GetStashItem(id)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return "", fmt.Errorf("未找到目标收藏条目: %s", id)
		}
		if args.MarkRead != nil && *args.MarkRead {
			existing.ReadCount++
			existing.LastReviewedAt = store.NowShanghai()
		}
		if t := notifier.StripEmoji(strings.TrimSpace(args.Title)); t != "" {
			existing.Title = t
		}
		if strings.TrimSpace(args.UserNote) != "" {
			existing.UserNote = notifier.StripEmoji(strings.TrimSpace(args.UserNote))
		}
		if strings.TrimSpace(args.Tags) != "" {
			existing.Tags = notifier.StripEmoji(strings.TrimSpace(args.Tags))
		}
		if args.Pinned != nil {
			existing.Pinned = *args.Pinned
		}
		if st := strings.TrimSpace(args.Status); st != "" {
			existing.Status = store.NormalizeStashStatus(st)
		}
		existing.UpdatedAt = store.NowShanghai()
		saved, err := e.store.UpsertStashItemFull(*existing)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("stash")
		out, _ := json.Marshal(map[string]any{"ok": true, "item": saved})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 收藏条目时 id 必填")
		}
		ok, err := e.store.DeleteStashItem(id)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("stash")
		out, _ := json.Marshal(map[string]any{"ok": ok, "deleted_id": id})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_stash action: %s", action)
	}
}
