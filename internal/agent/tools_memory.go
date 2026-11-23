package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func rememberFactToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "remember_fact",
				Description: "将不可实时推导的高信噪比长期信息写入隔离记忆库（MEMORY.md）：优先记录纠偏纪律(feedback)、用户画像偏好(user)、长期项目决策(project)或外部参考指针(reference)；文中相对时间必须换算为绝对日期(YYYY-MM-DD)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类（推荐 Claude 4 类）：feedback（用户纠偏与工作纪律）/ user（用户画像与偏好）/ project（长期项目与决策）/ reference（外部参考与指针）；亦兼容旧分类 preference/relation/emotion/infra/schedule/insight/general",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "简短唯一的事实键名（同作用域下同名会覆盖更新），如 user_tea_habit / deploy_rule",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "要记住的具体事实内容",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要程度 1~5（默认 3，核心约定可设为 5）",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶为高优先常驻记忆",
						},
						"scope": map[string]any{
							"type":        "string",
							"description": "可选：'self'（默认，仅当前 Bot/群私有）或 'global'（显式跨全团队共享）",
						},
					},
					"required": []string{"fact_key", "content"},
				},
			},
		},
		StatusLabel: "正在整理长期记忆...",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolRememberFact(ctx, cleanArgs)
		},
	}
}

func recallMemoryToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "recall_memory",
				Description: "检索当前 Bot 或群聊专属作用域内的长期记忆（自动隔离其他 Bot 隐私记忆），支持关键词、分类和仅置顶过滤，并按相关性与权重排序。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "可选：检索关键词（支持空格分隔多关键词；留空则返回当前作用域核心记忆）",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "可选：按记忆分类过滤，如 preference / infra / schedule / insight / relation / emotion / general",
						},
						"pinned_only": map[string]any{
							"type":        "boolean",
							"description": "可选：设为 true 时仅返回置顶的高优先记忆",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "可选：返回最大条数（默认 20，最大 40）",
						},
					},
				},
			},
		},
		ReadOnly:    true,
		StatusLabel: "正在检索历史记忆...",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolRecallMemory(ctx, cleanArgs)
		},
	}
}

func manageMemoryToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_memory",
				Description: "对当前 Bot 或群聊作用域内的长期记忆进行整理：更正旧事实(upsert)、置顶或取消置顶(pin)、遗忘过时或错误条目(forget)、合并去重相似记忆(consolidate)、触发四阶段深度做梦整理(dream，回放近期对话消歧矛盾并绝对化日期)、或列出现有记忆键(list)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "upsert", "pin", "forget", "consolidate", "dream"},
							"description": "操作类型：list | upsert | pin | forget | consolidate | dream",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "目标记忆事实键名（upsert/pin/forget 时必填）",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "更新后的记忆内容（upsert 时必填）",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类（preference/relation/emotion/infra/schedule/insight/general）",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要度权重 1~5",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在整理长期记忆...",
		DomainKeywords: []string{"记忆", "记住", "忘掉", "修改记忆", "置顶"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageMemory(ctx, cleanArgs)
		},
	}
}

func resolveMemoryTargetScope(sc ToolScope, requestedScope string) string {
	targetScope := "global"
	if strings.TrimSpace(sc.TimelineBotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.TimelineBotID)
	} else if strings.TrimSpace(sc.BotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.BotID)
	}
	if strings.EqualFold(strings.TrimSpace(requestedScope), "global") && sc.MemoryPolicy != store.MemoryPolicyIsolated {
		targetScope = "global"
	}
	return targetScope
}

func clampMemoryImportance(imp int) int {
	if imp <= 0 {
		return 3
	}
	if imp > 5 {
		return 5
	}
	return imp
}

func (e *Engine) toolRememberFact(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Category   string `json:"category"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Importance int    `json:"importance"`
		Pinned     bool   `json:"pinned"`
		Scope      string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 remember_fact 参数失败: %w", err)
	}
	key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
	content := notifier.StripEmoji(strings.TrimSpace(args.Content))
	cat := notifier.StripEmoji(strings.TrimSpace(args.Category))
	if cat == "" {
		cat = "general"
	}

	sc := toolScopeFromContext(ctx)
	targetScope := resolveMemoryTargetScope(sc, args.Scope)
	imp := clampMemoryImportance(args.Importance)

	if err := e.store.SaveAgentMemoryDetailed(targetScope, cat, key, content, imp, args.Pinned); err != nil {
		return "", err
	}
	e.publishAssistantUpdate("memories")
	out, _ := json.Marshal(map[string]any{
		"ok":         true,
		"scope":      targetScope,
		"category":   cat,
		"fact_key":   key,
		"content":    content,
		"importance": imp,
		"pinned":     args.Pinned,
	})
	return string(out), nil
}

func (e *Engine) toolRecallMemory(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query      string `json:"query"`
		Category   string `json:"category"`
		PinnedOnly bool   `json:"pinned_only"`
		Limit      int    `json:"limit"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	limit := args.Limit
	if limit <= 0 || limit > 40 {
		limit = 20
	}

	sc := toolScopeFromContext(ctx)
	var scopes []string
	if sc.TimelineBotID != "" || sc.BotID != "" {
		seen := make(map[string]bool)
		addScope := func(s string) {
			if s != "" && !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
		if sc.TimelineBotID != "" {
			addScope("bot:" + sc.TimelineBotID)
		}
		if sc.BotID != "" {
			addScope("bot:" + sc.BotID)
		}
		if sc.MemoryPolicy != store.MemoryPolicyIsolated {
			addScope("global")
		}
	}

	memories, err := e.store.ListScopedMemories(scopes, strings.TrimSpace(args.Query), 80)
	if err != nil {
		return "", err
	}
	catFilter := strings.ToLower(strings.TrimSpace(args.Category))
	filtered := make([]store.AgentMemory, 0, len(memories))
	for _, m := range memories {
		if args.PinnedOnly && !m.Pinned {
			continue
		}
		if catFilter != "" && strings.ToLower(strings.TrimSpace(m.Category)) != catFilter {
			continue
		}
		filtered = append(filtered, m)
	}
	ranked := selectRelevantMemories(filtered, args.Query, limit)

	ids := make([]int64, 0, len(ranked))
	for _, m := range ranked {
		ids = append(ids, m.ID)
	}
	e.store.TouchRecalledMemories(ids)

	out, _ := json.Marshal(map[string]any{
		"count":    len(ranked),
		"scopes":   scopes,
		"memories": ranked,
	})
	return string(out), nil
}

func (e *Engine) toolManageMemory(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action     string `json:"action"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Importance int    `json:"importance"`
		Pinned     *bool  `json:"pinned"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_memory 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	targetScope := resolveMemoryTargetScope(sc, "")

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		mems, err := e.store.ListScopedMemories([]string{targetScope}, "", 40)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"scope":    targetScope,
			"count":    len(mems),
			"memories": mems,
		})
		return string(out), nil

	case "upsert":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		content := notifier.StripEmoji(strings.TrimSpace(args.Content))
		if key == "" || content == "" {
			return "", fmt.Errorf("upsert 记忆时 fact_key 与 content 不能为空")
		}
		cat := notifier.StripEmoji(strings.TrimSpace(args.Category))
		if cat == "" {
			cat = "general"
		}
		imp := clampMemoryImportance(args.Importance)
		pin := args.Pinned != nil && *args.Pinned
		if err := e.store.SaveAgentMemoryDetailed(targetScope, cat, key, content, imp, pin); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("memories")
		out, _ := json.Marshal(map[string]any{
			"ok":       true,
			"scope":    targetScope,
			"fact_key": key,
			"content":  content,
		})
		return string(out), nil

	case "pin":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		if key == "" {
			return "", fmt.Errorf("pin 记忆时 fact_key 不能为空")
		}
		normKey := normalizeDistilledFactKey(key)
		mems, err := e.store.ListScopedMemories([]string{targetScope}, "", 100)
		if err != nil {
			return "", err
		}
		for _, m := range mems {
			if strings.EqualFold(m.FactKey, key) || (normKey != "" && m.FactKey == normKey) {
				nextPin := !m.Pinned
				if args.Pinned != nil {
					nextPin = *args.Pinned
				}
				nextImp := m.Importance
				if args.Importance > 0 {
					nextImp = clampMemoryImportance(args.Importance)
				}
				if err := e.store.UpdateAgentMemory(m.ID, m.Scope, m.Category, m.FactKey, m.Content, nextImp, nextPin); err != nil {
					return "", err
				}
				e.publishAssistantUpdate("memories")
				out, _ := json.Marshal(map[string]any{"ok": true, "fact_key": m.FactKey, "pinned": nextPin})
				return string(out), nil
			}
		}
		return "", fmt.Errorf("未在当前作用域找到记忆键: %s", key)

	case "forget":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		if key == "" {
			return "", fmt.Errorf("forget 记忆时 fact_key 不能为空")
		}
		deleted, err := e.store.DeleteAgentMemoryByScopeKey(targetScope, key)
		if err != nil {
			return "", err
		}
		if deleted {
			e.publishAssistantUpdate("memories")
		}
		out, _ := json.Marshal(map[string]any{
			"ok":        deleted,
			"scope":     targetScope,
			"fact_key":  key,
			"forgotten": deleted,
		})
		return string(out), nil

	case "consolidate":
		merged, err := e.store.MergeDuplicateMemories(targetScope)
		if err != nil {
			return "", err
		}
		if merged > 0 {
			e.publishAssistantUpdate("memories")
		}
		out, _ := json.Marshal(map[string]any{
			"ok":     true,
			"scope":  targetScope,
			"merged": merged,
		})
		return string(out), nil

	case "dream":
		rep, err := e.RunAutoDream(ctx, targetScope, false)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":     true,
			"report": rep,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_memory action: %s", action)
	}
}
