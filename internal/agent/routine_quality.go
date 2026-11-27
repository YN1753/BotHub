package agent

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	reParenExample  = regexp.MustCompile(`[（(]\s*(?:比如|例如|譬如|举例|举个例子|像是|像这样|如：|如:|e\.g\.|for example)[^）)]*[）)]`)
	reClauseExample = regexp.MustCompile(`(?:比如说|比如|例如说|例如|譬如|举个例子|举例来说|举例|像这样|像是|如：|e\.g\.|for example)\s*[:：]?\s*([^。！？!\?\n；;]+)`)
)

// extractPromptExamples separates example clauses ("比如...", "例如...", etc.) from a routine prompt
// so the model understands the abstract goal without treating user examples as literal output scripts.
func extractPromptExamples(prompt string) (coreGoal string, examples []string) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", nil
	}

	seen := make(map[string]bool)
	addExample := func(raw string) {
		clean := strings.TrimSpace(raw)
		clean = strings.Trim(clean, "「」『』\"'“”‘’()（）:：,，、.。;；")
		clean = strings.TrimSpace(clean)
		// Strip trailing meta words like "之类的", "等等", "这样"
		for _, suffix := range []string{"之类的", "等等", "等", "这样子的", "这样的", "这样"} {
			if strings.HasSuffix(clean, suffix) && len([]rune(clean)) > len([]rune(suffix))+3 {
				clean = strings.TrimSpace(strings.TrimSuffix(clean, suffix))
			}
		}
		if len([]rune(clean)) < 4 {
			return
		}
		// Split list-style examples separated by 、 / 或 / 或者 so individual example items are also guarded
		subParts := strings.FieldsFunc(clean, func(r rune) bool {
			return r == '、' || r == '/' || r == '｜' || r == '|'
		})
		if len(subParts) > 1 {
			for _, sp := range subParts {
				sp = strings.TrimSpace(strings.Trim(sp, "「」『』\"'“”‘’"))
				if len([]rune(sp)) >= 4 && !seen[sp] {
					seen[sp] = true
					examples = append(examples, sp)
				}
			}
		}
		if !seen[clean] {
			seen[clean] = true
			examples = append(examples, clean)
		}
	}

	// Strip any appended execution-boundary footer before scanning examples so meta-text isn't mistaken for user examples
	scanText := trimmed
	if idx := strings.Index(scanText, "（执行边界："); idx >= 0 {
		scanText = strings.TrimSpace(scanText[:idx])
	}

	// First scan parenthesized examples
	for _, m := range reParenExample.FindAllString(scanText, -1) {
		inner := strings.Trim(m, "（）() ")
		if sub := reClauseExample.FindStringSubmatch(inner); len(sub) > 1 {
			addExample(sub[1])
		} else {
			addExample(inner)
		}
	}

	// Then scan inline example clauses
	for _, sub := range reClauseExample.FindAllStringSubmatch(scanText, -1) {
		if len(sub) > 1 {
			addExample(sub[1])
		}
	}

	// Build a cleaner coreGoal where parenthesized examples are stripped (if non-empty remainder)
	cleanedGoal := strings.TrimSpace(reParenExample.ReplaceAllString(scanText, ""))
	if len([]rune(cleanedGoal)) >= 6 {
		coreGoal = cleanedGoal
	} else {
		coreGoal = scanText
	}
	return coreGoal, examples
}

// normalizeRoutinePromptOnCreate ensures prompts saved via manage_tasks do not leave raw user examples
// unguarded as literal templates.
func normalizeRoutinePromptOnCreate(prompt string) string {
	clean := notifier.StripEmoji(strings.TrimSpace(prompt))
	if clean == "" {
		return ""
	}
	if strings.Contains(clean, "（执行边界：") || strings.Contains(clean, "严禁原样照搬") || strings.Contains(clean, "严禁照搬") || strings.Contains(clean, "严禁原样复读") {
		return clean
	}
	_, examples := extractPromptExamples(clean)
	if len(examples) == 0 {
		return clean
	}
	return clean + "\n（执行边界：以上“比如/例如”仅作方向与风格参考，每次触发必须产出全新具体内容，严禁原样照搬或复读示例。）"
}

// isMonitoringRoutine distinguishes status-check / alert routines (which should stay silent when unchanged)
// from creative / recommendation / knowledge / daily briefing routines (which should rewrite on similarity).
func isMonitoringRoutine(taskName, prompt string) bool {
	combined := strings.ToLower(taskName + " " + prompt)
	creativeKeywords := []string{
		"冷知识", "推荐", "早安", "晚安", "问候", "名言", "金句", "笑话", "段子",
		"故事", "新闻", "热搜", "趋势", "学习", "单词", "英语", "灵感", "创意",
		"简报", "早报", "日报", "晚报", "周报", "复盘", "总结", "回顾", "读书",
		"科普", "技巧", "分享", "陪伴", "聊聊", "随笔", "食谱", "穿搭", "电影", "音乐",
	}
	for _, kw := range creativeKeywords {
		if strings.Contains(combined, kw) {
			return false
		}
	}
	monitorKeywords := []string{
		"巡检", "监控", "探针", "报警", "告警", "异常", "健康检查", "服务器", "节点",
		"宕机", "离线", "到期", "续费", "资产检查", "账单提醒", "余额", "状态检查",
		"待办检查", "逾期", "monitor", "health", "alert", "watch", "audit", "ping",
	}
	for _, kw := range monitorKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// isResearchOrDigestRoutine reports whether a scheduled routine is a live-info
// recommendation, tech digest, or deals/freebies scouting task that benefits from
// multi-angle bilingual search expansion.
func isResearchOrDigestRoutine(taskName, prompt string) bool {
	if isMonitoringRoutine(taskName, prompt) {
		return false
	}
	combined := strings.ToLower(taskName + " " + prompt)
	researchKeywords := []string{
		"推荐", "资讯", "日报", "早报", "晚报", "周报", "动态", "前沿", "热点", "趋势",
		"雷达", "开源", "羊毛", "优惠", "黑五", "白嫖", "免费", "福利", "折扣", "促销",
		"活动", "速递", "盘点", "精选", "情报", "快讯", "ai", "golang", "技术",
	}
	for _, kw := range researchKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

var dailyRoutineAngles = []string{
	"反常识细节与隐藏机制（挑一个大多数人忽略的冷门切入点）",
	"立刻可落地的一条实用技巧或微行动（具体到步骤或参数）",
	"底层原理与演进脉络（用极简大白话讲清“为什么会这样”）",
	"典型误区与避坑反例（先指出常见错误认知，再给出正确做法）",
	"具体真实案例拆解（用一个小而具体的真实场景或数据说话）",
	"跨领域冷门联系（把当前主题与另一个意想不到的领域联系起来）",
	"极简对照清单（对比两种常见选择/做法的真实差异与取舍）",
	"当下时间节点视角（结合今天是周几、季节或近期节奏切入）",
	"数据/数字驱动视角（给出有信息量的关键数字、尺度或估算）",
	"深度追问视角（抛出一个有启发性的小问题并给出凝练解答）",
}

func pickDailyRoutineAngle(seedKey string, now time.Time) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seedKey + "|" + now.Format("2006-01-02")))
	idx := int(h.Sum32()) % len(dailyRoutineAngles)
	return dailyRoutineAngles[idx]
}

func shanghaiDayPeriod(hour int) string {
	switch {
	case hour < 6:
		return "凌晨"
	case hour < 9:
		return "清晨"
	case hour < 12:
		return "上午"
	case hour < 14:
		return "午间"
	case hour < 18:
		return "下午"
	case hour < 21:
		return "傍晚"
	default:
		return "夜间"
	}
}

// buildRoutineContextSnapshot injects real-world daily context (date, rotating editorial angle,
// active todos, unread stash items) so pushes stay grounded and concrete instead of generic filler.
func buildRoutineContextSnapshot(st *store.Store, taskSeed string, now time.Time) string {
	return buildRoutineContextSnapshotForMode(st, taskSeed, now, false)
}

func buildRoutineContextSnapshotForMode(st *store.Store, taskSeed string, now time.Time, isResearch bool) string {
	weekday := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[now.Weekday()]
	period := shanghaiDayPeriod(now.Hour())

	var sb strings.Builder
	if isResearch {
		sb.WriteString("[今日实时时间窗口]\n")
		sb.WriteString(fmt.Sprintf("- 当前时间：%s %s %s\n", now.Format("2006-01-02 15:04"), weekday, period))
		sb.WriteString(fmt.Sprintf("- 时效优先窗口：近 72 小时（%s 至 %s），优先近期发布或当前仍在有效期内的内容\n", now.AddDate(0, 0, -3).Format("2006-01-02"), now.Format("2006-01-02")))
		return strings.TrimSpace(sb.String())
	}

	angle := pickDailyRoutineAngle(taskSeed, now)
	sb.WriteString("[今日实时上下文快照（按需结合，勿生硬罗列）]\n")
	sb.WriteString(fmt.Sprintf("- 当前时间：%s %s %s\n", now.Format("2006-01-02 15:04"), weekday, period))
	sb.WriteString(fmt.Sprintf("- 今日轮换切入维度：%s\n", angle))

	if st != nil {
		if todos, err := st.ListTodos("open", "", 5); err == nil && len(todos) > 0 {
			var items []string
			for _, td := range todos {
				entry := strings.TrimSpace(td.Title)
				if entry == "" {
					continue
				}
				if td.DueAt != "" {
					entry += fmt.Sprintf("（截止 %s）", td.DueAt)
				}
				items = append(items, entry)
			}
			if len(items) > 0 {
				sb.WriteString("- 当前未完成待办：" + strings.Join(items, "；") + "\n")
			}
		}
		if stashes, err := st.SearchStashItems("", "unread", "", 3); err == nil && len(stashes) > 0 {
			var items []string
			for _, it := range stashes {
				t := strings.TrimSpace(it.Title)
				if t == "" {
					t = truncateRunes(strings.TrimSpace(it.RawInput), 36)
				}
				if t != "" {
					items = append(items, t)
				}
			}
			if len(items) > 0 {
				sb.WriteString("- 近期未读收藏/闪念：" + strings.Join(items, "；") + "\n")
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// buildRoutineAntiRepeatLedger formats the task's recent push notifications into an explicit
// negative ledger so the model never repeats yesterday's topic, example, or opening.
func buildRoutineAntiRepeatLedger(past []store.Notification) string {
	if len(past) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[历史已推送去重账本（最近 %d 次，严禁重复相同主题、案例或相似开头）]\n", len(past)))
	for i, n := range past {
		body := strings.TrimSpace(n.MarkdownBody)
		if body == "" {
			body = strings.TrimSpace(n.Summary)
		}
		body = strings.ReplaceAll(body, "\r\n", " ")
		body = strings.ReplaceAll(body, "\n", " ")
		body = strings.Join(strings.Fields(body), " ")
		excerpt := truncateRunes(body, 140)
		dateTag := strings.TrimSpace(n.CreatedAt)
		if len(dateTag) > 16 {
			dateTag = dateTag[:16]
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, dateTag, excerpt))
	}
	return strings.TrimSpace(sb.String())
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func normalizeCompactText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || isCJKRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tokenContainmentRatio measures what fraction of reference bigrams appear in candidate.
// Useful for catching short example snippets embedded inside a longer routine reply.
func bigramContainmentRatio(candidate, reference string) float64 {
	rc := []rune(normalizeCompactText(candidate))
	rr := []rune(normalizeCompactText(reference))
	if len(rr) < 4 || len(rc) < 4 {
		return 0
	}
	candSet := make(map[string]bool, len(rc))
	for i := 0; i < len(rc)-1; i++ {
		candSet[string(rc[i:i+2])] = true
	}
	refBigrams := make(map[string]bool, len(rr))
	for i := 0; i < len(rr)-1; i++ {
		refBigrams[string(rr[i:i+2])] = true
	}
	if len(refBigrams) == 0 {
		return 0
	}
	hit := 0
	for bg := range refBigrams {
		if candSet[bg] {
			hit++
		}
	}
	return float64(hit) / float64(len(refBigrams))
}

type routineQualityCheck struct {
	EchoedExample   string
	MaxPastSim      float64
	MostSimilarPast *store.Notification
}

func (c routineQualityCheck) needsRewrite(isMonitoring bool) bool {
	if c.EchoedExample != "" {
		return true
	}
	if !isMonitoring && c.MaxPastSim >= 0.38 {
		return true
	}
	return false
}

func (c routineQualityCheck) shouldSuppressMonitoring(isMonitoring bool) bool {
	return isMonitoring && c.EchoedExample == "" && c.MaxPastSim >= 0.72
}

// evaluateRoutineDraft checks a generated routine reply against both extracted user examples
// and recent notification history to catch verbatim example echoing or cross-day repetition.
func evaluateRoutineDraft(reply string, past []store.Notification, exampleSnippets []string) routineQualityCheck {
	var res routineQualityCheck
	normReply := normalizeCompactText(reply)
	if normReply == "" {
		return res
	}

	for _, ex := range exampleSnippets {
		normEx := normalizeCompactText(ex)
		if len([]rune(normEx)) < 4 {
			continue
		}
		if strings.Contains(normReply, normEx) {
			res.EchoedExample = ex
			break
		}
		if len([]rune(normEx)) >= 6 && bigramContainmentRatio(reply, ex) >= 0.72 {
			res.EchoedExample = ex
			break
		}
	}

	for i := range past {
		pastText := strings.TrimSpace(past[i].MarkdownBody)
		if pastText == "" {
			pastText = strings.TrimSpace(past[i].Summary)
		}
		if pastText == "" {
			continue
		}
		sim := textBigramSimilarity(reply, pastText)
		if sim > res.MaxPastSim {
			res.MaxPastSim = sim
			cp := past[i]
			res.MostSimilarPast = &cp
		}
	}
	return res
}

func buildRoutinePromptMessage(
	taskName string,
	rawPrompt string,
	now time.Time,
	contextSnapshot string,
	antiRepeatLedger string,
	examples []string,
	isMonitoring bool,
) string {
	coreGoal, _ := extractPromptExamples(rawPrompt)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[后台定时例程「%s」于 %s 自动触发]\n", taskName, now.Format("2006-01-02 15:04:05")))
	sb.WriteString("执行铁律：\n")
	sb.WriteString("1. 直接输出高信息密度、可直接阅读的干货正文，禁止寒暄套话（如“你好呀”“新的一天开始啦”“希望你今天开心”等空洞开场白）与空泛鸡汤。\n")
	sb.WriteString("2. 严禁把任务指令中的「比如/例如/示例」原样复读或换个词重写交差，必须独立产出全新具体内容。\n")
	sb.WriteString("3. 涉及外部资讯、实时数据或系统状态时，直接调用可用工具核实后再总结，绝不凭空编造。\n")
	if isMonitoring {
		sb.WriteString("4. 本任务属于状态巡检/异常提醒类：若当前各项指标正常且相比上次无新增异常/变化，请仅输出 [SILENT_OK]，不要发送例行“一切正常”的打扰消息。\n")
	} else {
		sb.WriteString("4. 严禁与下方「历史已推送去重账本」重复相同主题、案例或结构；请主动换一个全新具体切入点。\n")
	}

	if !isMonitoring && isResearchOrDigestRoutine(taskName, rawPrompt) {
		sb.WriteString("\n[多维发散调研策略]\n")
		sb.WriteString("1. 禁止只用单一宽泛词搜一次就交差：先围绕主题拆解出 3 到 5 个具体子方向，并发起多组精准搜索（中英文结合），务必传入 time_range（\"day\" 或 \"week\"）、匹配 topic（\"tech\"/\"deals\"/\"news\"）和 lang: \"all\"。\n")
		sb.WriteString("2. 技术/内容推荐类（topic=\"tech\"）：由核心关键词向外发散关联生态与近期突破（如最新模型/Agent/MCP/开源高星新项目/工程最佳实践/论文），挖掘真正值得读的一手项目或深文，别每次只搜大词首页水文。\n")
		sb.WriteString("3. 羊毛/优惠/活动类（topic=\"deals\"）：分别搜索云厂商免费额度与黑五大促、AI 平台限时免费体验与优惠、开发者工具福利及实时社区讨论，核实仍在有效期内、写明领取条件与直达链接，淘汰过期旧闻。\n")
		sb.WriteString("4. 对高价值条目进一步抓取原文核实核心数据与细节，输出时每条尽量标注发布时间或有效期与来源链接。\n")
	}

	if len(examples) > 0 {
		sb.WriteString("\n[严禁原样照搬的用户举例清单（仅说明方向，绝对不要输出这些具体例子）]\n")
		for _, ex := range examples {
			sb.WriteString("- " + ex + "\n")
		}
	}

	if contextSnapshot != "" {
		sb.WriteString("\n" + contextSnapshot + "\n")
	}
	if antiRepeatLedger != "" {
		sb.WriteString("\n" + antiRepeatLedger + "\n")
	}

	sb.WriteString("\n[本次例程任务目标]\n")
	sb.WriteString(strings.TrimSpace(coreGoal))
	return sb.String()
}

func buildRoutineRewriteInstruction(check routineQualityCheck, rejectedDraft string) string {
	var reasons []string
	if check.EchoedExample != "" {
		reasons = append(reasons, fmt.Sprintf("你刚刚生成的草稿原样照搬了用户举例「%s」", check.EchoedExample))
	}
	if check.MostSimilarPast != nil && check.MaxPastSim >= 0.38 {
		reasons = append(reasons, fmt.Sprintf("你刚刚生成的草稿与 %s 的历史推送相似度过高（%.0f%%）", check.MostSimilarPast.CreatedAt, check.MaxPastSim*100))
	}
	reasonText := strings.Join(reasons, "，且")
	if reasonText == "" {
		reasonText = "你刚刚生成的草稿信息密度偏低或与历史推送雷同"
	}
	return fmt.Sprintf(
		"[系统质量拦截强制重写：%s。请彻底抛弃刚才那版草稿的主题、案例、开头与模板：\n---被拦截草稿---\n%s\n---结束---\n请立即换一个完全不同的全新切入点与具体事实/案例重新撰写。要求：直接给干货，不复读任何用户举例，不解释为什么重写，不要提及“重写/换一个”等元话术。]",
		reasonText,
		truncateRunes(strings.TrimSpace(rejectedDraft), 360),
	)
}
