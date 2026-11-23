package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

const (
	ConfigKeyLastAutoDreamAt     = "last_autodream_at"
	ConfigKeyLastAutoDreamReport = "last_autodream_report"
)

// CanonicalMemoryBucket maps both Claude's 4 canonical memory types (feedback, user, project, reference)
// and legacy BotHub categories (preference, relation, emotion, infra, schedule, insight, general)
// into a clean 4-section MEMORY.md taxonomy.
func CanonicalMemoryBucket(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "feedback", "insight":
		return "feedback"
	case "user", "preference", "relation", "emotion":
		return "user"
	case "project", "schedule", "infra":
		return "project"
	default:
		return "reference"
	}
}

func memoryBucketTitle(bucket string) string {
	switch bucket {
	case "feedback":
		return "纠偏与工作纪律 (feedback)"
	case "user":
		return "用户画像与偏好 (user)"
	case "project":
		return "长期项目与决策 (project)"
	default:
		return "外部参考与指针 (reference)"
	}
}

func shortMemoryDate(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// RenderMemoryIndexMarkdown renders memories in Anthropic/Claude's transparent MEMORY.md format:
// grouped by the 4 high-signal types (feedback -> user -> project -> reference), showing full concise
// notes for the highest-priority/most-relevant items and compact single-line index pointers for
// long-tail entries so the agent can pull details just-in-time via recall_memory.
func RenderMemoryIndexMarkdown(detailed []store.AgentMemory, allInScope []store.AgentMemory, scopeLabel string, includeHeader bool) string {
	if len(detailed) == 0 && len(allInScope) == 0 {
		if includeHeader {
			return fmt.Sprintf("# MEMORY.md (%s)\n\n暂无长期记忆条目。\n", scopeLabel)
		}
		return ""
	}

	detailedIDs := make(map[int64]bool, len(detailed))
	buckets := map[string][]store.AgentMemory{
		"feedback":  {},
		"user":      {},
		"project":   {},
		"reference": {},
	}
	for _, m := range detailed {
		detailedIDs[m.ID] = true
		b := CanonicalMemoryBucket(m.Category)
		buckets[b] = append(buckets[b], m)
	}

	var tailIndex []store.AgentMemory
	for _, m := range allInScope {
		if !detailedIDs[m.ID] {
			tailIndex = append(tailIndex, m)
		}
	}

	var sb strings.Builder
	if includeHeader {
		sb.WriteString(fmt.Sprintf("# MEMORY.md (%s)\n\n", scopeLabel))
	}

	order := []string{"feedback", "user", "project", "reference"}
	for _, b := range order {
		items := buckets[b]
		if len(items) == 0 {
			continue
		}
		sb.WriteString("## " + memoryBucketTitle(b) + "\n")
		for _, m := range items {
			pinTag := ""
			if m.Pinned {
				pinTag = "[置顶] "
			}
			dateTag := ""
			if d := shortMemoryDate(m.UpdatedAt); d != "" {
				dateTag = fmt.Sprintf(" (%s)", d)
			}
			sb.WriteString(fmt.Sprintf("- %s`%s`%s: %s\n", pinTag, m.FactKey, dateTag, strings.TrimSpace(m.Content)))
		}
		sb.WriteString("\n")
	}

	if len(tailIndex) > 0 {
		sb.WriteString("## 长尾记忆索引 (按需调用 recall_memory 即时展开)\n")
		maxTail := 24
		for i, m := range tailIndex {
			if i >= maxTail {
				sb.WriteString(fmt.Sprintf("- ... 另有 %d 条历史记忆可通过 recall_memory(query) 检索\n", len(tailIndex)-maxTail))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s` [%s, %s]\n", m.FactKey, CanonicalMemoryBucket(m.Category), shortMemoryDate(m.UpdatedAt)))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// DreamReport summarizes the 4-phase AutoDream sleep-time consolidation run.
type DreamReport struct {
	Scope             string   `json:"scope"`
	RanAt             string   `json:"ran_at"`
	UsedLLM           bool     `json:"used_llm"`
	ScopesProcessed   int      `json:"scopes_processed"`
	TranscriptTurns   int      `json:"transcript_turns"`
	Upserted          int      `json:"upserted"`
	Forgotten         int      `json:"forgotten"`
	MergedDuplicates  int      `json:"merged_duplicates"`
	PrunedCold        int      `json:"pruned_cold"`
	Summary           string   `json:"summary"`
	Highlights        []string `json:"highlights,omitempty"`
	RenderedMemoryDoc string   `json:"memory_markdown,omitempty"`
}

// RunAutoDream executes Claude's 4-Phase Sleep-Time Memory Consolidation ("Dreaming"):
// Phase 1 (Orientation): Inventory existing memories in scope(s).
// Phase 2 (Gather Signal): Replay recent chat transcripts to catch corrections, preferences, and decisions.
// Phase 3 (Consolidation): Resolve contradictions, convert relative dates to absolute YYYY-MM-DD, merge overlaps.
// Phase 4 (Prune & Index): Persist updates, dedupe, prune cold overflow, and rebuild MEMORY.md index.
func (e *Engine) RunAutoDream(ctx context.Context, targetScope string, isBackground bool) (*DreamReport, error) {
	if e == nil || e.store == nil {
		return nil, fmt.Errorf("引擎存储未初始化")
	}

	now := time.Now().In(store.ShanghaiLocation())
	todayStr := now.Format("2006-01-02")
	nowStr := now.Format("2006-01-02 15:04:05")
	lastDreamAt, _ := e.store.GetConfig(ConfigKeyLastAutoDreamAt)

	// Phase 1: Determine scopes to process
	scopeArg := strings.TrimSpace(targetScope)
	var scopes []string
	if scopeArg == "" || strings.EqualFold(scopeArg, "all") {
		scopeSet := map[string]bool{"global": true}
		scopes = append(scopes, "global")
		allMems, _ := e.store.ListAgentMemories("", 200)
		for _, m := range allMems {
			sc := strings.TrimSpace(m.Scope)
			if sc == "" {
				sc = "global"
			}
			if !scopeSet[sc] {
				scopeSet[sc] = true
				scopes = append(scopes, sc)
			}
		}
		bots, _ := e.store.ListBots()
		for _, b := range bots {
			if b.IsGroup {
				continue
			}
			sc := "bot:" + b.ID
			if !scopeSet[sc] {
				// Only include bot scope if it has recent dialogue
				if msgs, _ := e.store.ListRecentChatTranscriptForScope(sc, lastDreamAt, 4); len(msgs) >= 2 {
					scopeSet[sc] = true
					scopes = append(scopes, sc)
				}
			}
		}
	} else {
		scopes = []string{scopeArg}
	}

	report := &DreamReport{
		Scope:   scopeArg,
		RanAt:   nowStr,
		UsedLLM: false,
	}
	if report.Scope == "" {
		report.Scope = "all"
	}

	hasLLM := e.hasLightTarget()
	var highlights []string

	for _, sc := range scopes {
		if ctx.Err() != nil {
			break
		}
		existing, _ := e.store.ListScopedMemories([]string{sc}, "", 120)
		// Phase 2: Gather Signal from recent chat transcripts
		sinceFilter := ""
		if isBackground {
			sinceFilter = lastDreamAt
		}
		transcript, _ := e.store.ListRecentChatTranscriptForScope(sc, sinceFilter, 32)
		if len(existing) == 0 && len(transcript) < 2 {
			continue
		}
		report.ScopesProcessed++
		report.TranscriptTurns += len(transcript)

		pinnedKeys := make(map[string]bool)
		for _, m := range existing {
			if m.Pinned {
				pinnedKeys[strings.ToLower(strings.TrimSpace(m.FactKey))] = true
			}
		}

		// Phase 3: LLM-driven Consolidation (resolve contradictions, absolute dates, distill corrections)
		if hasLLM && (len(existing) >= 2 || len(transcript) >= 2) {
			llmCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			upserts, forgets, note, err := e.runAutoDreamLLMPhase(llmCtx, sc, todayStr, existing, transcript)
			cancel()
			if err == nil {
				report.UsedLLM = true
				for _, u := range upserts {
					cleanKey := normalizeDistilledFactKey(u.FactKey)
					if cleanKey == "" {
						cleanKey = strings.TrimSpace(u.FactKey)
					}
					cleanContent := notifier.StripEmoji(strings.TrimSpace(u.Content))
					if cleanKey == "" || cleanContent == "" {
						continue
					}
					cat := normalizeClaudeMemoryCategory(u.Category)
					imp := clampMemoryImportance(u.Importance)
					pin := u.Pinned || pinnedKeys[strings.ToLower(cleanKey)]
					if err := e.store.SaveAgentMemoryDetailed(sc, cat, cleanKey, cleanContent, imp, pin); err == nil {
						report.Upserted++
					}
				}
				for _, fk := range forgets {
					cleanKey := strings.TrimSpace(fk)
					if cleanKey == "" || pinnedKeys[strings.ToLower(cleanKey)] {
						continue
					}
					if deleted, _ := e.store.DeleteAgentMemoryByScopeKey(sc, cleanKey); deleted {
						report.Forgotten++
					} else if norm := normalizeDistilledFactKey(cleanKey); norm != "" && norm != cleanKey {
						if deleted2, _ := e.store.DeleteAgentMemoryByScopeKey(sc, norm); deleted2 {
							report.Forgotten++
						}
					}
				}
				if strings.TrimSpace(note) != "" {
					highlights = append(highlights, fmt.Sprintf("[%s] %s", sc, notifier.StripEmoji(strings.TrimSpace(note))))
				}
			}
		}

		// Phase 4: Prune & Index (deterministic deduplication + cap overflow)
		if merged, err := e.store.MergeDuplicateMemories(sc); err == nil && merged > 0 {
			report.MergedDuplicates += int(merged)
		}
		if pruned, err := e.store.PruneColdMemories(sc, 60); err == nil && pruned > 0 {
			report.PrunedCold += int(pruned)
		}
	}

	if len(highlights) > 0 {
		report.Highlights = highlights
		report.Summary = strings.Join(highlights, "；")
	} else if report.Upserted > 0 || report.Forgotten > 0 || report.MergedDuplicates > 0 || report.PrunedCold > 0 {
		report.Summary = fmt.Sprintf("AutoDream 完成 %d 个作用域深度整理：提炼/修正 %d 条，清理矛盾或过时 %d 条，合并重复 %d 条，裁剪冷门 %d 条。",
			report.ScopesProcessed, report.Upserted, report.Forgotten, report.MergedDuplicates, report.PrunedCold)
	} else {
		report.Summary = fmt.Sprintf("AutoDream 巡检了 %d 个作用域（回放 %d 条近期对话），当前记忆库结构整洁、无冲突或冗余条目。",
			report.ScopesProcessed, report.TranscriptTurns)
	}

	// Render updated MEMORY.md preview for the requested scope
	var viewScopes []string
	viewLabel := "全部作用域"
	if scopeArg != "" && !strings.EqualFold(scopeArg, "all") {
		viewScopes = []string{scopeArg}
		viewLabel = scopeArg
	}
	latestMems, _ := e.store.ListScopedMemories(viewScopes, "", 120)
	report.RenderedMemoryDoc = RenderMemoryIndexMarkdown(latestMems, latestMems, viewLabel, true)

	_ = e.store.SetConfig(ConfigKeyLastAutoDreamAt, nowStr)
	if rawRep, err := json.Marshal(map[string]any{
		"ran_at":            report.RanAt,
		"scope":             report.Scope,
		"used_llm":          report.UsedLLM,
		"upserted":          report.Upserted,
		"forgotten":         report.Forgotten,
		"merged_duplicates": report.MergedDuplicates,
		"pruned_cold":       report.PrunedCold,
		"summary":           report.Summary,
	}); err == nil {
		_ = e.store.SetConfig(ConfigKeyLastAutoDreamReport, string(rawRep))
	}

	if report.Upserted > 0 || report.Forgotten > 0 || report.MergedDuplicates > 0 || report.PrunedCold > 0 {
		e.publishAssistantUpdate("memories")
	}
	return report, nil
}

type autoDreamUpsertItem struct {
	Category   string `json:"category"`
	FactKey    string `json:"fact_key"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
	Pinned     bool   `json:"pinned"`
}

func normalizeClaudeMemoryCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "user", "feedback", "project", "reference", "preference", "relation", "emotion", "insight", "infra", "schedule", "general":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "reference"
	}
}

func (e *Engine) runAutoDreamLLMPhase(
	ctx context.Context,
	scope string,
	todayStr string,
	existing []store.AgentMemory,
	transcript []store.AgentMessage,
) ([]autoDreamUpsertItem, []string, string, error) {
	var inv strings.Builder
	inv.WriteString(fmt.Sprintf("当前日期（绝对日期基准）: %s (Asia/Shanghai)\n", todayStr))
	inv.WriteString(fmt.Sprintf("整理作用域: %s\n\n", scope))

	inv.WriteString("[Phase 1: 现有长期记忆清单]\n")
	if len(existing) == 0 {
		inv.WriteString("(当前为空)\n")
	} else {
		for _, m := range existing {
			pin := ""
			if m.Pinned {
				pin = " [PINNED]"
			}
			inv.WriteString(fmt.Sprintf("- key=%s | cat=%s | imp=%d | updated=%s%s | %s\n",
				m.FactKey, m.Category, m.Importance, shortMemoryDate(m.UpdatedAt), pin, strings.TrimSpace(m.Content)))
		}
	}

	if len(transcript) > 0 {
		inv.WriteString("\n[Phase 2: 近期对话日志回放 (用于提取遗漏的纠偏/决策/新偏好)]\n")
		for _, msg := range transcript {
			role := "用户"
			if msg.Role == "assistant" {
				role = "助手"
			}
			line := truncateRunes(strings.ReplaceAll(strings.TrimSpace(msg.Content), "\n", " "), 220)
			if line != "" {
				inv.WriteString(fmt.Sprintf("- (%s) %s: %s\n", shortMemoryDate(msg.CreatedAt), role, line))
			}
		}
	}

	sysPrompt := strings.TrimSpace(`你是借鉴 Anthropic Claude AutoDream 架构的「离线睡眠记忆整理引擎」。
请完成 Phase 3（深度整合与消歧），并仅输出一个合法的 JSON 对象（禁止 Markdown 代码块与任何 Emoji）：
{
  "upserts": [
    {"category": "feedback|user|project|reference", "fact_key": "简短英文蛇形键名", "content": "精炼中文记忆(<=140字)", "importance": 4, "pinned": false}
  ],
  "forgets": ["已过时或被新决定推翻或重复合并掉的旧fact_key"],
  "summary": "一句话总结本次做梦整理做了哪些消歧、合并或新提炼"
}

严格遵循 Claude 记忆写入四铁律：
1. 四大高信噪比分类：
   - feedback: 用户纠偏、工作流纪律（明确要求不要做什么、偏好何种格式/风格及原因，最高价值！）
   - user: 稳定的用户画像、技术栈、背景与生活习惯
   - project: 正在推进的长期项目、目标与关键架构决策
   - reference: 外部系统、常用服务、链接或账号非敏感指针
2. 矛盾消歧：若新旧记忆或近期对话存在冲突（例如上周说用 A，最近改用 B），用最新决定更新 fact_key 并将旧冲突键放入 forgets。
3. 绝对日期原则：检查所有记忆中残留的“今天/昨天/明天/上周/下周/最近”等相对时间，全部改写为基于当前日期的绝对日期（YYYY-MM-DD）并通过 upserts 写回。
4. 拒绝记忆腐化与噪音：不要写入随时可用工具实时查到的临时状态（如某天具体待办、某刻 CPU 负载）；已失效的一次性旧事件或语义完全重叠的旧键放入 forgets；不要删除带 [PINNED] 标记的条目；如果当前记忆库已经很干净且近期对话无新长期价值，返回 {"upserts":[],"forgets":[],"summary":"记忆库整洁，无需变更"}。`)

	raw, err := e.CallLightModel(ctx, sysPrompt, inv.String())
	if err != nil {
		return nil, nil, "", err
	}
	clean := stripJSONCodeFence(raw)
	var parsed struct {
		Upserts []autoDreamUpsertItem `json:"upserts"`
		Forgets []string              `json:"forgets"`
		Summary string                `json:"summary"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil, nil, "", fmt.Errorf("解析 AutoDream 结果 JSON 失败: %w", err)
	}
	return parsed.Upserts, parsed.Forgets, parsed.Summary, nil
}

// StartAutoDreamLoop launches the background sleep-time compute loop (inspired by Claude AutoDream).
// It checks periodically and silently consolidates memories using the Free Pool when >= 24h have elapsed
// since the last dream and at least 5 new user dialogue turns occurred.
func (e *Engine) StartAutoDreamLoop(ctx context.Context) {
	if e == nil || e.store == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(2 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.maybeRunScheduledAutoDream(ctx)
			}
		}
	}()
}

func (e *Engine) maybeRunScheduledAutoDream(ctx context.Context) {
	if e == nil || e.store == nil || !e.hasLightTarget() {
		return
	}
	now := time.Now().In(store.ShanghaiLocation())
	lastStr, _ := e.store.GetConfig(ConfigKeyLastAutoDreamAt)
	if strings.TrimSpace(lastStr) != "" {
		if ts, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(lastStr), store.ShanghaiLocation()); err == nil {
			if now.Sub(ts) < 22*time.Hour {
				return
			}
		}
	}
	// Prefer low-traffic sleep hours (01:00 - 06:00 Shanghai) unless overdue > 36h
	overdue := false
	if strings.TrimSpace(lastStr) == "" {
		overdue = true
	} else if ts, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(lastStr), store.ShanghaiLocation()); err == nil {
		overdue = now.Sub(ts) >= 36*time.Hour
	}
	if !overdue && (now.Hour() < 1 || now.Hour() >= 6) {
		return
	}
	// Require at least 5 new user turns since last dream to avoid idle no-op compute
	if e.store.CountUserMessagesSince(lastStr) < 5 {
		return
	}

	dreamCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rep, err := e.RunAutoDream(dreamCtx, "all", true)
	if err != nil {
		log.Printf("[autodream] background sleep-time consolidation skipped: %v", err)
		return
	}
	log.Printf("[autodream] background consolidation complete: scopes=%d upserted=%d forgotten=%d merged=%d",
		rep.ScopesProcessed, rep.Upserted, rep.Forgotten, rep.MergedDuplicates)
}
