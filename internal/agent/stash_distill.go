package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var reBullet = regexp.MustCompile(`^\s*(?:[-*•]+|\d+[.)、])\s*`)

func manageStashToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_stash",
				Description: "管理与检索用户的「智能收藏与闪念匣（Smart Stash）」。支持 capture（秒收网页链接、B站分享文案、GitHub仓库或随手文字闪念并触发异步AI干货提炼）、search（基于全文检索引擎搜之前收藏过的文章/视频/闪念）、get（读取单条完整要点与正文快照）、update（修改标题/批注/标签/置顶/归档/标记已读）、delete（删除）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"capture", "search", "get", "update", "delete"},
							"description": "操作类型：capture | search | get | update | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "收藏条目 ID（get / update / delete 时使用）",
						},
						"raw_input": map[string]any{
							"type":        "string",
							"description": "当 action=capture 时必填：原始链接、手机端带标题前缀分享文案或纯文字闪念",
						},
						"user_note": map[string]any{
							"type":        "string",
							"description": "可选：附带的个人批注或想法备注",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "当 action=search 时可选：中英文全文检索关键词（支持长短词召回）",
						},
						"filter": map[string]any{
							"type":        "string",
							"description": "可选：筛选视图 unread | all | pinned | note | archived",
						},
						"tag": map[string]any{
							"type":        "string",
							"description": "可选：按单个标签精确过滤",
						},
						"kind": map[string]any{
							"type":        "string",
							"description": "可选：按来源类型过滤 link | bilibili | github | note",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "当 action=update 时可选：修改后的标题",
						},
						"tags": map[string]any{
							"type":        "string",
							"description": "可选：逗号分隔分类标签，如 AI,架构,Bilibili",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "当 action=update 时可选：ready | archived",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "当 action=update 时可选：是否置顶",
						},
						"mark_read": map[string]any{
							"type":        "boolean",
							"description": "当 action=update 时可选：设为 true 则增加已读计数",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "当 action=search 时返回最大条数（默认 20，最大 50）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在检索或收纳收藏匣...",
		DomainKeywords: []string{"收藏", "闪念", "稍后读", "b站", "bilibili", "b23.tv", "书签", "stash"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageStash(ctx, cleanArgs)
		},
	}
}

func (e *Engine) stashSemaphore() chan struct{} {
	if e.stashSem != nil {
		return e.stashSem
	}
	return make(chan struct{}, 3)
}

// CaptureAndDistillStash persists a newly captured link/share/thought immediately (<50ms) and schedules background distillation.
func (e *Engine) CaptureAndDistillStash(rawInput, userNote, tags string, forceReprocess bool) (store.StashItem, bool, error) {
	cleanRaw := notifier.StripEmoji(strings.TrimSpace(rawInput))
	cleanNote := notifier.StripEmoji(strings.TrimSpace(userNote))
	cleanTags := notifier.StripEmoji(strings.TrimSpace(tags))
	if cleanRaw == "" {
		return store.StashItem{}, false, fmt.Errorf("收藏或闪念内容不能为空")
	}

	item, deduplicated, shouldDistill, err := e.store.CaptureStashItem(cleanRaw, cleanNote, cleanTags, forceReprocess)
	if err != nil {
		return store.StashItem{}, false, err
	}
	e.publishAssistantUpdate("stash")
	if shouldDistill {
		e.DistillStashAsync(item.ID)
	}
	return item, deduplicated, nil
}

// ReprocessStashItem resets an existing stash item to processing and launches background source extraction + AI distillation.
func (e *Engine) ReprocessStashItem(id string) (*store.StashItem, error) {
	existing, err := e.store.GetStashItem(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}
	existing.Status = store.StashStatusProcessing
	existing.ErrorMsg = ""
	existing.UpdatedAt = store.NowShanghai()
	saved, err := e.store.UpsertStashItemFull(*existing)
	if err != nil {
		return nil, err
	}
	e.publishAssistantUpdate("stash")
	e.DistillStashAsync(saved.ID)
	return &saved, nil
}

// DistillStashAsync runs source extraction and AI summarization on a bounded background worker pool.
func (e *Engine) DistillStashAsync(id string) {
	if e == nil || e.store == nil || strings.TrimSpace(id) == "" {
		return
	}
	sem := e.stashSemaphore()
	go func(stashID string) {
		sem <- struct{}{}
		defer func() { <-sem }()

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		// Background unattended stash distillation must never burn paid quota.
		ctx = store.WithPaidFallback(ctx, false)
		_ = e.DistillStashNow(ctx, stashID)
	}(strings.TrimSpace(id))
}

type aiDistillPayload struct {
	Title  string   `json:"title"`
	TLDR   string   `json:"tldr"`
	Points []string `json:"points"`
	Tags   []string `json:"tags"`
}

func parseAIDistillOutput(raw string) (*aiDistillPayload, error) {
	s := stripJSONCodeFence(raw)
	if s == "" {
		return nil, fmt.Errorf("AI 返回为空")
	}
	if start := strings.IndexByte(s, '{'); start >= 0 {
		if end := strings.LastIndexByte(s, '}'); end > start {
			s = s[start : end+1]
		}
	}
	s = reTrailingJSONComma.ReplaceAllString(s, "$1")

	var out aiDistillPayload
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}

	out.Title = store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(out.Title)), 120)
	out.TLDR = store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(out.TLDR)), 70)

	cleanPts := make([]string, 0, len(out.Points))
	for _, p := range out.Points {
		cp := notifier.StripEmoji(strings.TrimSpace(reBullet.ReplaceAllString(p, "")))
		if cp != "" {
			cleanPts = append(cleanPts, store.TruncateRunes(cp, 180))
		}
		if len(cleanPts) >= 5 {
			break
		}
	}
	out.Points = cleanPts

	cleanTags := make([]string, 0, len(out.Tags))
	for _, t := range out.Tags {
		ct := notifier.StripEmoji(strings.TrimSpace(strings.TrimPrefix(t, "#")))
		if ct != "" {
			cleanTags = append(cleanTags, store.TruncateRunes(ct, 16))
		}
		if len(cleanTags) >= 4 {
			break
		}
	}
	out.Tags = cleanTags
	return &out, nil
}

func buildLocalFallbackSummary(title, desc, excerpt string) (string, []string) {
	base := strings.TrimSpace(desc)
	if base == "" {
		base = strings.TrimSpace(excerpt)
	}
	lines := strings.Split(base, "\n")
	points := make([]string, 0, 4)
	for _, line := range lines {
		clean := notifier.StripEmoji(strings.TrimSpace(reBullet.ReplaceAllString(line, "")))
		if len([]rune(clean)) < 6 {
			continue
		}
		points = append(points, store.TruncateRunes(clean, 140))
		if len(points) >= 4 {
			break
		}
	}
	tldr := ""
	if len(points) > 0 {
		tldr = store.TruncateRunes(points[0], 70)
	} else if title != "" {
		tldr = store.TruncateRunes(title, 70)
	}
	return tldr, points
}

// DistillStashNow executes synchronous source extraction + lightweight AI summarization for a stash item.
func (e *Engine) DistillStashNow(ctx context.Context, id string) error {
	item, err := e.store.GetStashItem(id)
	if err != nil || item == nil {
		return err
	}

	var src *extractedStashSource
	var fetchErr error

	if item.Kind == store.StashKindNote || strings.TrimSpace(item.URL) == "" {
		src = &extractedStashSource{
			Kind:         store.StashKindNote,
			Title:        item.Title,
			Excerpt:      store.TruncateRunes(notifier.StripEmoji(strings.TrimSpace(item.RawInput)), store.MaxStashExcerptRunes),
			SuggestedTag: []string{"闪念"},
		}
	} else {
		switch item.Kind {
		case store.StashKindBilibili:
			src, fetchErr = e.extractBilibiliSource(ctx, item.URL)
		case store.StashKindGitHub:
			src, fetchErr = e.extractGitHubSource(ctx, item.URL)
		default:
			src, fetchErr = e.extractGenericWebSource(ctx, item.URL)
		}
	}

	if src != nil {
		if src.CanonicalURL != "" {
			item.URL = src.CanonicalURL
		}
		if src.Domain != "" {
			item.Domain = src.Domain
		}
		if src.Kind != "" {
			item.Kind = src.Kind
		}
		if src.Title != "" {
			item.Title = notifier.StripEmoji(strings.TrimSpace(src.Title))
		}
		if src.Excerpt != "" {
			item.ContentExcerpt = store.TruncateRunes(notifier.StripEmoji(src.Excerpt), store.MaxStashExcerptRunes)
		}
		if len(src.SuggestedTag) > 0 {
			item.Tags = store.MergeStashTags(item.Tags, strings.Join(src.SuggestedTag, ","))
		}
	}

	material := strings.TrimSpace(item.ContentExcerpt)
	if material == "" && item.Kind == store.StashKindNote {
		material = strings.TrimSpace(item.RawInput)
	}

	if fetchErr != nil && material == "" {
		if item.Title == "" {
			if item.Domain != "" {
				item.Title = item.Domain
			} else {
				item.Title = store.TruncateRunes(item.RawInput, 60)
			}
		}
		item.Status = store.StashStatusFailed
		item.ErrorMsg = notifier.StripEmoji(fmt.Sprintf("抓取受限，已保留原始链接与批注 (%v)", fetchErr))
		item.UpdatedAt = store.NowShanghai()
		_, uErr := e.store.UpsertStashItemFull(*item)
		e.publishAssistantUpdate("stash")
		return uErr
	}

	if e.hasLightTarget() {
		sysPrompt := `你是 BotHub 智能收藏与闪念蒸馏器。请阅读输入材料并输出严格合法的 JSON 对象（不要使用 Markdown 代码块包裹，不要输出任何额外说明），格式如下：
{"title":"简明准确的标题（不超过48字）","tldr":"一句话核心结论（不超过70字）","points":["核心干货要点1","核心干货要点2","核心干货要点3"],"tags":["标签1","标签2"]}
严格规范：
1. tldr 必须控制在 70 字以内，直击本质结论、核心方案或关键洞察，拒绝空话。
2. points 数组提炼 3 到 5 条高信息密度干货要点（若输入是很短的随手闪念，可提炼 2 到 3 条关键行动或延伸要点）。
3. tags 数组给出 2 到 4 个简短中文或技术分类标签（不含井号）。
4. 绝对禁止在任何字段中使用 Emoji 表情或装饰性符号。`

		var userPrompt strings.Builder
		userPrompt.WriteString(fmt.Sprintf("来源类型：%s\n", item.Kind))
		if item.URL != "" {
			userPrompt.WriteString(fmt.Sprintf("原始链接：%s\n", item.URL))
		}
		if item.Title != "" {
			userPrompt.WriteString(fmt.Sprintf("候选标题：%s\n", item.Title))
		}
		if item.UserNote != "" {
			userPrompt.WriteString(fmt.Sprintf("用户批注：%s\n", item.UserNote))
		}
		userPrompt.WriteString("\n正文材料：\n")
		userPrompt.WriteString(store.TruncateRunes(material, 7500))

		rawAI, aiErr := e.CallLightModel(ctx, sysPrompt, userPrompt.String())
		if aiErr == nil {
			if parsed, pErr := parseAIDistillOutput(rawAI); pErr == nil {
				if parsed.Title != "" && (item.Kind == store.StashKindNote || item.Title == "" || item.Title == item.Domain) {
					item.Title = parsed.Title
				} else if item.Title == "" && parsed.Title != "" {
					item.Title = parsed.Title
				}
				if parsed.TLDR != "" {
					item.AITldr = parsed.TLDR
				}
				if len(parsed.Points) > 0 {
					item.Points = parsed.Points
				}
				if len(parsed.Tags) > 0 {
					item.Tags = store.MergeStashTags(item.Tags, strings.Join(parsed.Tags, ","))
				}
				item.Status = store.StashStatusReady
				item.ErrorMsg = ""
				item.UpdatedAt = store.NowShanghai()
				_, uErr := e.store.UpsertStashItemFull(*item)
				e.publishAssistantUpdate("stash")
				return uErr
			}
		}
	}

	desc := ""
	if src != nil {
		desc = src.Description
	}
	fallbackTLDR, fallbackPts := buildLocalFallbackSummary(item.Title, desc, material)
	if item.Title == "" {
		item.Title = store.TruncateRunes(material, 48)
	}
	if item.AITldr == "" {
		item.AITldr = fallbackTLDR
	}
	if len(item.Points) == 0 && len(fallbackPts) > 0 {
		item.Points = fallbackPts
	}
	item.Status = store.StashStatusReady
	if !e.hasLightTarget() {
		item.ErrorMsg = "未配置 AI 模型密钥，已基于抓取快照生成本地摘要"
	} else {
		item.ErrorMsg = "AI 模型暂时繁忙，已保留抓取快照与本地摘要，可稍后重提炼"
	}
	item.UpdatedAt = store.NowShanghai()
	_, uErr := e.store.UpsertStashItemFull(*item)
	e.publishAssistantUpdate("stash")
	return uErr
}

// toolManageStash implements the built-in manage_stash agent tool (capture | search | get | update | delete).
func (e *Engine) toolManageStash(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Action         string `json:"action"`
		ID             string `json:"id"`
		RawInput       string `json:"raw_input"`
		URL            string `json:"url"`
		Title          string `json:"title"`
		UserNote       string `json:"user_note"`
		Query          string `json:"query"`
		Filter         string `json:"filter"`
		Tag            string `json:"tag"`
		Kind           string `json:"kind"`
		Tags           string `json:"tags"`
		Status         string `json:"status"`
		Pinned         *bool  `json:"pinned"`
		MarkRead       *bool  `json:"mark_read"`
		ForceReprocess bool   `json:"force_reprocess"`
		Limit          int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_stash 参数失败: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "search"
	}

	switch action {
	case "capture", "add":
		rawInput := strings.TrimSpace(args.RawInput)
		if rawInput == "" {
			if strings.TrimSpace(args.URL) != "" {
				if strings.TrimSpace(args.Title) != "" {
					rawInput = strings.TrimSpace(args.Title) + " " + strings.TrimSpace(args.URL)
				} else {
					rawInput = strings.TrimSpace(args.URL)
				}
			} else if strings.TrimSpace(args.Title) != "" {
				rawInput = strings.TrimSpace(args.Title)
			}
		}
		if rawInput == "" {
			return "", fmt.Errorf("capture 收藏或闪念时 raw_input（或 url）不能为空")
		}
		saved, dedup, err := e.CaptureAndDistillStash(rawInput, args.UserNote, args.Tags, args.ForceReprocess)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"deduplicated": dedup,
			"item":         saved,
			"note":         "已收入智能收藏与闪念匣，后台正异步抓取并提炼核心要点",
		})
		return string(out), nil

	case "search", "list":
		limit := args.Limit
		if limit <= 0 || limit > 50 {
			limit = 20
		}
		filter := strings.TrimSpace(args.Filter)
		if filter == "" && strings.TrimSpace(args.Status) != "" {
			filter = strings.TrimSpace(args.Status)
		}
		items, err := e.store.SearchStashItemsWithKind(args.Query, filter, args.Tag, args.Kind, limit)
		if err != nil {
			return "", err
		}
		type compactItem struct {
			ID        string   `json:"id"`
			Kind      string   `json:"kind"`
			Title     string   `json:"title"`
			URL       string   `json:"url,omitempty"`
			AITldr    string   `json:"ai_tldr,omitempty"`
			Points    []string `json:"points,omitempty"`
			Tags      string   `json:"tags,omitempty"`
			UserNote  string   `json:"user_note,omitempty"`
			Status    string   `json:"status"`
			Pinned    bool     `json:"pinned"`
			ReadCount int      `json:"read_count"`
			CreatedAt string   `json:"created_at"`
		}
		summaries := make([]compactItem, 0, len(items))
		for _, it := range items {
			summaries = append(summaries, compactItem{
				ID:        it.ID,
				Kind:      it.Kind,
				Title:     it.Title,
				URL:       it.URL,
				AITldr:    it.AITldr,
				Points:    it.Points,
				Tags:      it.Tags,
				UserNote:  it.UserNote,
				Status:    it.Status,
				Pinned:    it.Pinned,
				ReadCount: it.ReadCount,
				CreatedAt: it.CreatedAt,
			})
		}
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"count": len(summaries),
			"items": summaries,
		})
		return string(out), nil

	case "get":
		var target *store.StashItem
		var err error
		if id := strings.TrimSpace(args.ID); id != "" {
			target, err = e.store.GetStashItem(id)
			if err != nil {
				return "", err
			}
		} else if q := strings.TrimSpace(args.Query); q != "" {
			list, _ := e.store.SearchStashItems(q, "everything", "", 5)
			if len(list) > 0 {
				target = &list[0]
			}
		}
		if target == nil {
			return "", fmt.Errorf("未找到目标收藏条目（请提供准确 id 或通过 search 检索）")
		}
		out, _ := json.Marshal(map[string]any{"ok": true, "item": target})
		return string(out), nil

	case "update":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("update 收藏条目时 id 必填")
		}
		existing, err := e.store.GetStashItem(id)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return "", fmt.Errorf("未找到目标收藏条目: %s", id)
		}
		if args.MarkRead != nil && *args.MarkRead {
			existing.ReadCount++
			existing.LastReviewedAt = store.NowShanghai()
		}
		if t := notifier.StripEmoji(strings.TrimSpace(args.Title)); t != "" {
			existing.Title = t
		}
		if strings.TrimSpace(args.UserNote) != "" {
			existing.UserNote = notifier.StripEmoji(strings.TrimSpace(args.UserNote))
		}
		if strings.TrimSpace(args.Tags) != "" {
			existing.Tags = notifier.StripEmoji(strings.TrimSpace(args.Tags))
		}
		if args.Pinned != nil {
			existing.Pinned = *args.Pinned
		}
		if st := strings.TrimSpace(args.Status); st != "" {
			existing.Status = store.NormalizeStashStatus(st)
		}
		existing.UpdatedAt = store.NowShanghai()
		saved, err := e.store.UpsertStashItemFull(*existing)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("stash")
		out, _ := json.Marshal(map[string]any{"ok": true, "item": saved})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 收藏条目时 id 必填")
		}
		ok, err := e.store.DeleteStashItem(id)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("stash")
		out, _ := json.Marshal(map[string]any{"ok": ok, "deleted_id": id})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_stash action: %s", action)
	}
}
