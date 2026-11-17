package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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

func (s *Server) appendGroupSystemNotice(botID, content string) *store.AgentMessage {
	msg := store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		MsgKind:   "system",
		Role:      "system",
		Content:   notifier.StripEmoji(strings.TrimSpace(content)),
	}
	id, err := s.store.AppendAgentMessage(msg)
	if err != nil {
		return nil
	}
	msg.ID = id
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", msg)
	}
	return &msg
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
		Name           string  `json:"name"`
		RoleTitle      string  `json:"role_title"`
		AvatarStyle    string  `json:"avatar_style"`
		SystemPrompt   string  `json:"system_prompt"`
		BotMode        *string `json:"bot_mode"`
		MemoryPolicy   *string `json:"memory_policy"`
		Traits         *string `json:"traits"`
		Model          string  `json:"model"`
		EnableThinking *bool   `json:"enable_thinking"`
		AllowedTools   string  `json:"allowed_tools"`
		SkillIDs       *string `json:"skill_ids"`
		MemberIDs      string  `json:"member_ids"`
		GroupMode      *string `json:"group_mode"`
		Announcement   *string `json:"announcement"`
		ActiveThreadID *string `json:"active_thread_id"`
		IsPinned       *bool   `json:"is_pinned"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
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

func (s *Server) handleUpdateGroupMembers(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	groupBot, err := s.store.GetBot(id)
	if err != nil || groupBot == nil {
		writeError(w, http.StatusNotFound, "群聊不存在")
		return
	}
	if !groupBot.IsGroup {
		writeError(w, http.StatusBadRequest, "仅支持管理群聊成员，单聊请使用「临时拉人建群」")
		return
	}

	var req struct {
		Action string   `json:"action"` // add | remove | set
		BotIDs []string `json:"bot_ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	allBots, _ := s.store.ListBots()
	botMap := make(map[string]store.Bot, len(allBots))
	for _, b := range allBots {
		if !b.IsGroup {
			botMap[b.ID] = b
		}
	}

	current := groupBot.MemberIDList()
	curSet := make(map[string]bool, len(current))
	for _, mid := range current {
		curSet[mid] = true
	}

	var changedNames []string
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "add"
	}

	switch action {
	case "add":
		for _, bid := range req.BotIDs {
			bid = strings.TrimSpace(bid)
			b, ok := botMap[bid]
			if !ok || curSet[bid] {
				continue
			}
			curSet[bid] = true
			current = append(current, bid)
			changedNames = append(changedNames, b.Name)
		}
	case "remove":
		rmSet := make(map[string]bool, len(req.BotIDs))
		for _, bid := range req.BotIDs {
			bid = strings.TrimSpace(bid)
			rmSet[bid] = true
			if b, ok := botMap[bid]; ok && curSet[bid] {
				changedNames = append(changedNames, b.Name)
			}
		}
		var next []string
		for _, mid := range current {
			if !rmSet[mid] {
				next = append(next, mid)
			}
		}
		current = next
	case "set":
		seen := make(map[string]bool)
		var next []string
		for _, bid := range req.BotIDs {
			bid = strings.TrimSpace(bid)
			if _, ok := botMap[bid]; ok && !seen[bid] {
				seen[bid] = true
				next = append(next, bid)
			}
		}
		current = next
	default:
		writeError(w, http.StatusBadRequest, "不支持的操作类型")
		return
	}

	if current == nil {
		current = []string{}
	}
	rawMembers, _ := json.Marshal(current)
	groupBot.MemberIDs = string(rawMembers)
	if len(changedNames) > 0 {
		if action == "add" {
			groupBot.LastSummary = fmt.Sprintf("已邀请 %s 加入群聊", strings.Join(changedNames, "、"))
		} else if action == "remove" {
			groupBot.LastSummary = fmt.Sprintf("%s 已移出群聊", strings.Join(changedNames, "、"))
		}
	}
	if err := s.store.SaveBot(*groupBot); err != nil {
		writeError(w, http.StatusInternalServerError, "更新群成员失败: "+err.Error())
		return
	}

	var sysMsg *store.AgentMessage
	if len(changedNames) > 0 {
		if action == "add" {
			sysMsg = s.appendGroupSystemNotice(groupBot.ID, fmt.Sprintf("你邀请了「%s」加入群聊", strings.Join(changedNames, "、")))
		} else if action == "remove" {
			sysMsg = s.appendGroupSystemNotice(groupBot.ID, fmt.Sprintf("「%s」已被移出群聊", strings.Join(changedNames, "、")))
		}
	}

	updated, _ := s.store.GetBot(groupBot.ID)
	if s.push != nil && s.push.Hub() != nil && updated != nil {
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       updated.ID,
			"state":        updated.State,
			"last_summary": updated.LastSummary,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"bot":            updated,
		"system_message": sysMsg,
	})
}

func (s *Server) handleCreateAdhocGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceBotID  string   `json:"source_bot_id"`
		MemberIDs    []string `json:"member_ids"`
		Name         string   `json:"name"`
		GroupMode    string   `json:"group_mode"`
		CarryContext bool     `json:"carry_context"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	allBots, _ := s.store.ListBots()
	botMap := make(map[string]store.Bot, len(allBots))
	for _, b := range allBots {
		if !b.IsGroup {
			botMap[b.ID] = b
		}
	}

	seen := make(map[string]bool)
	members := make([]string, 0, len(req.MemberIDs)+1)
	memberNames := make([]string, 0, len(req.MemberIDs)+1)
	if srcID := strings.TrimSpace(req.SourceBotID); srcID != "" {
		if b, ok := botMap[srcID]; ok {
			seen[srcID] = true
			members = append(members, srcID)
			memberNames = append(memberNames, b.Name)
		}
	}
	for _, bid := range req.MemberIDs {
		bid = strings.TrimSpace(bid)
		if b, ok := botMap[bid]; ok && !seen[bid] {
			seen[bid] = true
			members = append(members, bid)
			memberNames = append(memberNames, b.Name)
		}
	}
	if len(members) < 2 {
		writeError(w, http.StatusBadRequest, "发起群聊至少需要 2 位成员")
		return
	}

	groupName := notifier.StripEmoji(strings.TrimSpace(req.Name))
	if groupName == "" {
		if len(memberNames) == 2 {
			groupName = fmt.Sprintf("%s × %s 协同群", memberNames[0], memberNames[1])
		} else {
			groupName = fmt.Sprintf("%s 等 %d 人讨论群", memberNames[0], len(memberNames))
		}
	}

	toolSet := make(map[string]bool)
	skillSet := make(map[string]bool)
	mergedTools := make([]string, 0)
	mergedSkills := make([]string, 0)
	allCompanion := true
	hasWorker := false
	groupModel := ""
	for _, mid := range members {
		m := botMap[mid]
		if groupModel == "" && strings.TrimSpace(m.Model) != "" {
			groupModel = strings.TrimSpace(m.Model)
		}
		if m.BotMode != store.BotModeCompanion {
			allCompanion = false
		}
		if m.BotMode == store.BotModeWorker {
			hasWorker = true
		}
		for _, t := range m.AllowedToolList() {
			if !toolSet[t] {
				toolSet[t] = true
				mergedTools = append(mergedTools, t)
			}
		}
		for _, sid := range m.SkillIDList() {
			if !skillSet[sid] {
				skillSet[sid] = true
				mergedSkills = append(mergedSkills, sid)
			}
		}
	}
	if groupModel == "" {
		if cfgModel, _ := s.store.GetConfig(store.KeyAIModel); strings.TrimSpace(cfgModel) != "" {
			groupModel = strings.TrimSpace(cfgModel)
		} else {
			groupModel = store.DefaultAIModel
		}
	}
	derivedMode := store.BotModeHybrid
	if allCompanion {
		derivedMode = store.BotModeCompanion
	} else if hasWorker && len(mergedTools)+len(mergedSkills) > 0 {
		derivedMode = store.BotModeWorker
	}
	rawTools, _ := json.Marshal(mergedTools)
	rawSkills, _ := json.Marshal(mergedSkills)
	rawMembers, _ := json.Marshal(members)
	newGroup := store.Bot{
		ID:             newBotID(true),
		Name:           groupName,
		RoleTitle:      fmt.Sprintf("临时协同群 (%s)", strings.Join(memberNames, " · ")),
		AvatarStyle:    "swarm:gold",
		SystemPrompt:   "根据每位群成员的性格、专业与分工自然协作交流，必要时汇总决策或轻松互动，严禁使用 Emoji。",
		Model:          groupModel,
		EnableThinking: !allCompanion,
		AllowedTools:   string(rawTools),
		BotMode:        derivedMode,
		MemoryPolicy:   store.MemoryPolicyStandard,
		SkillIDs:       string(rawSkills),
		State:          store.BotStateIdle,
		IsGroup:        true,
		MemberIDs:      string(rawMembers),
		GroupMode:      normalizeGroupMode(req.GroupMode),
		SortOrder:      5,
		LastSummary:    fmt.Sprintf("已拉入 %s 开始群聊", strings.Join(memberNames, "、")),
	}
	if err := s.store.SaveBot(newGroup); err != nil {
		writeError(w, http.StatusInternalServerError, "创建协同群聊失败: "+err.Error())
		return
	}

	_ = s.appendGroupSystemNotice(newGroup.ID, fmt.Sprintf("你拉起了新群聊，成员：%s", strings.Join(memberNames, "、")))

	if req.CarryContext && req.SourceBotID != "" {
		if srcMsgs, err := s.store.ListBotMessages(req.SourceBotID, 8); err == nil && len(srcMsgs) > 0 {
			var lines []string
			for _, m := range srcMsgs {
				if m.Role != "user" && m.Role != "assistant" {
					continue
				}
				label := "管理员"
				if m.Role == "assistant" {
					if sp, ok := botMap[m.SenderBotID]; ok {
						label = sp.Name
					} else {
						label = "Bot"
					}
				}
				text := strings.Join(strings.Fields(m.Content), " ")
				runes := []rune(text)
				if len(runes) > 90 {
					text = string(runes[:90]) + "..."
				}
				lines = append(lines, fmt.Sprintf("%s: %s", label, text))
				if len(lines) >= 4 {
					break
				}
			}
			if len(lines) > 0 {
				srcName := req.SourceBotID
				if sb, ok := botMap[req.SourceBotID]; ok {
					srcName = sb.Name
				}
				_ = s.appendGroupSystemNotice(newGroup.ID, fmt.Sprintf("已同步「%s」最近上下文：\n%s", srcName, strings.Join(lines, "\n")))
			}
		}
	}

	created, _ := s.store.GetBot(newGroup.ID)
	s.publishAssistantEvent("bots")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"bot": created,
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

func (s *Server) handleGetBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(id)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}

	threadID := strings.TrimSpace(r.URL.Query().Get("thread_id"))
	hasThreadParam := r.URL.Query().Has("thread_id")
	if !hasThreadParam {
		threadID = strings.TrimSpace(bot.ActiveThreadID)
	} else if threadID != strings.TrimSpace(bot.ActiveThreadID) {
		_ = s.store.SetBotActiveThread(id, threadID)
		bot.ActiveThreadID = threadID
	}

	msgs, err := s.store.ListBotMessagesByThread(id, threadID, 120)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 Bot 对话流失败: "+err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(id)
	memScopes := []string{"bot:" + id}
	if bot.MemoryPolicy != store.MemoryPolicyIsolated {
		memScopes = append(memScopes, "global")
	}
	memories, _ := s.store.ListScopedMemories(memScopes, "", 60)
	executions, _ := s.store.ListBotExecutions(id, 40)

	_ = s.store.ClearBotUnread(id)
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       id,
			"state":        store.BotStateIdle,
			"unread_count": 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"bot":        bot,
		"thread_id":  threadID,
		"messages":   msgs,
		"threads":    threads,
		"memories":   memories,
		"executions": executions,
	})
}

func (s *Server) handleClearBotMessages(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if r.URL.Query().Has("thread_id") {
		tid := strings.TrimSpace(r.URL.Query().Get("thread_id"))
		if err := s.store.ClearBotMessagesInThread(id, tid); err != nil {
			writeError(w, http.StatusInternalServerError, "清空话题对话流失败: "+err.Error())
			return
		}
	} else if err := s.store.ClearBotMessages(id); err != nil {
		writeError(w, http.StatusInternalServerError, "清空 Bot 对话流失败: "+err.Error())
		return
	}
	_ = s.store.ClearBotUnread(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes)).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if maxRunes > 0 && len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return s
}

func (s *Server) handleCreateBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := decodeOptionalJSON(w, r, 8*1024, &req); err != nil {
		writeError(w, http.StatusBadRequest, "话题请求格式错误")
		return
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	threadID := "th_" + hex.EncodeToString(b)
	title := truncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Title)), 80)
	if title == "" {
		title = "新话题"
	}
	th := store.BotThread{
		ID:    threadID,
		BotID: botID,
		Title: title,
	}
	if err := s.store.SaveBotThread(th); err != nil {
		writeError(w, http.StatusInternalServerError, "创建话题失败: "+err.Error())
		return
	}
	_ = s.store.SetBotActiveThread(botID, threadID)
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"thread":  th,
		"threads": threads,
	})
}

func (s *Server) handleForkBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}
	var req struct {
		SourceThreadID string `json:"source_thread_id"`
		UpToMsgID      int64  `json:"up_to_msg_id"`
		Title          string `json:"title"`
	}
	if err := decodeOptionalJSON(w, r, 8*1024, &req); err != nil {
		writeError(w, http.StatusBadRequest, "分支请求格式错误")
		return
	}

	srcMsgs, _ := s.store.ListBotMessagesByThread(botID, strings.TrimSpace(req.SourceThreadID), 60)
	var contextSlice []store.AgentMessage
	for _, m := range srcMsgs {
		if req.UpToMsgID > 0 && m.ID > req.UpToMsgID {
			break
		}
		if m.Role == "user" || m.Role == "assistant" {
			contextSlice = append(contextSlice, m)
		}
	}
	if len(contextSlice) > 12 {
		contextSlice = contextSlice[len(contextSlice)-12:]
	}

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	newThreadID := "th_" + hex.EncodeToString(b)
	title := truncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Title)), 80)
	if title == "" {
		if len(contextSlice) > 0 {
			lastTxt := strings.TrimSpace(contextSlice[len(contextSlice)-1].Content)
			runes := []rune(lastTxt)
			if len(runes) > 14 {
				lastTxt = string(runes[:14]) + "..."
			}
			title = "分支: " + lastTxt
		} else {
			title = "新话题分支"
		}
	}

	th := store.BotThread{
		ID:    newThreadID,
		BotID: botID,
		Title: title,
	}
	if err := s.store.SaveBotThread(th); err != nil {
		writeError(w, http.StatusInternalServerError, "创建分支话题失败: "+err.Error())
		return
	}
	_ = s.store.SetBotActiveThread(botID, newThreadID)

	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		ThreadID:  newThreadID,
		MsgKind:   "system",
		Role:      "system",
		Content:   fmt.Sprintf("已从历史对话分叉开启话题「%s」（继承 %d 条上下文）", title, len(contextSlice)),
	})
	for _, m := range contextSlice {
		_, _ = s.store.AppendAgentMessage(store.AgentMessage{
			SessionID:    "bot:" + botID,
			BotID:        botID,
			ThreadID:     newThreadID,
			SenderBotID:  m.SenderBotID,
			MsgKind:      m.MsgKind,
			Role:         m.Role,
			Content:      m.Content,
			ReplyPreview: m.ReplyPreview,
			ModelUsed:    m.ModelUsed,
		})
	}

	threads, _ := s.store.ListBotThreads(botID)
	msgs, _ := s.store.ListBotMessagesByThread(botID, newThreadID, 60)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"thread":   th,
		"threads":  threads,
		"messages": msgs,
	})
}

func (s *Server) handleUpdateBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	th, err := s.store.GetBotThread(botID, threadID)
	if err != nil || th == nil {
		writeError(w, http.StatusNotFound, "话题不存在")
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Archived *bool   `json:"archived"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	if req.Title != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Title)); v != "" {
			th.Title = v
		}
	}
	if req.Archived != nil {
		th.Archived = *req.Archived
	}
	if err := s.store.SaveBotThread(*th); err != nil {
		writeError(w, http.StatusInternalServerError, "更新话题失败: "+err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "thread": th, "threads": threads})
}

func (s *Server) handleDeleteBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	if err := s.store.DeleteBotThread(botID, threadID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	threads, _ := s.store.ListBotThreads(botID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "threads": threads})
}

func (s *Server) handleSummarizeBotThread(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimSpace(r.PathValue("id"))
	threadID := strings.TrimSpace(r.PathValue("threadId"))
	if threadID == "main" {
		threadID = ""
	}
	bot, err := s.store.GetBot(botID)
	if err != nil || bot == nil {
		writeError(w, http.StatusNotFound, "Bot 不存在")
		return
	}

	var req struct {
		SaveToMemory *bool `json:"save_to_memory"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	saveMem := true
	if req.SaveToMemory != nil {
		saveMem = *req.SaveToMemory
	}

	var th *store.BotThread
	if threadID != "" {
		th, err = s.store.GetBotThread(botID, threadID)
		if err != nil || th == nil {
			writeError(w, http.StatusNotFound, "话题不存在")
			return
		}
	}

	msgs, err := s.store.ListBotMessagesByThread(botID, threadID, 80)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取话题记录失败: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	summary, savedFactKey, err := s.agent.SummarizeThreadToMemory(ctx, bot, th, msgs, saveMem)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	threads, _ := s.store.ListBotThreads(botID)
	memScopes := []string{"bot:" + botID}
	if bot.MemoryPolicy != store.MemoryPolicyIsolated {
		memScopes = append(memScopes, "global")
	}
	memories, _ := s.store.ListScopedMemories(memScopes, "", 60)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"summary":        summary,
		"saved_fact_key": savedFactKey,
		"threads":        threads,
		"memories":       memories,
	})
}

func (s *Server) handleDeleteBotMessage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	msgID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("msgId")), 10, 64)
	if err != nil || msgID <= 0 {
		writeError(w, http.StatusBadRequest, "无效消息 ID")
		return
	}
	if err := s.store.DeleteBotMessage(id, msgID); err != nil {
		writeError(w, http.StatusInternalServerError, "删除消息失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleForwardBotMessage(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(r.PathValue("id"))
	targetBot, err := s.store.GetBot(targetID)
	if err != nil || targetBot == nil {
		writeError(w, http.StatusNotFound, "目标会话不存在")
		return
	}

	var req struct {
		FromName string `json:"from_name"`
		Content  string `json:"content"`
		Note     string `json:"note"`
		ThreadID string `json:"thread_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效转发请求体")
		return
	}

	cleanContent := notifier.StripEmoji(strings.TrimSpace(req.Content))
	if cleanContent == "" {
		writeError(w, http.StatusBadRequest, "转发内容不能为空")
		return
	}
	fromName := notifier.StripEmoji(strings.TrimSpace(req.FromName))
	if fromName == "" {
		fromName = "其他会话"
	}
	note := truncateRunes(notifier.StripEmoji(strings.TrimSpace(req.Note)), 500)
	targetThreadID := strings.TrimSpace(req.ThreadID)
	if targetThreadID == "" {
		targetThreadID = strings.TrimSpace(targetBot.ActiveThreadID)
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("[来自「%s」的转发记录]\n", fromName))
	body.WriteString(cleanContent)
	if note != "" {
		body.WriteString("\n\n附言：" + note)
	}

	sysMsg := store.AgentMessage{
		SessionID: "bot:" + targetBot.ID,
		BotID:     targetBot.ID,
		ThreadID:  targetThreadID,
		MsgKind:   "system",
		Role:      "system",
		Content:   fmt.Sprintf("已从「%s」转发一条消息", fromName),
	}
	if sid, sErr := s.store.AppendAgentMessage(sysMsg); sErr == nil {
		sysMsg.ID = sid
		if s.push != nil && s.push.Hub() != nil {
			s.push.Hub().PublishEvent("bot_message", sysMsg)
		}
	}
	sysNotice := &sysMsg

	fwdMsg := store.AgentMessage{
		SessionID: "bot:" + targetBot.ID,
		BotID:     targetBot.ID,
		ThreadID:  targetThreadID,
		MsgKind:   "chat",
		Role:      "user",
		Content:   body.String(),
	}
	id, err := s.store.AppendAgentMessage(fwdMsg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "转发写入失败: "+err.Error())
		return
	}
	fwdMsg.ID = id
	_ = s.store.IncrementBotUnread(targetBot.ID, fmt.Sprintf("收到来自「%s」的转发消息", fromName))
	updatedTarget, _ := s.store.GetBot(targetBot.ID)
	unread := 0
	if updatedTarget != nil {
		unread = updatedTarget.UnreadCount
	}
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("bot_message", fwdMsg)
		s.push.Hub().PublishEvent("bot_state", map[string]any{
			"bot_id":       targetBot.ID,
			"unread_count": unread,
			"last_summary": fmt.Sprintf("收到来自「%s」的转发消息", fromName),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"message":       fwdMsg,
		"system_notice": sysNotice,
	})
}
