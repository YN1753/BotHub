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
	// Explicit collective request triggers all group members
	for _, kw := range []string{"大家", "全部", "各位", "一起", "全员", "接力", "群策群力"} {
		if strings.Contains(userMsg, kw) {
			return members
		}
	}

	toolDomainKeywords := map[string][]string{
		"query_nezha_nodes": {"探针", "哪吒", "vps", "节点", "服务器", "cpu", "内存", "负载", "掉线", "宕机", "告警", "延迟", "ssh", "nezha"},
		"manage_assets":     {"资产", "域名", "到期", "续费", "账单", "会员", "cloudflare"},
		"manage_tasks":      {"定时", "例程", "cron", "日程", "晨报", "调度"},
		"web_search":        {"github", "开源", "趋势", "雷达", "新闻", "前沿", "hn", "hacker", "搜索", "推荐"},
		"read_url":          {"网页", "url", "链接", "http", "文章"},
	}

	type scored struct {
		bot   store.Bot
		score int
	}
	ranked := make([]scored, 0, len(members))
	for _, m := range members {
		pts := 0
		tools := m.AllowedToolList()
		for _, toolName := range tools {
			for _, kw := range toolDomainKeywords[toolName] {
				if strings.Contains(lower, kw) {
					pts += 3
					break
				}
			}
		}
		metaText := strings.ToLower(m.Name + " " + m.RoleTitle + " " + m.SystemPrompt)
		for _, token := range strings.Fields(lower) {
			if len([]rune(token)) >= 2 && strings.Contains(metaText, token) {
				pts += 2
			}
		}
		for _, word := range strings.Fields(strings.ToLower(m.RoleTitle + " " + m.Name)) {
			if len([]rune(word)) >= 2 && strings.Contains(lower, word) {
				pts += 2
			}
		}
		if pts > 0 {
			ranked = append(ranked, scored{bot: m, score: pts})
		}
	}

	if len(ranked) > 0 {
		for i := 0; i < len(ranked); i++ {
			for j := i + 1; j < len(ranked); j++ {
				if ranked[j].score > ranked[i].score {
					ranked[i], ranked[j] = ranked[j], ranked[i]
				}
			}
		}
		limit := 2
		if len(ranked) < limit {
			limit = len(ranked)
		}
		out := make([]store.Bot, 0, limit)
		for i := 0; i < limit; i++ {
			out = append(out, ranked[i].bot)
		}
		return out
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
		allowAutoRelay := false
		switch strings.TrimSpace(bot.GroupMode) {
		case "sequential":
			targets = members
		case "mention_only":
			if len(members) > 0 {
				targets = members[:1]
			}
		default: // "smart"
			targets = selectSmartGroupMembers(members, userMsg)
			allowAutoRelay = len(targets) == 1 && len(members) > 1
		}
		if len(targets) > 0 {
			return e.runTeamRoomHandoffWithRelay(ctx, req, bot, targets, members, allowAutoRelay, memberSet, history, promptMsg, baseURL, apiKey, emit)
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
	return e.runTeamRoomHandoffWithRelay(ctx, req, roomBot, specialists, nil, false, memberSet, roomHistory, userMsg, baseURL, apiKey, emit)
}

func (e *Engine) runTeamRoomHandoffWithRelay(
	ctx context.Context,
	req ChatRequest,
	roomBot *store.Bot,
	specialists []store.Bot,
	groupPool []store.Bot,
	allowAutoRelay bool,
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

	calledSet := make(map[string]bool, len(specialists)+1)
	for _, s := range specialists {
		calledSet[s.ID] = true
	}

	e.emitBotState(roomBot.ID, store.BotStateWorking, "多 Bot 协同处理中...")

	executeList := append([]store.Bot(nil), specialists...)
	autoRelayed := false

	for i := 0; i < len(executeList); i++ {
		sp := executeList[i]
		if roomBot.ID != sp.ID {
			e.emitBotState(roomBot.ID, store.BotStateWorking, fmt.Sprintf("%s 发言中 (%d/%d)...", sp.Name, i+1, len(executeList)))
		}
		if emit != nil {
			emit("bot_turn", map[string]any{
				"bot_id":        roomBot.ID,
				"sender_bot_id": sp.ID,
				"sender_name":   sp.Name,
				"role_title":    sp.RoleTitle,
				"avatar_style":  sp.AvatarStyle,
				"is_temp_guest": memberSet != nil && !memberSet[sp.ID],
				"step":          i + 1,
				"total_steps":   len(executeList),
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

		// 1-hop smart group relay when the lead specialist explicitly @mentions another group member
		if allowAutoRelay && !autoRelayed && len(groupPool) > 0 {
			followups := e.resolveMentionedBots(resp.Content, groupPool)
			for _, f := range followups {
				if !calledSet[f.ID] {
					calledSet[f.ID] = true
					executeList = append(executeList, f)
					autoRelayed = true
					break
				}
			}
		}
	}

	e.emitBotState(roomBot.ID, store.BotStateIdle, truncateTitle(combinedContent.String()))

	lastResponder := executeList[len(executeList)-1]
	finalResp := &ChatResponse{
		BotID:            roomBot.ID,
		SenderBotID:      lastResponder.ID,
		SessionID:        "bot:" + roomBot.ID,
		Model:            executeList[0].Model,
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
	rawModel := strings.TrimSpace(req.Model)
	if rawModel == "" {
		rawModel = actingBot.Model
	}
	targets, err := e.ResolveChatTargets(rawModel)
	if err != nil {
		e.emitBotState(actingBot.ID, store.BotStateBlocked, err.Error())
		return nil, err
	}
	activeTarget := targets[0]

	var thinkFlag *bool
	if req.EnableThinking != nil {
		thinkFlag = req.EnableThinking
	} else {
		v := actingBot.EnableThinking
		thinkFlag = &v
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
		roundReasoning, roundContent, toolCalls, roundUsage, usedTarget, err := e.streamRoundWithFailover(ctx, targets, messages, tools, thinkFlag, emit, func(deltaReasoning, deltaContent string) {
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
		activeTarget = usedTarget
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
				ModelUsed:        activeTarget.Model,
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
			ModelUsed:        activeTarget.Model,
		})
		messages = append(messages, apiMessage{
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        toolCalls,
		})

		e.emitBotState(actingBot.ID, store.BotStateWorking, "正在调用工具...")
		for _, tc := range toolCalls {
			if timelineBotID != "" && timelineBotID != actingBot.ID {
				e.emitBotState(timelineBotID, store.BotStateWorking, fmt.Sprintf("%s: 调用 %s...", actingBot.Name, tc.Function.Name))
			}
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
		Model:            activeTarget.Model,
		ProviderLabel:    activeTarget.Label,
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
