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
	reBiliBVID = regexp.MustCompile(`(?i)(BV[0-9A-Za-z]{10})`)
	reBiliAID  = regexp.MustCompile(`(?i)(?:/video/av|(?:\?|&)aid=)(\d+)`)
)

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

func (e *Engine) extractBilibiliSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	resolvedURL := targetURL
	bvid, aid := extractBiliVideoIDs(resolvedURL)

	// Follow b23.tv or short-link redirects first when BV/av ID is not yet visible
	if bvid == "" && aid == "" {
		_, finalURL, _, htmlBody, err := e.fetchWebResource(ctx, targetURL, "", map[string]string{
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
	sc, _, _, viewRaw, err := e.fetchWebResource(ctx, viewAPI, "application/json", biliHeaders)
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

func (e *Engine) extractGenericWebSource(ctx context.Context, targetURL string) (*extractedStashSource, error) {
	sc, finalURL, contentType, rawBody, err := e.fetchWebResource(ctx, targetURL, "", nil)
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
		if jsc, _, _, jBody, jErr := e.fetchWebResource(jinaCtx, "https://r.jina.ai/"+targetURL, "text/plain", nil); jErr == nil && jsc >= 200 && jsc < 300 {
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
