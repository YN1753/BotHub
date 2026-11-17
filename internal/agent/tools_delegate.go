package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func consultBotToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "consult_bot",
				Description: "主动协同工作区中的专业 Bot 成员协助完成子任务。支持单人委派（bot_id / bot_name + instruction）或传入 targets 数组并发委派多位成员同步作业，支持最多 2 层深度且自动防循环。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"bot_id": map[string]any{
							"type":        "string",
							"description": "单人委派时：目标成员 Bot ID（与 bot_name 二选一）",
						},
						"bot_name": map[string]any{
							"type":        "string",
							"description": "单人委派时：目标成员 Bot 名称",
						},
						"instruction": map[string]any{
							"type":        "string",
							"description": "单人委派时：清晰的执行指令与上下文",
						},
						"context_snippet": map[string]any{
							"type":        "string",
							"description": "可选：附带给协办成员的关键前置上下文片段，减少重复检索",
						},
						"targets": map[string]any{
							"type":        "array",
							"description": "可选：并发委派多位成员同步协办",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"bot_id":          map[string]any{"type": "string"},
									"bot_name":        map[string]any{"type": "string"},
									"instruction":     map[string]any{"type": "string"},
									"context_snippet": map[string]any{"type": "string"},
								},
								"required": []string{"instruction"},
							},
						},
					},
				},
			},
		},
		Timeout:     60 * time.Second,
		StatusLabel: "正在协同队友协办...",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolConsultBot(ctx, cleanArgs)
		},
	}
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
	resp, err := e.runSingleBotTurn(subCtx, subReq, target, target.ID, nil, delegationPrompt, nil)
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
