package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

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
	if !decodeJSON(w, r, 16*1024, &req) {
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
	var addedNames []string
	var removedNames []string
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
			addedNames = append(addedNames, b.Name)
		}
	case "remove":
		rmSet := make(map[string]bool, len(req.BotIDs))
		for _, bid := range req.BotIDs {
			bid = strings.TrimSpace(bid)
			rmSet[bid] = true
			if b, ok := botMap[bid]; ok && curSet[bid] {
				changedNames = append(changedNames, b.Name)
				removedNames = append(removedNames, b.Name)
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
			if b, ok := botMap[bid]; ok && !seen[bid] {
				seen[bid] = true
				next = append(next, bid)
				if !curSet[bid] {
					addedNames = append(addedNames, b.Name)
					changedNames = append(changedNames, b.Name)
				}
			}
		}
		for _, mid := range current {
			if !seen[mid] {
				if b, ok := botMap[mid]; ok {
					removedNames = append(removedNames, b.Name)
					changedNames = append(changedNames, b.Name)
				}
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

	var noticeParts []string
	if len(addedNames) > 0 {
		noticeParts = append(noticeParts, fmt.Sprintf("你邀请了「%s」加入群聊", strings.Join(addedNames, "、")))
	}
	if len(removedNames) > 0 {
		noticeParts = append(noticeParts, fmt.Sprintf("「%s」已被移出群聊", strings.Join(removedNames, "、")))
	}
	if len(noticeParts) > 0 {
		if action == "add" && len(removedNames) == 0 {
			groupBot.LastSummary = fmt.Sprintf("已邀请 %s 加入群聊", strings.Join(addedNames, "、"))
		} else if action == "remove" && len(addedNames) == 0 {
			groupBot.LastSummary = fmt.Sprintf("%s 已移出群聊", strings.Join(removedNames, "、"))
		} else {
			groupBot.LastSummary = strings.Join(noticeParts, "；")
		}
	}
	if err := s.store.SaveBot(*groupBot); err != nil {
		writeError(w, http.StatusInternalServerError, "更新群成员失败: "+err.Error())
		return
	}

	var sysMsg *store.AgentMessage
	if len(noticeParts) > 0 {
		sysMsg = s.appendGroupSystemNotice(groupBot.ID, strings.Join(noticeParts, "；"))
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
	if !decodeJSON(w, r, 16*1024, &req) {
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
