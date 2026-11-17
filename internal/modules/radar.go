package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
	"golang.org/x/net/html"
)

type RadarModule struct {
	store      *store.Store
	push       *notifier.Client
	http       *http.Client
	mu         sync.RWMutex
	summarizer LightSummarizer
}

func NewRadarModule(st *store.Store, push *notifier.Client) *RadarModule {
	return &RadarModule{store: st, push: push, http: NewHTTPClient(20 * time.Second)}
}

func (r *RadarModule) SetSummarizer(sum LightSummarizer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summarizer = sum
}

func (r *RadarModule) getSummarizer() LightSummarizer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.summarizer
}

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

func (r *RadarModule) RunWithTask(ctx context.Context, task store.Task) (RunResult, error) {
	limit := 8
	minScore := 120

	var params map[string]any
	_ = json.Unmarshal([]byte(task.Params), &params)
	if params != nil {
		if v, ok := params["limit"]; ok {
			switch n := v.(type) {
			case float64: if int(n) > 0 { limit = int(n) }
			case int: if n > 0 { limit = n }
			case string: if p, err := strconv.Atoi(n); err == nil && p > 0 { limit = p }
			}
		}
		if v, ok := params["hn_min_score"]; ok {
			switch n := v.(type) {
			case float64: if int(n) > 0 { minScore = int(n) }
			case int: if n > 0 { minScore = n }
			case string: if p, err := strconv.Atoi(n); err == nil && p > 0 { minScore = p }
			}
		}
	}

	if limit == 8 {
		if v, _ := r.store.GetConfig(store.KeyRadarLimit); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
	}
	if minScore == 120 {
		if v, _ := r.store.GetConfig(store.KeyRadarHNMinScore); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				minScore = n
			}
		}
	}

	taskName := task.Name
	if taskName == "" {
		taskName = "BotHub 开源技术雷达"
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
	sb.WriteString(fmt.Sprintf("### %s · %s\n\n", taskName, time.Now().In(ShanghaiLocation()).Format("2006-01-02 15:04")))

	if len(freshGH) > 0 {
		sb.WriteString("#### 1. GitHub Trending 每日热门\n")
		for i, repo := range freshGH {
			lang := notifier.StripEmoji(strings.TrimSpace(repo.Language))
			if lang == "" {
				lang = "Multi"
			}
			if repo.StarsToday != "" {
				lang += " · " + notifier.StripEmoji(strings.TrimSpace(repo.StarsToday))
			}
			cleanDesc := notifier.StripEmoji(strings.TrimSpace(repo.Desc))
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%s）\n   %s\n", i+1, notifier.StripEmoji(repo.Name), repo.URL, lang, cleanDesc))
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
			sb.WriteString(fmt.Sprintf("%d. **[%s](%s)**（%d pts · [讨论](https://news.ycombinator.com/item?id=%d)）\n", i+1, notifier.StripEmoji(strings.TrimSpace(st.Title)), target, st.Score, st.ID))
		}
		sb.WriteString("\n")
	}

	// Optional AI synthesis & curation when Light/Pool/DeepSeek API Key is configured
	sysPrompt := "你是 BotHub 的开源技术雷达架构师。请从以下抓取的 GitHub Trending 与 Hacker News 列表中，挑选最值得关注的 3 个项目或话题，逐条给出核心亮点、技术价值与适用场景简评（中文，高信息密度）。严禁输出任何 Emoji 或特殊装饰符号。"
	if aiReview, aiErr := summarizeModuleContent(ctx, r.store, r.getSummarizer(), sysPrompt, sb.String()); aiErr == nil && aiReview != "" {
		sb.WriteString("#### 3. DeepSeek 架构师精选锐评\n")
		sb.WriteString(aiReview)
		sb.WriteString("\n\n")
	}

	sb.WriteString("---\n*来自 BotHub 自动化汇报*")
	title := fmt.Sprintf("%s · 新增 %d 条精选（GH %d / HN %d）", taskName, totalNew, len(freshGH), len(freshHN))

	if r.push != nil {
		if err := r.push.SendMarkdown(ctx, task.ID, title, sb.String()); err != nil {
			return RunResult{Summary: "推送失败: " + err.Error()}, err
		}
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
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub Trending HTTP %d", resp.StatusCode)
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, 1<<20))
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
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HackerNews topstories HTTP %d", resp.StatusCode)
	}

	var ids []int64
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&ids); err != nil {
		return nil, err
	}
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
				var st hnStory
				if sResp.StatusCode < 400 && json.NewDecoder(io.LimitReader(sResp.Body, 64<<10)).Decode(&st) == nil && st.Type == "story" && st.Score >= minScore {
					mu.Lock()
					stories = append(stories, st)
					mu.Unlock()
				}
				_ = sResp.Body.Close()
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
