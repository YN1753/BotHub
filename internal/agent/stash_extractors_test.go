package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestShareCopyNoiseCleaningAndPlatformClassification(t *testing.T) {
	u, dom, title, kind := store.ParseStashInput("4.82 复制打开抖音，看看【极客小王的作品】Go 1.24 性能实测 #后端开发 #Golang https://v.douyin.com/iRNB123/ 07/19 v@f.bA")
	if kind != store.StashKindDouyin {
		t.Fatalf("expected douyin kind, got %q", kind)
	}
	if dom != "v.douyin.com" || !strings.HasPrefix(u, "https://v.douyin.com/") {
		t.Fatalf("unexpected url/dom: %q / %q", u, dom)
	}
	if strings.Contains(title, "复制打开抖音") || strings.Contains(title, "07/19") {
		t.Fatalf("expected boilerplate stripped from douyin title, got %q", title)
	}
	if !strings.Contains(title, "Go 1.24") {
		t.Fatalf("expected actual title preserved, got %q", title)
	}

	_, _, xhsTitle, xhsKind := store.ParseStashInput("本地部署 AI 笔记流完整配置 http://xhslink.com/a/abc123 复制本条信息，打开【小红书】App查看精彩内容！")
	if xhsKind != store.StashKindXiaohongshu {
		t.Fatalf("expected xiaohongshu kind, got %q", xhsKind)
	}
	if strings.Contains(xhsTitle, "复制本条信息") {
		t.Fatalf("expected xhs boilerplate stripped, got %q", xhsTitle)
	}
	if !strings.Contains(xhsTitle, "本地部署 AI 笔记流完整配置") {
		t.Fatalf("expected xhs title preserved, got %q", xhsTitle)
	}

	_, _, _, wxKind := store.ParseStashInput("架构复盘 https://mp.weixin.qq.com/s?__biz=MzA1&mid=123&idx=1&sn=abc")
	if wxKind != store.StashKindWeChat {
		t.Fatalf("expected wechat kind, got %q", wxKind)
	}
}

func TestBilibili412FallbackToInitialStateHTML(t *testing.T) {
	htmlPage := `<!doctype html><html><head>
<meta property="og:title" content="Grok Bot Harness 优化实测_哔哩哔哩_bilibili">
<meta name="description" content="实测新版 Agent Harness 调度策略与订阅增长">
</head><body><script>window.__INITIAL_STATE__={"videoData":{"bvid":"BV1AB411c7mD","title":"Grok Bot Harness 优化实测","desc":"详细拆解调度与延迟优化要点","tname":"科技","owner":{"name":"硬核评测君"},"pages":[{"page":1,"part":"核心改造"},{"page":2,"part":"基准对比"}]},"tags":[{"tag_name":"AI Agent"},{"tag_name":"性能"}]};(function(){})();</script></body></html>`

	var srvURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/short":
			http.Redirect(w, r, srvURL+"/video/BV1AB411c7mD", http.StatusFound)
		case r.URL.Path == "/video/BV1AB411c7mD":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlPage))
		case r.URL.Path == "/x/web-interface/view":
			w.WriteHeader(http.StatusPreconditionFailed) // HTTP 412
			_, _ = w.Write([]byte(`{"code":-412,"message":"request was banned"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	srvURL = ts.URL

	st, err := store.Open(filepath.Join(t.TempDir(), "bili412.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, nil)
	eng.ConfigureTestEndpoints(true, ts.URL, ts.URL, ts.URL)

	src, err := eng.extractBilibiliSource(context.Background(), ts.URL+"/short")
	if err != nil {
		t.Fatalf("expected HTML fallback to rescue HTTP 412, got error: %v", err)
	}
	if !strings.Contains(src.Title, "Grok Bot Harness 优化实测") || !strings.Contains(src.Title, "硬核评测君") {
		t.Fatalf("unexpected title from fallback: %q", src.Title)
	}
	if !strings.Contains(src.Excerpt, "详细拆解调度与延迟优化要点") || !strings.Contains(src.Excerpt, "P2: 基准对比") {
		t.Fatalf("unexpected excerpt from fallback: %q", src.Excerpt)
	}
}

func TestXiaohongshuAndDouyinHTMLParsers(t *testing.T) {
	xhsHTML := `<html><head><meta property="og:title" content="Mac 本地知识库搭建 - 小红书"></head><body><script>window.__INITIAL_STATE__={"note":{"noteDetailMap":{"66a1":{"note":{"title":"Mac 本地知识库搭建","desc":"三步打通 Obsidian 与本地向量检索","user":{"nickname":"效率研究员"},"tagList":[{"name":"知识管理"},{"name":"AI工具"}]}}},"extra":undefined}};</script></body></html>`
	xhs := parseXiaohongshuHTML(xhsHTML, "https://www.xiaohongshu.com/explore/66a1", "xiaohongshu.com")
	if xhs == nil {
		t.Fatalf("expected xhs HTML parsed")
	}
	if xhs.Kind != store.StashKindXiaohongshu || !strings.Contains(xhs.Excerpt, "效率研究员") || !strings.Contains(xhs.Excerpt, "Obsidian") {
		t.Fatalf("unexpected xhs parsed result: %+v", xhs)
	}

	dyHTML := `<html><head></head><body><script>window._ROUTER_DATA = {"loaderData":{"video_(id)/page":{"videoInfoRes":{"item_list":[{"desc":"三分钟讲透 Go 并发调度器 #Golang #后端","author":{"nickname":"代码观察室"},"music":{"title":"原声"},"text_extra":[{"hashtag_name":"Golang"},{"hashtag_name":"后端"}]}]}}}}</script></body></html>`
	dy := parseDouyinHTML(dyHTML, "https://www.douyin.com/video/7399999999999999999", "douyin.com")
	if dy == nil {
		t.Fatalf("expected douyin HTML parsed")
	}
	if dy.Kind != store.StashKindDouyin || !strings.Contains(dy.Title, "代码观察室") || !strings.Contains(dy.Excerpt, "Go 并发调度器") {
		t.Fatalf("unexpected douyin parsed result: %+v", dy)
	}
}

func TestClientPrefetchHintAndGracefulShareFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
	}))
	defer ts.Close()

	st, err := store.Open(filepath.Join(t.TempDir(), "stash_hint.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, nil)
	eng.ConfigureTestEndpoints(true, ts.URL, ts.URL, ts.URL)

	// 1. Client-prefetched metadata survives server 412 cleanly
	item1, _, err := eng.CaptureAndDistillStashWithHint(
		"测试视频 "+ts.URL+"/mock1",
		"值得复看",
		"",
		false,
		&ClientPrefetchHint{
			CanonicalURL: ts.URL + "/video/BV1AB411c7mD",
			Title:        "[硬核UP] 手机端预抓标题",
			Excerpt:      "UP主：硬核UP\n视频简介：\n这是手机端本地网络预抓到的完整简介内容，用于规避海外机房 IP 412 风控。",
			Tags:         "Bilibili,AI",
		},
	)
	if err != nil {
		t.Fatalf("capture with hint: %v", err)
	}
	if err := eng.DistillStashNow(context.Background(), item1.ID); err != nil {
		t.Fatalf("distill with hint: %v", err)
	}
	got1, _ := st.GetStashItem(item1.ID)
	if got1.Status != store.StashStatusReady {
		t.Fatalf("expected status ready using client prefetch, got %s (%s)", got1.Status, got1.ErrorMsg)
	}
	if !strings.Contains(got1.ContentExcerpt, "手机端本地网络预抓到的完整简介内容") {
		t.Fatalf("expected client excerpt preserved, got %q", got1.ContentExcerpt)
	}

	// 2. Without client hint, rich share text gracefully degrades to ready instead of hard failed
	item2, _, err := eng.CaptureAndDistillStash(
		"小更新杀疯！Grok Bot疯狂祭出harness优化，订阅+7%，日常15%冲顶 "+ts.URL+"/mock2",
		"",
		"",
		false,
	)
	if err != nil {
		t.Fatalf("capture fallback: %v", err)
	}
	if err := eng.DistillStashNow(context.Background(), item2.ID); err != nil {
		t.Fatalf("distill fallback: %v", err)
	}
	got2, _ := st.GetStashItem(item2.ID)
	if got2.Status != store.StashStatusReady {
		t.Fatalf("expected graceful degradation to ready, got %s (%s)", got2.Status, got2.ErrorMsg)
	}
	if !strings.Contains(got2.AITldr, "Grok Bot") {
		t.Fatalf("expected fallback summary from share text, got %q", got2.AITldr)
	}
}
