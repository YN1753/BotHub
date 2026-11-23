package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/notifier"
)

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	var req agent.ChatRequest
	if !decodeJSON(w, r, 64*1024, &req) {
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "对话内容不能为空")
		return
	}

	flusher, ok := startSSEStream(w)
	if !ok {
		return
	}
	flusher.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	var sseMu sync.Mutex
	emitSSE := func(event string, data any) {
		b, err := json.Marshal(data)
		if err != nil {
			return
		}
		sseMu.Lock()
		defer sseMu.Unlock()
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(b))
		flusher.Flush()
	}

	// Send periodic SSE keepalive comments so reverse proxies never drop slow reasoning/tool streams
	hbDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(12 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-hbDone:
				return
			case <-ticker.C:
				sseMu.Lock()
				_, _ = fmt.Fprint(w, ": keepalive\n\n")
				flusher.Flush()
				sseMu.Unlock()
			}
		}
	}()
	defer close(hbDone)

	if _, err := s.agent.ChatStream(ctx, req, emitSSE); err != nil {
		if ctx.Err() == nil {
			emitSSE("error", map[string]any{
				"error": notifier.StripEmoji(err.Error()),
			})
		}
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
	if !decodeJSON(w, r, 16384, &req) {
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
	imp := clampMemoryImportance(req.Importance)
	if cat == "" {
		cat = "general"
	}
	if scope == "" {
		scope = "global"
	}
	if err := s.store.SaveAgentMemoryDetailed(scope, cat, key, content, imp, req.Pinned); err != nil {
		writeError(w, http.StatusInternalServerError, "保存记忆失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("memories")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func clampMemoryImportance(v int) int {
	if v <= 0 {
		return 3
	}
	if v > 5 {
		return 5
	}
	return v
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
	if !decodeJSON(w, r, 16384, &req) {
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
	if req.Importance != nil {
		imp = clampMemoryImportance(*req.Importance)
	}
	pinned := existing.Pinned
	if req.Pinned != nil {
		pinned = *req.Pinned
	}
	if err := s.store.UpdateAgentMemory(id, scope, cat, key, content, imp, pinned); err != nil {
		writeError(w, http.StatusInternalServerError, "更新记忆失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("memories")
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
	s.publishAssistantEvent("memories")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePruneAgentMemories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope   string `json:"scope"`
		MaxKeep int    `json:"max_keep"`
	}
	if !decodeJSON(w, r, 8192, &req) {
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
	if removed > 0 {
		s.publishAssistantEvent("memories")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"scope":   scope,
		"removed": removed,
	})
}

func (s *Server) handleConsolidateAgentMemories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"`
	}
	if !decodeJSON(w, r, 4096, &req) {
		return
	}
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "all"
	}
	merged, err := s.store.MergeDuplicateMemories(scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "合并重复记忆失败: "+err.Error())
		return
	}
	if merged > 0 {
		s.publishAssistantEvent("memories")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"scope":  scope,
		"merged": merged,
	})
}

func (s *Server) handleDreamAgentMemories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scope string `json:"scope"`
	}
	_ = decodeOptionalJSON(w, r, 4096, &req)
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "all"
	}
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "Agent 引擎未初始化")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	rep, err := s.agent.RunAutoDream(ctx, scope, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AutoDream 记忆整理失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"report": rep,
	})
}

func (s *Server) handleGetMemoryMarkdown(w http.ResponseWriter, r *http.Request) {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	var scopes []string
	label := "全部作用域"
	if scope != "" && !strings.EqualFold(scope, "all") {
		scopes = []string{scope}
		label = scope
	}
	mems, err := s.store.ListScopedMemories(scopes, "", 160)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 MEMORY.md 视图失败: "+err.Error())
		return
	}
	md := agent.RenderMemoryIndexMarkdown(mems, mems, label, true)
	lastDreamAt, _ := s.store.GetConfig(agent.ConfigKeyLastAutoDreamAt)
	lastDreamRaw, _ := s.store.GetConfig(agent.ConfigKeyLastAutoDreamReport)
	var lastDreamReport map[string]any
	if strings.TrimSpace(lastDreamRaw) != "" {
		_ = json.Unmarshal([]byte(lastDreamRaw), &lastDreamReport)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"scope":             label,
		"count":             len(mems),
		"markdown":          md,
		"last_dream_at":     lastDreamAt,
		"last_dream_report": lastDreamReport,
	})
}
