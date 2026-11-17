package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseStashInputAndNormalizeURL(t *testing.T) {
	// 1. Bilibili mobile share text with prefix and tracking query params
	rawBili := "【何同学】自制家庭异地双栈软路由实测 https://b23.tv/xYz123Ab?share_source=copy_web&vd_source=998877&spm_id_from=333.1007！"
	u, dom, title, kind := ParseStashInput(rawBili)
	if kind != StashKindBilibili {
		t.Fatalf("expected kind=%s, got %s", StashKindBilibili, kind)
	}
	if u != "https://b23.tv/xYz123Ab" {
		t.Fatalf("expected stripped clean URL https://b23.tv/xYz123Ab, got %s", u)
	}
	if dom != "b23.tv" {
		t.Fatalf("expected domain b23.tv, got %s", dom)
	}
	if title != "【何同学】自制家庭异地双栈软路由实测" {
		t.Fatalf("unexpected candidate title: %q", title)
	}

	// 2. Bilibili mobile web vs desktop URL normalization (preserving multi-part p=2)
	uDesktop, _, _, ok1 := NormalizeStashURL("https://www.bilibili.com/video/BV1xx411c7mD/?p=2&spm_id_from=333.999&vd_source=abcdef")
	uMobile, _, _, ok2 := NormalizeStashURL("https://m.bilibili.com/video/BV1xx411c7mD?utm_source=share&p=2&share_plat=android")
	if !ok1 || !ok2 || uDesktop != uMobile {
		t.Fatalf("expected desktop and mobile bilibili URLs to normalize identically, got %q vs %q", uDesktop, uMobile)
	}
	if uDesktop != "https://www.bilibili.com/video/BV1xx411c7mD?p=2" {
		t.Fatalf("expected p=2 preserved and tracking stripped, got %q", uDesktop)
	}

	// 3. GitHub repo URL without prefix -> derives owner/repo title
	uGH, domGH, titleGH, kindGH := ParseStashInput("https://github.com/YN1753/BotHub?utm_campaign=trending")
	if kindGH != StashKindGitHub || domGH != "github.com" || titleGH != "YN1753/BotHub" || uGH != "https://github.com/YN1753/BotHub" {
		t.Fatalf("unexpected GitHub parse result: url=%q dom=%q title=%q kind=%q", uGH, domGH, titleGH, kindGH)
	}

	// 4. Plain text flash note without URL
	uNote, domNote, titleNote, kindNote := ParseStashInput("周末把 NAS 的 Restic 异地加密备份脚本加上失败告警\n顺便验证恢复演练")
	if kindNote != StashKindNote || uNote != "" || domNote != "" {
		t.Fatalf("expected plain text classified as note, got kind=%q url=%q dom=%q", kindNote, uNote, domNote)
	}
	if titleNote != "周末把 NAS 的 Restic 异地加密备份脚本加上失败告警" {
		t.Fatalf("unexpected note candidate title: %q", titleNote)
	}
}

func TestStashFTS5TrigramShortWordFallbackDedupAndReview(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stash_test.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	// 1. URL Deduplication & Note Merging
	first, dedup1, shouldDistill1, err := st.CaptureStashItem(
		"【网络专栏】WireGuard 异地组网避坑 https://www.bilibili.com/video/BV1Ab411c7DE?spm_id_from=333.1007",
		"第一条批注",
		"网络,WireGuard",
		false,
	)
	if err != nil || dedup1 || !shouldDistill1 {
		t.Fatalf("expected new capture, got err=%v dedup=%v distill=%v", err, dedup1, shouldDistill1)
	}
	first.Status = StashStatusReady
	first.AITldr = "通过 WireGuard 与 BGP 实现低延迟异地双栈组网容灾"
	first.Points = []string{"配置 MTU 1420 避免分片", "利用 Keepalive 穿透 NAT"}
	first.ContentExcerpt = "本文详细拆解了家庭实验室与云服务器之间的 WireGuard 异地双栈组网方案。"
	first, err = st.UpsertStashItemFull(first)
	if err != nil {
		t.Fatalf("upsert first failed: %v", err)
	}

	second, dedup2, shouldDistill2, err := st.CaptureStashItem(
		"https://m.bilibili.com/video/BV1Ab411c7DE/?vd_source=mobile_share",
		"补充第二条批注",
		"双栈",
		false,
	)
	if err != nil {
		t.Fatalf("second capture failed: %v", err)
	}
	if !dedup2 || shouldDistill2 || second.ID != first.ID {
		t.Fatalf("expected deduplicated merge onto %s, got id=%s dedup=%v distill=%v", first.ID, second.ID, dedup2, shouldDistill2)
	}
	if !strings.Contains(second.UserNote, "第一条批注") || !strings.Contains(second.UserNote, "补充第二条批注") {
		t.Fatalf("expected merged user_note, got %q", second.UserNote)
	}
	if !strings.Contains(second.Tags, "双栈") || !strings.Contains(second.Tags, "WireGuard") {
		t.Fatalf("expected merged tags, got %q", second.Tags)
	}

	// 2. Insert additional items for FTS5 trigram + 1-2 char Chinese LIKE recall tests
	noteItem, _, _, err := st.CaptureStashItem(
		"SQLite WAL 模式下的每日流式备份与校验备忘",
		"记得放进周日凌晨 cron",
		"数据库,运维",
		false,
	)
	if err != nil {
		t.Fatalf("capture note failed: %v", err)
	}
	noteItem.Status = StashStatusReady
	noteItem.Pinned = true
	noteItem.AITldr = "使用 SQLite 在线备份接口配合对象存储实现零停机冷备"
	noteItem.Points = []string{"开启 WAL 提升并发读性能", "恢复后执行 FTS5 rebuild 校验"}
	if _, err := st.UpsertStashItemFull(noteItem); err != nil {
		t.Fatalf("update note failed: %v", err)
	}

	// >= 3 runes English & Chinese FTS5 trigram search
	ftsHits1, err := st.SearchStashItems("WireGuard", "all", "", 20)
	if err != nil || len(ftsHits1) != 1 || ftsHits1[0].ID != first.ID {
		t.Fatalf("expected FTS5 trigram match for 'WireGuard', got err=%v len=%d", err, len(ftsHits1))
	}
	ftsHits2, err := st.SearchStashItems("异地双栈组网", "all", "", 20)
	if err != nil || len(ftsHits2) != 1 || ftsHits2[0].ID != first.ID {
		t.Fatalf("expected FTS5 trigram match for '异地双栈组网', got err=%v len=%d", err, len(ftsHits2))
	}

	// 1-2 rune short Chinese query fallback ("双栈", "备份")
	shortHits1, err := st.SearchStashItems("双栈", "all", "", 20)
	if err != nil || len(shortHits1) != 1 || shortHits1[0].ID != first.ID {
		t.Fatalf("expected short-word LIKE recall for '双栈', got err=%v len=%d", err, len(shortHits1))
	}
	shortHits2, err := st.SearchStashItems("备份", "all", "", 20)
	if err != nil || len(shortHits2) != 1 || shortHits2[0].ID != noteItem.ID {
		t.Fatalf("expected short-word LIKE recall for '备份', got err=%v len=%d", err, len(shortHits2))
	}

	// Special quote escaping safety in search
	quoteHits, err := st.SearchStashItems(`"WireGuard"`, "all", "", 20)
	if err != nil || len(quoteHits) != 1 {
		t.Fatalf("expected quoted FTS query to succeed safely, got err=%v len=%d", err, len(quoteHits))
	}

	// Rebuild FTS5 index (as used after backup restore) and verify trigram search still works
	if err := st.RebuildStashFTS(); err != nil {
		t.Fatalf("RebuildStashFTS failed: %v", err)
	}
	afterRebuild, err := st.SearchStashItems("在线备份接口", "all", "", 20)
	if err != nil || len(afterRebuild) != 1 || afterRebuild[0].ID != noteItem.ID {
		t.Fatalf("expected FTS5 search after rebuild to return noteItem, got err=%v len=%d", err, len(afterRebuild))
	}

	// 3. Morning Review Picker (must only pick ready items >= 3 days old, prioritizing unread)
	if earlyPick, _ := st.PickStashForReview(); earlyPick != nil {
		t.Fatalf("expected nil review pick when all items were created just now, got %s", earlyPick.ID)
	}

	fourDaysAgo := time.Now().In(ShanghaiLocation()).Add(-96 * time.Hour).Format("2006-01-02 15:04:05")
	first.CreatedAt = fourDaysAgo
	first.ReadCount = 2 // already read
	_, _ = st.UpsertStashItemFull(first)

	noteItem.CreatedAt = fourDaysAgo
	noteItem.ReadCount = 0 // unread -> should be prioritized!
	_, _ = st.UpsertStashItemFull(noteItem)

	picked1, err := st.PickStashForReview()
	if err != nil || picked1 == nil || picked1.ID != noteItem.ID {
		t.Fatalf("expected unread 4-day-old item %s picked first, got %v (err=%v)", noteItem.ID, picked1, err)
	}
	if err := st.MarkStashReviewed(picked1.ID); err != nil {
		t.Fatalf("MarkStashReviewed failed: %v", err)
	}

	// Delete trigger verification
	deleted, err := st.DeleteStashItem(noteItem.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteStashItem failed: err=%v deleted=%v", err, deleted)
	}
	postDelHits, err := st.SearchStashItems("在线备份接口", "all", "", 20)
	if err != nil || len(postDelHits) != 0 {
		t.Fatalf("expected deleted item removed from FTS5 index, got %d hits", len(postDelHits))
	}
}
