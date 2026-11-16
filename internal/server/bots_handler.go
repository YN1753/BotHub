package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	case "sequential", "mention_only":
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
	req.SystemPrompt = notifier.StripEmoji(strings.TrimSpace(req.SystemPrompt))
	if req.SystemPrompt == "" {
		if req.IsGroup {
			req.SystemPrompt = "你是多 Bot 协同群聊的统筹协调官，请结合群内各专家视角输出简洁、精准、零冗余的结论，严禁使用 Emoji。"
		} else {
			req.SystemPrompt = "你是用户的专属 AI 助手，回答高信息密度、结构清晰，严禁使用 Emoji。"
		}
	}
	if strings.TrimSpace(req.MemberIDs) == "" {
		req.MemberIDs = "[]"
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
		Model          string  `json:"model"`
		EnableThinking *bool   `json:"enable_thinking"`
		AllowedTools   string  `json:"allowed_tools"`
		MemberIDs      string  `json:"member_ids"`
		GroupMode      *string `json:"group_mode"`
		Announcement   *string `json:"announcement"`
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
	if v := strings.TrimSpace(req.AvatarStyle); v != "" {
		existing.AvatarStyle = v
	}
	if v := strings.TrimSpace(req.Model); v != "" {
		existing.Model = v
	}
	if req.AllowedTools != "" {
		existing.AllowedTools = req.AllowedTools
	}
	if req.MemberIDs != "" {
		existing.MemberIDs = req.MemberIDs
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
	if (existing.IsGroup || existing.ID == "team_room") && prevName != existing.Name {
		_ = s.appendGroupSystemNotice(existing.ID, fmt.Sprintf("群聊名称已修改为「%s」", existing.Name))
	}
	if (existing.IsGroup || existing.ID == "team_room") && announcementChanged {
		_ = s.appendGroupSystemNotice(existing.ID, "群公告已更新："+existing.Announcement)
	}
	updated, _ := s.store.GetBot(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"bot": updated,
	})
}

func (s *Server) handleUpdateGroupMembers(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	groupBot, err := s.store.GetBot(id)
	if err != nil || groupBot == nil {
		writeError(w, http.StatusNotFound, "群聊不存在")
		return
	}
	if !groupBot.IsGroup && groupBot.ID != "team_room" {
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
		if !b.IsGroup && b.ID != "team_room" {
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
		if !b.IsGroup && b.ID != "team_room" {
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

	rawMembers, _ := json.Marshal(members)
	newGroup := store.Bot{
		ID:             newBotID(true),
		Name:           groupName,
		RoleTitle:      fmt.Sprintf("临时协同群 (%s)", strings.Join(memberNames, " · ")),
		AvatarStyle:    "swarm:gold",
		SystemPrompt:   "你是临时多人协同讨论群的协调官，汇总各专家成员的专业视角后输出结构化决策，严禁使用 Emoji。",
		Model:          store.DefaultAIModel,
		EnableThinking: true,
		AllowedTools:   `["read_url","web_search","query_nezha_nodes","manage_assets","manage_tasks","remember_fact","recall_memory"]`,
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
