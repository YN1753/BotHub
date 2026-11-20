package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestSmartStashPipelineBackupAndMorningReview(t *testing.T) {
	// Mock upstream source server (b23.tv redirect, Bilibili view + subtitle API, GitHub README, and Light LLM)
	var mockUpstream *httptest.Server
	mockUpstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/b23/short1":
			http.Redirect(w, r, mockUpstream.URL+"/video/BV1Ab411c7DE?spm_id_from=333.999", http.StatusFound)

		case strings.HasPrefix(r.URL.Path, "/video/"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><head><title>深入理解 SQLite FTS5</title></head><body>video page</body></html>"))

		case r.URL.Path == "/x/web-interface/view":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"bvid":  "BV1Ab411c7DE",
					"title": "深入理解 SQLite FTS5 Trigram 全文索引",
					"desc":  "本期视频详解 SQLite FTS5 在中英混合场景下的索引原理与性能优化。",
					"tname": "计算机技术",
					"cid":   10086,
					"owner": map[string]any{"name": "硬核架构师"},
					"pages": []map[string]any{
						{"page": 1, "part": "为什么选择 Trigram", "cid": 10086},
						{"page": 2, "part": "短词 LIKE 双路回退设计", "cid": 10087},
					},
					"subtitle": map[string]any{
						"list": []map[string]any{
							{"lan_doc": "中文", "subtitle_url": mockUpstream.URL + "/subtitle/10086.json"},
						},
					},
				},
			})

		case r.URL.Path == "/x/tag/archive/tags":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": []map[string]any{
					{"tag_name": "SQLite"},
					{"tag_name": "全文检索"},
				},
			})

		case r.URL.Path == "/subtitle/10086.json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"body": []map[string]any{
					{"content": "大家好，今天我们来看纯静态 Go 服务如何内嵌全文搜索。"},
					{"content": "Trigram 分词器对三字以上中文非常高效，一两字短词再用 LIKE 兜底。"},
				},
			})

		case r.URL.Path == "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			// Intentionally wrap in ```json fence and include escaped Unicode emoji codepoints to verify tolerant parsing + StripEmoji
			reply := "```json\n{\n  \"title\": \"SQLite FTS5 Trigram 全文索引实战 " + "\U0001F680" + "\",\n  \"tldr\": \"结合 FTS5 trigram 与短词 LIKE 双路回退，在零外部依赖下实现中英混合毫秒级检索 " + "\u2728" + "\",\n  \"points\": [\n    \"使用 modernc.org/sqlite 纯 Go 驱动保持 CGO_ENABLED=0\",\n    \"通过 AFTER INSERT/UPDATE/DELETE 触发器实时同步虚拟表\",\n    \"1~2 字中文短词自动降级参数化 LIKE 确保 100% 召回\"\n  ],\n  \"tags\": [\"SQLite\", \"Go后端\", \"全文检索\"]\n}\n```"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"content": reply}},
				},
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer mockUpstream.Close()

	dbPath := filepath.Join(t.TempDir(), "bothub_stash_srv.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	_ = st.SetConfig(store.KeyAppBearerToken, "bh_test_stash_token")
	_ = st.SetConfig(store.KeyBiliSessdata, "test_sessdata_cookie")
	_ = st.SetConfig(store.KeyLightAPIKey, "sk-test-light")
	_ = st.SetConfig(store.KeyLightBaseURL, mockUpstream.URL+"/v1")
	_ = st.SetConfig(store.KeyLightModel, "deepseek-chat")

	mockFS := fstest.MapFS{
		"static/index.html": &fstest.MapFile{Data: []byte("<html><body>ok</body></html>")},
	}
	push := notifier.NewClient(st)
	srv, err := New(st, nil, push, mockFS, "v2.32.0", "testsha", "now")
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.ClosePushStream()

	srv.agent.ConfigureTestEndpoints(true, mockUpstream.URL, mockUpstream.URL, mockUpstream.URL)
	h := srv.Handler()

	authHeader := "Bearer bh_test_stash_token"

	// 1. Capture Bilibili short-link share text via POST /api/v2/stash (<50ms non-blocking response)
	sharePayload := fmt.Sprintf(`{"raw_input":"【硬核架构师】深入理解 SQLite FTS5 %s/b23/short1","user_note":"周末重构收藏模块参考"}`, mockUpstream.URL)
	start := time.Now()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v2/stash", bytes.NewBufferString(sharePayload))
	reqCreate.Header.Set("Authorization", authHeader)
	reqCreate.Header.Set("Content-Type", "application/json")
	recCreate := httptest.NewRecorder()
	h.ServeHTTP(recCreate, reqCreate)
	elapsed := time.Since(start)

	if recCreate.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/stash failed: %d %s", recCreate.Code, recCreate.Body.String())
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected fast non-blocking capture, took %v", elapsed)
	}

	var createResp struct {
		OK   bool            `json:"ok"`
		Item store.StashItem `json:"item"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	stashID := createResp.Item.ID
	if !strings.HasPrefix(stashID, "st_") {
		t.Fatalf("expected st_ prefix ID, got %s", stashID)
	}

	// Poll until async distillation transitions item to status == "ready"
	var distilled *store.StashItem
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cur, _ := st.GetStashItem(stashID)
		if cur != nil && cur.Status == store.StashStatusReady && strings.Contains(cur.AITldr, "双路回退") {
			distilled = cur
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if distilled == nil {
		cur, _ := st.GetStashItem(stashID)
		t.Fatalf("timed out waiting for async distillation, current=%+v", cur)
	}

	// Verify reprocess endpoint also resets to processing and finishes cleanly
	reqReproc := httptest.NewRequest(http.MethodPost, "/api/v2/stash/"+stashID+"/reprocess", bytes.NewBufferString(`{}`))
	reqReproc.Header.Set("Authorization", authHeader)
	recReproc := httptest.NewRecorder()
	h.ServeHTTP(recReproc, reqReproc)
	if recReproc.Code != http.StatusOK {
		t.Fatalf("reprocess failed: %d %s", recReproc.Code, recReproc.Body.String())
	}
	deadline = time.Now().Add(5 * time.Second)
	distilled = nil
	for time.Now().Before(deadline) {
		cur, _ := st.GetStashItem(stashID)
		if cur != nil && cur.Status == store.StashStatusReady {
			distilled = cur
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if distilled == nil {
		t.Fatalf("timed out waiting for reprocess completion")
	}

	// Verify StripEmoji stripped emojis from AI output and Bilibili subtitle excerpt was captured
	if strings.Contains(distilled.AITldr, "\u2728") || strings.Contains(distilled.Title, "\U0001F680") {
		t.Fatalf("expected emojis stripped from distilled item: title=%q tldr=%q", distilled.Title, distilled.AITldr)
	}
	if !strings.Contains(distilled.ContentExcerpt, "字幕内容摘录") || !strings.Contains(distilled.ContentExcerpt, "为什么选择 Trigram") {
		t.Fatalf("expected Bilibili pages + subtitle in excerpt, got: %s", distilled.ContentExcerpt)
	}
	if len(distilled.Points) != 3 {
		t.Fatalf("expected 3 distilled points, got %d", len(distilled.Points))
	}

	// 2. Test GET /api/v2/stash search (both >=3 char FTS5 and 2-char LIKE) & mark_read update
	reqSearch := httptest.NewRequest(http.MethodGet, "/api/v2/stash?q=双路回退&filter=unread", nil)
	reqSearch.Header.Set("Authorization", authHeader)
	recSearch := httptest.NewRecorder()
	h.ServeHTTP(recSearch, reqSearch)
	if recSearch.Code != http.StatusOK || !strings.Contains(recSearch.Body.String(), stashID) {
		t.Fatalf("GET /api/v2/stash search failed: %d %s", recSearch.Code, recSearch.Body.String())
	}

	reqUpdate := httptest.NewRequest(http.MethodPost, "/api/v2/stash/"+stashID, bytes.NewBufferString(`{"pinned":true,"mark_read":true}`))
	reqUpdate.Header.Set("Authorization", authHeader)
	recUpdate := httptest.NewRecorder()
	h.ServeHTTP(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/stash/{id} failed: %d %s", recUpdate.Code, recUpdate.Body.String())
	}
	afterRead, _ := st.GetStashItem(stashID)
	if !afterRead.Pinned || afterRead.ReadCount != 1 {
		t.Fatalf("expected pinned=true read_count=1, got pinned=%v read_count=%d", afterRead.Pinned, afterRead.ReadCount)
	}

	// Verify mark_unread and bulk mark-all-read endpoints
	reqUnread := httptest.NewRequest(http.MethodPost, "/api/v2/stash/"+stashID, bytes.NewBufferString(`{"mark_unread":true}`))
	reqUnread.Header.Set("Authorization", authHeader)
	recUnread := httptest.NewRecorder()
	h.ServeHTTP(recUnread, reqUnread)
	if recUnread.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/stash/{id} mark_unread failed: %d %s", recUnread.Code, recUnread.Body.String())
	}
	afterUnread, _ := st.GetStashItem(stashID)
	if afterUnread.ReadCount != 0 {
		t.Fatalf("expected read_count=0 after mark_unread, got %d", afterUnread.ReadCount)
	}

	reqAllRead := httptest.NewRequest(http.MethodPost, "/api/v2/stash/mark-all-read", bytes.NewBufferString(`{}`))
	reqAllRead.Header.Set("Authorization", authHeader)
	recAllRead := httptest.NewRecorder()
	h.ServeHTTP(recAllRead, reqAllRead)
	if recAllRead.Code != http.StatusOK || !strings.Contains(recAllRead.Body.String(), `"updated":1`) {
		t.Fatalf("POST /api/v2/stash/mark-all-read failed: %d %s", recAllRead.Code, recAllRead.Body.String())
	}
	afterAllRead, _ := st.GetStashItem(stashID)
	if afterAllRead.ReadCount < 1 {
		t.Fatalf("expected read_count>=1 after mark-all-read, got %d", afterAllRead.ReadCount)
	}

	// 3. Test Agent Tool manage_stash (capture plain-text flash note + search)
	toolOut, err := srv.agent.ExecuteTool(context.Background(), "manage_stash", `{"action":"capture","raw_input":"灵感：给家里软路由加上双栈连通性巡检","tags":"网络,灵感"}`)
	if err != nil || !strings.Contains(toolOut, `"ok":true`) {
		t.Fatalf("manage_stash capture failed: err=%v out=%s", err, toolOut)
	}
	searchToolOut, err := srv.agent.ExecuteTool(context.Background(), "manage_stash", `{"action":"search","query":"双栈"}`)
	if err != nil || !strings.Contains(searchToolOut, "软路由") {
		t.Fatalf("manage_stash short-word search failed: err=%v out=%s", err, searchToolOut)
	}

	// 4. Test Dashboard Slice ("workhub" and "stash")
	reqSlice := httptest.NewRequest(http.MethodGet, "/api/v2/dashboard/workhub", nil)
	reqSlice.Header.Set("Authorization", authHeader)
	recSlice := httptest.NewRecorder()
	h.ServeHTTP(recSlice, reqSlice)
	if recSlice.Code != http.StatusOK || !strings.Contains(recSlice.Body.String(), `"stash"`) {
		t.Fatalf("expected workhub slice to include stash, got %d %s", recSlice.Code, recSlice.Body.String())
	}

	// 5. Test Backup Export & Import into fresh DB (including FTS5 rebuild)
	reqExport := httptest.NewRequest(http.MethodGet, "/api/system/export", nil)
	reqExport.Header.Set("Authorization", authHeader)
	recExport := httptest.NewRecorder()
	h.ServeHTTP(recExport, reqExport)
	if recExport.Code != http.StatusOK || !strings.Contains(recExport.Body.String(), `"stash_items"`) {
		t.Fatalf("export backup missing stash_items: %d", recExport.Code)
	}

	dbPath2 := filepath.Join(t.TempDir(), "restored_stash.db")
	st2, err := store.Open(dbPath2)
	if err != nil {
		t.Fatalf("open st2: %v", err)
	}
	defer st2.Close()
	_ = st2.SetConfig(store.KeyAppBearerToken, "bh_restore_token")
	srv2, _ := New(st2, nil, notifier.NewClient(st2), mockFS, "v2.32.0", "testsha", "now")
	defer srv2.ClosePushStream()

	reqImport := httptest.NewRequest(http.MethodPost, "/api/system/import", bytes.NewReader(recExport.Body.Bytes()))
	reqImport.Header.Set("Authorization", "Bearer bh_restore_token")
	recImport := httptest.NewRecorder()
	srv2.Handler().ServeHTTP(recImport, reqImport)
	if recImport.Code != http.StatusOK || !strings.Contains(recImport.Body.String(), `"stash_restored"`) {
		t.Fatalf("import backup failed: %d %s", recImport.Code, recImport.Body.String())
	}

	restoredHits, err := st2.SearchStashItems("字幕内容摘录", "all", "", 10)
	if err != nil || len(restoredHits) != 1 {
		t.Fatalf("expected restored FTS5 index searchable immediately, got err=%v len=%d", err, len(restoredHits))
	}

	// 6. Test MorningModule anti-dust review section ("收藏匣记忆重温")
	aged := restoredHits[0]
	aged.CreatedAt = time.Now().In(store.ShanghaiLocation()).Add(-96 * time.Hour).Format("2006-01-02 15:04:05")
	aged.ReadCount = 0
	_, _ = st2.UpsertStashItemFull(aged)

	push2 := notifier.NewClient(st2)
	morningMod := modules.NewMorningModule(st2, push2)
	if _, err := morningMod.Run(context.Background()); err != nil {
		t.Fatalf("morning module run failed: %v", err)
	}
	notifs, _ := st2.ListNotifications(0, 5, false)
	if len(notifs) == 0 || !strings.Contains(notifs[0].MarkdownBody, "收藏匣记忆重温") || !strings.Contains(notifs[0].MarkdownBody, "双路回退") {
		t.Fatalf("expected morning briefing notification to contain 收藏匣记忆重温 section, got: %+v", notifs)
	}
	reviewedAfter, _ := st2.GetStashItem(aged.ID)
	if reviewedAfter.LastReviewedAt == "" {
		t.Fatalf("expected last_reviewed_at updated after morning briefing review")
	}
}
