package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func (e *Engine) resolveMentionedBots(userMsg string, allBots []store.Bot) []store.Bot {
	lower := strings.ToLower(userMsg)
	type hit struct {
		bot store.Bot
		pos int
	}
	var hits []hit
	seen := make(map[string]bool)
	for _, b := range allBots {
		if b.IsGroup || seen[b.ID] {
			continue
		}
		pos := -1
		if b.Name != "" {
			if idx := strings.Index(userMsg, "@"+b.Name); idx >= 0 {
				pos = idx
			}
		}
		if pos < 0 {
			if idx := strings.Index(lower, "@"+strings.ToLower(b.ID)); idx >= 0 {
				pos = idx
			}
		}
		if pos >= 0 {
			seen[b.ID] = true
			hits = append(hits, hit{bot: b, pos: pos})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].pos < hits[j].pos
	})
	matched := make([]store.Bot, 0, len(hits))
	for _, h := range hits {
		matched = append(matched, h.bot)
	}
	return matched
}

func resolveGroupMemberBots(groupBot *store.Bot, allBots []store.Bot) []store.Bot {
	botMap := make(map[string]store.Bot, len(allBots))
	for _, b := range allBots {
		if !b.IsGroup {
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
	return selectSmartGroupMembersWithSkills(members, nil, userMsg)
}

func selectSmartGroupMembersWithSkills(members []store.Bot, skillMap map[string]store.Skill, userMsg string) []store.Bot {
	if len(members) <= 1 {
		return members
	}
	lower := strings.ToLower(userMsg)
	for _, kw := range []string{"大家", "全部", "各位", "一起", "全员", "接力", "群策群力"} {
		if strings.Contains(userMsg, kw) {
			return members
		}
	}

	userTokens := extractMatchTokens(userMsg)

	type scored struct {
		bot   store.Bot
		score int
	}
	ranked := make([]scored, 0, len(members))
	for _, m := range members {
		pts := 0
		tools := m.AllowedToolList()
		for _, toolName := range tools {
			spec, ok := LookupToolSpec(toolName)
			if !ok {
				continue
			}
			for _, kw := range spec.DomainKeywords {
				if strings.Contains(lower, kw) {
					pts += 4
					break
				}
			}
		}
		if skillMap != nil {
			for _, sid := range m.SkillIDList() {
				if sk, ok := skillMap[sid]; ok && sk.Enabled {
					skText := strings.ToLower(sk.Name + " " + sk.Description + " " + sk.Slug)
					for _, tok := range userTokens {
						if strings.Contains(skText, tok) {
							pts += 4
						}
					}
				}
			}
		}
		metaText := strings.ToLower(m.Name + " " + m.RoleTitle + " " + m.SystemPrompt + " " + m.Traits)
		for _, token := range userTokens {
			if strings.Contains(metaText, token) {
				pts += 2
			}
		}
		if strings.Contains(lower, strings.ToLower(m.Name)) {
			pts += 6
		}
		// Companion bots get a natural conversational affinity boost on short/casual messages
		if pts == 0 && len([]rune(strings.TrimSpace(userMsg))) <= 24 && store.NormalizeBotMode(m.BotMode) == store.BotModeCompanion {
			pts += 2
		}
		if pts > 0 {
			ranked = append(ranked, scored{bot: m, score: pts})
		}
	}

	if len(ranked) > 0 {
		sort.SliceStable(ranked, func(i, j int) bool {
			return ranked[i].score > ranked[j].score
		})
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

// selectGroupSpeakersWithLLM optionally consults the lightweight model to pick 1-2 ideal group speakers
// when deterministic keyword scoring is ambiguous across >=3 group members.
func (e *Engine) selectGroupSpeakersWithLLM(ctx context.Context, members []store.Bot, skillMap map[string]store.Skill, userMsg string) []store.Bot {
	heuristic := selectSmartGroupMembersWithSkills(members, skillMap, userMsg)
	if len(members) < 3 || !e.hasLightTarget() {
		return heuristic
	}

	type memberSummary struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Mode      string   `json:"mode"`
		RoleTitle string   `json:"role"`
		Traits    string   `json:"traits,omitempty"`
		Tools     []string `json:"tools,omitempty"`
	}
	roster := make([]memberSummary, 0, len(members))
	memberMap := make(map[string]store.Bot, len(members))
	for _, m := range members {
		memberMap[m.ID] = m
		roster = append(roster, memberSummary{
			ID:        m.ID,
			Name:      m.Name,
			Mode:      store.NormalizeBotMode(m.BotMode),
			RoleTitle: m.RoleTitle,
			Traits:    m.Traits,
			Tools:     m.AllowedToolList(),
		})
	}
	rosterBytes, _ := json.Marshal(roster)
	routeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	sys := `你是群聊智能主持人。根据群成员定位与用户发言，挑出最适合回答或接话的 1~2 位成员 ID。
只输出纯 JSON（不要代码块）：{"member_ids":["bot_1"]}`
	raw, err := e.CallLightModel(routeCtx, sys, fmt.Sprintf("群成员：%s\n用户消息：%s", string(rosterBytes), userMsg))
	if err != nil {
		return heuristic
	}
	clean := stripJSONCodeFence(raw)

	var parsed struct {
		MemberIDs []string `json:"member_ids"`
	}
	if json.Unmarshal([]byte(clean), &parsed) != nil || len(parsed.MemberIDs) == 0 {
		return heuristic
	}
	picked := make([]store.Bot, 0, 2)
	seen := make(map[string]bool)
	for _, id := range parsed.MemberIDs {
		if b, ok := memberMap[strings.TrimSpace(id)]; ok && !seen[b.ID] {
			seen[b.ID] = true
			picked = append(picked, b)
			if len(picked) >= 2 {
				break
			}
		}
	}
	if len(picked) == 0 {
		return heuristic
	}
	return picked
}

func (e *Engine) runTeamRoomHandoff(
	ctx context.Context,
	req ChatRequest,
	roomBot *store.Bot,
	specialists []store.Bot,
	memberSet map[string]bool,
	roomHistory []store.AgentMessage,
	userMsg string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	return e.runTeamRoomHandoffWithRelay(ctx, req, roomBot, specialists, nil, false, memberSet, roomHistory, userMsg, emit)
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
	userMsg string,
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
			priorText := combinedContent.String()
			if r := []rune(priorText); len(r) > 2800 {
				priorText = string(r[:1200]) + "\n...\n[中段讨论已折叠]\n...\n" + string(r[len(r)-1400:])
			}
			if store.NormalizeBotMode(sp.BotMode) == store.BotModeCompanion {
				turnPrompt = fmt.Sprintf("用户说：%s\n\n[同群伙伴刚接了这几句话]\n%s\n\n请你以自己的性格自然接话、吐槽或回应，不要重复对方已经说过的话。", userMsg, priorText)
			} else {
				turnPrompt = fmt.Sprintf("用户指令：%s\n\n[前置协同成员输出参考]\n%s\n\n请紧扣你的专业定位补充差异化结论或执行结果，避免重复前文。", userMsg, priorText)
			}
		}
		spCopy := sp
		resp, err := e.runSingleBotTurn(ctx, req, &spCopy, roomBot.ID, roomHistory, turnPrompt, emit)
		if err != nil {
			if combinedContent.Len() > 0 {
				combinedContent.WriteString("\n\n---\n\n")
				combinedContent.WriteString(fmt.Sprintf("**[%s]**\n[暂未响应: %s]", sp.Name, err.Error()))
				continue
			}
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
		ThreadID:         strings.TrimSpace(req.ThreadID),
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

// runTeamRoomParallel executes multiple specialists simultaneously in parallel goroutines,
// streaming the lead specialist live while buffering peers and flushing them in deterministic order.
func (e *Engine) runTeamRoomParallel(
	ctx context.Context,
	req ChatRequest,
	roomBot *store.Bot,
	specialists []store.Bot,
	memberSet map[string]bool,
	roomHistory []store.AgentMessage,
	userMsg string,
	emit func(event string, data any),
) (*ChatResponse, error) {
	if len(specialists) <= 1 {
		return e.runTeamRoomHandoff(ctx, req, roomBot, specialists, memberSet, roomHistory, userMsg, emit)
	}
	parStart := time.Now()
	e.emitBotState(roomBot.ID, store.BotStateWorking, fmt.Sprintf("并发协同 %d 位成员并行作答...", len(specialists)))

	type sseFrame struct {
		event string
		data  any
	}
	type workerResult struct {
		sp     store.Bot
		resp   *ChatResponse
		err    error
		frames []sseFrame
	}

	results := make([]workerResult, len(specialists))
	var emitMu sync.Mutex
	safeEmit := func(event string, data any) {
		if emit == nil {
			return
		}
		emitMu.Lock()
		defer emitMu.Unlock()
		emit(event, data)
	}

	// Emit step 1 turn header before concurrent start so UI binds live deltas to specialist[0]
	first := specialists[0]
	safeEmit("bot_turn", map[string]any{
		"bot_id":        roomBot.ID,
		"sender_bot_id": first.ID,
		"sender_name":   first.Name,
		"role_title":    first.RoleTitle,
		"avatar_style":  first.AvatarStyle,
		"is_temp_guest": memberSet != nil && !memberSet[first.ID],
		"step":          1,
		"total_steps":   len(specialists),
	})

	var wg sync.WaitGroup
	for idx, sp := range specialists {
		wg.Add(1)
		go func(i int, member store.Bot) {
			defer wg.Done()
			spCopy := member
			var localFrames []sseFrame
			workerEmit := safeEmit
			if i > 0 {
				workerEmit = func(ev string, d any) {
					localFrames = append(localFrames, sseFrame{event: ev, data: d})
				}
			}
			parallelPrompt := fmt.Sprintf("用户指令：%s\n\n（当前为多成员并行作答模式：请直接聚焦你的专业领域或视角给出清晰、独立、不冗长的结论或行动结果。）", userMsg)
			if store.NormalizeBotMode(member.BotMode) == store.BotModeCompanion {
				parallelPrompt = fmt.Sprintf("用户说：%s\n\n（大家正在群里同时聊天，请按你自己的性格自然回一句。）", userMsg)
			}
			resp, err := e.runSingleBotTurn(ctx, req, &spCopy, roomBot.ID, roomHistory, parallelPrompt, workerEmit)
			results[i] = workerResult{
				sp:     member,
				resp:   resp,
				err:    err,
				frames: localFrames,
			}
		}(idx, sp)
	}
	wg.Wait()

	var combinedContent strings.Builder
	var combinedReasoning strings.Builder
	var allTraces []ToolTrace
	var totalUsage TokenUsage
	hasUsage := false
	var lastSuccessResponder *store.Bot

	for i, r := range results {
		if i > 0 {
			safeEmit("bot_turn", map[string]any{
				"bot_id":        roomBot.ID,
				"sender_bot_id": r.sp.ID,
				"sender_name":   r.sp.Name,
				"role_title":    r.sp.RoleTitle,
				"avatar_style":  r.sp.AvatarStyle,
				"is_temp_guest": memberSet != nil && !memberSet[r.sp.ID],
				"step":          i + 1,
				"total_steps":   len(results),
			})
			for _, f := range r.frames {
				safeEmit(f.event, f.data)
			}
		}
		if r.err != nil {
			if combinedContent.Len() > 0 {
				combinedContent.WriteString("\n\n---\n\n")
			}
			combinedContent.WriteString(fmt.Sprintf("**[%s]**\n[暂未响应: %s]", r.sp.Name, r.err.Error()))
			continue
		}
		spRef := r.sp
		lastSuccessResponder = &spRef
		if combinedContent.Len() > 0 {
			combinedContent.WriteString("\n\n---\n\n")
		}
		combinedContent.WriteString(fmt.Sprintf("**[%s]**\n%s", r.sp.Name, r.resp.Content))
		if r.resp.ReasoningContent != "" {
			combinedReasoning.WriteString(r.resp.ReasoningContent + "\n")
		}
		allTraces = append(allTraces, r.resp.ToolTraces...)
		if r.resp.Usage != nil {
			hasUsage = true
			totalUsage.PromptTokens += r.resp.Usage.PromptTokens
			totalUsage.CompletionTokens += r.resp.Usage.CompletionTokens
			totalUsage.TotalTokens += r.resp.Usage.TotalTokens
		}
	}

	if lastSuccessResponder == nil {
		firstErr := results[0].err
		if firstErr == nil {
			firstErr = fmt.Errorf("所有群聊成员均未能返回结果")
		}
		e.emitBotState(roomBot.ID, store.BotStateBlocked, firstErr.Error())
		return nil, firstErr
	}

	e.emitBotState(roomBot.ID, store.BotStateIdle, truncateTitle(combinedContent.String()))

	memberNames := make([]string, 0, len(specialists))
	for _, sp := range specialists {
		memberNames = append(memberNames, sp.Name)
	}
	e.store.RecordBotExecution(store.BotExecutionLog{
		BotID:         roomBot.ID,
		ThreadID:      strings.TrimSpace(req.ThreadID),
		ExecType:      "delegate",
		Name:          fmt.Sprintf("并发协同 %d 人", len(specialists)),
		InputSummary:  truncateTitle(userMsg),
		OutputSummary: strings.Join(memberNames, "、"),
		Status:        "ok",
		DurationMs:    time.Since(parStart).Milliseconds(),
	})

	finalResp := &ChatResponse{
		BotID:            roomBot.ID,
		ThreadID:         strings.TrimSpace(req.ThreadID),
		SenderBotID:      lastSuccessResponder.ID,
		SessionID:        "bot:" + roomBot.ID,
		Model:            lastSuccessResponder.Model,
		ReasoningContent: combinedReasoning.String(),
		Content:          combinedContent.String(),
		ToolTraces:       allTraces,
	}
	if hasUsage {
		finalResp.Usage = &totalUsage
	}
	safeEmit("done", finalResp)
	return finalResp, nil
}
