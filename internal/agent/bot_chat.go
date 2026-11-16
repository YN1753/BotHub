package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func (e *Engine) emitBotState(botID, state, lastSummary string) {
	if botID == "" {
		return
	}
	_ = e.store.SetBotState(botID, state, lastSummary)
	if e.hub != nil {
		e.hub.PublishEvent("bot_state", map[string]any{
			"bot_id":       botID,
			"state":        state,
			"last_summary": lastSummary,
		})
	}
}

func filterToolsForBot(bot *store.Bot) []ToolDefinition {
	all := BuiltinTools()
	if bot == nil {
		return all
	}
	allowed := bot.AllowedToolList()
	if len(allowed) == 0 {
		return nil
	}
	allowSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowSet[strings.TrimSpace(name)] = true
	}
	filtered := make([]ToolDefinition, 0, len(allowed))
	for _, td := range all {
		if allowSet[td.Function.Name] {
			filtered = append(filtered, td)
		}
	}
	return filtered
}

func buildBotSystemPrompt(bot *store.Bot, memories []store.AgentMemory) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(bot.SystemPrompt))
	sb.WriteString("\n\n[专属身份与行为准则]\n")
	sb.WriteString(fmt.Sprintf("- 你的身份：%s (%s)\n", bot.Name, bot.RoleTitle))
	sb.WriteString("- 当前平台为 Vesper BotHub（零 Docker 单二进制架构 + SQLite WAL）。\n")
	sb.WriteString("- 严格禁止在任何回复中使用 Emoji 表情或装饰性特殊符号。\n")
	sb.WriteString("- 如果用户询问系统资产、定时任务、哪吒节点、实时网页或历史偏好，请优先调用绑定工具获取真实数据，不要凭空编造。\n")

	scopedMemories := make([]store.AgentMemory, 0, len(memories))
	botScope := "bot:" + bot.ID
	for _, m := range memories {
		sc := strings.TrimSpace(m.Scope)
		if sc == "" || sc == "global" || sc == botScope {
			scopedMemories = append(scopedMemories, m)
		}
	}
	if len(scopedMemories) > 0 {
		sb.WriteString("\n[长期记忆库背景]\n")
		for _, m := range scopedMemories {
			sb.WriteString(fmt.Sprintf("- (%s) %s: %s\n", m.Category, m.FactKey, m.Content))
		}
	}
	return sb.String()
}

func (e *Engine) resolveMentionedBots(userMsg string, allBots []store.Bot) []store.Bot {
	lower := strings.ToLower(userMsg)
	matched := make([]store.Bot, 0)
	seen := make(map[string]bool)
	for _, b := range allBots {
		if b.IsGroup || b.ID == "team_room" || seen[b.ID] {
			continue
		}
		if strings.Contains(userMsg, "@"+b.Name) || strings.Contains(lower, "@"+strings.ToLower(b.ID)) {
			seen[b.ID] = true
			matched = append(matched, b)
		}
	}
	return matched
}

func resolveGroupMemberBots(groupBot *store.Bot, allBots []store.Bot) []store.Bot {
	botMap := make(map[string]store.Bot, len(allBots))
	for _, b := range allBots {
		if !b.IsGroup && b.ID != "team_room" {
			botMap[b.ID] = b
		}
	}
	var members []store.Bot
	for _, id := range groupBot.MemberIDList() {
		if b, ok := botMap[strings.TrimSpace(id)]; ok {
			members = append(members, b)
		}
	}
	return members
}

func selectSmartGroupMembers(members []store.Bot, userMsg string) []store.Bot {
	if len(members) <= 1 {
		return members
	}
	lower := strings.ToLower(userMsg)
	// Explicit "all" collaboration request triggers all group members
	for _, kw := range []string{"大家", "全部", "各位", "一起", "全员", "接力", "群策群力"} {
		if strings.Contains(userMsg, kw) {
			return members
		}
	}
	type rule struct {
		botID    string
		keywords []string
	}
	rules := []rule{
		{botID: "sentinel", keywords: []string{"探针", "哪吒", "vps", "节点", "服务器", "cpu", "内存", "负载", "掉线", "宕机", "告警", "延迟", "ssh", "nezha"}},
		{botID: "butler", keywords: []string{"资产", "域名", "到期", "续费", "账单", "晨报", "定时", "例程", "cron", "日程", "会员", "cloudflare"}},
		{botID: "radar", keywords: []string{"github", "开源", "趋势", "雷达", "新闻", "前沿", "hn", "hacker", "技术", "搜索", "网页", "url", "推荐"}},
		{botID: "grok", keywords: []string{"架构", "锐评", "审查", "评审", "对比", "为什么", "设计", "优化", "方案", "分析", "总结"}},
	}

	picked := make([]store.Bot, 0, 2)
	seen := make(map[string]bool)
	memberMap := make(map[string]store.Bot, len(members))
	for _, m := range members {
		memberMap[m.ID] = m
	}

	for _, r := range rules {
		sp, exists := memberMap[r.botID]
		if !exists || seen[sp.ID] {
			continue
		}
		for _, kw := range r.keywords {
			if strings.Contains(lower, kw) {
				seen[sp.ID] = true
				picked = append(picked, sp)
				break
			}
		}
		if len(picked) >= 2 {
			break
		}
	}
	// Also score custom specialist bots by matching role_title / name words
	if len(picked) == 0 {
		for _, m := range members {
			roleTokens := strings.Fields(strings.ToLower(m.RoleTitle + " " + m.Name))
			for _, tk := range roleTokens {
				if len([]rune(tk)) >= 2 && strings.Contains(lower, tk) {
					picked = append(picked, m)
					seen[m.ID] = true
					break
				}
			}
			if len(picked) >= 2 {
				break
			}
		}
	}
	if len(picked) > 0 {
		return picked
	}
	// Default smart lead: prefer grok or butler if in room, else first member
	for _, preferred := range []string{"grok", "butler"} {
		if sp, ok := memberMap[preferred]; ok {
			return []store.Bot{sp}
		}
	}
	return members[:1]
}

func (e *Engine) chatStreamForBot(
	ctx context.Context,
	req ChatRequest,
	baseURL, apiKey string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	botID := strings.TrimSpace(req.BotID)
	bot, err := e.store.GetBot(botID)
	if err != nil || bot == nil {
		return nil, fmt.Errorf("未找到专属 Bot: %s", botID)
	}

	userMsg := notifier.StripEmoji(strings.TrimSpace(req.Message))
	if userMsg == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	replyPreview := notifier.StripEmoji(strings.TrimSpace(req.ReplyPreview))

	// Ensure timeline anchor session exists cleanly & load prior history
	history, _ := e.store.ListBotMessages(bot.ID, 40)

	// Persist user message once in this Bot's timeline
	userMsgID, err := e.store.AppendAgentMessage(store.AgentMessage{
		SessionID:    "bot:" + bot.ID,
		BotID:        bot.ID,
		MsgKind:      "chat",
		Role:         "user",
		Content:      userMsg,
		ReplyToID:    req.ReplyToID,
		ReplyPreview: replyPreview,
	})
	if err != nil {
		return nil, fmt.Errorf("记录消息失败: %w", err)
	}
	_ = userMsgID

	promptMsg := userMsg
	if replyPreview != "" {
		promptMsg = fmt.Sprintf("[用户正在引用以下历史消息进行回复]\n> %s\n\n用户当前指令：%s", replyPreview, userMsg)
	}

	allBots, _ := e.store.ListBots()
	mentioned := e.resolveMentionedBots(userMsg, allBots)

	// Multi-bot group chat (@mention targets including outside temporary guests, or collaboration mode routing)
	if bot.IsGroup || bot.ID == "team_room" {
		members := resolveGroupMemberBots(bot, allBots)
		memberSet := make(map[string]bool, len(members))
		for _, m := range members {
			memberSet[m.ID] = true
		}

		if len(mentioned) > 0 {
			var guests []string
			for _, sp := range mentioned {
				if !memberSet[sp.ID] {
					guests = append(guests, sp.Name)
				}
			}
			if len(guests) > 0 {
				notice := fmt.Sprintf("已临时呼叫「%s」支援本轮群聊讨论", strings.Join(guests, "、"))
				sysMsg := store.AgentMessage{
					SessionID: "bot:" + bot.ID,
					BotID:     bot.ID,
					MsgKind:   "system",
					Role:      "system",
					Content:   notice,
				}
				if id, err := e.store.AppendAgentMessage(sysMsg); err == nil {
					sysMsg.ID = id
					if emit != nil {
						emit("system_notice", sysMsg)
					}
				}
			}
			return e.runTeamRoomHandoff(ctx, req, bot, mentioned, memberSet, history, promptMsg, baseURL, apiKey, emit)
		}

		var targets []store.Bot
		switch strings.TrimSpace(bot.GroupMode) {
		case "sequential":
			targets = members
		case "mention_only":
			if len(members) > 0 {
				targets = members[:1]
			}
		default: // "smart"
			targets = selectSmartGroupMembers(members, userMsg)
		}
		if len(targets) > 0 {
			return e.runTeamRoomHandoff(ctx, req, bot, targets, memberSet, history, promptMsg, baseURL, apiKey, emit)
		}
	}

	// 1v1 DM chat also supports temporary @Bot guest summon
	if len(mentioned) > 0 {
		var external []store.Bot
		var guestNames []string
		for _, sp := range mentioned {
			if sp.ID != bot.ID {
				external = append(external, sp)
				guestNames = append(guestNames, sp.Name)
			}
		}
		if len(external) > 0 {
			notice := fmt.Sprintf("已临时邀请「%s」协同回答本轮指令", strings.Join(guestNames, "、"))
			sysMsg := store.AgentMessage{
				SessionID: "bot:" + bot.ID,
				BotID:     bot.ID,
				MsgKind:   "system",
				Role:      "system",
				Content:   notice,
			}
			if id, err := e.store.AppendAgentMessage(sysMsg); err == nil {
				sysMsg.ID = id
				if emit != nil {
					emit("system_notice", sysMsg)
				}
			}
			return e.runTeamRoomHandoff(ctx, req, bot, mentioned, map[string]bool{bot.ID: true}, history, promptMsg, baseURL, apiKey, emit)
		}
	}

	return e.runSingleBotTurn(ctx, req, bot, bot.ID, history, promptMsg, baseURL, apiKey, emit)
}

func (e *Engine) runTeamRoomHandoff(
	ctx context.Context,
	req ChatRequest,
	roomBot *store.Bot,
	specialists []store.Bot,
	memberSet map[string]bool,
	roomHistory []store.AgentMessage,
	userMsg, baseURL, apiKey string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	var combinedContent strings.Builder
	var combinedReasoning strings.Builder
	var allTraces []ToolTrace
	var totalUsage TokenUsage
	hasUsage := false

	e.emitBotState(roomBot.ID, store.BotStateWorking, "多 Bot 协同处理中...")

	for i, sp := range specialists {
		if emit != nil {
			emit("bot_turn", map[string]any{
				"bot_id":        roomBot.ID,
				"sender_bot_id": sp.ID,
				"sender_name":   sp.Name,
				"role_title":    sp.RoleTitle,
				"avatar_style":  sp.AvatarStyle,
				"is_temp_guest": memberSet != nil && !memberSet[sp.ID],
				"step":          i + 1,
				"total_steps":   len(specialists),
			})
		}

		turnPrompt := userMsg
		if combinedContent.Len() > 0 {
			turnPrompt = fmt.Sprintf("用户指令：%s\n\n[前置协同 Bot 结论参考]\n%s\n\n请基于你的专属职责继续补充分析。", userMsg, combinedContent.String())
		}
		spCopy := sp
		resp, err := e.runSingleBotTurn(ctx, req, &spCopy, roomBot.ID, roomHistory, turnPrompt, baseURL, apiKey, emit)
		if err != nil {
			e.emitBotState(roomBot.ID, store.BotStateBlocked, err.Error())
			return nil, err
		}
		if combinedContent.Len() > 0 {
			combinedContent.WriteString("\n\n---\n\n")
		}
		combinedContent.WriteString(fmt.Sprintf("**[%s]**\n%s", sp.Name, resp.Content))
		if resp.ReasoningContent != "" {
			combinedReasoning.WriteString(resp.ReasoningContent + "\n")
		}
		allTraces = append(allTraces, resp.ToolTraces...)
		if resp.Usage != nil {
			hasUsage = true
			totalUsage.PromptTokens += resp.Usage.PromptTokens
			totalUsage.CompletionTokens += resp.Usage.CompletionTokens
			totalUsage.TotalTokens += resp.Usage.TotalTokens
		}
	}

	e.emitBotState(roomBot.ID, store.BotStateIdle, truncateTitle(combinedContent.String()))

	finalResp := &ChatResponse{
		BotID:            roomBot.ID,
		SenderBotID:      specialists[len(specialists)-1].ID,
		SessionID:        "bot:" + roomBot.ID,
		Model:            specialists[0].Model,
		ReasoningContent: combinedReasoning.String(),
		Content:          combinedContent.String(),
		ToolTraces:       allTraces,
	}
	if hasUsage {
		finalResp.Usage = &totalUsage
	}
	if emit != nil {
		emit("done", finalResp)
	}
	return finalResp, nil
}

func (e *Engine) runSingleBotTurn(
	ctx context.Context,
	req ChatRequest,
	actingBot *store.Bot,
	timelineBotID string,
	history []store.AgentMessage,
	userMsg, baseURL, apiKey string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = actingBot.Model
	}
	model = NormalizeModelID(model)

	useThinking := actingBot.EnableThinking
	if req.EnableThinking != nil {
		useThinking = *req.EnableThinking
	}
	var thinkingCfg *thinkingConfig
	if strings.Contains(strings.ToLower(model), "flash") {
		if useThinking {
			thinkingCfg = &thinkingConfig{Type: "enabled"}
		} else {
			thinkingCfg = &thinkingConfig{Type: "disabled"}
		}
	}

	e.emitBotState(actingBot.ID, store.BotStateThinking, "正在思考处理...")
	defer func() {
		_ = e.store.ClearBotUnread(timelineBotID)
	}()

	memories, _ := e.store.ListAgentMemories("", 40)
	sysPrompt := buildBotSystemPrompt(actingBot, memories)

	messages := make([]apiMessage, 0, len(history)+8)
	messages = append(messages, apiMessage{
		Role:    "system",
		Content: sysPrompt,
	})
	for _, m := range history {
		switch m.Role {
		case "system":
			if strings.TrimSpace(m.Content) != "" {
				messages = append(messages, apiMessage{Role: "user", Content: "[群聊系统动态] " + m.Content})
			}
		case "user":
			content := m.Content
			if strings.TrimSpace(m.ReplyPreview) != "" {
				content = fmt.Sprintf("[引用历史消息: %s]\n%s", m.ReplyPreview, content)
			}
			messages = append(messages, apiMessage{Role: "user", Content: content})
		case "assistant":
			msg := apiMessage{Role: "assistant", Content: m.Content}
			if m.ToolCalls != "" && m.ToolCalls != "[]" {
				var tcs []apiToolCall
				if json.Unmarshal([]byte(m.ToolCalls), &tcs) == nil && len(tcs) > 0 {
					msg.ToolCalls = tcs
				}
			}
			messages = append(messages, msg)
		case "tool":
			content := m.Content
			runes := []rune(content)
			if len(runes) > 600 {
				content = string(runes[:600]) + "\n...[历史工具输出已折叠，核心结论见下方助手回复]"
			}
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: m.ToolCallID,
			})
		}
	}
	messages = append(messages, apiMessage{Role: "user", Content: userMsg})

	tools := filterToolsForBot(actingBot)
	var totalReasoning strings.Builder
	var finalContent strings.Builder
	traces := make([]ToolTrace, 0)
	var totalUsage TokenUsage
	hasUsage := false

	const maxRounds = 5
	for round := 0; round < maxRounds; round++ {
		roundReasoning, roundContent, toolCalls, roundUsage, err := e.streamSingleRound(ctx, baseURL, apiKey, model, messages, tools, thinkingCfg, func(deltaReasoning, deltaContent string) {
			if deltaReasoning != "" {
				clean := notifier.StripEmoji(deltaReasoning)
				totalReasoning.WriteString(clean)
				if emit != nil {
					emit("reasoning", map[string]any{"delta": clean, "sender_bot_id": actingBot.ID})
				}
			}
			if deltaContent != "" {
				clean := notifier.StripEmoji(deltaContent)
				finalContent.WriteString(clean)
				if emit != nil {
					emit("content", map[string]any{"delta": clean, "sender_bot_id": actingBot.ID})
				}
			}
		})
		if err != nil {
			e.emitBotState(actingBot.ID, store.BotStateBlocked, err.Error())
			return nil, err
		}
		if roundUsage != nil {
			hasUsage = true
			totalUsage.PromptTokens += roundUsage.PromptTokens
			totalUsage.CompletionTokens += roundUsage.CompletionTokens
			totalUsage.TotalTokens += roundUsage.TotalTokens
		}

		if len(toolCalls) == 0 {
			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID:        "bot:" + timelineBotID,
				BotID:            timelineBotID,
				SenderBotID:      actingBot.ID,
				MsgKind:          "chat",
				Role:             "assistant",
				Content:          notifier.StripEmoji(roundContent),
				ReasoningContent: notifier.StripEmoji(totalReasoning.String()),
				ToolCalls:        "[]",
			})
			break
		}

		tcBytes, _ := json.Marshal(toolCalls)
		_, _ = e.store.AppendAgentMessage(store.AgentMessage{
			SessionID:        "bot:" + timelineBotID,
			BotID:            timelineBotID,
			SenderBotID:      actingBot.ID,
			MsgKind:          "chat",
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        string(tcBytes),
		})
		messages = append(messages, apiMessage{
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        toolCalls,
		})

		e.emitBotState(actingBot.ID, store.BotStateWorking, "正在调用工具...")
		for _, tc := range toolCalls {
			if emit != nil {
				emit("tool_start", map[string]any{
					"id":            tc.ID,
					"name":          tc.Function.Name,
					"arguments":     tc.Function.Arguments,
					"sender_bot_id": actingBot.ID,
				})
			}
			toolOutput, toolErr := e.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				errPayload, _ := json.Marshal(map[string]any{"ok": false, "error": toolErr.Error()})
				toolOutput = string(errPayload)
			}
			toolOutput = notifier.StripEmoji(toolOutput)
			traces = append(traces, ToolTrace{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Result:    toolOutput,
			})
			if emit != nil {
				emit("tool_result", map[string]any{
					"id":            tc.ID,
					"name":          tc.Function.Name,
					"result":        toolOutput,
					"sender_bot_id": actingBot.ID,
				})
			}
			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID:   "bot:" + timelineBotID,
				BotID:       timelineBotID,
				SenderBotID: actingBot.ID,
				MsgKind:     "chat",
				Role:        "tool",
				Content:     toolOutput,
				ToolCallID:  tc.ID,
			})
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})
		}
	}

	preview := truncateTitle(finalContent.String())
	e.emitBotState(actingBot.ID, store.BotStateIdle, preview)

	resp := &ChatResponse{
		BotID:            timelineBotID,
		SenderBotID:      actingBot.ID,
		SessionID:        "bot:" + timelineBotID,
		Persona:          actingBot.ID,
		Model:            model,
		ReasoningContent: totalReasoning.String(),
		Content:          finalContent.String(),
		ToolTraces:       traces,
	}
	if hasUsage {
		resp.Usage = &totalUsage
	}
	// Only emit "done" directly for single-bot calls (team_room emits after all specialists finish, or UI handles per-turn)
	if timelineBotID == actingBot.ID && emit != nil {
		emit("done", resp)
	}
	return resp, nil
}
