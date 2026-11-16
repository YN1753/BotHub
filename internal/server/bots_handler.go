package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func newBotID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "bot_" + hex.EncodeToString(b)
}

func (s *Server) handleListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := s.store.ListBots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取专属 Bot 列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"bots": bots,
	})
}

func (s *Server) handleCreateBot(w http.ResponseWriter, r *http.Request) {
	var req store.Bot
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		req.ID = newBotID()
	}
	req.Name = notifier.StripEmoji(strings.TrimSpace(req.Name))
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Bot 名称不能为空")
		return
	}
	req.RoleTitle = notifier.StripEmoji(strings.TrimSpace(req.RoleTitle))
	if req.RoleTitle == "" {
		req.RoleTitle = "专属智能助理"
	}
	req.SystemPrompt = notifier.StripEmoji(strings.TrimSpace(req.SystemPrompt))
	if req.SystemPrompt == "" {
		req.SystemPrompt = "你是用户的专属 AI 助手，回答高信息密度、结构清晰，严禁使用 Emoji。"
	}
	if req.SortOrder == 0 {
		req.SortOrder = 20
	}
	req.IsBuiltin = false
	req.State = store.BotStateIdle
	req.LastSummary = "已就绪，随时待命"

	if err := s.store.SaveBot(req); err != nil {
		writeError(w, http.StatusInternalServerError, "创建 Bot 失败: "+err.Error())
		return
	}
	created, _ := s.store.GetBot(req.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"bot": created,
	})
}

func (s *Server) handleUpdateBot(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetBot(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}

	var req store.Bot
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	if v := notifier.StripEmoji(strings.TrimSpace(req.Name)); v != "" {
		existing.Name = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(req.RoleTitle)); v != "" {
		existing.RoleTitle = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(req.SystemPrompt)); v != "" {
		existing.SystemPrompt = v
	}
	if v := strings.TrimSpace(req.AvatarStyle); v != "" {
		existing.AvatarStyle = v
	}
	if v := strings.TrimSpace(req.Model); v != "" {
		existing.Model = v
	}
	if req.AllowedTools != "" {
		existing.AllowedTools = req.AllowedTools
	}
	existing.EnableThinking = req.EnableThinking
	existing.IsPinned = req.IsPinned

	if err := s.store.SaveBot(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "更新 Bot 失败: "+err.Error())
		return
	}
	updated, _ := s.store.GetBot(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"bot": updated,
	})
}

func (s *Server) handleDeleteBot(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetBot(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	if existing.IsBuiltin {
		writeError(w, http.StatusBadRequest, "内置核心 Bot 团队成员不可删除")
		return
	}
	if err := s.store.DeleteBot(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除 Bot 失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(id)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	msgs, err := s.store.ListBotMessages(id, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 Bot 对话流失败: "+err.Error())
		return
	}
	_ = s.store.ClearBotUnread(id)
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       id,
			"state":        store.BotStateIdle,
			"unread_count": 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"bot":      bot,
		"messages": msgs,
	})
}

func (s *Server) handleClearBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if err := s.store.ClearBotMessages(id); err != nil {
		writeError(w, http.StatusInternalServerError, "清空 Bot 对话流失败: "+err.Error())
		return
	}
	_ = s.store.ClearBotUnread(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
