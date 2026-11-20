package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	reInlineHTTPURL   = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `，。！？；：、（）()\[\]【】{}]+`)
	reInlineThinkFull = regexp.MustCompile(`(?is)<think(?:ing)?>\s*(.*?)\s*</think(?:ing)?>`)
	reXmlToolCall     = regexp.MustCompile(`(?is)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
	reFuncTagCall     = regexp.MustCompile(`(?is)<function=([a-zA-Z0-9_]+)>\s*(\{.*?\})\s*</function>`)
	reCodeFenceJSON   = regexp.MustCompile("(?is)```(?:json|tool_call|tool_code)?\\s*(\\{.*?\\})\\s*```")

	// Domain intent clusters for dynamic tool pruning when a bot has many (>7) tools enabled.
	toolIntentGroups = []struct {
		tools    []string
		keywords []string
	}{
		{
			tools:    []string{"web_search", "read_url"},
			keywords: []string{"搜索", "搜一下", "联网", "查一下", "最新", "新闻", "资讯", "官网", "文档", "教程", "网页", "链接", "文章", "http://", "https://", "github", "开源", "趋势", "热点", "价格", "多少钱", "评测", "对比", "天气", "search", "google", "bing"},
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

	// Implicit live/factual query signals that require fresh web grounding even if user didn't say "搜一下".
	implicitWebFactSignals = []string{
		"2025", "2026", "今年", "本月", "今天", "今日", "最近", "近期", "近日", "昨晚",
		"现在多少钱", "多少钱", "最新价格", "股价", "汇率", "行情", "票房", "天气",
		"比分", "排名", "榜单", "什么时候发布", "什么时候出", "出了吗", "更新了什么",
		"哪个好用", "对比评测", "报错", "怎么解决", "官方文档", "官网地址",
	}

	localWorkspaceExclusions = []string{
		"待办", "todo", "定时任务", "巡检任务", "资产", "订阅", "到期", "续费",
		"服务器", "vps", "探针", "哪吒", "nezha", "我的笔记", "笔记里", "笔记中",
		"我的收藏", "收藏里", "收藏夹", "稍后读", "stash", "我的记忆", "记忆库",
		"模型池", "免费池", "付费池", "机器人列表", "我的bot", "我的待办", "工作台",
	}

	workspaceMutationVerbs = []string{
		"添加", "新增", "创建", "新建", "加一个", "加一条", "记一条", "记一下", "写一篇",
		"删除", "删掉", "移除", "清空", "修改", "更新", "改成", "改为", "标记", "完成",
		"勾选", "置顶", "取消", "关闭", "开启", "禁用", "启用", "记住", "忘掉",
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
		"请问一下", "请问", "能不能告诉我", "能告诉我", "帮我看看", "我想知道", "告诉我",
	}

	searchTailNoise = []string{
		"是怎么回事", "是什么情况", "有哪些内容", "详细说说", "详细介绍一下",
		"是什么", "怎么样", "好不好", "可以吗", "吗？", "吗?", "呢？", "呢?", "啊？", "啊?",
	}
)

// pruneToolsForTurn narrows a bot's tool schema list from up to 19 tools down to the
// 4-7 tools most relevant to the current user message and recent 2 turns.
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

// distillSearchKeywords cleans a conversational prompt into a high-hit primary search query
// and an optional broader fallback query (used automatically when primary returns <2 hits).
func distillSearchKeywords(userMsg string) (primary string, fallback string) {
	q := strings.TrimSpace(userMsg)
	for _, lead := range []string{
		"不对，", "不对,", "不对 ", "错了，", "错了,", "不是这个，", "不是这个,",
		"重新来，", "重新来,", "没按要求，", "没按我要求，",
	} {
		if strings.HasPrefix(q, lead) {
			q = strings.TrimSpace(strings.TrimPrefix(q, lead))
		}
	}
	// If the user wrote a multi-clause instruction (e.g., "搜一下 xxx，要求列出3点并对比"),
	// isolate the search topic clause before formatting instructions.
	for _, sep := range []string{"，要求", ",要求", "。要求", "，请按", "，并按", "，不要", "，用表格", "，列个表"} {
		if idx := strings.Index(q, sep); idx > 0 {
			q = strings.TrimSpace(q[:idx])
		}
	}
	for _, prefix := range searchPrefixStripper {
		if strings.HasPrefix(q, prefix) {
			q = strings.TrimSpace(strings.TrimPrefix(q, prefix))
			break
		}
	}
	for _, tail := range searchTailNoise {
		if strings.HasSuffix(q, tail) && len([]rune(q)) > len([]rune(tail))+2 {
			q = strings.TrimSpace(strings.TrimSuffix(q, tail))
		}
	}
	q = strings.Trim(q, "：:，,。！？?!\"'“” ")
	runes := []rune(q)
	if len(runes) > 100 {
		q = string(runes[:100])
	}
	primary = strings.TrimSpace(q)

	// Build a compact keyword-only fallback by removing conversational filler particles
	fb := primary
	for _, filler := range []string{"关于", "请问", "最新的", "目前的", "现在的", "有什么", "有哪些", "怎么", "如何", "为什么", "的"} {
		fb = strings.ReplaceAll(fb, filler, " ")
	}
	fb = strings.Join(strings.Fields(fb), " ")
	if fb != "" && fb != primary && len([]rune(fb)) >= 2 {
		fallback = fb
	}
	return primary, fallback
}

func cleanPreflightSearchQuery(userMsg string) string {
	primary, _ := distillSearchKeywords(userMsg)
	return primary
}

func detectLocalWorkspaceReadPreflight(lower string, tools []ToolDefinition) (toolName string, argsJSON string, ok bool) {
	// Never preflight a read tool if the user is asking to create, update, or delete data.
	for _, verb := range workspaceMutationVerbs {
		if strings.Contains(lower, verb) {
			return "", "", false
		}
	}
	isQuestionOrCheck := strings.Contains(lower, "有哪些") || strings.Contains(lower, "有什么") ||
		strings.Contains(lower, "看看") || strings.Contains(lower, "查看") ||
		strings.Contains(lower, "查一下") || strings.Contains(lower, "查查") ||
		strings.Contains(lower, "列表") || strings.Contains(lower, "状态") ||
		strings.Contains(lower, "多少") || strings.Contains(lower, "几个") ||
		strings.Contains(lower, "正常吗") || strings.Contains(lower, "到期") ||
		strings.Contains(lower, "未读") || strings.Contains(lower, "简报") ||
		strings.Contains(lower, "概览") || strings.HasSuffix(lower, "?") || strings.HasSuffix(lower, "？")
	if !isQuestionOrCheck {
		return "", "", false
	}

	if hasToolNamed(tools, "query_nezha_nodes") &&
		(strings.Contains(lower, "服务器") || strings.Contains(lower, "探针") || strings.Contains(lower, "哪吒") || strings.Contains(lower, "vps状态") || strings.Contains(lower, "节点负载")) {
		return "query_nezha_nodes", `{}`, true
	}
	if hasToolNamed(tools, "manage_assets") &&
		(strings.Contains(lower, "资产") || strings.Contains(lower, "订阅") || strings.Contains(lower, "续费")) {
		if strings.Contains(lower, "快到期") || strings.Contains(lower, "即将到期") {
			return "manage_assets", `{"action":"expiring"}`, true
		}
		return "manage_assets", `{"action":"list"}`, true
	}
	if hasToolNamed(tools, "manage_stash") &&
		(strings.Contains(lower, "收藏") || strings.Contains(lower, "稍后读") || strings.Contains(lower, "stash")) {
		if strings.Contains(lower, "未读") {
			return "manage_stash", `{"action":"list","status":"unread"}`, true
		}
		return "manage_stash", `{"action":"list"}`, true
	}
	if hasToolNamed(tools, "manage_notes") &&
		(strings.Contains(lower, "我的笔记") || strings.Contains(lower, "笔记列表") || strings.Contains(lower, "有哪些笔记")) {
		return "manage_notes", `{"action":"list"}`, true
	}
	if hasToolNamed(tools, "workspace_briefing") &&
		(strings.Contains(lower, "工作简报") || strings.Contains(lower, "今日简报") || strings.Contains(lower, "工作台概览") || strings.Contains(lower, "全局概览")) {
		return "workspace_briefing", `{}`, true
	}
	return "", "", false
}

// detectPreflightGrounding determines whether a turn should deterministically execute
// read_url, a read-only workspace inspection tool, or web_search BEFORE Round 0 LLM inference
// so weak/free models always receive real grounded facts instead of skipping tool calls.
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

	// 2. Read-only local workspace inspection queries -> preflight workspace read tool
	if wsTool, wsArgs, wsOK := detectLocalWorkspaceReadPreflight(lower, tools); wsOK {
		return wsTool, wsArgs, true
	}

	// 3. Explicit OR implicit web-search intent -> web_search (never hijack local workspace queries)
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
		for _, sig := range implicitWebFactSignals {
			if strings.Contains(lower, sig) {
				triggered = true
				break
			}
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

// preflightResultHasUsefulData checks whether a preflight tool output contains usable facts.
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
		var generic map[string]any
		if json.Unmarshal([]byte(rawOutput), &generic) == nil {
			if okVal, exists := generic["ok"].(bool); exists && !okVal {
				return false
			}
		}
		return true
	}
}

// splitInlineThinkTags separates <think>...</think> blocks that some free/relay models emit
// inside normal content delta from user-facing content.
func splitInlineThinkTags(content string) (extractedReasoning string, cleanContent string) {
	if !strings.Contains(strings.ToLower(content), "<think") && !strings.Contains(strings.ToLower(content), "</think") {
		return "", content
	}
	var thinkParts []string
	cleaned := reInlineThinkFull.ReplaceAllStringFunc(content, func(m string) string {
		sub := reInlineThinkFull.FindStringSubmatch(m)
		if len(sub) > 1 && strings.TrimSpace(sub[1]) != "" {
			thinkParts = append(thinkParts, strings.TrimSpace(sub[1]))
		}
		return ""
	})
	// Handle orphaned closing </think> (when model omitted opening <think>)
	lowerCleaned := strings.ToLower(cleaned)
	if closeIdx := strings.Index(lowerCleaned, "</think>"); closeIdx >= 0 {
		prefix := strings.TrimSpace(cleaned[:closeIdx])
		if prefix != "" {
			thinkParts = append(thinkParts, prefix)
		}
		cleaned = cleaned[closeIdx+len("</think>"):]
	} else if openIdx := strings.Index(lowerCleaned, "<think>"); openIdx >= 0 {
		// Unclosed <think> at end of stream
		tail := strings.TrimSpace(cleaned[openIdx+len("<think>"):])
		if tail != "" {
			thinkParts = append(thinkParts, tail)
		}
		cleaned = cleaned[:openIdx]
	}
	return strings.Join(thinkParts, "\n"), strings.TrimSpace(cleaned)
}

// rescueEmptyContentFromReasoning extracts a concise final answer when a reasoning model
// emits all text into reasoning_content and leaves content blank.
func rescueEmptyContentFromReasoning(reasoning string) string {
	trimmed := strings.TrimSpace(reasoning)
	if trimmed == "" {
		return ""
	}
	paragraphs := strings.Split(trimmed, "\n\n")
	for i := len(paragraphs) - 1; i >= 0; i-- {
		p := strings.TrimSpace(paragraphs[i])
		if len([]rune(p)) >= 12 {
			runes := []rune(p)
			if len(runes) > 800 {
				return string(runes[len(runes)-800:])
			}
			return p
		}
	}
	runes := []rune(trimmed)
	if len(runes) > 600 {
		return string(runes[len(runes)-600:])
	}
	return trimmed
}

// extractTextEmbeddedToolCalls rescues tool invocations that weaker models or non-standard relays
// emit as <tool_call>{...}</tool_call>, <function=name>{...}</function>, or ```json {"name":...} ```
// in text content instead of structured OpenAI delta.ToolCalls.
func extractTextEmbeddedToolCalls(content string, allowedTools []ToolDefinition) (cleanedContent string, calls []apiToolCall) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || len(allowedTools) == 0 {
		return content, nil
	}
	allowSet := make(map[string]bool, len(allowedTools))
	for _, td := range allowedTools {
		allowSet[td.Function.Name] = true
	}

	cleaned := content
	addCall := func(name string, argsAny any) bool {
		name = strings.TrimSpace(name)
		if !allowSet[name] {
			return false
		}
		var argsStr string
		switch v := argsAny.(type) {
		case string:
			argsStr = strings.TrimSpace(v)
		case map[string]any:
			b, _ := json.Marshal(v)
			argsStr = string(b)
		default:
			if v != nil {
				b, _ := json.Marshal(v)
				argsStr = string(b)
			}
		}
		if argsStr == "" {
			argsStr = "{}"
		}
		calls = append(calls, apiToolCall{
			ID:   fmt.Sprintf("textcall_%d", len(calls)+1),
			Type: "function",
			Function: apiToolFunction{
				Name:      name,
				Arguments: repairToolArguments(name, argsStr),
			},
		})
		return true
	}

	tryParseJSONCall := func(rawJSON string) bool {
		var obj map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(rawJSON)), &obj) != nil {
			return false
		}
		name, _ := obj["name"].(string)
		if name == "" {
			name, _ = obj["tool"].(string)
		}
		if name == "" {
			name, _ = obj["function"].(string)
		}
		if name == "" {
			return false
		}
		args := obj["arguments"]
		if args == nil {
			args = obj["parameters"]
		}
		if args == nil {
			args = obj["args"]
		}
		if args == nil {
			// Flat object where parameters are top-level alongside "name"/"tool"
			rest := make(map[string]any)
			for k, v := range obj {
				if k != "name" && k != "tool" && k != "function" && k != "type" {
					rest[k] = v
				}
			}
			args = rest
		}
		return addCall(name, args)
	}

	// 1. <tool_call>{...}</tool_call>
	for _, m := range reXmlToolCall.FindAllStringSubmatch(cleaned, 4) {
		if len(m) > 1 && tryParseJSONCall(m[1]) {
			cleaned = strings.Replace(cleaned, m[0], "", 1)
		}
	}
	// 2. <function=tool_name>{...}</function>
	for _, m := range reFuncTagCall.FindAllStringSubmatch(cleaned, 4) {
		if len(m) > 2 && addCall(m[1], m[2]) {
			cleaned = strings.Replace(cleaned, m[0], "", 1)
		}
	}
	if len(calls) > 0 {
		return strings.TrimSpace(cleaned), calls
	}

	// 3. ```json {"name": "allowed_tool", "arguments": {...}} ```
	for _, m := range reCodeFenceJSON.FindAllStringSubmatch(cleaned, 2) {
		if len(m) > 1 && tryParseJSONCall(m[1]) {
			cleaned = strings.Replace(cleaned, m[0], "", 1)
		}
	}
	if len(calls) > 0 {
		return strings.TrimSpace(cleaned), calls
	}

	// 4. Bare JSON object when entire response is a single tool call JSON
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		if tryParseJSONCall(trimmed) {
			return "", calls
		}
	}
	return content, nil
}

// repairToolArguments normalizes malformed JSON and common parameter alias mistakes
// made by smaller/free models before tool execution.
func repairToolArguments(toolName string, rawArgs string) string {
	s := normalizeToolArgsJSON(rawArgs)
	if s == "" || s == "{}" {
		s = "{}"
	} else if !strings.HasPrefix(s, "{") {
		if start := strings.IndexByte(s, '{'); start >= 0 {
			if end := strings.LastIndexByte(s, '}'); end > start {
				s = s[start : end+1]
			}
		}
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
		return s
	}

	promoteAlias := func(canonical string, aliases ...string) {
		if val, ok := m[canonical]; ok && val != nil && fmt.Sprintf("%v", val) != "" {
			return
		}
		for _, al := range aliases {
			if val, ok := m[al]; ok && val != nil && fmt.Sprintf("%v", val) != "" {
				m[canonical] = val
				delete(m, al)
				return
			}
		}
	}

	strVal := func(k string) string {
		if v, ok := m[k]; ok && v != nil {
			if sv, ok := v.(string); ok {
				return strings.TrimSpace(sv)
			}
		}
		return ""
	}

	switch strings.TrimSpace(toolName) {
	case "web_search":
		promoteAlias("query", "q", "keyword", "keywords", "search", "search_query", "text")
		if limStr, ok := m["limit"].(string); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(limStr)); err == nil {
				m["limit"] = n
			} else {
				delete(m, "limit")
			}
		}
	case "read_url":
		promoteAlias("url", "link", "href", "target", "target_url", "page_url", "uri")
	case "remember_fact":
		promoteAlias("fact_key", "key", "name", "topic", "id")
		promoteAlias("content", "value", "fact", "text", "memory", "description")
	case "recall_memory":
		promoteAlias("query", "q", "keyword", "keywords", "fact_key", "key", "search")
	case "manage_todos":
		promoteAlias("action", "op", "type", "command", "method")
		promoteAlias("title", "text", "content", "name", "task", "todo")
		switch strings.ToLower(strVal("action")) {
		case "create", "insert", "new", "upsert", "save":
			m["action"] = "add"
		case "done", "finish", "check", "mark_done":
			m["action"] = "complete"
		case "remove", "del":
			m["action"] = "delete"
		case "get", "query", "search", "show", "view":
			m["action"] = "list"
		case "":
			if strVal("title") != "" && strVal("id") == "" {
				m["action"] = "add"
			} else if strVal("id") != "" && strings.EqualFold(strVal("status"), "done") {
				m["action"] = "complete"
			} else if strVal("id") != "" {
				m["action"] = "update"
			} else {
				m["action"] = "list"
			}
		}
	case "manage_notes":
		promoteAlias("action", "op", "type", "command")
		promoteAlias("title", "name", "heading", "subject")
		promoteAlias("content", "text", "body", "markdown", "note")
		promoteAlias("query", "q", "keyword")
		switch strings.ToLower(strVal("action")) {
		case "create", "add", "save", "update", "write", "edit", "new":
			m["action"] = "upsert"
		case "remove", "del":
			m["action"] = "delete"
		case "read", "view", "show":
			if strVal("id") != "" || strVal("title") != "" {
				m["action"] = "get"
			} else {
				m["action"] = "list"
			}
		case "":
			if strVal("content") != "" {
				m["action"] = "upsert"
			} else if strVal("id") != "" {
				m["action"] = "get"
			} else if strVal("query") != "" {
				m["action"] = "search"
			} else {
				m["action"] = "list"
			}
		}
	case "manage_stash":
		promoteAlias("action", "op", "type", "command")
		promoteAlias("url", "link", "href")
		promoteAlias("text", "content", "note")
		promoteAlias("query", "q", "keyword")
		switch strings.ToLower(strVal("action")) {
		case "create", "save", "bookmark", "insert", "new":
			m["action"] = "capture"
		case "remove", "del":
			m["action"] = "delete"
		case "mark_read", "read":
			m["action"] = "update"
			m["status"] = "read"
		case "mark_unread", "unread":
			m["action"] = "update"
			m["status"] = "unread"
		case "":
			if strVal("url") != "" || strVal("text") != "" {
				m["action"] = "capture"
			} else if strVal("id") != "" {
				m["action"] = "get"
			} else {
				m["action"] = "list"
			}
		}
	case "manage_assets":
		promoteAlias("action", "op", "type", "command")
		switch strings.ToLower(strVal("action")) {
		case "create", "add", "save", "update", "new":
			m["action"] = "upsert"
		case "remove", "del":
			m["action"] = "delete"
		case "query", "get", "search", "show":
			m["action"] = "list"
		case "":
			if strVal("name") != "" && strVal("id") == "" {
				m["action"] = "upsert"
			} else {
				m["action"] = "list"
			}
		}
	case "manage_tasks":
		promoteAlias("action", "op", "type", "command")
		switch strings.ToLower(strVal("action")) {
		case "create", "add", "save", "update", "new":
			m["action"] = "upsert"
		case "trigger", "execute", "start":
			m["action"] = "run"
		case "enable", "disable":
			m["action"] = "toggle"
		case "remove", "del":
			m["action"] = "delete"
		case "":
			m["action"] = "list"
		}
	case "manage_memory":
		promoteAlias("action", "op", "type", "command")
		promoteAlias("fact_key", "key", "name")
		promoteAlias("content", "value", "fact", "text")
		switch strings.ToLower(strVal("action")) {
		case "create", "add", "save", "update":
			m["action"] = "upsert"
		case "delete", "remove", "del":
			m["action"] = "forget"
		case "query", "search", "get":
			m["action"] = "list"
		case "":
			if strVal("content") != "" && strVal("fact_key") != "" {
				m["action"] = "upsert"
			} else {
				m["action"] = "list"
			}
		}
	}

	b, err := json.Marshal(m)
	if err != nil {
		return s
	}
	return string(b)
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

// lastAssistantReplyFromHistory returns the most recent non-empty assistant reply in history.
func lastAssistantReplyFromHistory(history []store.AgentMessage) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && strings.TrimSpace(history[i].Content) != "" {
			return strings.TrimSpace(history[i].Content)
		}
	}
	return ""
}

// textBigramSimilarity computes character-bigram Jaccard similarity in [0.0, 1.0]
// between two responses (ignoring whitespace and punctuation) to detect when a model
// repeats essentially the same answer on a correction turn.
func textBigramSimilarity(a, b string) float64 {
	norm := func(s string) []rune {
		var out []rune
		for _, r := range strings.ToLower(s) {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || isCJKRune(r) {
				out = append(out, r)
			}
		}
		return out
	}
	ra, rb := norm(a), norm(b)
	if len(ra) < 8 || len(rb) < 8 {
		return 0
	}
	setA := make(map[string]bool, len(ra))
	for i := 0; i < len(ra)-1; i++ {
		setA[string(ra[i:i+2])] = true
	}
	setB := make(map[string]bool, len(rb))
	for i := 0; i < len(rb)-1; i++ {
		setB[string(rb[i:i+2])] = true
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for k := range setA {
		if setB[k] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
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

	lastAssistantIdx := -1
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role == "assistant" && strings.TrimSpace(out[i].Content) != "" {
			lastAssistantIdx = i
			break
		}
	}

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
