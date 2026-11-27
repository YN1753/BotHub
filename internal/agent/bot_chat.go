package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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

func (e *Engine) publishLiveChatMessage(msg store.AgentMessage) {
	if e == nil || e.hub == nil || msg.ID <= 0 {
		return
	}
	if strings.TrimSpace(msg.CreatedAt) == "" {
		msg.CreatedAt = store.NowShanghai()
	}
	e.hub.BumpRevision("messages")
	e.hub.PublishEvent("bot_message", msg)
}

func sanitizeChatImages(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		s := strings.TrimSpace(item)
		if s == "" || seen[s] {
			continue
		}
		if !(strings.HasPrefix(s, "/api/v2/chat/uploads/") ||
			strings.HasPrefix(s, "https://") ||
			strings.HasPrefix(s, "http://") ||
			strings.HasPrefix(s, "data:image/")) {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func parseAttachmentsJSON(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "[]" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(s), &list); err != nil {
		return nil
	}
	return sanitizeChatImages(list)
}

func (e *Engine) chatStreamForBot(
	ctx context.Context,
	req ChatRequest,
	emit func(event string, data any),
) (*ChatResponse, error) {
	botID := strings.TrimSpace(req.BotID)
	bot, err := e.store.GetBot(botID)
	if err != nil || bot == nil {
		return nil, fmt.Errorf("未找到专属 Bot: %s", botID)
	}

	cleanImages := sanitizeChatImages(req.Images)
	req.Images = cleanImages
	userMsg := notifier.StripEmoji(strings.TrimSpace(req.Message))
	if userMsg == "" && len(cleanImages) > 0 {
		userMsg = "请分析这张图片"
	}
	if userMsg == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	replyPreview := notifier.StripEmoji(strings.TrimSpace(req.ReplyPreview))
	threadID := strings.TrimSpace(req.ThreadID)

	// Built-in /compact slash command to immediately fold earlier turns into a rolling summary
	if len(cleanImages) == 0 && (strings.EqualFold(userMsg, "/compact") || userMsg == "/压缩") {
		summary, compactedCount, cErr := e.CompactBotThread(ctx, bot, threadID)
		var replyText string
		if cErr != nil {
			replyText = fmt.Sprintf("压缩上下文失败：%v", cErr)
		} else if compactedCount == 0 {
			replyText = "当前未压缩对话轮次较少，暂无需压缩上下文。"
		} else {
			replyText = fmt.Sprintf("已完成上下文压缩：已将早期 %d 条消息折叠进滚动摘要，后续对话将更轻盈连贯。\n\n### 当前滚动摘要\n%s", compactedCount, summary)
		}
		if emit != nil {
			emit("content", map[string]any{"delta": replyText, "sender_bot_id": bot.ID})
		}
		replyEntry := store.AgentMessage{
			SessionID:   "bot:" + bot.ID,
			BotID:       bot.ID,
			ThreadID:    threadID,
			SenderBotID: bot.ID,
			MsgKind:     "chat",
			Role:        "assistant",
			Content:     replyText,
			ToolCalls:   "[]",
		}
		if aid, aErr := e.store.AppendAgentMessage(replyEntry); aErr == nil {
			replyEntry.ID = aid
			e.publishLiveChatMessage(replyEntry)
		}
		resp := &ChatResponse{
			BotID:       bot.ID,
			ThreadID:    threadID,
			SenderBotID: bot.ID,
			SessionID:   "bot:" + bot.ID,
			Persona:     bot.ID,
			Content:     replyText,
		}
		if emit != nil {
			emit("done", resp)
		}
		return resp, nil
	}

	// Load prior messages after the latest rolling compaction checkpoint
	compState := e.store.GetThreadCompactionState(bot.ID, threadID)
	history, _ := e.store.ListBotMessagesAfterID(bot.ID, threadID, compState.UpToID, 60)

	// Auto-title custom topic thread on first user message
	if threadID != "" {
		if th, _ := e.store.GetBotThread(bot.ID, threadID); th != nil && (th.Title == "新话题" || th.Title == "新对话" || strings.TrimSpace(th.Title) == "") {
			th.Title = truncateTitle(userMsg)
			_ = e.store.SaveBotThread(*th)
		}
	}

	attachmentsJSON := ""
	if len(cleanImages) > 0 {
		if b, mErr := json.Marshal(cleanImages); mErr == nil {
			attachmentsJSON = string(b)
		}
	}

	// Persist user message once in this Bot/Group's isolated thread timeline
	userEntry := store.AgentMessage{
		SessionID:    "bot:" + bot.ID,
		BotID:        bot.ID,
		ThreadID:     threadID,
		MsgKind:      "chat",
		Role:         "user",
		Content:      userMsg,
		Attachments:  attachmentsJSON,
		ReplyToID:    req.ReplyToID,
		ReplyPreview: replyPreview,
	}
	mid, err := e.store.AppendAgentMessage(userEntry)
	if err != nil {
		return nil, fmt.Errorf("记录消息失败: %w", err)
	}
	userEntry.ID = mid
	e.publishLiveChatMessage(userEntry)

	promptMsg := userMsg
	if replyPreview != "" {
		promptMsg = fmt.Sprintf("[用户正在引用以下历史消息进行回复]\n> %s\n\n用户当前指令：%s", replyPreview, userMsg)
	}

	allBots, _ := e.store.ListBots()
	mentioned := e.resolveMentionedBots(userMsg, allBots)

	// Multi-bot group chat (@mention targets including outside temporary guests, or collaboration mode routing)
	if bot.IsGroup {
		members := resolveGroupMemberBots(bot, allBots)
		memberSet := make(map[string]bool, len(members))
		for _, m := range members {
			memberSet[m.ID] = true
		}

		// Group @全体成员 / @所有人 triggers all group members cleanly like QQ/WeChat
		if (strings.Contains(userMsg, "@全体成员") || strings.Contains(userMsg, "@所有人")) && len(members) > 0 {
			if strings.TrimSpace(bot.GroupMode) == "parallel" && len(members) > 1 {
				return e.runTeamRoomParallel(ctx, req, bot, members, memberSet, history, promptMsg, emit)
			}
			return e.runTeamRoomHandoff(ctx, req, bot, members, memberSet, history, promptMsg, emit)
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
					ThreadID:  threadID,
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
			if strings.TrimSpace(bot.GroupMode) == "parallel" && len(mentioned) > 1 {
				return e.runTeamRoomParallel(ctx, req, bot, mentioned, memberSet, history, promptMsg, emit)
			}
			return e.runTeamRoomHandoff(ctx, req, bot, mentioned, memberSet, history, promptMsg, emit)
		}

		allSkills, _ := e.store.ListSkills()
		skillMap := make(map[string]store.Skill, len(allSkills))
		for _, sk := range allSkills {
			skillMap[sk.ID] = sk
		}

		var targets []store.Bot
		allowAutoRelay := false
		switch strings.TrimSpace(bot.GroupMode) {
		case "parallel":
			if len(members) > 1 {
				return e.runTeamRoomParallel(ctx, req, bot, members, memberSet, history, promptMsg, emit)
			}
			targets = members
		case "sequential":
			targets = members
		case "mention_only":
			if len(members) > 0 {
				targets = members[:1]
			}
		default: // "smart"
			targets = e.selectGroupSpeakersWithLLM(ctx, members, skillMap, userMsg)
			allowAutoRelay = len(targets) == 1 && len(members) > 1
		}
		if len(targets) > 0 {
			return e.runTeamRoomHandoffWithRelay(ctx, req, bot, targets, members, allowAutoRelay, memberSet, history, promptMsg, emit)
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
				ThreadID:  threadID,
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
			return e.runTeamRoomHandoff(ctx, req, bot, mentioned, map[string]bool{bot.ID: true}, history, promptMsg, emit)
		}
	}

	return e.runSingleBotTurn(ctx, req, bot, bot.ID, history, promptMsg, emit)
}

func (e *Engine) runSingleBotTurn(
	ctx context.Context,
	req ChatRequest,
	actingBot *store.Bot,
	timelineBotID string,
	history []store.AgentMessage,
	userMsg string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	rawModel := strings.TrimSpace(req.Model)
	if rawModel == "" {
		rawModel = actingBot.Model
	}

	newlyAutoMounted := e.autoMountRelevantMCPs(actingBot, userMsg)
	attachedSkills, _ := e.store.GetSkillsByIDs(actingBot.SkillIDList())
	allWorkspaceSkills, _ := e.store.ListSkills()
	allBotTools := filterToolsForBot(actingBot, attachedSkills)
	tools := pruneToolsForTurn(allBotTools, userMsg, history)

	targets, err := e.ResolveChatTargetsWithOptions(ctx, rawModel, ModelResolveOptions{
		NeedTools: len(tools) > 0,
	})
	if err != nil {
		e.emitBotState(actingBot.ID, store.BotStateBlocked, err.Error())
		return nil, err
	}
	activeTarget := targets[0]

	threadID := strings.TrimSpace(req.ThreadID)
	statusHint := "正在思考..."
	if store.NormalizeBotMode(actingBot.BotMode) == store.BotModeCompanion {
		statusHint = "正在输入..."
	}
	e.emitBotState(actingBot.ID, store.BotStateThinking, statusHint)
	defer func() {
		_ = e.store.ClearBotUnread(timelineBotID)
		if e.hub != nil {
			if tb, err := e.store.GetBot(timelineBotID); err == nil && tb != nil {
				e.hub.PublishEvent("bot_state", map[string]any{
					"bot_id":       timelineBotID,
					"state":        tb.State,
					"last_summary": tb.LastSummary,
					"unread_count": 0,
				})
			}
		}
	}()

	// Load strictly scoped memories ranked by pin + importance + query relevance, and record recall stats
	memScopes := []string{"bot:" + timelineBotID}
	if actingBot.ID != "" && actingBot.ID != timelineBotID {
		memScopes = append(memScopes, "bot:"+actingBot.ID)
	}
	if actingBot.MemoryPolicy != store.MemoryPolicyIsolated {
		memScopes = append(memScopes, "global")
	}
	rawMemories, _ := e.store.ListScopedMemories(memScopes, "", 80)
	memories := selectRelevantMemories(rawMemories, userMsg, 16)
	if len(memories) > 0 {
		recalledIDs := make([]int64, 0, len(memories))
		for _, m := range memories {
			recalledIDs = append(recalledIDs, m.ID)
		}
		e.store.TouchRecalledMemories(recalledIDs)
	}

	hasRosterTool := false
	for _, td := range tools {
		if td.Function.Name == "consult_bot" || td.Function.Name == "manage_bots" {
			hasRosterTool = true
			break
		}
	}

	var allBots []store.Bot
	if timelineBotID != actingBot.ID || hasRosterTool {
		allBots, _ = e.store.ListBots()
	}

	var threadTitle, threadSummary string
	if threadID != "" {
		if th, _ := e.store.GetBotThread(timelineBotID, threadID); th != nil {
			threadTitle = th.Title
			threadSummary = strings.TrimSpace(th.Summary)
		}
	}

	var peerBots []store.Bot
	if hasRosterTool {
		peerBots = allBots
	}
	workingCtx := e.buildWorkingContextBlock(actingBot, tools)
	if unmountedDir := buildUnmountedMCPDirectoryBlock(actingBot, allWorkspaceSkills); unmountedDir != "" {
		if workingCtx != "" {
			workingCtx += "\n\n" + unmountedDir
		} else {
			workingCtx = unmountedDir
		}
	}
	zones := buildBotSystemPromptZones(actingBot, attachedSkills, memories, rawMemories, peerBots, threadTitle, timelineBotID != actingBot.ID, workingCtx)
	sysPrompt := zones.Static
	if strings.TrimSpace(zones.Dynamic) != "" {
		sysPrompt += "\n\n" + zones.Dynamic
	}
	compState := e.store.GetThreadCompactionState(timelineBotID, threadID)

	var botNameMap map[string]string
	if timelineBotID != actingBot.ID && len(allBots) > 0 {
		botNameMap = make(map[string]string, len(allBots))
		for _, ab := range allBots {
			botNameMap[ab.ID] = ab.Name
		}
	}

	// Only hydrate image attachments for the latest 4 user messages in history to keep multimodal token budget lean.
	hydrateImageFromIdx := len(history)
	userImagesSeen := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" && strings.TrimSpace(history[i].Attachments) != "" && strings.TrimSpace(history[i].Attachments) != "[]" {
			userImagesSeen++
			hydrateImageFromIdx = i
			if userImagesSeen >= 4 {
				break
			}
		}
	}

	messages := make([]apiMessage, 0, len(history)+8)
	messages = append(messages, apiMessage{
		Role:                "system",
		Content:             sysPrompt,
		StaticSystemPrefix:  zones.Static,
		DynamicSystemSuffix: zones.Dynamic,
	})
	if strings.TrimSpace(compState.Summary) != "" {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: fmt.Sprintf("[早期对话滚动摘要（上下文已压缩）]\n%s", compState.Summary),
		})
	} else if threadSummary != "" && len(history) >= 16 {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: fmt.Sprintf("[此前较早对话压缩前情摘要]\n%s", threadSummary),
		})
	}
	for idx, m := range history {
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
			uMsg := apiMessage{Role: "user", Content: content}
			if idx >= hydrateImageFromIdx {
				uMsg.Images = parseAttachmentsJSON(m.Attachments)
			}
			messages = append(messages, uMsg)
		case "assistant":
			content := m.Content
			if botNameMap != nil && m.SenderBotID != "" && m.SenderBotID != actingBot.ID {
				speaker := botNameMap[m.SenderBotID]
				if speaker == "" {
					speaker = m.SenderBotID
				}
				content = fmt.Sprintf("[同群队友 %s 此前说]: %s", speaker, content)
			}
			msg := apiMessage{Role: "assistant", Content: content}
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
	messages = append(messages, apiMessage{
		Role:    "user",
		Content: userMsg,
		Images:  sanitizeChatImages(req.Images),
	})
	messages = budgetChatMessages(messages, 48000)

	parentScope := toolScopeFromContext(ctx)
	toolCtx := WithToolScope(ctx, ToolScope{
		BotID:         actingBot.ID,
		TimelineBotID: timelineBotID,
		ThreadID:      threadID,
		MemoryPolicy:  actingBot.MemoryPolicy,
		Depth:         parentScope.Depth,
		VisitedBots:   parentScope.VisitedBots,
	})
	var totalReasoning strings.Builder
	var finalContent strings.Builder
	traces := make([]ToolTrace, 0)
	var totalUsage TokenUsage
	hasUsage := false

	maxRounds := 8
	if strings.TrimSpace(req.MsgKind) == "routine" {
		maxRounds = 10
	}
	webSearchCalls := 0
	endedWithTools := false
	fallbackSearchExecuted := false
	var turnCacheMu sync.Mutex
	turnReadCache := make(map[string]string)
	executedCallCounts := make(map[string]int)

	if len(newlyAutoMounted) > 0 {
		mountedNames := make([]string, 0, len(newlyAutoMounted))
		mountedIDs := make([]string, 0, len(newlyAutoMounted))
		var unlockedTools []string
		for _, sk := range newlyAutoMounted {
			mountedNames = append(mountedNames, sk.Name)
			mountedIDs = append(mountedIDs, sk.ID)
			unlockedTools = append(unlockedTools, mcpExposedToolNames(sk)...)
		}
		autoArgsBytes, _ := json.Marshal(map[string]any{"mcp_id": strings.Join(mountedIDs, ",")})
		autoResBytes, _ := json.Marshal(map[string]any{
			"ok":             true,
			"auto_mounted":   true,
			"mounted_mcps":   mountedNames,
			"unlocked_tools": unlockedTools,
		})
		autoArgs := string(autoArgsBytes)
		autoRes := string(autoResBytes)
		if emit != nil {
			emit("tool_start", map[string]any{
				"id":            "automount_1",
				"name":          "mount_mcp",
				"arguments":     autoArgs,
				"sender_bot_id": actingBot.ID,
			})
			emit("tool_result", map[string]any{
				"id":            "automount_1",
				"name":          "mount_mcp",
				"result":        autoRes,
				"status":        "ok",
				"duration_ms":   1,
				"sender_bot_id": actingBot.ID,
			})
		}
		traces = append(traces, ToolTrace{
			ID:        "automount_1",
			Name:      "mount_mcp",
			Arguments: autoArgs,
			Result:    autoRes,
		})
		e.store.RecordBotExecution(store.BotExecutionLog{
			BotID:         timelineBotID,
			ThreadID:      threadID,
			ExecType:      "tool",
			Name:          "mount_mcp",
			InputSummary:  store.TruncateRunes(autoArgs, 160),
			OutputSummary: store.TruncateRunes(autoRes, 200),
			Status:        "ok",
			DurationMs:    1,
		})
	}

	// Harness 1: Correction boost & anti-repetition guardrail when the user explicitly corrects a prior reply.
	isCorrection := detectCorrectionTurn(userMsg, history)
	prevAssistantReply := ""
	if isCorrection {
		prevAssistantReply = lastAssistantReplyFromHistory(history)
		messages = applyCorrectionGuardrail(messages)
	}

	// Harness 2: Deterministic preflight grounding for inline URLs, workspace read queries, or web search intents.
	if (req.MsgKind == "" || req.MsgKind == "chat") && len(tools) > 0 {
		if pfTool, pfArgs, ok := detectPreflightGrounding(userMsg, tools); ok {
			pfArgs = repairToolArguments(pfTool, pfArgs)
			e.emitBotState(actingBot.ID, store.BotStateWorking, humanToolStatus(pfTool, pfArgs))
			pfID := "preflight_1"
			if emit != nil {
				emit("tool_start", map[string]any{
					"id":            pfID,
					"name":          pfTool,
					"arguments":     pfArgs,
					"sender_bot_id": actingBot.ID,
				})
			}
			pfStart := time.Now()
			pfOut, pfErr := e.ExecuteTool(toolCtx, pfTool, pfArgs)
			pfDurMs := time.Since(pfStart).Milliseconds()
			pfStatus := "ok"
			if pfErr != nil {
				pfStatus = "error"
				pfOut = toolErrorPayload(pfTool, pfErr)
			}
			cleanPFOut := notifier.StripEmoji(pfOut)
			if emit != nil {
				emit("tool_result", map[string]any{
					"id":            pfID,
					"name":          pfTool,
					"result":        cleanPFOut,
					"status":        pfStatus,
					"duration_ms":   pfDurMs,
					"sender_bot_id": actingBot.ID,
				})
			}
			traces = append(traces, ToolTrace{
				ID:        pfID,
				Name:      pfTool,
				Arguments: pfArgs,
				Result:    cleanPFOut,
			})
			e.store.RecordBotExecution(store.BotExecutionLog{
				BotID:         timelineBotID,
				ThreadID:      threadID,
				ExecType:      "tool",
				Name:          pfTool,
				InputSummary:  store.TruncateRunes(pfArgs, 160),
				OutputSummary: store.TruncateRunes(cleanPFOut, 200),
				Status:        pfStatus,
				DurationMs:    pfDurMs,
			})
			if pfErr == nil && preflightResultHasUsefulData(pfTool, cleanPFOut) {
				cacheKey := pfTool + "\x00" + repairToolArguments(pfTool, pfArgs)
				turnReadCache[cacheKey] = cleanPFOut
				executedCallCounts[cacheKey] = 1
				messages = append(messages, apiMessage{
					Role: "system",
					Content: fmt.Sprintf(
						"[系统已自动执行预检索/状态查询（工具: %s，参数: %s）]\n%s\n\n请优先严格基于上述实时查询/抓取结果回答用户本轮问题（若为联网结果请附上关键来源链接）；如信息已足够请直接作答，请勿重复调用完全相同的查询参数；若信息不足或需不同角度，可调用 web_search（更换新关键词）或 read_url 精读原文，严禁忽略已查到的事实或自行编造。",
						pfTool, pfArgs, cleanPFOut,
					),
				})
			}
			e.emitBotState(actingBot.ID, store.BotStateThinking, "正在整合检索结果...")
		}
	}

	for round := 0; round < maxRounds; round++ {
		contentPrefixBeforeRound := finalContent.String()
		roundReasoning, roundContent, toolCalls, roundUsage, usedTarget, err := e.streamRoundWithFailover(toolCtx, targets, messages, tools, emit, func(deltaReasoning, deltaContent string) {
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
			totalUsage.CachedTokens += roundUsage.CachedTokens
			totalUsage.CompletionTokens += roundUsage.CompletionTokens
			totalUsage.TotalTokens += roundUsage.TotalTokens
			totalUsage.CostUSD += roundUsage.CostUSD
		}

		// Harness 3: Rescue text-embedded tool calls (<tool_call>, <function=...>, ```json {"name":...} ```)
		// when a weaker model or relay outputs tool calls in plain text instead of OpenAI delta.ToolCalls.
		if len(toolCalls) == 0 && len(tools) > 0 {
			if cleanedText, rescuedCalls := extractTextEmbeddedToolCalls(roundContent, tools); len(rescuedCalls) > 0 {
				roundContent = cleanedText
				toolCalls = rescuedCalls
			}
		}
		if finalContent.String() != contentPrefixBeforeRound+notifier.StripEmoji(roundContent) {
			finalContent.Reset()
			finalContent.WriteString(contentPrefixBeforeRound)
			finalContent.WriteString(notifier.StripEmoji(roundContent))
		}

		msgKind := strings.TrimSpace(req.MsgKind)
		if msgKind == "" {
			msgKind = "chat"
		}
		// Never write background routine runs into the Bot's chat timeline; routines push to 服务通知.
		persistTimeline := msgKind != "delegate" && msgKind != "routine"
		if len(toolCalls) == 0 {
			endedWithTools = false
			// Harness 4: Empty-content rescue when a reasoning model puts all text into reasoning_content
			if strings.TrimSpace(roundContent) == "" && strings.TrimSpace(finalContent.String()) == "" && strings.TrimSpace(totalReasoning.String()) != "" {
				if rescued := notifier.StripEmoji(rescueEmptyContentFromReasoning(totalReasoning.String())); rescued != "" {
					roundContent = rescued
					finalContent.WriteString(rescued)
					if emit != nil {
						emit("content", map[string]any{"delta": rescued, "sender_bot_id": actingBot.ID})
					}
				}
			}

			// Harness 4b: No-Internet Excuse Interceptor & Scheduled Routine Live Web Search Fallback.
			// If a model falsely claims it has "no internet access / no search tool" or skips web_search on a
			// live-info scheduled routine, automatically execute web_search in Go and force a grounded rewrite.
			hasExcuse := containsNoInternetExcuse(finalContent.String())
			needsRoutineWeb := msgKind == "routine" && !hasExecutedWebSearch(traces) && routineNeedsLiveWebSearch(userMsg)
			if !fallbackSearchExecuted && (hasExcuse || needsRoutineWeb) && hasToolNamed(tools, "web_search") && ctx.Err() == nil && round+1 < maxRounds {
				fallbackSearchExecuted = true
				fbQuery := buildFallbackWebSearchQuery(userMsg)
				if fbQuery != "" {
					fbArgsBytes, _ := json.Marshal(map[string]any{
						"query":      fbQuery,
						"time_range": "week",
						"topic":      detectSearchTopic(fbQuery),
						"lang":       "all",
						"limit":      8,
					})
					fbArgs := string(fbArgsBytes)
					e.emitBotState(actingBot.ID, store.BotStateWorking, humanToolStatus("web_search", fbArgs))
					fbID := fmt.Sprintf("auto_web_%d", round+1)
					if emit != nil {
						emit("tool_start", map[string]any{
							"id":            fbID,
							"name":          "web_search",
							"arguments":     fbArgs,
							"sender_bot_id": actingBot.ID,
						})
					}
					fbStart := time.Now()
					fbOut, fbErr := e.ExecuteTool(toolCtx, "web_search", fbArgs)
					fbDur := time.Since(fbStart).Milliseconds()
					fbStatus := "ok"
					if fbErr != nil {
						fbStatus = "error"
						fbOut = toolErrorPayload("web_search", fbErr)
					}
					cleanFBOut := notifier.StripEmoji(fbOut)
					if emit != nil {
						emit("tool_result", map[string]any{
							"id":            fbID,
							"name":          "web_search",
							"result":        cleanFBOut,
							"status":        fbStatus,
							"duration_ms":   fbDur,
							"sender_bot_id": actingBot.ID,
						})
					}
					traces = append(traces, ToolTrace{
						ID:        fbID,
						Name:      "web_search",
						Arguments: fbArgs,
						Result:    cleanFBOut,
					})
					e.store.RecordBotExecution(store.BotExecutionLog{
						BotID:         timelineBotID,
						ThreadID:      threadID,
						ExecType:      "tool",
						Name:          "web_search",
						InputSummary:  store.TruncateRunes(fbArgs, 160),
						OutputSummary: store.TruncateRunes(cleanFBOut, 200),
						Status:        fbStatus,
						DurationMs:    fbDur,
					})
					if fbErr == nil {
						finalContent.Reset()
						messages = append(messages, apiMessage{
							Role: "system",
							Content: fmt.Sprintf(
								"[系统强制联网检索结果（查询词: %s）]\n%s\n\n重要指令：你具备完整的实时联网搜索能力，系统已直接为你拉取上述最新全网实时检索结果。请立即彻底抛弃任何“没有联网方式”、“无法访问互联网”或“没有实时信息”的说法，直接基于上述实时检索结果输出高质量、有信息密度的正文。",
								fbQuery, cleanFBOut,
							),
						})
						continue
					}
				}
			}
			// Harness 5: Post-turn anti-repetition rewrite guard on correction turns
			if isCorrection && prevAssistantReply != "" && textBigramSimilarity(prevAssistantReply, finalContent.String()) >= 0.70 && ctx.Err() == nil {
				rewriteTargets := targets
				if len(targets) > 1 {
					rewriteTargets = append(append([]ModelTarget{}, targets[1:]...), targets[0])
				}
				rewriteMsgs := append(append([]apiMessage{}, messages...),
					apiMessage{Role: "assistant", Content: finalContent.String()},
					apiMessage{
						Role:    "system",
						Content: "[系统强制重写指令：你刚刚生成的回复与上一轮已被用户否定的旧回复相似度过高（几乎原样复读）。请立即彻底抛弃旧结构、旧开头与旧模板，严格对照用户本轮最新要求重新撰写完全不同的回答。]",
					},
				)
				var rwContentBuf strings.Builder
				rwReasoning, rwContent, _, rwUsage, rwTarget, rwErr := e.streamRoundWithFailover(toolCtx, rewriteTargets, rewriteMsgs, nil, nil, func(dR, dC string) {
					if dC != "" {
						rwContentBuf.WriteString(notifier.StripEmoji(dC))
					}
				})
				if rwErr == nil && strings.TrimSpace(rwContent) != "" && textBigramSimilarity(prevAssistantReply, rwContent) < textBigramSimilarity(prevAssistantReply, finalContent.String()) {
					activeTarget = rwTarget
					if rwUsage != nil {
						hasUsage = true
						totalUsage.PromptTokens += rwUsage.PromptTokens
						totalUsage.CachedTokens += rwUsage.CachedTokens
						totalUsage.CompletionTokens += rwUsage.CompletionTokens
						totalUsage.TotalTokens += rwUsage.TotalTokens
						totalUsage.CostUSD += rwUsage.CostUSD
					}
					cleanRW := notifier.StripEmoji(rwContent)
					roundContent = cleanRW
					finalContent.Reset()
					finalContent.WriteString(cleanRW)
					if rwReasoning != "" {
						totalReasoning.WriteString("\n" + notifier.StripEmoji(rwReasoning))
					}
				}
			}
			if persistTimeline {
				finalEntry := store.AgentMessage{
					SessionID:        "bot:" + timelineBotID,
					BotID:            timelineBotID,
					ThreadID:         threadID,
					SenderBotID:      actingBot.ID,
					MsgKind:          msgKind,
					Role:             "assistant",
					Content:          notifier.StripEmoji(roundContent),
					ReasoningContent: notifier.StripEmoji(totalReasoning.String()),
					ToolCalls:        "[]",
					ModelUsed:        activeTarget.Model,
					ModelTargetID:    activeTarget.ID,
				}
				if aid, aErr := e.store.AppendAgentMessage(finalEntry); aErr == nil && msgKind != "routine" {
					finalEntry.ID = aid
					e.publishLiveChatMessage(finalEntry)
				}
			}
			break
		}
		endedWithTools = true

		// Normalize/repair tool arguments and check duplicate read-only tool loop circuit breaker
		allDuplicateReadOnly := true
		for i := range toolCalls {
			toolCalls[i].Function.Arguments = repairToolArguments(toolCalls[i].Function.Name, toolCalls[i].Function.Arguments)
			ck := toolCalls[i].Function.Name + "\x00" + toolCalls[i].Function.Arguments
			if !e.isReadOnlyTool(toolCalls[i].Function.Name) || executedCallCounts[ck] == 0 {
				allDuplicateReadOnly = false
			}
		}
		if allDuplicateReadOnly && len(traces) > 0 {
			// Model is repeating identical read-only tool calls already executed in this turn;
			// break immediately into final synthesis instead of looping up to maxRounds.
			break
		}

		tcBytes, _ := json.Marshal(toolCalls)
		if persistTimeline {
			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID:        "bot:" + timelineBotID,
				BotID:            timelineBotID,
				ThreadID:         threadID,
				SenderBotID:      actingBot.ID,
				MsgKind:          msgKind,
				Role:             "assistant",
				Content:          notifier.StripEmoji(roundContent),
				ReasoningContent: notifier.StripEmoji(roundReasoning),
				ToolCalls:        string(tcBytes),
				ModelUsed:        activeTarget.Model,
				ModelTargetID:    activeTarget.ID,
			})
		}
		messages = append(messages, apiMessage{
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        toolCalls,
		})

		e.emitBotState(actingBot.ID, store.BotStateWorking, humanToolStatus(toolCalls[0].Function.Name, toolCalls[0].Function.Arguments))

		allReadOnly := len(toolCalls) > 1
		for _, tc := range toolCalls {
			if !e.isReadOnlyTool(tc.Function.Name) {
				allReadOnly = false
				break
			}
		}

		type batchExecResult struct {
			output     string
			status     string
			durationMs int64
		}
		batchResults := make([]batchExecResult, len(toolCalls))

		runOne := func(idx int, tc apiToolCall) {
			isRO := e.isReadOnlyTool(tc.Function.Name)
			cacheKey := tc.Function.Name + "\x00" + normalizeToolArgsJSON(tc.Function.Arguments)
			if isRO {
				turnCacheMu.Lock()
				cachedOut, ok := turnReadCache[cacheKey]
				turnCacheMu.Unlock()
				if ok {
					batchResults[idx] = batchExecResult{
						output:     cachedOut,
						status:     "ok",
						durationMs: 0,
					}
					return
				}
			} else {
				turnCacheMu.Lock()
				clear(turnReadCache)
				turnCacheMu.Unlock()
			}

			toolStart := time.Now()
			toolOutput, toolErr := e.ExecuteTool(toolCtx, tc.Function.Name, tc.Function.Arguments)
			toolDurationMs := time.Since(toolStart).Milliseconds()
			execStatus := "ok"
			if toolErr != nil {
				execStatus = "error"
				toolOutput = toolErrorPayload(tc.Function.Name, toolErr)
			}
			cleanOut := notifier.StripEmoji(toolOutput)
			if isRO && toolErr == nil {
				turnCacheMu.Lock()
				turnReadCache[cacheKey] = cleanOut
				turnCacheMu.Unlock()
			}
			batchResults[idx] = batchExecResult{
				output:     cleanOut,
				status:     execStatus,
				durationMs: toolDurationMs,
			}
		}

		for _, tc := range toolCalls {
			if emit != nil {
				emit("tool_start", map[string]any{
					"id":            tc.ID,
					"name":          tc.Function.Name,
					"arguments":     tc.Function.Arguments,
					"sender_bot_id": actingBot.ID,
				})
			}
		}

		if allReadOnly {
			if timelineBotID != "" && timelineBotID != actingBot.ID {
				e.emitBotState(timelineBotID, store.BotStateWorking, fmt.Sprintf("%s: 并发查询 %d 项工具...", actingBot.Name, len(toolCalls)))
			}
			var wg sync.WaitGroup
			for idx, tc := range toolCalls {
				wg.Add(1)
				go func(i int, call apiToolCall) {
					defer wg.Done()
					runOne(i, call)
				}(idx, tc)
			}
			wg.Wait()
		} else {
			for idx, tc := range toolCalls {
				if timelineBotID != "" && timelineBotID != actingBot.ID {
					e.emitBotState(timelineBotID, store.BotStateWorking, fmt.Sprintf("%s: %s", actingBot.Name, humanToolStatus(tc.Function.Name, tc.Function.Arguments)))
				}
				runOne(idx, tc)
			}
		}

		for idx, tc := range toolCalls {
			res := batchResults[idx]
			toolOutput := res.output
			execStatus := res.status
			toolDurationMs := res.durationMs

			execType := "tool"
			if strings.HasPrefix(tc.Function.Name, "skill_") {
				execType = "skill"
			}
			inSummary := tc.Function.Arguments
			if len([]rune(inSummary)) > 160 {
				inSummary = string([]rune(inSummary)[:160]) + "..."
			}
			outSummary := toolOutput
			if len([]rune(outSummary)) > 200 {
				outSummary = string([]rune(outSummary)[:200]) + "..."
			}
			e.store.RecordBotExecution(store.BotExecutionLog{
				BotID:         timelineBotID,
				ThreadID:      threadID,
				ExecType:      execType,
				Name:          tc.Function.Name,
				InputSummary:  inSummary,
				OutputSummary: outSummary,
				Status:        execStatus,
				DurationMs:    toolDurationMs,
			})

			ck := tc.Function.Name + "\x00" + tc.Function.Arguments
			if e.isReadOnlyTool(tc.Function.Name) {
				executedCallCounts[ck]++
			}
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
					"status":        execStatus,
					"duration_ms":   toolDurationMs,
					"sender_bot_id": actingBot.ID,
				})
			}
			if persistTimeline {
				_, _ = e.store.AppendAgentMessage(store.AgentMessage{
					SessionID:   "bot:" + timelineBotID,
					BotID:       timelineBotID,
					ThreadID:    threadID,
					SenderBotID: actingBot.ID,
					MsgKind:     "chat",
					Role:        "tool",
					Content:     toolOutput,
					ToolCallID:  tc.ID,
				})
			}
			toolMsgContent := toolOutput
			if tc.Function.Name == "web_search" {
				webSearchCalls++
				softLimit := 4
				if msgKind == "routine" {
					softLimit = 6
				}
				if webSearchCalls >= softLimit {
					toolMsgContent += "\n[系统提示：已完成多轮联网搜索，请勿继续重复发起搜索，下一轮请直接基于以上全部检索结果综合回答用户问题]"
				}
			}
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    toolMsgContent,
				ToolCallID: tc.ID,
			})
		}

		// Hot-reload Bot's mounted MCP tools in-place if mount_mcp was called during this round
		mountedInRound := false
		for _, tc := range toolCalls {
			if tc.Function.Name == "mount_mcp" {
				mountedInRound = true
				break
			}
		}
		if mountedInRound {
			if freshBot, fErr := e.store.GetBot(actingBot.ID); fErr == nil && freshBot != nil {
				*actingBot = *freshBot
				attachedSkills, _ = e.store.GetSkillsByIDs(actingBot.SkillIDList())
				tools = filterToolsForBot(actingBot, attachedSkills)
			}
		}
	}

	// Guarantee a final user-facing synthesis if the tool loop hit maxRounds while still emitting tool calls
	if endedWithTools && ctx.Err() == nil {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: "已达到本轮最大工具调用步数。请立即根据以上收集到的全部工具结果直接给出完整、清晰的中文最终答复，不要再发起新的工具调用。",
		})
		sReasoning, sContent, _, sUsage, usedTarget, sErr := e.streamRoundWithFailover(ctx, targets, messages, nil, emit, func(deltaReasoning, deltaContent string) {
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
		if sErr == nil {
			activeTarget = usedTarget
			if sUsage != nil {
				hasUsage = true
				totalUsage.PromptTokens += sUsage.PromptTokens
				totalUsage.CachedTokens += sUsage.CachedTokens
				totalUsage.CompletionTokens += sUsage.CompletionTokens
				totalUsage.TotalTokens += sUsage.TotalTokens
				totalUsage.CostUSD += sUsage.CostUSD
			}
			if strings.TrimSpace(sContent) == "" && strings.TrimSpace(finalContent.String()) == "" {
				fallback := "已完成工具调用，但模型未返回进一步总结，请在下方工具记录中查看执行详情。"
				finalContent.WriteString(fallback)
				sContent = fallback
				if emit != nil {
					emit("content", map[string]any{"delta": fallback, "sender_bot_id": actingBot.ID})
				}
			}
			msgKind := strings.TrimSpace(req.MsgKind)
			if msgKind == "" {
				msgKind = "chat"
			}
			if msgKind != "delegate" && msgKind != "routine" {
				finalEntry := store.AgentMessage{
					SessionID:        "bot:" + timelineBotID,
					BotID:            timelineBotID,
					ThreadID:         threadID,
					SenderBotID:      actingBot.ID,
					MsgKind:          msgKind,
					Role:             "assistant",
					Content:          notifier.StripEmoji(sContent),
					ReasoningContent: notifier.StripEmoji(totalReasoning.String() + sReasoning),
					ToolCalls:        "[]",
					ModelUsed:        activeTarget.Model,
					ModelTargetID:    activeTarget.ID,
				}
				if aid, aErr := e.store.AppendAgentMessage(finalEntry); aErr == nil && msgKind != "routine" {
					finalEntry.ID = aid
					e.publishLiveChatMessage(finalEntry)
				}
			}
		} else if strings.TrimSpace(finalContent.String()) == "" {
			fallback := fmt.Sprintf("已完成工具调用，但生成最终汇总时出错：%v", sErr)
			finalContent.WriteString(fallback)
			if emit != nil {
				emit("content", map[string]any{"delta": fallback, "sender_bot_id": actingBot.ID})
			}
		}
	}

	preview := truncateTitle(finalContent.String())
	e.emitBotState(actingBot.ID, store.BotStateIdle, preview)

	// Background scoped memory distillation for real chat turns (never synthetic cron routine runs)
	rememberedInTurn := false
	for _, tr := range traces {
		if tr.Name == "remember_fact" || tr.Name == "manage_memory" {
			rememberedInTurn = true
			break
		}
	}
	if req.MsgKind != "routine" && req.MsgKind != "delegate" && !rememberedInTurn && strings.TrimSpace(finalContent.String()) != "" {
		go e.distillMemoryWithScope("bot:"+timelineBotID, actingBot.BotMode, userMsg, finalContent.String())
	}
	if (req.MsgKind == "" || req.MsgKind == "chat") && len(history) >= 28 {
		go e.autoCompactThreadAsync(actingBot, timelineBotID, threadID)
	}
	if req.MsgKind == "chat" && threadID != "" && len(history) >= 22 {
		go e.maybeAutoSummarizeThread(actingBot, timelineBotID, threadID, len(history))
	}

	resp := &ChatResponse{
		BotID:            timelineBotID,
		ThreadID:         threadID,
		SenderBotID:      actingBot.ID,
		SessionID:        "bot:" + timelineBotID,
		Persona:          actingBot.ID,
		Model:            activeTarget.Model,
		ModelTargetID:    activeTarget.ID,
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

// CompactBotThread folds older messages in a Bot thread (including the default main timeline)
// into a dense rolling summary so long conversations stay sharp without losing key context.
func (e *Engine) CompactBotThread(ctx context.Context, bot *store.Bot, threadID string) (string, int, error) {
	if e == nil || e.store == nil || bot == nil {
		return "", 0, fmt.Errorf("未指定有效 Bot")
	}
	return e.compactBotThreadInternal(ctx, bot, bot.ID, strings.TrimSpace(threadID), 4)
}

func (e *Engine) autoCompactThreadAsync(actingBot *store.Bot, timelineBotID, threadID string) {
	if e == nil || e.store == nil || actingBot == nil || !e.hasLightTarget() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, _ = e.compactBotThreadInternal(ctx, actingBot, timelineBotID, threadID, 16)
}

func (e *Engine) compactBotThreadInternal(ctx context.Context, actingBot *store.Bot, timelineBotID, threadID string, keepRecent int) (string, int, error) {
	if !e.hasLightTarget() {
		return "", 0, fmt.Errorf("当前未配置可用模型，无法压缩对话上下文")
	}
	lockKey := timelineBotID + ":" + threadID
	if _, loaded := e.compactingThreads.LoadOrStore(lockKey, true); loaded {
		return "", 0, nil
	}
	defer e.compactingThreads.Delete(lockKey)

	state := e.store.GetThreadCompactionState(timelineBotID, threadID)
	msgs, err := e.store.ListBotMessagesAfterID(timelineBotID, threadID, state.UpToID, 200)
	if err != nil {
		return "", 0, err
	}
	if keepRecent < 2 {
		keepRecent = 2
	}
	if len(msgs) <= keepRecent {
		if len(msgs) >= 3 && keepRecent > 2 {
			keepRecent = 2
		} else {
			return state.Summary, 0, nil
		}
	}

	toFold := msgs[:len(msgs)-keepRecent]
	newUpToID := toFold[len(toFold)-1].ID

	var sb strings.Builder
	if strings.TrimSpace(state.Summary) != "" {
		sb.WriteString("[已有滚动摘要]\n")
		sb.WriteString(strings.TrimSpace(state.Summary))
		sb.WriteString("\n\n")
	}
	sb.WriteString("[新增待压缩对话记录]\n")
	for _, m := range toFold {
		if m.Role == "tool" {
			continue
		}
		roleLabel := m.Role
		switch m.Role {
		case "user":
			roleLabel = "用户"
		case "assistant":
			roleLabel = actingBot.Name
		case "system":
			roleLabel = "系统"
		}
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		text = store.TruncateRunes(text, 420)
		sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel, text))
		if sb.Len() > 12000 {
			break
		}
	}

	sysPrompt := `你是对话上下文压缩引擎（Context Compactor）。请将「已有滚动摘要」与「新增待压缩对话记录」合并提炼为一份精炼、高信息密度的中文滚动摘要（严格 180~550 字，严禁使用 Emoji）。
必须保留：
1. 用户的核心目标、明确偏好与已确定的约束条件；
2. 已经达成的关键结论、执行结果与重要数据/链接/ID；
3. 尚未完成或后续仍需跟进的事项。
剔除：寒暄客套、重复试错过程与无关废话。直接输出纯文本摘要，不要加前缀解释。`

	summary, err := e.CallLightModel(ctx, sysPrompt, sb.String())
	if err != nil {
		return "", 0, err
	}
	summary = notifier.StripEmoji(strings.TrimSpace(summary))
	if summary == "" {
		return "", 0, fmt.Errorf("模型返回空摘要")
	}
	if err := e.store.SaveThreadCompactionState(timelineBotID, threadID, summary, newUpToID); err != nil {
		return "", 0, err
	}
	e.publishBotThreadEvent(timelineBotID, "compacted")
	return summary, len(toFold), nil
}

func (e *Engine) maybeAutoSummarizeThread(actingBot *store.Bot, timelineBotID, threadID string, historyLen int) {
	if e == nil || e.store == nil || actingBot == nil || threadID == "" || !e.hasLightTarget() {
		return
	}
	th, err := e.store.GetBotThread(timelineBotID, threadID)
	if err != nil || th == nil {
		return
	}
	if strings.TrimSpace(th.Summary) != "" && historyLen%16 != 0 {
		return
	}
	msgs, err := e.store.ListBotMessagesByThread(timelineBotID, threadID, 60)
	if err != nil || len(msgs) < 16 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, _, _ = e.SummarizeThreadToMemory(ctx, actingBot, th, msgs, false)
}

func isSilentRoutineReply(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	upper := strings.ToUpper(trimmed)
	if upper == "[SILENT_OK]" || upper == "SILENT_OK" {
		return true
	}
	if strings.Contains(upper, "[SILENT_OK]") && len([]rune(trimmed)) <= 32 {
		return true
	}
	return false
}

// RunBotRoutine implements modules.BotRoutineExecutor so cron tasks wake up any Bot cleanly
// with its own personality, scoped memories, and tools without leaking chat-room examples
// or repeating recent push notifications.
func (e *Engine) RunBotRoutine(ctx context.Context, botID, prompt, taskName string) (string, error) {
	return e.runBotRoutineInternal(ctx, botID, prompt, taskName, "", "", false)
}

// RerollBotRoutine re-generates a fresh push replacement for a rejected notification,
// incorporating the rejected content and optional user feedback while enforcing anti-repeat guards.
func (e *Engine) RerollBotRoutine(ctx context.Context, botID, prompt, taskName, rejectedBody, userFeedback string) (string, error) {
	return e.runBotRoutineInternal(ctx, botID, prompt, taskName, rejectedBody, userFeedback, true)
}

func (e *Engine) runBotRoutineInternal(ctx context.Context, botID, prompt, taskName, rejectedBody, userFeedback string, isReroll bool) (string, error) {
	bot, err := e.store.GetBot(strings.TrimSpace(botID))
	if err != nil || bot == nil {
		return "", fmt.Errorf("未找到例程绑定 Bot: %s", botID)
	}

	prevSummary := ""
	if !strings.HasPrefix(strings.TrimSpace(bot.LastSummary), "正在执行例程:") {
		prevSummary = bot.LastSummary
	}
	prevUnread := bot.UnreadCount

	threadID := strings.TrimSpace(bot.ActiveThreadID)
	taskID := store.TaskIDFromContext(ctx)
	var recentPushes []store.Notification
	if taskID != "" {
		recentPushes, _ = e.store.ListRecentNotificationsByModule(taskID, 7)
	}
	if len(recentPushes) == 0 && strings.TrimSpace(taskName) != "" {
		recentPushes, _ = e.store.ListRecentNotificationsByTitlePrefix(fmt.Sprintf("[%s] %s", bot.Name, taskName), 7)
	}

	now := time.Now().In(store.ShanghaiLocation())
	_, promptExamples := extractPromptExamples(prompt)
	isMonitoring := !isReroll && isMonitoringRoutine(taskName, prompt)
	taskSeed := taskID
	if taskSeed == "" {
		taskSeed = bot.ID + ":" + taskName
	}
	if isReroll {
		// Perturb the daily angle seed so reroll picks a different editorial focus immediately
		taskSeed += fmt.Sprintf(":reroll:%d", now.UnixNano()%100000)
	}
	isResearch := !isMonitoring && isResearchOrDigestRoutine(taskName, prompt)
	contextSnapshot := buildRoutineContextSnapshotForMode(e.store, taskSeed, now, isResearch)
	antiRepeatLedger := buildRoutineAntiRepeatLedger(recentPushes)
	routinePrompt := buildRoutinePromptMessage(taskName, prompt, now, contextSnapshot, antiRepeatLedger, promptExamples, isMonitoring)

	if isReroll {
		var fb strings.Builder
		fb.WriteString("\n\n[用户对上一条推送不满意，触发「换一条重推」指令]\n")
		if cleanFB := notifier.StripEmoji(strings.TrimSpace(userFeedback)); cleanFB != "" {
			fb.WriteString("- 用户明确反馈/调教要求：" + cleanFB + "\n")
		} else {
			fb.WriteString("- 用户希望换一个完全不同的全新方向、具体案例与切入点。\n")
		}
		if cleanRej := strings.TrimSpace(rejectedBody); cleanRej != "" {
			fb.WriteString("- 被否定的旧推送（严禁重复其主题、案例或结构）：" + truncateRunes(cleanRej, 240) + "\n")
		}
		fb.WriteString("- 请直接输出全新的高质量推送正文，不要道歉、不要提及“换一条/重写”等元话术。")
		routinePrompt += fb.String()
	}

	reqModel := strings.TrimSpace(bot.Model)
	if taskOverride := store.TaskModelFromContext(ctx); taskOverride != "" {
		reqModel = taskOverride
	}
	req := ChatRequest{
		BotID:    bot.ID,
		ThreadID: threadID,
		MsgKind:  "routine",
		Message:  routinePrompt,
		Model:    reqModel,
	}

	// Intentionally pass nil chat history so background routines rely on the Bot's persona,
	// scoped long-term memories, live tools, and the recent push ledger—never parroting
	// casual examples from the interactive chat room.
	var routineHistory []store.AgentMessage
	executeTurn := func(turnPrompt string) (*ChatResponse, error) {
		turnReq := req
		turnReq.Message = turnPrompt
		if bot.IsGroup {
			allBots, _ := e.store.ListBots()
			members := resolveGroupMemberBots(bot, allBots)
			if len(members) == 0 {
				return nil, fmt.Errorf("群聊「%s」暂无可用成员", bot.Name)
			}
			memberSet := make(map[string]bool, len(members))
			for _, m := range members {
				memberSet[m.ID] = true
			}
			allSkills, _ := e.store.ListSkills()
			skillMap := make(map[string]store.Skill, len(allSkills))
			for _, sk := range allSkills {
				skillMap[sk.ID] = sk
			}
			switch strings.TrimSpace(bot.GroupMode) {
			case "parallel":
				return e.runTeamRoomParallel(ctx, turnReq, bot, members, memberSet, routineHistory, turnPrompt, nil)
			case "sequential":
				return e.runTeamRoomHandoff(ctx, turnReq, bot, members, memberSet, routineHistory, turnPrompt, nil)
			default:
				targets := selectSmartGroupMembersWithSkills(members, skillMap, prompt)
				return e.runTeamRoomHandoff(ctx, turnReq, bot, targets, memberSet, routineHistory, turnPrompt, nil)
			}
		}
		return e.runSingleBotTurn(ctx, turnReq, bot, bot.ID, routineHistory, turnPrompt, nil)
	}

	resp, err := executeTurn(routinePrompt)
	if err != nil {
		return "", err
	}

	// Include rejectedBody in anti-repeat comparison when rerolling
	comparePushes := recentPushes
	if isReroll && strings.TrimSpace(rejectedBody) != "" {
		comparePushes = append([]store.Notification{{
			MarkdownBody: rejectedBody,
			CreatedAt:    now.Format("2006-01-02 15:04"),
		}}, recentPushes...)
	}

	finalReply := resp.Content
	if !isSilentRoutineReply(finalReply) {
		quality := evaluateRoutineDraft(finalReply, comparePushes, promptExamples)
		if quality.shouldSuppressMonitoring(isMonitoring) {
			finalReply = ""
		} else if quality.needsRewrite(isMonitoring) && ctx.Err() == nil {
			rewritePrompt := routinePrompt + "\n\n" + buildRoutineRewriteInstruction(quality, finalReply)
			if rwResp, rwErr := executeTurn(rewritePrompt); rwErr == nil && rwResp != nil && !isSilentRoutineReply(rwResp.Content) {
				rwQuality := evaluateRoutineDraft(rwResp.Content, comparePushes, promptExamples)
				improvedExample := quality.EchoedExample != "" && rwQuality.EchoedExample == ""
				improvedSim := rwQuality.EchoedExample == "" && rwQuality.MaxPastSim < quality.MaxPastSim
				if improvedExample || improvedSim || !rwQuality.needsRewrite(isMonitoring) {
					finalReply = rwResp.Content
				}
			}
		}
	}

	// Always restore the Bot's chat state & summary cleanly without polluting chat history or unread badge;
	// BotPromptModule delivers the routine result directly to 服务通知 (notifications).
	_ = e.store.SetBotState(bot.ID, store.BotStateIdle, prevSummary)
	if e.hub != nil {
		e.hub.PublishEvent("bot_state", map[string]any{
			"bot_id":       bot.ID,
			"state":        store.BotStateIdle,
			"unread_count": prevUnread,
			"last_summary": prevSummary,
		})
	}
	if !isReroll && isSilentRoutineReply(finalReply) {
		return "", nil
	}
	return strings.TrimSpace(finalReply), nil
}
