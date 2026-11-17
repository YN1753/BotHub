package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

func (e *Engine) publishAssistantUpdate(kind string) {
	if e != nil && e.hub != nil {
		e.hub.PublishEvent("assistant_update", map[string]any{
			"kind": kind,
			"time": store.NowShanghai(),
		})
	}
}

func (e *Engine) toolManageTodos(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action   string `json:"action"`
		ID       string `json:"id"`
		Title    string `json:"title"`
		Notes    string `json:"notes"`
		Status   string `json:"status"`
		Priority string `json:"priority"`
		DueAt    string `json:"due_at"`
		Tags     string `json:"tags"`
		Filter   string `json:"filter"`
		Items    []struct {
			Title    string `json:"title"`
			Notes    string `json:"notes"`
			Priority string `json:"priority"`
			DueAt    string `json:"due_at"`
			Tags     string `json:"tags"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_todos 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	ownerBot := strings.TrimSpace(sc.TimelineBotID)
	if ownerBot == "" {
		ownerBot = strings.TrimSpace(sc.BotID)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		filter := strings.TrimSpace(args.Filter)
		if filter == "" {
			filter = "open"
		}
		items, err := e.store.ListTodos(filter, "", 100)
		if err != nil {
			return "", err
		}
		nowStr := store.NowShanghai()
		overdue := 0
		for _, it := range items {
			if it.Status != store.TodoStatusDone && it.DueAt != "" && it.DueAt < nowStr[:10] {
				overdue++
			}
		}
		out, _ := json.Marshal(map[string]any{
			"filter":        filter,
			"count":         len(items),
			"overdue_count": overdue,
			"todos":         items,
		})
		return string(out), nil

	case "add":
		title := notifier.StripEmoji(strings.TrimSpace(args.Title))
		if title == "" {
			return "", fmt.Errorf("add 待办时 title 不能为空")
		}
		saved, err := e.store.SaveTodo(store.TodoItem{
			ID:       strings.TrimSpace(args.ID),
			BotID:    ownerBot,
			ThreadID: strings.TrimSpace(sc.ThreadID),
			Title:    title,
			Notes:    notifier.StripEmoji(strings.TrimSpace(args.Notes)),
			Status:   args.Status,
			Priority: args.Priority,
			DueAt:    strings.TrimSpace(args.DueAt),
			Tags:     notifier.StripEmoji(strings.TrimSpace(args.Tags)),
		})
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("todos")
		out, _ := json.Marshal(map[string]any{"ok": true, "todo": saved})
		return string(out), nil

	case "batch_add":
		if len(args.Items) == 0 {
			return "", fmt.Errorf("batch_add 时 items 不能为空")
		}
		if len(args.Items) > 20 {
			args.Items = args.Items[:20]
		}
		created := make([]store.TodoItem, 0, len(args.Items))
		for _, item := range args.Items {
			t := notifier.StripEmoji(strings.TrimSpace(item.Title))
			if t == "" {
				continue
			}
			prio := strings.TrimSpace(item.Priority)
			if prio == "" {
				prio = args.Priority
			}
			due := strings.TrimSpace(item.DueAt)
			if due == "" {
				due = strings.TrimSpace(args.DueAt)
			}
			tags := notifier.StripEmoji(strings.TrimSpace(item.Tags))
			if tags == "" {
				tags = notifier.StripEmoji(strings.TrimSpace(args.Tags))
			}
			saved, err := e.store.SaveTodo(store.TodoItem{
				BotID:    ownerBot,
				ThreadID: strings.TrimSpace(sc.ThreadID),
				Title:    t,
				Notes:    notifier.StripEmoji(strings.TrimSpace(item.Notes)),
				Priority: prio,
				DueAt:    due,
				Tags:     tags,
			})
			if err != nil {
				return "", err
			}
			created = append(created, saved)
		}
		if len(created) == 0 {
			return "", fmt.Errorf("batch_add 未包含有效标题的待办项")
		}
		e.publishAssistantUpdate("todos")
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"count": len(created),
			"todos": created,
		})
		return string(out), nil

	case "update", "complete":
		id := strings.TrimSpace(args.ID)
		var existing *store.TodoItem
		var err error
		if id != "" {
			existing, err = e.store.GetTodo(id)
			if err != nil {
				return "", err
			}
		} else if action == "complete" && strings.TrimSpace(args.Title) != "" {
			// Match by title substring when caller didn't pass ID
			candidates, _ := e.store.ListTodos("open", "", 100)
			needle := strings.ToLower(strings.TrimSpace(args.Title))
			for i := range candidates {
				if strings.Contains(strings.ToLower(candidates[i].Title), needle) {
					existing = &candidates[i]
					break
				}
			}
		}
		if existing == nil {
			return "", fmt.Errorf("未找到目标待办条目（请提供准确 id 或标题关键词）")
		}
		if t := notifier.StripEmoji(strings.TrimSpace(args.Title)); t != "" && action == "update" {
			existing.Title = t
		}
		if n := notifier.StripEmoji(strings.TrimSpace(args.Notes)); n != "" {
			existing.Notes = n
		}
		if strings.TrimSpace(args.Priority) != "" {
			existing.Priority = args.Priority
		}
		if strings.TrimSpace(args.DueAt) != "" {
			existing.DueAt = strings.TrimSpace(args.DueAt)
		}
		if strings.TrimSpace(args.Tags) != "" {
			existing.Tags = notifier.StripEmoji(strings.TrimSpace(args.Tags))
		}
		if action == "complete" {
			existing.Status = store.TodoStatusDone
		} else if strings.TrimSpace(args.Status) != "" {
			existing.Status = args.Status
		}
		saved, err := e.store.SaveTodo(*existing)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("todos")
		out, _ := json.Marshal(map[string]any{"ok": true, "todo": saved})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 待办时 id 必填")
		}
		ok, err := e.store.DeleteTodo(id)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("todos")
		out, _ := json.Marshal(map[string]any{"ok": ok, "deleted_id": id})
		return string(out), nil

	case "clear_done":
		removed, err := e.store.ClearCompletedTodos("")
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("todos")
		out, _ := json.Marshal(map[string]any{"ok": true, "removed": removed})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_todos action: %s", action)
	}
}

func (e *Engine) toolManageNotes(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action   string `json:"action"`
		ID       string `json:"id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
		Pinned   *bool  `json:"pinned"`
		Query    string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_notes 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	ownerBot := strings.TrimSpace(sc.TimelineBotID)
	if ownerBot == "" {
		ownerBot = strings.TrimSpace(sc.BotID)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "list"
	}

	switch action {
	case "list", "search":
		notes, err := e.store.ListNotes(strings.TrimSpace(args.Query), "", 60)
		if err != nil {
			return "", err
		}
		type noteSummary struct {
			ID        string `json:"id"`
			BotID     string `json:"bot_id,omitempty"`
			Title     string `json:"title"`
			Category  string `json:"category"`
			Pinned    bool   `json:"pinned"`
			Preview   string `json:"preview"`
			UpdatedAt string `json:"updated_at"`
		}
		summaries := make([]noteSummary, 0, len(notes))
		for _, n := range notes {
			summaries = append(summaries, noteSummary{
				ID:        n.ID,
				BotID:     n.BotID,
				Title:     n.Title,
				Category:  n.Category,
				Pinned:    n.Pinned,
				Preview:   truncateTitle(n.Content),
				UpdatedAt: n.UpdatedAt,
			})
		}
		out, _ := json.Marshal(map[string]any{
			"count": len(summaries),
			"notes": summaries,
		})
		return string(out), nil

	case "get":
		id := strings.TrimSpace(args.ID)
		var note *store.WorkNote
		var err error
		if id != "" {
			note, err = e.store.GetNote(id)
			if err != nil {
				return "", err
			}
		} else if strings.TrimSpace(args.Title) != "" {
			list, _ := e.store.ListNotes(strings.TrimSpace(args.Title), "", 10)
			if len(list) > 0 {
				note = &list[0]
			}
		}
		if note == nil {
			return "", fmt.Errorf("未找到目标工作文档")
		}
		out, _ := json.Marshal(map[string]any{"ok": true, "note": note})
		return string(out), nil

	case "upsert", "append":
		var note store.WorkNote
		foundExisting := false
		if id := strings.TrimSpace(args.ID); id != "" {
			if existing, _ := e.store.GetNote(id); existing != nil {
				note = *existing
				foundExisting = true
			} else {
				note.ID = id
				note.BotID = ownerBot
			}
		} else if t := notifier.StripEmoji(strings.TrimSpace(args.Title)); t != "" {
			// Locate existing note with matching title cleanly when ID is omitted in append mode
			if action == "append" {
				candidates, _ := e.store.ListNotes(t, "", 10)
				for _, cand := range candidates {
					if strings.EqualFold(strings.TrimSpace(cand.Title), t) {
						note = cand
						foundExisting = true
						break
					}
				}
			}
			if !foundExisting {
				note.BotID = ownerBot
			}
		} else {
			note.BotID = ownerBot
		}
		if t := notifier.StripEmoji(strings.TrimSpace(args.Title)); t != "" {
			note.Title = t
		}
		if note.Title == "" {
			return "", fmt.Errorf("%s 工作文档时 title 不能为空", action)
		}
		newChunk := notifier.StripEmoji(strings.TrimSpace(args.Content))
		if action == "append" && foundExisting && strings.TrimSpace(note.Content) != "" {
			if newChunk == "" {
				return "", fmt.Errorf("append 工作文档时 content 不能为空")
			}
			note.Content = strings.TrimRight(note.Content, "\n") + "\n\n---\n\n" + newChunk
		} else if newChunk != "" {
			note.Content = newChunk
		}
		if cat := notifier.StripEmoji(strings.TrimSpace(args.Category)); cat != "" {
			note.Category = cat
		}
		if note.Category == "" {
			note.Category = "deliverable"
		}
		if args.Pinned != nil {
			note.Pinned = *args.Pinned
		}
		if note.BotID == "" {
			note.BotID = ownerBot
		}
		saved, err := e.store.SaveNote(note)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("notes")
		out, _ := json.Marshal(map[string]any{"ok": true, "appended": action == "append" && foundExisting, "note": saved})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 工作文档时 id 必填")
		}
		ok, err := e.store.DeleteNote(id)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("notes")
		out, _ := json.Marshal(map[string]any{"ok": ok, "deleted_id": id})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_notes action: %s", action)
	}
}

func (e *Engine) toolSendNotification(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Title        string `json:"title"`
		MarkdownBody string `json:"markdown_body"`
		Priority     string `json:"priority"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 send_notification 参数失败: %w", err)
	}
	title := notifier.StripEmoji(strings.TrimSpace(args.Title))
	body := notifier.StripEmoji(strings.TrimSpace(args.MarkdownBody))
	if title == "" {
		return "", fmt.Errorf("通知 title 不能为空")
	}
	if body == "" {
		body = title
	}
	priority := strings.ToLower(strings.TrimSpace(args.Priority))
	if priority != "high" {
		priority = "normal"
	}

	sc := toolScopeFromContext(ctx)
	source := "assistant"
	if strings.TrimSpace(sc.TimelineBotID) != "" {
		source = "bot:" + strings.TrimSpace(sc.TimelineBotID)
	} else if strings.TrimSpace(sc.BotID) != "" {
		source = "bot:" + strings.TrimSpace(sc.BotID)
	}

	nClient := notifier.NewClientWithHub(e.store, e.hub)
	if err := nClient.SendWithPriority(ctx, source, title, body, priority); err != nil {
		return "", fmt.Errorf("推送通知失败: %w", err)
	}
	out, _ := json.Marshal(map[string]any{
		"ok":       true,
		"source":   source,
		"title":    title,
		"priority": priority,
		"sent_at":  store.NowShanghai(),
	})
	return string(out), nil
}

type consultTargetArg struct {
	BotID          string `json:"bot_id"`
	BotName        string `json:"bot_name"`
	Instruction    string `json:"instruction"`
	ContextSnippet string `json:"context_snippet"`
}

func (e *Engine) consultSingleBot(
	ctx context.Context,
	sc ToolScope,
	allBots []store.Bot,
	wantID, wantName, instruction, contextSnippet string,
) (map[string]any, error) {
	instruction = notifier.StripEmoji(strings.TrimSpace(instruction))
	if instruction == "" {
		return nil, fmt.Errorf("consult_bot 的 instruction 不能为空")
	}

	var target *store.Bot
	wantID = strings.TrimSpace(wantID)
	wantName = strings.TrimSpace(wantName)
	for i := range allBots {
		b := &allBots[i]
		if b.IsGroup {
			continue
		}
		if wantID != "" && b.ID == wantID {
			target = b
			break
		}
		if wantName != "" && strings.EqualFold(b.Name, wantName) {
			target = b
			break
		}
	}
	if target == nil && wantName != "" {
		lower := strings.ToLower(wantName)
		for i := range allBots {
			if !allBots[i].IsGroup && strings.Contains(strings.ToLower(allBots[i].Name), lower) {
				target = &allBots[i]
				break
			}
		}
	}
	if target == nil {
		return nil, fmt.Errorf("未找到目标成员 Bot (bot_id=%q bot_name=%q)", wantID, wantName)
	}
	if target.ID == sc.BotID {
		return nil, fmt.Errorf("不能向当前 Bot 自身发起嵌套协同调用")
	}
	for _, seenID := range sc.VisitedBots {
		if seenID == target.ID {
			return nil, fmt.Errorf("检测到循环委派 (%s 已在本次协同调用链中)", target.Name)
		}
	}

	nextVisited := append([]string(nil), sc.VisitedBots...)
	if sc.BotID != "" {
		nextVisited = append(nextVisited, sc.BotID)
	}
	nextVisited = append(nextVisited, target.ID)

	subCtx := WithToolScope(ctx, ToolScope{
		BotID:         target.ID,
		TimelineBotID: target.ID,
		ThreadID:      "",
		MemoryPolicy:  target.MemoryPolicy,
		Depth:         sc.Depth + 1,
		VisitedBots:   nextVisited,
	})

	callerLabel := sc.BotID
	for _, b := range allBots {
		if b.ID == sc.BotID && strings.TrimSpace(b.Name) != "" {
			callerLabel = b.Name
			break
		}
	}
	if callerLabel == "" {
		callerLabel = "主工作区"
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("[内部协同委派，来自「%s」]\n", callerLabel))
	if snip := notifier.StripEmoji(strings.TrimSpace(contextSnippet)); snip != "" {
		promptBuilder.WriteString("[前置参考上下文]\n")
		promptBuilder.WriteString(snip)
		promptBuilder.WriteString("\n\n")
	}
	promptBuilder.WriteString(instruction)
	promptBuilder.WriteString("\n\n请紧扣你的专业角色与可用工具直接完成并返回结果。")
	delegationPrompt := promptBuilder.String()

	subReq := ChatRequest{
		BotID:   target.ID,
		MsgKind: "delegate",
		Message: delegationPrompt,
		Model:   target.Model,
	}

	e.emitBotState(target.ID, store.BotStateWorking, fmt.Sprintf("响应「%s」协办委派...", callerLabel))
	start := time.Now()
	resp, err := e.runSingleBotTurn(subCtx, subReq, target, target.ID, nil, delegationPrompt, "", "", nil)
	durationMs := time.Since(start).Milliseconds()

	callerBot := sc.BotID
	if callerBot == "" {
		callerBot = sc.TimelineBotID
	}
	if err != nil {
		if callerBot != "" {
			e.store.RecordBotExecution(store.BotExecutionLog{
				BotID:         callerBot,
				ThreadID:      sc.ThreadID,
				ExecType:      "delegate",
				Name:          "consult:" + target.Name,
				InputSummary:  truncateTitle(instruction),
				OutputSummary: err.Error(),
				Status:        "error",
				DurationMs:    durationMs,
			})
		}
		return nil, fmt.Errorf("协同调用「%s」失败: %w", target.Name, err)
	}

	toolNames := make([]string, 0, len(resp.ToolTraces))
	for _, tr := range resp.ToolTraces {
		toolNames = append(toolNames, tr.Name)
	}
	if callerBot != "" {
		e.store.RecordBotExecution(store.BotExecutionLog{
			BotID:         callerBot,
			ThreadID:      sc.ThreadID,
			ExecType:      "delegate",
			Name:          "consult:" + target.Name,
			InputSummary:  truncateTitle(instruction),
			OutputSummary: truncateTitle(resp.Content),
			Status:        "ok",
			DurationMs:    durationMs,
		})
	}

	return map[string]any{
		"ok":          true,
		"bot_id":      target.ID,
		"bot_name":    target.Name,
		"role_title":  target.RoleTitle,
		"model_used":  resp.Model,
		"tools_used":  toolNames,
		"duration_ms": durationMs,
		"reply":       notifier.StripEmoji(resp.Content),
	}, nil
}

func (e *Engine) toolConsultBot(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		BotID          string             `json:"bot_id"`
		BotName        string             `json:"bot_name"`
		Instruction    string             `json:"instruction"`
		ContextSnippet string             `json:"context_snippet"`
		Targets        []consultTargetArg `json:"targets"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 consult_bot 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	if sc.Depth >= 2 {
		return "", fmt.Errorf("协同委派深度已达上限 (%d 层)，请由当前 Bot 直接综合作答", sc.Depth)
	}

	allBots, err := e.store.ListBots()
	if err != nil {
		return "", err
	}

	if len(args.Targets) > 0 {
		limit := len(args.Targets)
		if limit > 5 {
			limit = 5
		}
		results := make([]map[string]any, limit)
		var wg sync.WaitGroup
		for i := 0; i < limit; i++ {
			wg.Add(1)
			go func(idx int, t consultTargetArg) {
				defer wg.Done()
				snip := t.ContextSnippet
				if snip == "" {
					snip = args.ContextSnippet
				}
				res, cErr := e.consultSingleBot(ctx, sc, allBots, t.BotID, t.BotName, t.Instruction, snip)
				if cErr != nil {
					results[idx] = map[string]any{
						"ok":       false,
						"bot_id":   t.BotID,
						"bot_name": t.BotName,
						"error":    cErr.Error(),
					}
				} else {
					results[idx] = res
				}
			}(i, args.Targets[i])
		}
		wg.Wait()
		failedCount := 0
		for _, r := range results {
			if ok, _ := r["ok"].(bool); !ok {
				failedCount++
			}
		}
		out, _ := json.Marshal(map[string]any{
			"ok":             failedCount < limit,
			"parallel_count": limit,
			"failed_count":   failedCount,
			"results":        results,
		})
		return string(out), nil
	}

	res, err := e.consultSingleBot(ctx, sc, allBots, args.BotID, args.BotName, args.Instruction, args.ContextSnippet)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(res)
	return string(out), nil
}

func (e *Engine) toolWorkspaceBriefing(ctx context.Context, _ string) (string, error) {
	briefing, err := e.store.BuildWorkspaceBriefing()
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(briefing)
	return string(out), nil
}

var reAssetNumber = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

func (e *Engine) summarizeAssetsPayload(categoryFilter string) (string, error) {
	assets, err := e.store.ListAssets()
	if err != nil {
		return "", err
	}
	type currTotal struct {
		Currency   string  `json:"currency"`
		Monthly    float64 `json:"monthly_estimated"`
		Annualized float64 `json:"annualized_estimated"`
		Count      int     `json:"priced_items"`
	}
	currMap := make(map[string]*currTotal)
	var activeCount, expiring30Count, inactiveCount int
	cat := strings.TrimSpace(categoryFilter)

	for _, a := range assets {
		if cat != "" && a.Category != cat {
			continue
		}
		if store.NormalizeAssetStatus(a.Status) == "已停用" {
			inactiveCount++
			continue
		}
		activeCount++
		if a.DueDate != "" && !strings.HasPrefix(a.DueDate, "2099") && a.DaysLeft <= 30 {
			expiring30Count++
		}
		pStr := strings.TrimSpace(a.Price)
		if pStr == "" {
			continue
		}
		m := reAssetNumber.FindStringSubmatch(pStr)
		if len(m) < 2 {
			continue
		}
		amt, pErr := strconv.ParseFloat(m[1], 64)
		if pErr != nil || amt <= 0 {
			continue
		}
		currency := "USD"
		lowerP := strings.ToLower(pStr)
		switch {
		case strings.Contains(pStr, "¥") || strings.Contains(lowerP, "cny") || strings.Contains(lowerP, "rmb") || strings.Contains(pStr, "元"):
			currency = "CNY"
		case strings.Contains(pStr, "€") || strings.Contains(lowerP, "eur"):
			currency = "EUR"
		case strings.Contains(lowerP, "hkd"):
			currency = "HKD"
		}
		monthly := amt
		annual := amt * 12
		switch {
		case strings.Contains(pStr, "年") || strings.Contains(lowerP, "/yr") || strings.Contains(lowerP, "year") || strings.Contains(lowerP, "/y"):
			monthly = amt / 12
			annual = amt
		case strings.Contains(pStr, "季") || strings.Contains(lowerP, "quarter"):
			monthly = amt / 3
			annual = amt * 4
		case strings.Contains(pStr, "次") || strings.Contains(lowerP, "one"):
			monthly = 0
			annual = 0
		}
		entry, ok := currMap[currency]
		if !ok {
			entry = &currTotal{Currency: currency}
			currMap[currency] = entry
		}
		entry.Monthly = math.Round((entry.Monthly+monthly)*100) / 100
		entry.Annualized = math.Round((entry.Annualized+annual)*100) / 100
		entry.Count++
	}

	breakdown := make([]currTotal, 0, len(currMap))
	for _, v := range currMap {
		breakdown = append(breakdown, *v)
	}
	out, _ := json.Marshal(map[string]any{
		"active_count":       activeCount,
		"inactive_count":     inactiveCount,
		"expiring_30d_count": expiring30Count,
		"cost_by_currency":   breakdown,
	})
	return string(out), nil
}

// Safe recursive-descent math expression evaluator (no external shells or eval).
type mathParser struct {
	src string
	pos int
}

func (p *mathParser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func (p *mathParser) peek() byte {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *mathParser) parseExpr() (float64, error) {
	v, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.pos++
		rhs, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			v += rhs
		} else {
			v -= rhs
		}
	}
	return v, nil
}

func (p *mathParser) parseTerm() (float64, error) {
	v, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '*' && ch != '/' && ch != '%' {
			break
		}
		p.pos++
		rhs, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		switch ch {
		case '*':
			v *= rhs
		case '/':
			if rhs == 0 {
				return 0, fmt.Errorf("除数不能为 0")
			}
			v /= rhs
		case '%':
			if rhs == 0 {
				return 0, fmt.Errorf("取模除数不能为 0")
			}
			v = math.Mod(v, rhs)
		}
	}
	return v, nil
}

func (p *mathParser) parsePower() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

func (p *mathParser) parseUnary() (float64, error) {
	ch := p.peek()
	if ch == '+' {
		p.pos++
		return p.parseUnary()
	}
	if ch == '-' {
		p.pos++
		v, err := p.parseUnary()
		return -v, err
	}
	return p.parsePrimary()
}

func (p *mathParser) parsePrimary() (float64, error) {
	ch := p.peek()
	if ch == '(' {
		p.pos++
		v, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("括号未闭合")
		}
		p.pos++
		if p.peek() == '%' {
			p.pos++
			v /= 100
		}
		return v, nil
	}
	if unicode.IsLetter(rune(ch)) {
		start := p.pos
		for p.pos < len(p.src) && (unicode.IsLetter(rune(p.src[p.pos])) || unicode.IsDigit(rune(p.src[p.pos])) || p.src[p.pos] == '_') {
			p.pos++
		}
		ident := strings.ToLower(p.src[start:p.pos])
		if p.peek() != '(' {
			switch ident {
			case "pi":
				return math.Pi, nil
			case "e":
				return math.E, nil
			default:
				return 0, fmt.Errorf("未知常量: %s", ident)
			}
		}
		p.pos++
		var args []float64
		if p.peek() != ')' {
			for {
				a, err := p.parseExpr()
				if err != nil {
					return 0, err
				}
				args = append(args, a)
				if p.peek() == ',' {
					p.pos++
					continue
				}
				break
			}
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("函数 %s 缺少右括号", ident)
		}
		p.pos++
		return evalMathFunc(ident, args)
	}
	if (ch >= '0' && ch <= '9') || ch == '.' {
		start := p.pos
		dotSeen := false
		for p.pos < len(p.src) {
			c := p.src[p.pos]
			if c >= '0' && c <= '9' {
				p.pos++
			} else if c == '.' && !dotSeen {
				dotSeen = true
				p.pos++
			} else {
				break
			}
		}
		val, err := strconv.ParseFloat(p.src[start:p.pos], 64)
		if err != nil {
			return 0, fmt.Errorf("无效数字: %s", p.src[start:p.pos])
		}
		if p.peek() == '%' && (p.pos+1 >= len(p.src) || !(p.src[p.pos+1] >= '0' && p.src[p.pos+1] <= '9')) {
			// Check whether '%' is postfix percent (followed by operator, end, or ')') rather than modulo
			after := p.pos + 1
			for after < len(p.src) && unicode.IsSpace(rune(p.src[after])) {
				after++
			}
			if after >= len(p.src) || strings.ContainsRune("+-*/^),", rune(p.src[after])) {
				p.pos++
				val /= 100
			}
		}
		return val, nil
	}
	return 0, fmt.Errorf("无法解析表达式位置 %d", p.pos)
}

func evalMathFunc(name string, args []float64) (float64, error) {
	switch name {
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs 需要 1 个参数")
		}
		return math.Abs(args[0]), nil
	case "sqrt":
		if len(args) != 1 || args[0] < 0 {
			return 0, fmt.Errorf("sqrt 需要 1 个非负参数")
		}
		return math.Sqrt(args[0]), nil
	case "floor":
		if len(args) != 1 {
			return 0, fmt.Errorf("floor 需要 1 个参数")
		}
		return math.Floor(args[0]), nil
	case "ceil":
		if len(args) != 1 {
			return 0, fmt.Errorf("ceil 需要 1 个参数")
		}
		return math.Ceil(args[0]), nil
	case "round":
		if len(args) < 1 || len(args) > 2 {
			return 0, fmt.Errorf("round 需要 1 或 2 个参数")
		}
		digits := 0
		if len(args) == 2 {
			digits = int(args[1])
		}
		factor := math.Pow(10, float64(digits))
		return math.Round(args[0]*factor) / factor, nil
	case "min":
		if len(args) < 1 {
			return 0, fmt.Errorf("min 至少需要 1 个参数")
		}
		m := args[0]
		for _, v := range args[1:] {
			if v < m {
				m = v
			}
		}
		return m, nil
	case "max":
		if len(args) < 1 {
			return 0, fmt.Errorf("max 至少需要 1 个参数")
		}
		m := args[0]
		for _, v := range args[1:] {
			if v > m {
				m = v
			}
		}
		return m, nil
	default:
		return 0, fmt.Errorf("不支持的函数: %s", name)
	}
}

func (e *Engine) toolCalculateEval(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 calculate_eval 参数失败: %w", err)
	}
	expr := strings.TrimSpace(args.Expression)
	if expr == "" {
		return "", fmt.Errorf("expression 不能为空")
	}
	p := &mathParser{src: expr}
	val, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	if ch := p.peek(); ch != 0 {
		return "", fmt.Errorf("表达式末尾存在多余字符: %q", string(ch))
	}
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return "", fmt.Errorf("计算结果超出有限实数范围")
	}
	rounded := math.Round(val*1e8) / 1e8
	out, _ := json.Marshal(map[string]any{
		"ok":         true,
		"expression": expr,
		"result":     rounded,
		"formatted":  strconv.FormatFloat(rounded, 'f', -1, 64),
	})
	return string(out), nil
}

func weekdayCN(w time.Weekday) string {
	names := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(w)%7]
}

func (e *Engine) toolDatetimeCalc(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action       string `json:"action"`
		BaseTime     string `json:"base_time"`
		DeltaMinutes int    `json:"delta_minutes"`
		DeltaHours   int    `json:"delta_hours"`
		DeltaDays    int    `json:"delta_days"`
		CronExpr     string `json:"cron_expr"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 datetime_calc 参数失败: %w", err)
	}

	loc := store.ShanghaiLocation()
	base := time.Now().In(loc)
	if raw := strings.TrimSpace(args.BaseTime); raw != "" {
		parsedOK := false
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
				base = t
				parsedOK = true
				break
			}
		}
		if !parsedOK {
			return "", fmt.Errorf("无法识别 base_time %q（请使用 YYYY-MM-DD HH:MM 或 YYYY-MM-DD）", raw)
		}
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "now"
	}

	switch action {
	case "now", "add":
		target := base.AddDate(0, 0, args.DeltaDays).
			Add(time.Duration(args.DeltaHours)*time.Hour + time.Duration(args.DeltaMinutes)*time.Minute)
		out, _ := json.Marshal(map[string]any{
			"base":          base.Format("2006-01-02 15:04:05"),
			"result":        target.Format("2006-01-02 15:04:05"),
			"run_at_format": target.Format("2006-01-02 15:04"),
			"date":          target.Format("2006-01-02"),
			"weekday":       weekdayCN(target.Weekday()),
			"unix":          target.Unix(),
		})
		return string(out), nil

	case "cron_next":
		expr := strings.TrimSpace(args.CronExpr)
		if expr == "" {
			return "", fmt.Errorf("cron_next 必须传入 cron_expr")
		}
		sched, err := cron.ParseStandard(expr)
		if err != nil {
			return "", fmt.Errorf("无效 Cron 表达式: %w", err)
		}
		runs := make([]string, 0, 5)
		cur := base
		for i := 0; i < 5; i++ {
			next := sched.Next(cur).In(loc)
			runs = append(runs, fmt.Sprintf("%s (%s)", next.Format("2006-01-02 15:04"), weekdayCN(next.Weekday())))
			cur = next
		}
		out, _ := json.Marshal(map[string]any{
			"cron_expr": expr,
			"next_runs": runs,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 datetime_calc action: %s", action)
	}
}

var (
	reHTMLTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reDDGResult   = regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	reDDGSnippet  = regexp.MustCompile(`(?is)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)
)

func extractHTMLTitle(rawHTML string) string {
	m := reHTMLTitle.FindStringSubmatch(rawHTML)
	if len(m) < 2 {
		return ""
	}
	return notifier.StripEmoji(strings.TrimSpace(reMultiSpace.ReplaceAllString(html.UnescapeString(reHTMLTags.ReplaceAllString(m[1], " ")), " ")))
}

func unwrapDDGRedirectURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	if parsed, err := url.Parse(raw); err == nil {
		if uddg := parsed.Query().Get("uddg"); uddg != "" {
			return uddg
		}
	}
	return raw
}

func (e *Engine) searchDuckDuckGoHTML(ctx context.Context, query string, limit int) []struct {
	Title   string
	URL     string
	Snippet string
} {
	if e.allowLocalhostForTest || limit <= 0 {
		return nil
	}
	ddgURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ddgURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VesperBotHub/2.16)")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := e.httpClient.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil
	}
	raw := string(bodyBytes)
	titleMatches := reDDGResult.FindAllStringSubmatch(raw, limit)
	snippetMatches := reDDGSnippet.FindAllStringSubmatch(raw, limit)

	out := make([]struct {
		Title   string
		URL     string
		Snippet string
	}, 0, len(titleMatches))
	for i, m := range titleMatches {
		targetURL := unwrapDDGRedirectURL(m[1])
		title := notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(m[2])))
		if targetURL == "" || title == "" {
			continue
		}
		snippet := ""
		if i < len(snippetMatches) {
			snippet = notifier.StripEmoji(strings.TrimSpace(stripHTMLToText(snippetMatches[i][1])))
		}
		out = append(out, struct {
			Title   string
			URL     string
			Snippet string
		}{
			Title:   title,
			URL:     targetURL,
			Snippet: snippet,
		})
	}
	return out
}
