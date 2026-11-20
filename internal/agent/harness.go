package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	reInlineHTTPURL = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `，。！？；：、（）()\[\]【】{}]+`)

	// Domain intent clusters for dynamic tool pruning when a bot has many (>7) tools enabled.
	toolIntentGroups = []struct {
		tools    []string
		keywords []string
	}{
		{
			tools:    []string{"web_search", "read_url"},
			keywords: []string{"搜索", "搜一下", "联网", "查一下", "最新", "新闻", "资讯", "官网", "文档", "教程", "网页", "链接", "文章", "http://", "https://", "github", "开源", "趋势", "热点", "search", "google", "bing"},
		},
		{
			tools:    []string{"manage_todos", "datetime_calc"},
			keywords: []string{"待办", "todo", "任务清单", "日程", "提醒我", "计划", "安排", "清单", "ddl", "截止", "完成"},
		},
		{
			tools:    []string{"manage_notes"},
			keywords: []string{"笔记", "备忘", "记录", "写一篇", "文档", "note", "总结存下来", "草稿", "知识库"},
		},
		{
			tools:    []string{"manage_stash", "read_url"},
			keywords: []string{"收藏", "稍后读", "稍后看", "书签", "stash", "链接库", "闪念", "读过", "未读"},
		},
		{
			tools:    []string{"manage_assets", "calculate_eval", "datetime_calc"},
			keywords: []string{"资产", "订阅", "续费", "到期", "会员", "域名", "账单", "花费", "开销", "年费", "月费", "vps续费", "成本"},
		},
		{
			tools:    []string{"query_nezha_nodes"},
			keywords: []string{"服务器", "vps", "探针", "哪吒", "nezha", "节点", "cpu", "内存", "负载", "在线", "宕机", "流量", "监控"},
		},
		{
			tools:    []string{"manage_tasks"},
			keywords: []string{"定时", "巡检", "cron", "自动任务", "调度", "早报", "日报", "周期任务", "计划任务"},
		},
		{
			tools:    []string{"remember_fact", "recall_memory", "manage_memory"},
			keywords: []string{"记住", "记忆", "偏好", "忘掉", "忘记", "遗忘", "我叫", "我的习惯", "memory"},
		},
		{
			tools:    []string{"send_notification"},
			keywords: []string{"推送", "通知", "发通知", "告警", "提醒消息", "push"},
		},
		{
			tools:    []string{"manage_bots", "consult_bot", "manage_models", "manage_skills"},
			keywords: []string{"机器人", "bot", "助手", "队友", "协作", "模型", "渠道", "技能", "skill", "插件", "提示词", "人设"},
		},
		{
			tools:    []string{"workspace_briefing"},
			keywords: []string{"简报", "概览", "总览", "盘点", "汇报", "今日概况", "工作台", "汇总"},
		},
		{
			tools:    []string{"calculate_eval", "datetime_calc"},
			keywords: []string{"计算", "算一下", "等于", "汇率", "折扣", "几天", "几号", "星期几", "周几", "倒计时", "日期", "时间差"},
		},
	}

	explicitSearchTriggers = []string{
		"联网", "上网搜", "上网查", "全网搜", "搜索一下", "搜一下", "帮我搜", "帮我查",
		"查一下", "查查", "检索一下", "网上查", "网上搜", "最新消息", "最新进展",
		"最新动态", "实时新闻", "今日新闻", "今天的新闻", "近期新闻", "热搜",
		"最新价格", "最新版本", "发布了吗", "search ", "google ", "bing ",
	}

	localWorkspaceExclusions = []string{
		"待办", "todo", "定时任务", "巡检任务", "资产", "订阅", "到期", "续费",
		"服务器", "vps", "探针", "哪吒", "nezha", "我的笔记", "笔记里", "笔记中",
		"我的收藏", "收藏里", "收藏夹", "稍后读", "stash", "我的记忆", "记忆库",
		"模型池", "免费池", "付费池", "机器人列表", "我的bot", "我的待办", "工作台",
	}

	stashSaveIntentHints = []string{
		"收藏", "存到收藏", "加到收藏", "加入收藏", "稍后读", "稍后看", "收进", "存一下",
	}

	correctionSignals = []string{
		"不对", "错了", "不是这个", "不是我要", "不是问", "没按", "没有按", "按要求",
		"按我说的", "照我说的", "重新来", "重来", "重新回答", "重新搜", "重新查",
		"换个", "换一种", "别再", "不要再", "一样", "重复", "复读", "答非所问",
		"没看懂", "没听懂", "偏题", "跑题", "太蠢", "不行", "还是错", "不对啊",
		"我说了", "说了好几遍", "说了几次", "第三次", "第二次", "还是老样子",
	}

	searchPrefixStripper = []string{
		"请帮我联网搜索一下", "帮我联网搜索一下", "请联网搜索一下", "联网搜索一下",
		"请帮我联网搜一下", "帮我联网搜一下", "请联网搜一下", "联网搜一下",
		"请帮我上网搜索", "帮我上网搜索", "请上网搜一下", "上网搜一下", "上网查一下",
		"请帮我搜索一下", "帮我搜索一下", "请搜索一下", "搜索一下",
		"请帮我搜一下", "帮我搜一下", "请搜一下", "搜一下", "帮我搜",
		"请帮我查一下", "帮我查一下", "请查一下", "查一下", "帮我查",
		"请帮我检索一下", "帮我检索一下", "检索一下", "联网查一下", "联网查", "联网搜", "联网搜索",
		"重新搜一下", "重新查一下", "换个关键词搜一下", "换个关键词查一下",
	}
)

// pruneToolsForTurn narrows a bot's tool schema list from up to 19 tools down to the
// 4-7 tools most relevant to the current user message and recent 2 turns.
// This dramatically reduces schema confusion for smaller/free models.
func pruneToolsForTurn(allBotTools []ToolDefinition, userMsg string, recentHistory []store.AgentMessage) []ToolDefinition {
	if len(allBotTools) <= 7 {
		return allBotTools
	}

	var contextBuf strings.Builder
	contextBuf.WriteString(strings.ToLower(strings.TrimSpace(userMsg)))
	recentAdded := 0
	for i := len(recentHistory) - 1; i >= 0 && recentAdded < 2; i-- {
		m := recentHistory[i]
		if (m.Role == "user" || m.Role == "assistant") && strings.TrimSpace(m.Content) != "" {
			contextBuf.WriteByte('\n')
			runes := []rune(strings.ToLower(strings.TrimSpace(m.Content)))
			if len(runes) > 300 {
				runes = runes[:300]
			}
			contextBuf.WriteString(string(runes))
			recentAdded++
		}
	}
	haystack := contextBuf.String()
	userLower := strings.ToLower(strings.TrimSpace(userMsg))

	scores := make(map[string]int, len(allBotTools))
	for _, td := range allBotTools {
		name := td.Function.Name
		// Baseline core tools keep a small positive floor so web/memory capability is always reachable.
		switch name {
		case "web_search":
			scores[name] = 3
		case "read_url":
			scores[name] = 2
		case "remember_fact":
			scores[name] = 1
		}

		if sp, ok := LookupToolSpec(name); ok {
			for _, kw := range sp.DomainKeywords {
				kwLow := strings.ToLower(strings.TrimSpace(kw))
				if kwLow == "" {
					continue
				}
				if strings.Contains(userLower, kwLow) {
					scores[name] += 6
				} else if strings.Contains(haystack, kwLow) {
					scores[name] += 2
				}
			}
		}

		// Custom skills: match by name or description fragments
		if strings.HasPrefix(name, "skill_") {
			skillKey := strings.TrimPrefix(name, "skill_")
			if strings.Contains(userLower, skillKey) || strings.Contains(userLower, "技能") || strings.Contains(userLower, "skill") {
				scores[name] += 8
			} else if desc := strings.ToLower(td.Function.Description); desc != "" {
				for _, tok := range strings.Fields(userLower) {
					if len([]rune(tok)) >= 2 && strings.Contains(desc, tok) {
						scores[name] += 4
						break
					}
				}
			}
		}
	}

	for _, grp := range toolIntentGroups {
		hitUser := false
		hitCtx := false
		for _, kw := range grp.keywords {
			if strings.Contains(userLower, kw) {
				hitUser = true
				break
			}
			if strings.Contains(haystack, kw) {
				hitCtx = true
			}
		}
		if hitUser {
			for _, tName := range grp.tools {
				scores[tName] += 10
			}
		} else if hitCtx {
			for _, tName := range grp.tools {
				scores[tName] += 3
			}
		}
	}

	type scoredTool struct {
		idx   int
		td    ToolDefinition
		score int
	}
	ranked := make([]scoredTool, 0, len(allBotTools))
	for i, td := range allBotTools {
		ranked = append(ranked, scoredTool{idx: i, td: td, score: scores[td.Function.Name]})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].idx < ranked[j].idx
	})

	const maxPrunedTools = 7
	const minPrunedTools = 4
	selected := make(map[int]bool, maxPrunedTools)
	for _, st := range ranked {
		if len(selected) >= maxPrunedTools {
			break
		}
		if st.score > 0 || len(selected) < minPrunedTools {
			selected[st.idx] = true
		}
	}

	out := make([]ToolDefinition, 0, len(selected))
	for i, td := range allBotTools {
		if selected[i] {
			out = append(out, td)
		}
	}
	return out
}

func hasToolNamed(tools []ToolDefinition, name string) bool {
	for _, td := range tools {
		if td.Function.Name == name {
			return true
		}
	}
	return false
}

func cleanPreflightSearchQuery(userMsg string) string {
	q := strings.TrimSpace(userMsg)
	// Remove leading correction interjections before stripping search verbs
	for _, lead := range []string{"不对，", "不对,", "不对 ", "错了，", "错了,", "不是这个，", "不是这个,", "重新来，", "重新来,"} {
		if strings.HasPrefix(q, lead) {
			q = strings.TrimSpace(strings.TrimPrefix(q, lead))
		}
	}
	for _, prefix := range searchPrefixStripper {
		if strings.HasPrefix(q, prefix) {
			q = strings.TrimSpace(strings.TrimPrefix(q, prefix))
			break
		}
	}
	q = strings.Trim(q, "：:，,。！？?!\"'“” ")
	runes := []rune(q)
	if len(runes) > 120 {
		q = string(runes[:120])
	}
	return strings.TrimSpace(q)
}

// detectPreflightGrounding determines whether a turn should deterministically execute
// read_url or web_search BEFORE Round 0 LLM inference so weak/free models always receive
// real grounded web context instead of hallucinating or skipping tool calls.
func detectPreflightGrounding(userMsg string, tools []ToolDefinition) (toolName string, argsJSON string, ok bool) {
	trimmed := strings.TrimSpace(userMsg)
	if len([]rune(trimmed)) < 3 {
		return "", "", false
	}
	lower := strings.ToLower(trimmed)

	// 1. Explicit URL -> read_url (unless the user is explicitly asking to bookmark/stash it)
	if hasToolNamed(tools, "read_url") {
		if rawURL := reInlineHTTPURL.FindString(trimmed); rawURL != "" {
			isStashOnly := false
			for _, hint := range stashSaveIntentHints {
				if strings.Contains(lower, hint) {
					isStashOnly = true
					break
				}
			}
			if !isStashOnly {
				b, _ := json.Marshal(map[string]any{"url": rawURL})
				return "read_url", string(b), true
			}
		}
	}

	// 2. Explicit web-search intent -> web_search (never hijack local workspace queries)
	if !hasToolNamed(tools, "web_search") {
		return "", "", false
	}
	for _, excl := range localWorkspaceExclusions {
		if strings.Contains(lower, excl) {
			return "", "", false
		}
	}
	triggered := false
	for _, trig := range explicitSearchTriggers {
		if strings.Contains(lower, trig) {
			triggered = true
			break
		}
	}
	if !triggered {
		return "", "", false
	}
	query := cleanPreflightSearchQuery(trimmed)
	if len([]rune(query)) < 2 {
		return "", "", false
	}
	b, _ := json.Marshal(map[string]any{"query": query, "limit": 5})
	return "web_search", string(b), true
}

// preflightResultHasUsefulData checks whether a preflight web_search / read_url output
// actually contains usable results before injecting it into model context.
func preflightResultHasUsefulData(toolName, rawOutput string) bool {
	if strings.TrimSpace(rawOutput) == "" {
		return false
	}
	switch toolName {
	case "web_search":
		var parsed struct {
			Count int `json:"count"`
		}
		return json.Unmarshal([]byte(rawOutput), &parsed) == nil && parsed.Count > 0
	case "read_url":
		var parsed struct {
			CharCount int `json:"char_count"`
		}
		return json.Unmarshal([]byte(rawOutput), &parsed) == nil && parsed.CharCount >= 40
	default:
		return true
	}
}

// detectCorrectionTurn returns true when the user is explicitly correcting or expressing
// dissatisfaction with the assistant's prior response in an ongoing conversation.
func detectCorrectionTurn(userMsg string, history []store.AgentMessage) bool {
	hasPriorAssistant := false
	for _, m := range history {
		if m.Role == "assistant" && strings.TrimSpace(m.Content) != "" {
			hasPriorAssistant = true
			break
		}
	}
	if !hasPriorAssistant {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(userMsg))
	if lower == "" {
		return false
	}
	for _, sig := range correctionSignals {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}

// applyCorrectionGuardrail breaks repetitive reply loops on weaker models by:
// 1. Compacting older rejected assistant turns (earlier than the immediately preceding reply)
//    so their text stops acting as a copy-paste attractor.
// 2. Appending a high-salience final system directive requiring point-by-point adherence
//    and fresh search/reading rather than repeating prior templates.
func applyCorrectionGuardrail(messages []apiMessage) []apiMessage {
	if len(messages) == 0 {
		return messages
	}
	out := make([]apiMessage, len(messages))
	copy(out, messages)

	// Locate the most recent assistant message with text content (keep it intact as negative reference)
	lastAssistantIdx := -1
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "assistant" && strings.TrimSpace(out[i].Content) != "" {
			lastAssistantIdx = i
			break
		}
	}

	// Compact earlier rejected assistant turns to reduce attention-sink repetition
	for i := 0; i < lastAssistantIdx; i++ {
		if out[i].Role == "assistant" && len(out[i].ToolCalls) == 0 {
			runes := []rune(strings.TrimSpace(out[i].Content))
			if len(runes) > 160 {
				out[i].Content = fmt.Sprintf("[此前已被用户否定的旧回复已折叠: %s...]", string(runes[:120]))
			}
		}
	}

	out = append(out, apiMessage{
		Role: "system",
		Content: `[纠偏执行指令 - 最高优先级]
用户刚刚明确指出你此前的回答未满足要求或存在重复。本轮必须严格遵守：
1. 严禁复读、改写或沿用上一轮已被否定的结论与模板段落。
2. 先逐条拆解用户本轮最新提出的具体约束条件（范围、格式、来源、角度、数量等），逐项落实。
3. 如需联网获取信息，必须更换与此前不同的精准关键词调用 web_search，或直接调用 read_url 精读来源正文后再作答。
4. 若工具确实未查到所需内容，请坦诚说明已查过的方向与缺失项，严禁编造或重复旧答案敷衍。`,
	})
	return out
}
