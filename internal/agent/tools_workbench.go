package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func manageTodosToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_todos",
				Description: "管理用户的个人待办与行动清单（Muse 个人助手核心能力）。支持 list（查看待办）、add（新增单条待办）、batch_add（一次批量拆解多条子任务）、update（修改优先级/截止时间/状态）、complete（勾选完成，可传 id 或标题关键词）、delete（删除）、clear_done（清理已完成）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "add", "batch_add", "update", "complete", "delete", "clear_done"},
							"description": "操作类型：list | add | batch_add | update | complete | delete | clear_done",
						},
						"items": map[string]any{
							"type":        "array",
							"description": "当 action=batch_add 时传入批量待办数组（最多 20 条），每项含 title 及可选 priority/due_at/notes/tags",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"title":    map[string]any{"type": "string"},
									"priority": map[string]any{"type": "string", "enum": []string{"high", "normal", "low"}},
									"due_at":   map[string]any{"type": "string"},
									"notes":    map[string]any{"type": "string"},
									"tags":     map[string]any{"type": "string"},
								},
								"required": []string{"title"},
							},
						},
						"id": map[string]any{
							"type":        "string",
							"description": "待办条目 ID（update/delete 时使用）",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "待办事项标题（add 必填；complete 也可传标题关键词匹配）",
						},
						"notes": map[string]any{
							"type":        "string",
							"description": "可选：补充备注或子步骤说明",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "doing", "done"},
							"description": "状态：pending | doing | done",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"high", "normal", "low"},
							"description": "优先级：high | normal | low",
						},
						"due_at": map[string]any{
							"type":        "string",
							"description": "可选：截止日期或时间，如 2026-11-20 或 2026-11-20 18:00",
						},
						"tags": map[string]any{
							"type":        "string",
							"description": "可选：逗号分隔标签，如 工作,财务",
						},
						"filter": map[string]any{
							"type":        "string",
							"description": "当 action=list 时筛选状态：open（默认未完成）| all | done | doing | pending",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在梳理待办清单...",
		DomainKeywords: []string{"待办", "todo", "任务", "提醒", "清单", "ddl", "截止"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageTodos(ctx, cleanArgs)
		},
	}
}

func manageNotesToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_notes",
				Description: "读写个人工作文档与成果交付区（报告、周报、邮件草稿、行程单、脚本模板等长文成品）。支持 list、search、get、upsert（覆写或新建）、append（向已有文档末尾安全追加新章节或日志，不会覆盖原文）、delete。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "search", "get", "upsert", "append", "delete"},
							"description": "操作类型：list | search | get | upsert | append | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "文档 ID（更新或读取已有文档时使用）",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "文档标题",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "Markdown 格式的文档正文",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "分类标签，如 deliverable / draft / plan / summary",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶展示",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词（search 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在编写工作文档...",
		DomainKeywords: []string{"笔记", "备忘", "日记", "整理", "存档", "note"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageNotes(ctx, cleanArgs)
		},
	}
}

func sendNotificationToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "send_notification",
				Description: "主动向用户收件箱与桌面推送一条重要卡片通知（支持 high 高优先级提醒），适合在巡检异常、耗时分析完成或紧急提醒时使用。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "推送标题",
						},
						"markdown_body": map[string]any{
							"type":        "string",
							"description": "Markdown 格式的推送正文摘要",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"normal", "high"},
							"description": "通知优先级：normal | high",
						},
					},
					"required": []string{"title", "markdown_body"},
				},
			},
		},
		StatusLabel: "正在推送收件箱通知...",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolSendNotification(ctx, cleanArgs)
		},
	}
}

func workspaceBriefingToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "workspace_briefing",
				Description: "一键汇总用户的今日工作全景简报（逾期与今日到期待办、高优先待办、14天内到期资产、启用中的定时任务、未读通知与近期工作文档），适合个人助手规划今日日程。",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		ReadOnly:    true,
		StatusLabel: "正在汇总今日行动简报...",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolWorkspaceBriefing(ctx, cleanArgs)
		},
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

func (e *Engine) toolWorkspaceBriefing(_ context.Context, _ string) (string, error) {
	briefing, err := e.store.BuildWorkspaceBriefing()
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(briefing)
	return string(out), nil
}
