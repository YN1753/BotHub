package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/notifier"
)

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req agent.ChatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效对话请求体")
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "对话内容不能为空")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "当前连接不支持流式输出")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	emitSSE := func(event string, data any) {
		b, err := json.Marshal(data)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b))
		flusher.Flush()
	}

	if _, err := s.agent.ChatStream(ctx, req, emitSSE); err != nil {
		emitSSE("error", map[string]any{
			"error": notifier.StripEmoji(err.Error()),
		})
	}
}

func (s *Server) handleListAgentSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.ListAgentSessions(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取会话列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"personas": agent.BuiltinPersonas(),
	})
}

func (s *Server) handleGetAgentSession(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少会话 ID")
		return
	}
	sess, err := s.store.GetAgentSession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sess == nil {
		writeError(w, http.StatusNotFound, "会话不存在")
		return
	}
	msgs, err := s.store.ListAgentMessages(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session":  sess,
		"messages": msgs,
	})
}

func (s *Server) handleDeleteAgentSession(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少会话 ID")
		return
	}
	if err := s.store.DeleteAgentSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除会话失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListAgentMemories(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	var scopes []string
	if scope != "" {
		for _, part := range strings.Split(scope, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				scopes = append(scopes, trimmed)
			}
		}
	}
	memories, err := s.store.ListScopedMemories(scopes, query, 120)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取记忆列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"memories": memories,
	})
}

func (s *Server) handleSaveAgentMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope      string `json:"scope"`
		Category   string `json:"category"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Importance int    `json:"importance"`
		Pinned     bool   `json:"pinned"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	key := notifier.StripEmoji(strings.TrimSpace(req.FactKey))
	content := notifier.StripEmoji(strings.TrimSpace(req.Content))
	cat := notifier.StripEmoji(strings.TrimSpace(req.Category))
	scope := notifier.StripEmoji(strings.TrimSpace(req.Scope))
	if key == "" || content == "" {
		writeError(w, http.StatusBadRequest, "记忆键名与内容不能为空")
		return
	}
	imp := req.Importance
	if imp <= 0 {
		imp = 3
	}
	if err := s.store.SaveAgentMemoryDetailed(scope, cat, key, content, imp, req.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, "保存记忆失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdateAgentMemory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效记忆 ID")
		return
	}
	existing, err := s.store.GetAgentMemory(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "查询记忆失败: "+err.Error())
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "记忆条目不存在")
		return
	}

	var req struct {
		Scope      *string `json:"scope"`
		Category   *string `json:"category"`
		FactKey    *string `json:"fact_key"`
		Content    *string `json:"content"`
		Importance *int    `json:"importance"`
		Pinned     *bool   `json:"pinned"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	scope := existing.Scope
	if req.Scope != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Scope)); v != "" {
			scope = v
		}
	}
	cat := existing.Category
	if req.Category != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Category)); v != "" {
			cat = v
		}
	}
	key := existing.FactKey
	if req.FactKey != nil {
		key = notifier.StripEmoji(strings.TrimSpace(*req.FactKey))
	}
	content := existing.Content
	if req.Content != nil {
		content = notifier.StripEmoji(strings.TrimSpace(*req.Content))
	}
	if key == "" || content == "" {
		writeError(w, http.StatusBadRequest, "记忆键名与内容不能为空")
		return
	}
	imp := existing.Importance
	if req.Importance != nil && *req.Importance > 0 {
		imp = *req.Importance
	}
	pinned := existing.Pinned
	if req.Pinned != nil {
		pinned = *req.Pinned
	}
	if err := s.store.UpdateAgentMemory(id, scope, cat, key, content, imp, pinned); err != nil {
		writeError(w, http.StatusInternalServerError, "更新记忆失败: "+err.Error())
		return
	}
	updated, _ := s.store.GetAgentMemory(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "memory": updated})
}

func (s *Server) handleDeleteAgentMemory(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "无效记忆 ID")
		return
	}
	if err := s.store.DeleteAgentMemory(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除记忆失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePruneAgentMemories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope   string `json:"scope"`
		MaxKeep int    `json:"max_keep"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		writeError(w, http.StatusBadRequest, "必须指定清理的作用域 scope")
		return
	}
	if req.MaxKeep <= 0 {
		req.MaxKeep = 25
	}
	removed, err := s.store.PruneColdMemories(scope, req.MaxKeep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "整理冷门记忆失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"scope":   scope,
		"removed": removed,
	})
}
