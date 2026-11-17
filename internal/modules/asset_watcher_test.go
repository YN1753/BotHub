package modules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestCalcNextAnchorDateClamping(t *testing.T) {
	loc := ShanghaiLocation()

	// Anchor on 31st day
	anchorDay := 31

	// In any month, anchor calculation should clamp properly to the month's maximum days
	next := calcNextAnchorDate(anchorDay, loc)
	if next.Day() > 31 || next.Day() < 28 {
		t.Fatalf("unexpected anchor day result: %v", next)
	}

	// Verify month bounds
	y, m, d := next.Date()
	maxDays := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	if d > maxDays {
		t.Fatalf("day %d overflows maximum days %d in %v %v", d, maxDays, y, m)
	}
}

func TestAssetShouldWarn(t *testing.T) {
	w := &AssetWatcherModule{}

	// Case 1: Custom warn_days [30, 15, 7, 3, 1]
	asset30 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 30,
	}
	if !w.shouldWarn(asset30) {
		t.Errorf("expected warn at 30 days left")
	}

	asset15 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 15,
	}
	if !w.shouldWarn(asset15) {
		t.Errorf("expected warn at 15 days left")
	}

	asset20 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 20,
	}
	if w.shouldWarn(asset20) {
		t.Errorf("did not expect warn at 20 days left")
	}

	// Case 2: Critical window (<= 3 days)
	asset2 := store.Asset{
		ID:       "test_critical",
		WarnDays: "[7, 3, 1]",
		DaysLeft: 2,
	}
	if !w.shouldWarn(asset2) {
		t.Errorf("expected warn at critical 2 days left")
	}

	// Case 3: Normalized legacy asset statuses map cleanly to unified lifecycle states
	for _, legacy := range []string{"运行中", "会员有效", "活动体验中", "active", ""} {
		if got := store.NormalizeAssetStatus(legacy); got != "生效中" {
			t.Errorf("NormalizeAssetStatus(%q) = %q, want 生效中", legacy, got)
		}
	}
	if got := store.NormalizeAssetStatus("已停用"); got != "已停用" {
		t.Errorf("expected 已停用 preserved, got %q", got)
	}
}

func TestAssetWatcherSortsMostUrgentAlertFirst(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "watcher.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	loc := ShanghaiLocation()
	now := time.Now().In(loc)
	in7Days := now.AddDate(0, 0, 7).Format("2006-01-02")
	in1Day := now.AddDate(0, 0, 1).Format("2006-01-02")

	_ = st.SaveAsset(store.Asset{ID: "asset_later", Name: "次紧急服务器", Spec: "2C4G", Status: "生效中", DueDate: in7Days, WarnDays: "[7,3,1]"})
	_ = st.SaveAsset(store.Asset{ID: "asset_urgent", Name: "明日到期域名", Spec: "example.com", Status: "生效中", DueDate: in1Day, WarnDays: "[7,3,1]"})

	push := notifier.NewClient(st)
	mod := NewAssetWatcherModule(st, push)
	res, err := mod.Run(context.Background())
	if err != nil {
		t.Fatalf("AssetWatcherModule.Run failed: %v", err)
	}
	if res.ItemsPushed != 2 {
		t.Fatalf("expected 2 alerts pushed, got %d (%s)", res.ItemsPushed, res.Summary)
	}
	notifs, _ := st.ListNotifications(0, 10, false)
	if len(notifs) != 1 || !strings.Contains(notifs[0].Title, "最快剩余 1 天") {
		t.Fatalf("expected urgent-first sorted title with 最快剩余 1 天, got %+v", notifs)
	}
}

func TestRetiringNonRenewingAssetCountdownRakSV(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "rak_sv.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	loc := ShanghaiLocation()
	dueIn14Days := time.Now().In(loc).AddDate(0, 0, 14).Format("2006-01-02") + "T00:00:00+08:00"
	// Note: today's date + 14 days at 00:00:00 yields Ceil((14d - elapsedToday)/24h) == 14 days when elapsedToday > 0;
	// use exact 14*24h offset rounded to RFC3339 to make test time-of-day invariant.
	dueExact14 := time.Now().In(loc).Add(14*24*time.Hour - time.Hour).Format(time.RFC3339)
	_ = dueIn14Days

	err = st.SaveAsset(store.Asset{
		ID:          "rak_sv",
		Name:        "RakSmart SV",
		Category:    "compute",
		Spec:        "Silicon Valley VPS",
		Status:      "运行中",
		DueDate:     dueExact14,
		AutoRenew:   false,
		WarnDays:    "[14, 7, 3, 1]",
		ExtraConfig: `{"provider": "RakSmart", "retiring": true, "anchor_day": 12, "note": "2026-12-12 到期退役不续费，在此之前迁移散页"}`,
	})
	if err != nil {
		t.Fatalf("SaveAsset rak_sv: %v", err)
	}

	push := notifier.NewClient(st)
	mod := NewAssetWatcherModule(st, push)
	res, err := mod.Run(context.Background())
	if err != nil {
		t.Fatalf("AssetWatcherModule.Run failed: %v", err)
	}
	if res.ItemsPushed != 1 {
		t.Fatalf("expected 1 countdown alert for rak_sv at 14 days left, got %d (%s)", res.ItemsPushed, res.Summary)
	}

	after, err := st.GetAsset("rak_sv")
	if err != nil || after == nil {
		t.Fatalf("GetAsset rak_sv failed: %v", err)
	}
	if after.AutoRenew {
		t.Fatal("expected rak_sv auto_renew=false preserved without anchor_day rollover")
	}
	if after.DueDate != dueExact14 {
		t.Fatalf("expected rak_sv due_date %q preserved, got %q", dueExact14, after.DueDate)
	}

	notifs, _ := st.ListNotifications(0, 5, false)
	if len(notifs) != 1 || !strings.Contains(notifs[0].MarkdownBody, "到期退役不续费，在此之前迁移散页") {
		t.Fatalf("expected retiring note in notification markdown, got %+v", notifs)
	}
}
