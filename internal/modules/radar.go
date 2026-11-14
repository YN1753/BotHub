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
	return &RadarModule{store: st, push: push, http: NewHTTPClient(20 * time.Second)}
}

func (r *RadarModule) ID() string          { return "radar" }
func (r *RadarModule) Name() string        { return "全网开源技术雷达" }
func (r *RadarModule) Description() string { return "抓取 GitHub Trending 与 Hacker News 热门条目，去重推送" }

type ghRepo struct {
	Name, URL, Desc, Language, StarsToday string
}

type hnStory struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Score int    `json:"score"`
	Type  string `json:"type"`
}

func (r *RadarModule) Run(ctx context.Context) (RunResult, error) {
	limit := 8
	if v, _ := r.store.GetConfig(store.KeyRadarLimit); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	minScore := 120
	if v, _ := r.store.GetConfig(store.KeyRadarHNMinScore); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			minScore = n
		}
	}

	var ghRepos []ghRepo
	var hnItems []hnStory
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ghRepos, _ = r.fetchGitHubTrending(ctx, limit*2)
	}()
	go func() {
		defer wg.Done()
		hnItems, _ = r.fetchHackerNews(ctx, minScore, limit*2)
	}()
	wg.Wait()

	var freshGH []ghRepo
	for _, repo := range ghRepos {
		if seen, _ := r.store.IsSeen("github_trending", repo.Name); !seen {
			freshGH = append(freshGH, repo)
			if len(freshGH) >= limit {
				break
			}
		}
	}

	var freshHN []hnStory
	for _, st := range hnItems {
		if seen, _ := r.store.IsSeen("hackernews", strconv.FormatInt(st.ID, 10)); !seen {
			freshHN = append(freshHN, st)
			if len(freshHN) >= limit {
				break
			}
		}
	}

	totalNew := len(freshGH) + len(freshHN)
	if totalNew == 0 {
		return RunResult{Summary: "抓取完成，暂无新增条目", ItemsPushed: 0}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 开源技术雷达 · %s\n\n", time.Now().In(ShanghaiLocation()).Format("2006-01-02 15:04")))

	if len(freshGH) > 0 {
		sb.WriteString("#### 1. GitHub Trending 每日热门\n")
		for i, repo := range freshGH {
			lang := repo.Language
			if lang == "" { lang = "Multi" }
			if repo.StarsToday != "" { lang += " · " + repo.StarsToday }
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%s）\n   %s\n", i+1, repo.Name, repo.URL, lang, repo.Desc))
		}
		sb.WriteString("\n")
	}

	if len(freshHN) > 0 {
		sb.WriteString("#### 2. Hacker News 精选高分\n")
		for i, st := range freshHN {
			target := st.URL
			if target == "" {
				target = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", st.ID)
			}
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%d pts · [讨论](https://news.ycombinator.com/item?id=%d)）\n", i+1, st.Title, target, st.Score, st.ID))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n*来自 Vesper BotHub · bot.starryheavens.uk*")
	title := fmt.Sprintf("技术雷达 · 新增 %d 条精选（GH %d / HN %d）", totalNew, len(freshGH), len(freshHN))

	if err := r.push.SendMarkdown(ctx, r.ID(), title, sb.String()); err != nil {
		return RunResult{Summary: "推送失败: " + err.Error()}, err
	}

	toMark := make([]store.SeenItem, 0, len(freshGH)+len(freshHN))
	for _, repo := range freshGH {
		toMark = append(toMark, store.SeenItem{
			Source:  "github_trending",
			ItemKey: repo.Name,
			Title:   repo.Name,
			URL:     repo.URL,
		})
	}
	for _, st := range freshHN {
		target := st.URL
		if target == "" { target = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", st.ID) }
		toMark = append(toMark, store.SeenItem{
			Source:  "hackernews",
			ItemKey: strconv.FormatInt(st.ID, 10),
			Title:   st.Title,
			URL:     target,
		})
	}
	_ = r.store.BatchMarkSeen(toMark)

	return RunResult{Summary: fmt.Sprintf("已推送 %d 条精选（GitHub %d, HN %d）", totalNew, len(freshGH), len(freshHN)), ItemsPushed: totalNew}, nil
}

func (r *RadarModule) fetchGitHubTrending(ctx context.Context, maxItems int) ([]ghRepo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/trending?since=daily", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var repos []ghRepo
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(repos) >= maxItems { return }
		if n.Type == html.ElementNode && n.Data == "article" {
			for _, a := range n.Attr {
				if a.Key == "class" && strings.Contains(a.Val, "Box-row") {
					if repo := parseTrending(n); repo.Name != "" {
						repos = append(repos, repo)
					}
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling { walk(c) }
	}
	walk(doc)
	return repos, nil
}

func parseTrending(article *html.Node) ghRepo {
	var repo ghRepo
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h2":
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "a" {
						for _, a := range c.Attr {
							if a.Key == "href" {
								repo.Name = strings.TrimPrefix(strings.TrimSpace(a.Val), "/")
								repo.URL = "https://github.com/" + repo.Name
							}
						}
					}
				}
			case "p":
				if repo.Desc == "" { repo.Desc = strings.Join(strings.Fields(extractText(n)), " ") }
			case "span":
				for _, a := range n.Attr {
					if a.Key == "itemprop" && a.Val == "programmingLanguage" {
						repo.Language = strings.TrimSpace(extractText(n))
					} else if a.Key == "class" && strings.Contains(a.Val, "float-sm-right") {
						txt := strings.TrimSpace(extractText(n))
						if strings.Contains(txt, "star") { repo.StarsToday = txt }
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling { walk(c) }
	}
	walk(article)
	return repo
}

func (r *RadarModule) fetchHackerNews(ctx context.Context, minScore, maxItems int) ([]hnStory, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://hacker-news.firebaseio.com/v0/topstories.json", nil)
	resp, err := r.http.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	var ids []int64
	_ = json.NewDecoder(resp.Body).Decode(&ids)
	if len(ids) > 30 { ids = ids[:30] }

	var mu sync.Mutex
	var stories []hnStory
	var wg sync.WaitGroup
	// Concurrency limiter: keep simultaneous outgoing requests capped at 6 to prevent burst socket load
	sem := make(chan struct{}, 6)

	for _, id := range ids {
		wg.Add(1)
		go func(storyID int64) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			u := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", storyID)
			sReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if sResp, sErr := r.http.Do(sReq); sErr == nil {
				defer sResp.Body.Close()
				var st hnStory
				if json.NewDecoder(sResp.Body).Decode(&st) == nil && st.Type == "story" && st.Score >= minScore {
					mu.Lock()
					stories = append(stories, st)
					mu.Unlock()
				}
			}
		}(id)
	}
	wg.Wait()

	sort.Slice(stories, func(i, j int) bool { return stories[i].Score > stories[j].Score })
	if len(stories) > maxItems { stories = stories[:maxItems] }
	return stories, nil
}

func extractText(n *html.Node) string {
	if n.Type == html.TextNode { return n.Data }
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling { b.WriteString(extractText(c)) }
	return b.String()
}
