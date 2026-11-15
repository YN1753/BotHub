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
	memories, err := s.store.ListAgentMemories(query, 100)
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
		Category string `json:"category"`
		FactKey  string `json:"fact_key"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	key := notifier.StripEmoji(strings.TrimSpace(req.FactKey))
	content := notifier.StripEmoji(strings.TrimSpace(req.Content))
	cat := notifier.StripEmoji(strings.TrimSpace(req.Category))
	if key == "" || content == "" {
		writeError(w, http.StatusBadRequest, "记忆键名与内容不能为空")
		return
	}
	if err := s.store.SaveAgentMemory(cat, key, content); err != nil {
		writeError(w, http.StatusInternalServerError, "保存记忆失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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
