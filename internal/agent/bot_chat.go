package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

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

func filterToolsForBot(bot *store.Bot, attachedSkills []store.Skill) []ToolDefinition {
	all := BuiltinTools()
	var filtered []ToolDefinition
	if bot == nil {
		filtered = append(filtered, all...)
	} else {
		allowed := bot.AllowedToolList()
		if len(allowed) > 0 {
			allowSet := make(map[string]bool, len(allowed))
			for _, name := range allowed {
				allowSet[strings.TrimSpace(name)] = true
			}
			for _, td := range all {
				if allowSet[td.Function.Name] {
					filtered = append(filtered, td)
				}
			}
		}
	}
	for _, sk := range attachedSkills {
		if sk.Enabled && sk.Kind == "webhook" {
			filtered = append(filtered, CompileSkillTool(sk))
		}
	}
	return filtered
}

func buildBotSystemPrompt(bot *store.Bot, attachedSkills []store.Skill, memories []store.AgentMemory, inGroup bool) string {
	var sb strings.Builder
	if prompt := strings.TrimSpace(bot.SystemPrompt); prompt != "" {
		sb.WriteString(prompt)
		sb.WriteString("\n\n")
	}

	mode := store.NormalizeBotMode(bot.BotMode)
	sb.WriteString("[人格定位与交流方式]\n")
	sb.WriteString(fmt.Sprintf("- 你的名字：%s；定位：%s\n", bot.Name, bot.RoleTitle))
	if traits := strings.TrimSpace(bot.Traits); traits != "" {
		sb.WriteString(fmt.Sprintf("- 性格与说话风格：%s\n", traits))
	}

	switch mode {
	case store.BotModeCompanion:
		sb.WriteString("- 互动准则：你是一个真实、有温度、有自己性格特点的聊天伙伴。像老朋友或正常人类那样自然说话——有情绪共鸣、懂得接梗和轻松吐槽，回复节奏像真人日常聊天，不要端着公文腔、不要强行分条说教或推销建议。\n")
		sb.WriteString("- 记忆自然化：你记住过往细节是为了像真朋友一样自然延续话题，不要机械背诵记忆条目标题。\n")
	case store.BotModeWorker:
		sb.WriteString("- 工作准则：你是干练务实的专业执行者。涉及实时状态、资产、任务或外部技能时，优先调用手头工具查证后再作答，先给明确结论与可执行结果，绝不编造数据。\n")
	default:
		sb.WriteString("- 互动与执行准则：日常对话自然真诚、像真人般有温度；当需要查证数据、记录重要约定或执行任务时，主动调用已授权的工具与技能完成。\n")
	}

	if inGroup {
		sb.WriteString("- 群聊协作规范：你正与其他成员同在一个协同群聊里。自然接续前序成员或用户的上下文，重点给出你独特视角的补充或行动，避免重复已经说过的客套话。\n")
	}
	sb.WriteString("- 格式约束：严禁使用任何 Emoji 表情或装饰性特殊符号。\n")

	var promptSkills []store.Skill
	for _, sk := range attachedSkills {
		if sk.Enabled && sk.Kind == "prompt" && strings.TrimSpace(sk.Instruction) != "" {
			promptSkills = append(promptSkills, sk)
		}
	}
	if len(promptSkills) > 0 {
		sb.WriteString("\n[已装备专属技能规范]\n")
		for _, sk := range promptSkills {
			sb.WriteString(fmt.Sprintf("- 技能「%s」: %s\n", sk.Name, strings.TrimSpace(sk.Instruction)))
		}
	}

	if len(memories) > 0 {
		sb.WriteString("\n[关于此段关系/作用域的长期记忆]\n")
		for _, m := range memories {
			pinTag := ""
			if m.Pinned {
				pinTag = "[置顶] "
			}
			sb.WriteString(fmt.Sprintf("- %s(%s) %s: %s\n", pinTag, m.Category, m.FactKey, m.Content))
		}
	}
	return sb.String()
}

func isCJKRune(r rune) bool {
	return unicode.Is(unicode.Han, r) || (r >= 0x3040 && r <= 0x30ff) || (r >= 0xac00 && r <= 0xd7af)
}

// extractMatchTokens extracts both ASCII/alphanumeric words (>=2 chars) and CJK unigrams/bigrams
// so Chinese sentences without spaces match member names, traits, roles, skills, and memories accurately.
func extractMatchTokens(text string) []string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}
	seen := make(map[string]bool)
	tokens := make([]string, 0, 24)
	addTok := func(t string) {
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		tokens = append(tokens, t)
	}

	var asciiBuf strings.Builder
	var cjkRun []rune
	flushASCII := func() {
		if asciiBuf.Len() >= 2 {
			addTok(asciiBuf.String())
		}
		asciiBuf.Reset()
	}
	flushCJK := func() {
		if len(cjkRun) == 1 {
			addTok(string(cjkRun[0]))
		} else if len(cjkRun) >= 2 {
			for i := 0; i < len(cjkRun)-1; i++ {
				addTok(string(cjkRun[i : i+2]))
			}
		}
		cjkRun = cjkRun[:0]
	}

	for _, r := range lower {
		switch {
		case isCJKRune(r):
			flushASCII()
			cjkRun = append(cjkRun, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			flushCJK()
			asciiBuf.WriteRune(r)
		default:
			flushASCII()
			flushCJK()
		}
	}
	flushASCII()
	flushCJK()
	return tokens
}

// selectRelevantMemories scores memories by pin status, importance, query token overlap, and recall affinity.
func selectRelevantMemories(memories []store.AgentMemory, query string, maxItems int) []store.AgentMemory {
	if len(memories) <= maxItems || maxItems <= 0 {
		return memories
	}
	queryTokens := extractMatchTokens(query)
	type scoredMem struct {
		mem   store.AgentMemory
		score int
		idx   int
	}
	ranked := make([]scoredMem, 0, len(memories))
	for i, m := range memories {
		pts := m.Importance * 12
		if m.Pinned {
			pts += 1000
		}
		if m.RecallCount > 0 {
			bonus := m.RecallCount * 2
			if bonus > 16 {
				bonus = 16
			}
			pts += bonus
		}
		if len(queryTokens) > 0 {
			haystack := strings.ToLower(m.FactKey + " " + m.Category + " " + m.Content)
			hits := 0
			for _, tok := range queryTokens {
				if strings.Contains(haystack, tok) {
					hits++
				}
			}
			pts += hits * 10
		}
		ranked = append(ranked, scoredMem{mem: m, score: pts, idx: i})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].idx < ranked[j].idx
	})
	out := make([]store.AgentMemory, 0, maxItems)
	for i := 0; i < maxItems && i < len(ranked); i++ {
		out = append(out, ranked[i].mem)
	}
	return out
}

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
	toolDomainKeywords := map[string][]string{
		"query_nezha_nodes": {"探针", "哪吒", "vps", "节点", "服务器", "cpu", "内存", "负载", "掉线", "宕机", "告警", "延迟", "ssh", "nezha"},
		"manage_assets":     {"资产", "域名", "到期", "续费", "账单", "会员", "cloudflare"},
		"manage_tasks":      {"定时", "例程", "cron", "日程", "晨报", "调度"},
		"web_search":        {"github", "开源", "趋势", "雷达", "新闻", "前沿", "hn", "hacker", "搜索", "推荐", "查一下"},
		"read_url":          {"网页", "url", "链接", "http", "文章"},
		"manage_memory":     {"记忆", "记住", "忘掉", "修改记忆", "置顶"},
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
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

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
	userMsgID, err := e.store.AppendAgentMessage(store.AgentMessage{
		SessionID:    "bot:" + bot.ID,
		BotID:        bot.ID,
		ThreadID:     threadID,
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
	if bot.IsGroup {
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
			return e.runTeamRoomHandoff(ctx, req, bot, mentioned, memberSet, history, promptMsg, baseURL, apiKey, emit)
		}

		allSkills, _ := e.store.ListSkills()
		skillMap := make(map[string]store.Skill, len(allSkills))
		for _, sk := range allSkills {
			skillMap[sk.ID] = sk
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
			targets = e.selectGroupSpeakersWithLLM(ctx, members, skillMap, userMsg)
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
			if store.NormalizeBotMode(sp.BotMode) == store.BotModeCompanion {
				turnPrompt = fmt.Sprintf("用户说：%s\n\n[同群伙伴刚接了这几句话]\n%s\n\n请你以自己的性格自然接话、吐槽或回应，不要重复对方已经说过的话。", userMsg, combinedContent.String())
			} else {
				turnPrompt = fmt.Sprintf("用户指令：%s\n\n[前置协同成员输出参考]\n%s\n\n请紧扣你的专业定位补充差异化结论或执行结果，避免重复前文。", userMsg, combinedContent.String())
			}
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
	sysPrompt := buildBotSystemPrompt(actingBot, attachedSkills, memories, timelineBotID != actingBot.ID)

	var botNameMap map[string]string
	if timelineBotID != actingBot.ID {
		if allBots, bErr := e.store.ListBots(); bErr == nil {
			botNameMap = make(map[string]string, len(allBots))
			for _, ab := range allBots {
				botNameMap[ab.ID] = ab.Name
			}
		}
	}

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

	tools := filterToolsForBot(actingBot, attachedSkills)
	toolCtx := WithToolScope(ctx, ToolScope{
		BotID:         actingBot.ID,
		TimelineBotID: timelineBotID,
		ThreadID:      threadID,
		MemoryPolicy:  actingBot.MemoryPolicy,
	})
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
				ThreadID:         threadID,
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
			ThreadID:         threadID,
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
			toolStart := time.Now()
			toolOutput, toolErr := e.ExecuteTool(toolCtx, tc.Function.Name, tc.Function.Arguments)
			toolDurationMs := time.Since(toolStart).Milliseconds()
			execStatus := "ok"
			if toolErr != nil {
				execStatus = "error"
				errPayload, _ := json.Marshal(map[string]any{"ok": false, "error": toolErr.Error()})
				toolOutput = string(errPayload)
			}
			toolOutput = notifier.StripEmoji(toolOutput)

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
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})
		}
	}

	preview := truncateTitle(finalContent.String())
	e.emitBotState(actingBot.ID, store.BotStateIdle, preview)

	// Background scoped memory distillation for both Companion & Worker Bots
	rememberedInTurn := false
	for _, tr := range traces {
		if tr.Name == "remember_fact" {
			rememberedInTurn = true
			break
		}
	}
	if !rememberedInTurn && strings.TrimSpace(finalContent.String()) != "" {
		go e.distillMemoryWithScope("bot:"+timelineBotID, actingBot.BotMode, userMsg, finalContent.String())
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
