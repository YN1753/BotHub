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

	userMsg := notifier.StripEmoji(strings.TrimSpace(req.Message))
	if userMsg == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	replyPreview := notifier.StripEmoji(strings.TrimSpace(req.ReplyPreview))
	threadID := strings.TrimSpace(req.ThreadID)

	// Load prior messages strictly scoped to this Bot/Group + Thread
	history, _ := e.store.ListBotMessagesByThread(bot.ID, threadID, 40)

	// Auto-title custom topic thread on first user message
	if threadID != "" {
		if th, _ := e.store.GetBotThread(bot.ID, threadID); th != nil && (th.Title == "新话题" || th.Title == "新对话" || strings.TrimSpace(th.Title) == "") {
			th.Title = truncateTitle(userMsg)
			_ = e.store.SaveBotThread(*th)
		}
	}

	// Persist user message once in this Bot/Group's isolated thread timeline
	if _, err := e.store.AppendAgentMessage(store.AgentMessage{
		SessionID:    "bot:" + bot.ID,
		BotID:        bot.ID,
		ThreadID:     threadID,
		MsgKind:      "chat",
		Role:         "user",
		Content:      userMsg,
		ReplyToID:    req.ReplyToID,
		ReplyPreview: replyPreview,
	}); err != nil {
		return nil, fmt.Errorf("记录消息失败: %w", err)
	}

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

	threadID := strings.TrimSpace(req.ThreadID)
	statusHint := "正在思考..."
	if store.NormalizeBotMode(actingBot.BotMode) == store.BotModeCompanion {
		statusHint = "正在输入..."
	}
	e.emitBotState(actingBot.ID, store.BotStateThinking, statusHint)
	defer func() {
		_ = e.store.ClearBotUnread(timelineBotID)
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
	memories := selectRelevantMemories(rawMemories, userMsg, 24)
	if len(memories) > 0 {
		recalledIDs := make([]int64, 0, len(memories))
		for _, m := range memories {
			recalledIDs = append(recalledIDs, m.ID)
		}
		e.store.TouchRecalledMemories(recalledIDs)
	}

	attachedSkills, _ := e.store.GetSkillsByIDs(actingBot.SkillIDList())
	tools := filterToolsForBot(actingBot, attachedSkills)
	hasConsultTool := false
	for _, td := range tools {
		if td.Function.Name == "consult_bot" {
			hasConsultTool = true
			break
		}
	}

	var allBots []store.Bot
	if timelineBotID != actingBot.ID || hasConsultTool {
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
	if hasConsultTool {
		peerBots = allBots
	}
	workingCtx := e.buildWorkingContextBlock(actingBot, tools)
	sysPrompt := buildBotSystemPromptWithContext(actingBot, attachedSkills, memories, peerBots, threadTitle, timelineBotID != actingBot.ID, workingCtx)

	var botNameMap map[string]string
	if timelineBotID != actingBot.ID && len(allBots) > 0 {
		botNameMap = make(map[string]string, len(allBots))
		for _, ab := range allBots {
			botNameMap[ab.ID] = ab.Name
		}
	}

	messages := make([]apiMessage, 0, len(history)+8)
	messages = append(messages, apiMessage{
		Role:    "system",
		Content: sysPrompt,
	})
	if threadSummary != "" && len(history) >= 16 {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: fmt.Sprintf("[此前较早对话压缩前情摘要]\n%s", threadSummary),
		})
	}
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
	messages = append(messages, apiMessage{Role: "user", Content: userMsg})
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

	const maxRounds = 8
	endedWithTools := false
	var turnCacheMu sync.Mutex
	turnReadCache := make(map[string]string)

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

		msgKind := strings.TrimSpace(req.MsgKind)
		if msgKind == "" {
			msgKind = "chat"
		}
		persistTimeline := msgKind != "delegate"
		if len(toolCalls) == 0 {
			endedWithTools = false
			if persistTimeline {
				_, _ = e.store.AppendAgentMessage(store.AgentMessage{
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
				})
			}
			break
		}
		endedWithTools = true

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
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})
		}
	}

	// Guarantee a final user-facing synthesis if the tool loop hit maxRounds while still emitting tool calls
	if endedWithTools && ctx.Err() == nil {
		messages = append(messages, apiMessage{
			Role:    "system",
			Content: "已达到本轮最大工具调用步数。请立即根据以上收集到的全部工具结果直接给出完整、清晰的中文最终答复，不要再发起新的工具调用。",
		})
		sReasoning, sContent, _, sUsage, usedTarget, sErr := e.streamRoundWithFailover(ctx, targets, messages, nil, thinkFlag, emit, func(deltaReasoning, deltaContent string) {
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
				totalUsage.CompletionTokens += sUsage.CompletionTokens
				totalUsage.TotalTokens += sUsage.TotalTokens
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
			if msgKind != "delegate" {
				_, _ = e.store.AppendAgentMessage(store.AgentMessage{
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
				})
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
// with its own personality, scoped memories, and tools without storing artificial user prompts.
func (e *Engine) RunBotRoutine(ctx context.Context, botID, prompt, taskName string) (string, error) {
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
	history, _ := e.store.ListBotMessagesByThread(bot.ID, threadID, 20)
	prevMaxID := int64(0)
	if len(history) > 0 {
		prevMaxID = history[len(history)-1].ID
	}
	nowStr := store.NowShanghai()
	routinePrompt := fmt.Sprintf("[后台定时例程「%s」于 %s 自动触发，请用你本人的口吻与工具能力完成以下指令并直接输出给用户；若本指令属于状态监控、健康检查或异常巡检性质，且当前各项指标均正常无需报警，请仅输出 [SILENT_OK]，不要输出例行正常汇报]\n%s", taskName, nowStr, strings.TrimSpace(prompt))

	req := ChatRequest{
		BotID:    bot.ID,
		ThreadID: threadID,
		MsgKind:  "routine",
		Message:  routinePrompt,
		Model:    bot.Model,
	}

	var resp *ChatResponse
	if bot.IsGroup {
		allBots, _ := e.store.ListBots()
		members := resolveGroupMemberBots(bot, allBots)
		if len(members) == 0 {
			return "", fmt.Errorf("群聊「%s」暂无可用成员", bot.Name)
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
			resp, err = e.runTeamRoomParallel(ctx, req, bot, members, memberSet, history, routinePrompt, nil)
		case "sequential":
			resp, err = e.runTeamRoomHandoff(ctx, req, bot, members, memberSet, history, routinePrompt, nil)
		default:
			targets := selectSmartGroupMembersWithSkills(members, skillMap, prompt)
			resp, err = e.runTeamRoomHandoff(ctx, req, bot, targets, memberSet, history, routinePrompt, nil)
		}
	} else {
		resp, err = e.runSingleBotTurn(ctx, req, bot, bot.ID, history, routinePrompt, nil)
	}
	if err != nil {
		return "", err
	}

	if isSilentRoutineReply(resp.Content) {
		_ = e.store.DeleteBotMessagesAfterID(bot.ID, threadID, prevMaxID)
		_ = e.store.SetBotState(bot.ID, store.BotStateIdle, prevSummary)
		if e.hub != nil {
			e.hub.PublishEvent("bot_state", map[string]any{
				"bot_id":       bot.ID,
				"state":        store.BotStateIdle,
				"unread_count": prevUnread,
				"last_summary": prevSummary,
			})
		}
		return "", nil
	}

	summary := truncateTitle(resp.Content)
	_ = e.store.IncrementBotUnread(bot.ID, summary)
	if e.hub != nil {
		msgs, _ := e.store.ListBotMessagesByThread(bot.ID, threadID, 1)
		if len(msgs) > 0 {
			e.hub.PublishEvent("bot_message", msgs[len(msgs)-1])
		}
		updatedBot, _ := e.store.GetBot(bot.ID)
		unread := 0
		if updatedBot != nil {
			unread = updatedBot.UnreadCount
		}
		e.hub.PublishEvent("bot_state", map[string]any{
			"bot_id":       bot.ID,
			"state":        store.BotStateDone,
			"unread_count": unread,
			"last_summary": summary,
		})
	}
	return resp.Content, nil
}
