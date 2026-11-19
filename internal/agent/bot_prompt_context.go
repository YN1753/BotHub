package agent

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/store"
)

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

func (e *Engine) buildWorkingContextBlock(bot *store.Bot, tools []ToolDefinition) string {
	if e == nil || e.store == nil || bot == nil || store.NormalizeBotMode(bot.BotMode) == store.BotModeCompanion {
		return ""
	}
	hasTodos, hasTasks := false, false
	for _, td := range tools {
		switch td.Function.Name {
		case "manage_todos":
			hasTodos = true
		case "manage_tasks":
			hasTasks = true
		}
	}
	if !hasTodos && !hasTasks {
		return ""
	}
	var sb strings.Builder
	today := time.Now().In(store.ShanghaiLocation()).Format("2006-01-02")
	if hasTodos {
		todos, _ := e.store.ListTodos("open", "", 8)
		if len(todos) > 0 {
			sb.WriteString("[当前未完成待办焦点 (Live Snapshot)]\n")
			for i, td := range todos {
				if i >= 6 {
					sb.WriteString(fmt.Sprintf("- ... 另有 %d 项待办可通过 manage_todos(list) 查看\n", len(todos)-6))
					break
				}
				dueTag := ""
				if strings.TrimSpace(td.DueAt) != "" {
					day := td.DueAt
					if len(day) >= 10 {
						day = day[:10]
					}
					if day < today {
						dueTag = " [已逾期:" + td.DueAt + "]"
					} else if day == today {
						dueTag = " [今日到期:" + td.DueAt + "]"
					} else {
						dueTag = " [截止:" + td.DueAt + "]"
					}
				}
				sb.WriteString(fmt.Sprintf("- [%s|%s] %s (id=%s)%s\n", td.Priority, td.Status, truncateTitle(td.Title), td.ID, dueTag))
			}
		}
	}
	if hasTasks {
		tasks, _ := e.store.ListTasks()
		var enabled []store.Task
		for _, t := range tasks {
			if t.Enabled {
				enabled = append(enabled, t)
			}
		}
		if len(enabled) > 0 {
			sb.WriteString("[活跃定时/一次性提醒焦点]\n")
			for i, t := range enabled {
				if i >= 4 {
					sb.WriteString(fmt.Sprintf("- ... 另有 %d 项启用任务\n", len(enabled)-4))
					break
				}
				sched := t.CronExpr
				if t.RunAt != "" {
					sched = "一次性@" + t.RunAt
				}
				sb.WriteString(fmt.Sprintf("- %s (id=%s, %s)\n", truncateTitle(t.Name), t.ID, sched))
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildBotSystemPrompt(bot *store.Bot, attachedSkills []store.Skill, memories []store.AgentMemory, peerBots []store.Bot, threadTitle string, inGroup bool) string {
	return buildBotSystemPromptWithContext(bot, attachedSkills, memories, peerBots, threadTitle, inGroup, "")
}

func buildBotSystemPromptWithContext(bot *store.Bot, attachedSkills []store.Skill, memories []store.AgentMemory, peerBots []store.Bot, threadTitle string, inGroup bool, workingContext string) string {
	var sb strings.Builder
	if prompt := strings.TrimSpace(bot.SystemPrompt); prompt != "" {
		sb.WriteString(prompt)
		sb.WriteString("\n\n")
	}

	now := time.Now().In(store.ShanghaiLocation())
	sb.WriteString("[当前运行上下文]\n")
	sb.WriteString(fmt.Sprintf("- 当前时间：%s (%s Asia/Shanghai)\n", now.Format("2006-01-02 15:04"), weekdayCN(now.Weekday())))
	if strings.TrimSpace(threadTitle) != "" {
		sb.WriteString(fmt.Sprintf("- 当前话题：%s\n", strings.TrimSpace(threadTitle)))
	}
	sb.WriteString("\n")

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
		sb.WriteString("- 工作准则：你是干练务实的专业执行者。涉及实时状态、资产、待办、文档、任务或外部技能时，优先调用手头工具查证或直接落地执行，先给明确结论与可验证结果，绝不编造数据。\n")
	default:
		sb.WriteString("- 互动与执行准则：日常对话自然真诚、像真人般有温度；当用户交代要办的事、提醒、链接收藏、闪念记录、资料检索或产出长文档时，像靠谱的个人助手一样主动调用待办(manage_todos)、收藏与闪念匣(manage_stash)、文档(manage_notes)、定时提醒(manage_tasks)及授权工具直接帮你落地。\n")
	}

	if inGroup {
		sb.WriteString("- 群聊协作规范：你正与其他成员同在一个协同群聊里。自然接续前序成员或用户的上下文，重点给出你独特视角的补充或行动，避免重复已经说过的客套话。\n")
	}
	sb.WriteString("- 格式约束：严禁使用任何 Emoji 表情或装饰性特殊符号。\n")

	if len(peerBots) > 0 {
		var soloPeers, groupPeers []store.Bot
		for _, p := range peerBots {
			if p.IsGroup {
				groupPeers = append(groupPeers, p)
			} else if p.ID != bot.ID {
				soloPeers = append(soloPeers, p)
			}
		}
		if len(soloPeers) > 0 {
			sb.WriteString("\n[工作区已有 Bot 成员 (可通过 consult_bot 协同，或通过 manage_bots 管理/拉群)]\n")
			for _, p := range soloPeers {
				sb.WriteString(fmt.Sprintf("- %s (id=%s): %s\n", p.Name, p.ID, p.RoleTitle))
			}
		}
		if len(groupPeers) > 0 {
			sb.WriteString("\n[工作区已有协同群聊 (可通过 manage_bots 调整群成员与公告)]\n")
			for _, g := range groupPeers {
				sb.WriteString(fmt.Sprintf("- %s (id=%s, mode=%s, 成员数=%d): %s\n", g.Name, g.ID, g.GroupMode, len(g.MemberIDList()), g.RoleTitle))
			}
		}
	}

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

	if workingContext != "" {
		sb.WriteString("\n")
		sb.WriteString(workingContext)
		sb.WriteString("\n")
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

// selectRelevantMemories scores memories by pin status, importance, recency decay, query token overlap, and recall affinity.
func selectRelevantMemories(memories []store.AgentMemory, query string, maxItems int) []store.AgentMemory {
	if len(memories) <= maxItems || maxItems <= 0 {
		return memories
	}
	queryTokens := extractMatchTokens(query)
	loc := store.ShanghaiLocation()
	now := time.Now().In(loc)
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
		freshRef := m.LastRecalledAt
		if freshRef == "" {
			freshRef = m.UpdatedAt
		}
		if ts, err := time.ParseInLocation("2006-01-02 15:04:05", freshRef, loc); err == nil {
			ageHours := now.Sub(ts).Hours()
			switch {
			case ageHours <= 72:
				pts += 12
			case ageHours <= 24*14:
				pts += 6
			case ageHours <= 24*30:
				pts += 2
			case !m.Pinned && ageHours > 24*90:
				pts -= 10
			case !m.Pinned && ageHours > 24*60:
				pts -= 6
			}
		}
		if len(queryTokens) > 0 {
			keyLower := strings.ToLower(m.FactKey)
			catLower := strings.ToLower(m.Category)
			contentLower := strings.ToLower(m.Content)
			hits := 0
			for _, tok := range queryTokens {
				matched := false
				if strings.Contains(keyLower, tok) {
					pts += 16
					matched = true
				}
				if strings.Contains(contentLower, tok) {
					pts += 8
					matched = true
				} else if strings.Contains(catLower, tok) {
					pts += 6
					matched = true
				}
				if matched {
					hits++
				}
			}
			if hits > 0 {
				pts += (hits * 20) / len(queryTokens)
			}
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

// budgetChatMessages keeps all leading system messages and trims oldest conversation turns
// starting from clean user boundaries whenever total rune count exceeds maxRunes.
func budgetChatMessages(messages []apiMessage, maxRunes int) []apiMessage {
	if maxRunes <= 0 || len(messages) == 0 {
		return messages
	}
	total := 0
	for _, m := range messages {
		total += len([]rune(m.Content))
	}
	if total <= maxRunes {
		return messages
	}
	cur := messages
	for total > maxRunes {
		trimmed, ok := trimContextMessages(cur)
		if !ok {
			break
		}
		cur = trimmed
		total = 0
		for _, m := range cur {
			total += len([]rune(m.Content))
		}
	}
	return cur
}
