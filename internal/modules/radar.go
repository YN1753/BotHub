package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/store"
	"golang.org/x/net/html"
)

type RadarModule struct {
	store *store.Store
	push  *pushplus.Client
	http  *http.Client
}

func NewRadarModule(st *store.Store, push *pushplus.Client) *RadarModule {
	return &RadarModule{
		store: st,
		push:  push,
		http:  NewHTTPClient(20 * time.Second),
	}
}

func (r *RadarModule) ID() string   { return "radar" }
func (r *RadarModule) Name() string { return "全网开源技术雷达" }
func (r *RadarModule) Description() string {
	return "抓取 GitHub Trending 与 Hacker News 高分条目，自动去重并推送精选摘要"
}

type ghRepo struct {
	Name       string
	URL        string
	Desc       string
	Language   string
	StarsToday string
}

type hnStory struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Score int    `json:"score"`
	By    string `json:"by"`
	Type  string `json:"type"`
}

func (r *RadarModule) Run(ctx context.Context) (RunResult, error) {
	limit := 8
	if v, _ := r.store.GetConfig(store.KeyRadarLimit); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 20 {
			limit = n
		}
	}
	minScore := 120
	if v, _ := r.store.GetConfig(store.KeyRadarHNMinScore); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			minScore = n
		}
	}

	var (
		ghRepos []ghRepo
		hnItems []hnStory
		ghErr   error
		hnErr   error
		wg      sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ghRepos, ghErr = r.fetchGitHubTrending(ctx, limit*2)
	}()
	go func() {
		defer wg.Done()
		hnItems, hnErr = r.fetchHackerNews(ctx, minScore, limit*2)
	}()
	wg.Wait()

	if ghErr != nil && hnErr != nil {
		return RunResult{Summary: fmt.Sprintf("抓取失败: GH(%v) / HN(%v)", ghErr, hnErr)}, fmt.Errorf("both radar sources failed: gh=%v, hn=%v", ghErr, hnErr)
	}

	var freshGH []ghRepo
	for _, repo := range ghRepos {
		seen, err := r.store.IsSeen("github_trending", repo.Name)
		if err == nil && !seen {
			freshGH = append(freshGH, repo)
			if len(freshGH) >= limit {
				break
			}
		}
	}

	var freshHN []hnStory
	for _, st := range hnItems {
		key := strconv.FormatInt(st.ID, 10)
		seen, err := r.store.IsSeen("hackernews", key)
		if err == nil && !seen {
			freshHN = append(freshHN, st)
			if len(freshHN) >= limit {
				break
			}
		}
	}

	totalNew := len(freshGH) + len(freshHN)
	if totalNew == 0 {
		return RunResult{
			Summary:     "抓取完成，暂无未推送的新增热门条目",
			ItemsPushed: 0,
		}, nil
	}

	nowStr := time.Now().In(ShanghaiLocation()).Format("2006-01-02 15:04")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 开源技术雷达 · %s\n\n", nowStr))

	if len(freshGH) > 0 {
		sb.WriteString("#### 1. GitHub Trending 每日热门\n")
		for i, repo := range freshGH {
			meta := repo.Language
			if meta == "" {
				meta = "Multi"
			}
			if repo.StarsToday != "" {
				meta += " · " + repo.StarsToday
			}
			desc := repo.Desc
			if desc == "" {
				desc = "暂无仓库简介"
			}
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%s）\n   %s\n", i+1, repo.Name, repo.URL, meta, desc))
		}
		sb.WriteString("\n")
	} else if ghErr != nil {
		sb.WriteString(fmt.Sprintf("#### 1. GitHub Trending\n- 抓取异常：%v\n\n", ghErr))
	}

	if len(freshHN) > 0 {
		sb.WriteString("#### 2. Hacker News 精选高分\n")
		for i, st := range freshHN {
			targetURL := st.URL
			if targetURL == "" {
				targetURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", st.ID)
			}
			hnDiscuss := fmt.Sprintf("https://news.ycombinator.com/item?id=%d", st.ID)
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%d pts · [讨论](%s)）\n", i+1, st.Title, targetURL, st.Score, hnDiscuss))
		}
		sb.WriteString("\n")
	} else if hnErr != nil {
		sb.WriteString(fmt.Sprintf("#### 2. Hacker News\n- 抓取异常：%v\n\n", hnErr))
	}

	sb.WriteString("---\n")
	sb.WriteString("*来自 Vesper BotHub · bot.starryheavens.uk*")

	title := fmt.Sprintf("技术雷达 · 新增 %d 条精选（GH %d / HN %d）", totalNew, len(freshGH), len(freshHN))
	if err := r.push.SendMarkdown(ctx, r.ID(), title, sb.String()); err != nil {
		return RunResult{Summary: "推送失败: " + err.Error()}, err
	}

	for _, repo := range freshGH {
		_ = r.store.MarkSeen("github_trending", repo.Name, repo.Name, repo.URL)
	}
	for _, st := range freshHN {
		targetURL := st.URL
		if targetURL == "" {
			targetURL = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", st.ID)
		}
		_ = r.store.MarkSeen("hackernews", strconv.FormatInt(st.ID, 10), st.Title, targetURL)
	}

	return RunResult{
		Summary:     fmt.Sprintf("已推送 %d 条新增技术雷达（GitHub %d，HN %d）", totalNew, len(freshGH), len(freshHN)),
		ItemsPushed: totalNew,
	}, nil
}

func (r *RadarModule) fetchGitHubTrending(ctx context.Context, maxItems int) ([]ghRepo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/trending?since=daily", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VesperBotHub/1.0)")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github trending http %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var repos []ghRepo
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(repos) >= maxItems {
			return
		}
		if n.Type == html.ElementNode && n.Data == "article" && hasClass(n, "Box-row") {
			if repo, ok := parseTrendingRow(n); ok {
				repos = append(repos, repo)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return repos, nil
}

func parseTrendingRow(article *html.Node) (ghRepo, bool) {
	var repo ghRepo
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h2":
				if a := findFirstTag(n, "a"); a != nil {
					href := getAttr(a, "href")
					name := strings.TrimPrefix(strings.TrimSpace(href), "/")
					if name != "" {
						repo.Name = name
						repo.URL = "https://github.com/" + name
					}
				}
			case "p":
				if repo.Desc == "" {
					repo.Desc = normalizeSpace(extractText(n))
				}
			case "span":
				if getAttr(n, "itemprop") == "programmingLanguage" {
					repo.Language = normalizeSpace(extractText(n))
				} else if hasClass(n, "float-sm-right") {
					txt := normalizeSpace(extractText(n))
					if strings.Contains(txt, "star") {
						repo.StarsToday = txt
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(article)
	return repo, repo.Name != ""
}

func (r *RadarModule) fetchHackerNews(ctx context.Context, minScore, maxItems int) ([]hnStory, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://hacker-news.firebaseio.com/v0/topstories.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}
	if len(ids) > 40 {
		ids = ids[:40]
	}

	type result struct {
		story hnStory
		ok    bool
	}
	jobs := make(chan int64, len(ids))
	results := make(chan result, len(ids))

	workers := 8
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				st, err := r.fetchHNItem(ctx, id)
				if err == nil && st.Type == "story" && st.Title != "" && st.Score >= minScore {
					results <- result{story: st, ok: true}
				} else {
					results <- result{ok: false}
				}
			}
		}()
	}

	for _, id := range ids {
		jobs <- id
	}
	close(jobs)
	wg.Wait()
	close(results)

	var stories []hnStory
	for res := range results {
		if res.ok {
			stories = append(stories, res.story)
		}
	}
	sort.Slice(stories, func(i, j int) bool {
		return stories[i].Score > stories[j].Score
	})
	if len(stories) > maxItems {
		stories = stories[:maxItems]
	}
	return stories, nil
}

func (r *RadarModule) fetchHNItem(ctx context.Context, id int64) (hnStory, error) {
	u := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return hnStory{}, err
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return hnStory{}, err
	}
	defer resp.Body.Close()
	var st hnStory
	err = json.NewDecoder(resp.Body).Decode(&st)
	return st, err
}

func hasClass(n *html.Node, cls string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, f := range strings.Fields(a.Val) {
				if f == cls {
					return true
				}
			}
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func findFirstTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstTag(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(extractText(c))
	}
	return b.String()
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
