package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) handleListStash(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = strings.TrimSpace(r.URL.Query().Get("status"))
	}
	if filter == "" {
		filter = "all"
	}
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))

	limit := 80
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if n, err := strconv.Atoi(rawLimit); err == nil && n > 0 && n <= 300 {
			limit = n
		}
	}

	items, err := s.store.SearchStashItemsWithKind(q, filter, tag, kind, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询收藏匣失败: "+err.Error())
		return
	}
	topTags, _ := s.store.ListTopStashTags(14)
	if topTags == nil {
		topTags = []store.StashTagCount{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"items":    items,
		"top_tags": topTags,
	})
}

func (s *Server) handleCreateStash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RawInput       string `json:"raw_input"`
		UserNote       string `json:"user_note"`
		Tags           string `json:"tags"`
		ForceReprocess bool   `json:"force_reprocess"`
	}
	if !decodeJSON(w, r, 128*1024, &req) {
		return
	}
	if strings.TrimSpace(req.RawInput) == "" {
		writeError(w, http.StatusBadRequest, "请输入需要收藏的链接、分享文案或随手闪念")
		return
	}

	saved, dedup, err := s.agent.CaptureAndDistillStash(req.RawInput, req.UserNote, req.Tags, req.ForceReprocess)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存收藏失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"deduplicated": dedup,
		"item":         saved,
	})
}

func (s *Server) handleUpdateStash(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少收藏 ID")
		return
	}
	existing, err := s.store.GetStashItem(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "收藏条目不存在")
		return
	}

	var req struct {
		Title    *string `json:"title"`
		UserNote *string `json:"user_note"`
		Tags     *string `json:"tags"`
		Pinned   *bool   `json:"pinned"`
		Status   *string `json:"status"`
		MarkRead *bool   `json:"mark_read"`
	}
	if !decodeJSON(w, r, 64*1024, &req) {
		return
	}

	if req.MarkRead != nil && *req.MarkRead {
		existing.ReadCount++
		existing.LastReviewedAt = store.NowShanghai()
	}
	if req.Title != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Title)); v != "" {
			existing.Title = v
		}
	}
	if req.UserNote != nil {
		existing.UserNote = notifier.StripEmoji(strings.TrimSpace(*req.UserNote))
	}
	if req.Tags != nil {
		existing.Tags = notifier.StripEmoji(strings.TrimSpace(*req.Tags))
	}
	if req.Pinned != nil {
		existing.Pinned = *req.Pinned
	}
	if req.Status != nil {
		st := strings.TrimSpace(*req.Status)
		if st != "" {
			existing.Status = store.NormalizeStashStatus(st)
		}
	}
	existing.UpdatedAt = store.NowShanghai()

	saved, err := s.store.UpsertStashItemFull(*existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新收藏失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("stash")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"item": saved,
	})
}

func (s *Server) handleReprocessStash(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少收藏 ID")
		return
	}
	updated, err := s.agent.ReprocessStashItem(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "触发重提炼失败: "+err.Error())
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "收藏条目不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"item": updated,
	})
}

func (s *Server) handleDeleteStash(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少收藏 ID")
		return
	}
	ok, err := s.store.DeleteStashItem(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除收藏失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("stash")
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok})
}
