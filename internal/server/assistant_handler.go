package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) publishAssistantEvent(kind string) {
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("assistant_update", map[string]any{
			"kind": kind,
			"time": store.NowShanghai(),
		})
	}
}

func (s *Server) handleListTodos(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status == "" {
		status = "all"
	}
	botID := strings.TrimSpace(r.URL.Query().Get("bot_id"))
	todos, err := s.store.ListTodos(status, botID, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取待办列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"todos": todos,
	})
}

func (s *Server) handleCreateTodo(w http.ResponseWriter, r *http.Request) {
	var req store.TodoItem
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效待办请求体")
		return
	}
	req.Title = notifier.StripEmoji(strings.TrimSpace(req.Title))
	req.Notes = notifier.StripEmoji(strings.TrimSpace(req.Notes))
	req.Tags = notifier.StripEmoji(strings.TrimSpace(req.Tags))
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "待办标题不能为空")
		return
	}
	saved, err := s.store.SaveTodo(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存待办失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("todos")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "todo": saved})
}

func (s *Server) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetTodo(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "待办条目不存在")
		return
	}
	var req struct {
		BotID    *string `json:"bot_id"`
		Title    *string `json:"title"`
		Notes    *string `json:"notes"`
		Status   *string `json:"status"`
		Priority *string `json:"priority"`
		DueAt    *string `json:"due_at"`
		Tags     *string `json:"tags"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效更新请求体")
		return
	}
	if req.BotID != nil {
		existing.BotID = strings.TrimSpace(*req.BotID)
	}
	if req.Title != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Title)); v != "" {
			existing.Title = v
		}
	}
	if req.Notes != nil {
		existing.Notes = notifier.StripEmoji(strings.TrimSpace(*req.Notes))
	}
	if req.Status != nil {
		existing.Status = store.NormalizeTodoStatus(*req.Status)
	}
	if req.Priority != nil {
		existing.Priority = store.NormalizeTodoPriority(*req.Priority)
	}
	if req.DueAt != nil {
		existing.DueAt = strings.TrimSpace(*req.DueAt)
	}
	if req.Tags != nil {
		existing.Tags = notifier.StripEmoji(strings.TrimSpace(*req.Tags))
	}
	saved, err := s.store.SaveTodo(*existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新待办失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("todos")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "todo": saved})
}

func (s *Server) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少待办 ID")
		return
	}
	ok, err := s.store.DeleteTodo(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除待办失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("todos")
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok})
}

func (s *Server) handleClearDoneTodos(w http.ResponseWriter, r *http.Request) {
	removed, err := s.store.ClearCompletedTodos("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "清理已完成待办失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("todos")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": removed})
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	botID := strings.TrimSpace(r.URL.Query().Get("bot_id"))
	notes, err := s.store.ListNotes(q, botID, 120)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取工作文档列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"notes": notes,
	})
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req store.WorkNote
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效文档请求体")
		return
	}
	req.Title = notifier.StripEmoji(strings.TrimSpace(req.Title))
	req.Content = notifier.StripEmoji(strings.TrimSpace(req.Content))
	req.Category = notifier.StripEmoji(strings.TrimSpace(req.Category))
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "文档标题不能为空")
		return
	}
	saved, err := s.store.SaveNote(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存文档失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("notes")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": saved})
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetNote(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "工作文档不存在")
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Content  *string `json:"content"`
		Category *string `json:"category"`
		Pinned   *bool   `json:"pinned"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效更新请求体")
		return
	}
	if req.Title != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Title)); v != "" {
			existing.Title = v
		}
	}
	if req.Content != nil {
		existing.Content = notifier.StripEmoji(strings.TrimSpace(*req.Content))
	}
	if req.Category != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Category)); v != "" {
			existing.Category = v
		}
	}
	if req.Pinned != nil {
		existing.Pinned = *req.Pinned
	}
	saved, err := s.store.SaveNote(*existing)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新文档失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("notes")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": saved})
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少文档 ID")
		return
	}
	ok, err := s.store.DeleteNote(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除文档失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("notes")
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok})
}
