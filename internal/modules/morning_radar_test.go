package modules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestMorningModuleInvertedPyramidNoQuotesNoUnconfiguredNezha(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "morning_opt_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Seed yesterday's morning brief, an overdue todo, an expiring asset, and an unread stash item
	_, _ = st.CreateNotification("morning", "每日早报简报 · 昨日", "昨日早报摘要：注意带伞并处理服务器升级。", "昨日正文", "normal")
	yesterday := time.Now().In(ShanghaiLocation()).AddDate(0, 0, -1).Format("2006-01-02")
	_, _ = st.SaveTodo(store.TodoItem{
		Title:    "续费主力域名",
		Priority: "high",
		Status:   "pending",
		DueAt:    yesterday,
	})
	_ = st.SaveAsset(store.Asset{
		ID:      "asset_exp_1",
		Name:    "CloudCone VPS",
		Spec:    "2C2G",
		Price:   "$15/yr",
		DueDate: time.Now().In(ShanghaiLocation()).AddDate(0, 0, 3).Format("2006-01-02"),
	})
	_, _, _, _ = st.CaptureStashItem("https://example.com/go-perf", "Go 性能优化实战笔记", "go,performance", false)

	push := notifier.NewClient(st)
	mod := NewMorningModule(st, push)
	// Point HTTP client to a fast local stub so weather fallback fails cleanly without external network flakiness
	deadSrv := httptest.NewServer(http.NotFoundHandler())
	defer deadSrv.Close()
	mod.http = &http.Client{Timeout: 300 * time.Millisecond}

	var capturedUserPrompt string
	mod.SetSummarizer(LightSummarizerFunc(func(ctx context.Context, sysPrompt, userPrompt string) (string, error) {
		capturedUserPrompt = userPrompt
		return "1. 优先处理已逾期高优待办「续费主力域名」。\n2. 3 天内确认 CloudCone VPS（15 USD）续费。", nil
	}))

	res, err := mod.RunWithTask(context.Background(), store.Task{
		ID:   "morning",
		Name: "每日早报简报",
		Type: "morning",
	})
	if err != nil || res.ItemsPushed != 1 {
		t.Fatalf("MorningModule.RunWithTask failed: res=%+v err=%v", res, err)
	}

	notifs, err := st.ListRecentNotificationsByModule("morning", 1)
	if err != nil || len(notifs) == 0 {
		t.Fatalf("expected morning notification saved, err=%v", err)
	}
	body := notifs[0].MarkdownBody
	title := notifs[0].Title

	if !strings.Contains(title, "1 项逾期") {
		t.Fatalf("expected overdue badge in morning title, got %q", title)
	}
	if strings.Contains(body, "每日格言") || strings.Contains(body, "未配置探针地址") || strings.Contains(body, "来自 BotHub 自动化汇报") {
		t.Fatalf("expected clichés, unconfigured nezha noise, and footer removed, got:\n%s", body)
	}
	idxAI := strings.Index(body, "#### 今日核心行动建议")
	idxTodo := strings.Index(body, "#### 今日行动焦点（待办）")
	idxWeather := strings.Index(body, "#### 气象与出行")
	if idxAI < 0 || idxTodo < 0 || idxWeather < 0 || !(idxAI < idxTodo && idxTodo < idxWeather) {
		t.Fatalf("expected inverted pyramid order (AI -> Todo -> Weather), got:\n%s", body)
	}
	if !strings.Contains(capturedUserPrompt, "昨日早报摘要") {
		t.Fatalf("expected recent morning brief summary fed into AI prompt to prevent repetition, got:\n%s", capturedUserPrompt)
	}
}
