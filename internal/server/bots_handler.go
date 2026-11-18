package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func newBotID(isGroup bool) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	if isGroup {
		return "group_" + hex.EncodeToString(b)
	}
	return "bot_" + hex.EncodeToString(b)
}

func normalizeGroupMode(v string) string {
	switch strings.TrimSpace(v) {
	case "sequential", "parallel", "mention_only":
		return strings.TrimSpace(v)
	default:
		return "smart"
	}
}

func normalizeJSONStringArray(raw, excludeID string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "[]", true
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return "", false
	}
	seen := make(map[string]bool, len(items))
	cleaned := make([]string, 0, len(items))
	for _, it := range items {
		v := strings.TrimSpace(it)
		if v == "" || (excludeID != "" && v == excludeID) || seen[v] {
			continue
		}
		seen[v] = true
		cleaned = append(cleaned, v)
	}
	out, _ := json.Marshal(cleaned)
	return string(out), true
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
	if !decodeJSON(w, r, 32*1024, &req) {
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		req.ID = newBotID(req.IsGroup)
	}
	req.Name = notifier.StripEmoji(strings.TrimSpace(req.Name))
	if req.Name == "" {
		if req.IsGroup {
			writeError(w, http.StatusBadRequest, "群聊名称不能为空")
		} else {
			writeError(w, http.StatusBadRequest, "Bot 名称不能为空")
		}
		return
	}
	req.RoleTitle = notifier.StripEmoji(strings.TrimSpace(req.RoleTitle))
	if req.RoleTitle == "" {
		if req.IsGroup {
			req.RoleTitle = "多 Bot 协同讨论群"
		} else {
			req.RoleTitle = "专属智能助理"
		}
	}
	req.BotMode = store.NormalizeBotMode(req.BotMode)
	req.MemoryPolicy = store.NormalizeMemoryPolicy(req.MemoryPolicy)
	req.Traits = notifier.StripEmoji(strings.TrimSpace(req.Traits))
	req.SystemPrompt = notifier.StripEmoji(strings.TrimSpace(req.SystemPrompt))
	if req.SystemPrompt == "" {
		if req.IsGroup {
			req.SystemPrompt = "你是协同群聊成员的统筹伙伴，结合各位成员专长简洁协作，严禁使用 Emoji。"
		} else if req.BotMode == store.BotModeCompanion {
			req.SystemPrompt = "你是一个真诚、有温度、有自己性格的聊天伙伴。像老朋友一样自然聊天陪伴，记住我们的点滴约定，严禁使用 Emoji。"
		} else if req.BotMode == store.BotModeWorker {
			req.SystemPrompt = "你是干练可靠的专业执行助手，优先调动绑定工具与技能核查真实数据并直接落地解决，严禁使用 Emoji。"
		} else {
			req.SystemPrompt = "你是用户的专属智能伙伴，既能自然沟通陪伴也能调动工具高效办事，严禁使用 Emoji。"
		}
	}
	if v, ok := normalizeJSONStringArray(req.AllowedTools, ""); !ok {
		writeError(w, http.StatusBadRequest, "allowed_tools 必须为有效 JSON 字符串数组")
		return
	} else {
		req.AllowedTools = v
	}
	if v, ok := normalizeJSONStringArray(req.MemberIDs, req.ID); !ok {
		writeError(w, http.StatusBadRequest, "member_ids 必须为有效 JSON 字符串数组")
		return
	} else {
		req.MemberIDs = v
	}
	if v, ok := normalizeJSONStringArray(req.SkillIDs, ""); !ok {
		writeError(w, http.StatusBadRequest, "skill_ids 必须为有效 JSON 字符串数组")
		return
	} else {
		req.SkillIDs = v
	}
	req.GroupMode = normalizeGroupMode(req.GroupMode)
	req.Announcement = notifier.StripEmoji(strings.TrimSpace(req.Announcement))
	if req.SortOrder == 0 {
		if req.IsGroup {
			req.SortOrder = 10
		} else {
			req.SortOrder = 20
		}
	}
	req.IsBuiltin = false
	req.State = store.BotStateIdle
	if req.IsGroup {
		req.LastSummary = "群聊已创建，可 @成员 或直接发起群组协同"
	} else {
		req.LastSummary = "已就绪，随时待命"
	}

	if err := s.store.SaveBot(req); err != nil {
		writeError(w, http.StatusInternalServerError, "创建 Bot 失败: "+err.Error())
		return
	}
	if req.IsGroup {
		_ = s.appendGroupSystemNotice(req.ID, "多 Bot 协同群聊已创建，支持右侧一键拉人与 @点名协同")
	}
	created, _ := s.store.GetBot(req.ID)
	s.publishAssistantEvent("bots")
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

	var req struct {
		Name              string  `json:"name"`
		RoleTitle         string  `json:"role_title"`
		AvatarStyle       string  `json:"avatar_style"`
		SystemPrompt      string  `json:"system_prompt"`
		BotMode           *string `json:"bot_mode"`
		MemoryPolicy      *string `json:"memory_policy"`
		Traits            *string `json:"traits"`
		Model             string  `json:"model"`
		AllowPaidFallback *bool   `json:"allow_paid_fallback"`
		EnableThinking    *bool   `json:"enable_thinking"`
		AllowedTools   string  `json:"allowed_tools"`
		SkillIDs       *string `json:"skill_ids"`
		MemberIDs      string  `json:"member_ids"`
		GroupMode      *string `json:"group_mode"`
		Announcement   *string `json:"announcement"`
		ActiveThreadID *string `json:"active_thread_id"`
		IsPinned       *bool   `json:"is_pinned"`
	}
	if !decodeJSON(w, r, 32*1024, &req) {
		return
	}

	prevName := existing.Name
	if v := notifier.StripEmoji(strings.TrimSpace(req.Name)); v != "" {
		existing.Name = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(req.RoleTitle)); v != "" {
		existing.RoleTitle = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(req.SystemPrompt)); v != "" {
		existing.SystemPrompt = v
	}
	if req.BotMode != nil {
		existing.BotMode = store.NormalizeBotMode(*req.BotMode)
	}
	if req.MemoryPolicy != nil {
		existing.MemoryPolicy = store.NormalizeMemoryPolicy(*req.MemoryPolicy)
	}
	if req.Traits != nil {
		existing.Traits = notifier.StripEmoji(strings.TrimSpace(*req.Traits))
	}
	if v := strings.TrimSpace(req.AvatarStyle); v != "" {
		existing.AvatarStyle = v
	}
	if v := strings.TrimSpace(req.Model); v != "" {
		existing.Model = v
	}
	if req.AllowedTools != "" {
		v, ok := normalizeJSONStringArray(req.AllowedTools, "")
		if !ok {
			writeError(w, http.StatusBadRequest, "allowed_tools 必须为有效 JSON 字符串数组")
			return
		}
		existing.AllowedTools = v
	}
	if req.SkillIDs != nil {
		v, ok := normalizeJSONStringArray(*req.SkillIDs, "")
		if !ok {
			writeError(w, http.StatusBadRequest, "skill_ids 必须为有效 JSON 字符串数组")
			return
		}
		existing.SkillIDs = v
	}
	if req.MemberIDs != "" {
		v, ok := normalizeJSONStringArray(req.MemberIDs, existing.ID)
		if !ok {
			writeError(w, http.StatusBadRequest, "member_ids 必须为有效 JSON 字符串数组")
			return
		}
		existing.MemberIDs = v
	}
	if req.ActiveThreadID != nil {
		existing.ActiveThreadID = strings.TrimSpace(*req.ActiveThreadID)
	}
	if req.EnableThinking != nil {
		existing.EnableThinking = *req.EnableThinking
	}
	if req.AllowPaidFallback != nil {
		existing.AllowPaidFallback = *req.AllowPaidFallback
	}
	if req.IsPinned != nil {
		existing.IsPinned = *req.IsPinned
	}
	if req.GroupMode != nil {
		existing.GroupMode = normalizeGroupMode(*req.GroupMode)
	}
	announcementChanged := false
	if req.Announcement != nil {
		cleanNotice := notifier.StripEmoji(strings.TrimSpace(*req.Announcement))
		if cleanNotice != existing.Announcement {
			existing.Announcement = cleanNotice
			announcementChanged = cleanNotice != ""
		}
	}

	if err := s.store.SaveBot(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "更新 Bot 失败: "+err.Error())
		return
	}
	if existing.IsGroup && prevName != existing.Name {
		_ = s.appendGroupSystemNotice(existing.ID, fmt.Sprintf("群聊名称已修改为「%s」", existing.Name))
	}
	if existing.IsGroup && announcementChanged {
		_ = s.appendGroupSystemNotice(existing.ID, "群公告已更新："+existing.Announcement)
	}
	updated, _ := s.store.GetBot(id)
	s.publishAssistantEvent("bots")
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
	if err := s.store.DeleteBot(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除 Bot 失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("bots")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
