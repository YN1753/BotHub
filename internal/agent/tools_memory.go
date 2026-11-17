package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

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

	default:
		return "", fmt.Errorf("不支持的 manage_memory action: %s", action)
	}
}
