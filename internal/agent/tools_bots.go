package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

func manageBotsToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_bots",
				Description: "管理 BotHub 工作区中的专属 Bot 与多 Bot 协同群聊（大管家核心能力）。支持 list（查看全部 Bot 与群聊名录）、create_bot（新建专属 Bot）、update_bot（修改已有 Bot 配置）、delete_bot（删除 Bot 或群聊）、create_group（将多位 Bot 拉入协同群聊，支持按名称或 ID 匹配成员）、add_group_members（拉人进群）、remove_group_members（移出群成员）、update_group（更新群聊配置/成员/公告）、setup_blueprint（一站式原子编排：创建/更新专属 Bot + 自动生成或挂载技能卡 + 创建配套定时任务，全程默认锁定免费池）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "create_bot", "update_bot", "delete_bot", "create_group", "add_group_members", "remove_group_members", "update_group", "setup_blueprint"},
							"description": "操作类型：list | create_bot | update_bot | delete_bot | create_group | add_group_members | remove_group_members | update_group | setup_blueprint",
						},
						"bot_id": map[string]any{
							"type":        "string",
							"description": "目标 Bot 或群聊 ID（update_bot / delete_bot / update_group / 加减群成员时使用，可与 bot_name / group_id / group_name 互替）",
						},
						"bot_name": map[string]any{
							"type":        "string",
							"description": "目标 Bot 名称（用于按名称查找已有 Bot）",
						},
						"group_id": map[string]any{
							"type":        "string",
							"description": "目标群聊 ID（add_group_members / remove_group_members / update_group 时使用）",
						},
						"group_name": map[string]any{
							"type":        "string",
							"description": "目标群聊名称（用于按名称查找已有群聊）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "新建或修改后的 Bot 名称 / 群聊名称",
						},
						"role_title": map[string]any{
							"type":        "string",
							"description": "Bot 职责定位或群聊议题说明",
						},
						"avatar_style": map[string]any{
							"type":        "string",
							"description": "可选外观配色：butler:gold | sentinel:indigo | radar:emerald | swarm:violet | scholar:cyan | critic:rose",
						},
						"bot_mode": map[string]any{
							"type":        "string",
							"enum":        []string{"companion", "hybrid", "worker"},
							"description": "Bot 交流模式：companion（伴聊）| hybrid（综合，默认）| worker（专业执行）",
						},
						"memory_policy": map[string]any{
							"type":        "string",
							"enum":        []string{"isolated", "standard"},
							"description": "记忆隔离策略：isolated（严格私有，默认）| standard（私有 + 全局共享）",
						},
						"traits": map[string]any{
							"type":        "string",
							"description": "可选：性格特质与说话口吻",
						},
						"system_prompt": map[string]any{
							"type":        "string",
							"description": "可选：专属系统提示词与行为准则（新建未传时自动根据定位生成）",
						},
						"model": map[string]any{
							"type":        "string",
							"description": "可选：默认推理模型（默认 auto:free，也支持 auto:paid 或 pool:<id>[::<model>]）",
						},
						"allow_paid_fallback": map[string]any{
							"type":        "boolean",
							"description": "可选：是否允许该 Bot 或群聊在免费模型池不可用时回退付费池（默认 false）",
						},
						"enable_thinking": map[string]any{
							"type":        "boolean",
							"description": "可选：是否开启深度思考（companion 模式默认 false，其他默认 true）",
						},
						"allowed_tools": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "可选：授权内置工具白名单数组；也支持快捷预设 [\"all\"] / [\"assistant\"] / [\"companion\"] / [\"minimal\"]",
						},
						"is_pinned": map[string]any{
							"type":        "boolean",
							"description": "可选：是否置顶该会话",
						},
						"member_ids": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "群聊成员 Bot ID 列表（create_group / add_group_members / remove_group_members / update_group 使用）",
						},
						"member_names": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "群聊成员 Bot 名称列表（可按名字自动解析匹配现有 Bot，create_group / add_group_members / remove_group_members 使用）",
						},
						"include_self": map[string]any{
							"type":        "boolean",
							"description": "create_group 时可选：是否把当前大管家自己也拉入该群聊（默认 false）",
						},
						"group_mode": map[string]any{
							"type":        "string",
							"enum":        []string{"smart", "sequential", "parallel", "mention_only"},
							"description": "群协同策略：smart（智能接力，默认）| sequential（顺序轮答）| parallel（并发回复）| mention_only（仅 @ 点名）",
						},
						"announcement": map[string]any{
							"type":        "string",
							"description": "可选：群公告内容",
						},
						"skill_ids": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "可选：要挂载的已有技能 ID 或 slug 列表（setup_blueprint / create_bot 使用）",
						},
						"skill_specs": map[string]any{
							"type":        "array",
							"description": "setup_blueprint 时可选：要同步创建或更新的自定义技能定义列表（每项含 name, slug, kind, description, instruction, http_method, http_url）",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"slug":        map[string]any{"type": "string"},
									"name":        map[string]any{"type": "string"},
									"kind":        map[string]any{"type": "string"},
									"description": map[string]any{"type": "string"},
									"instruction": map[string]any{"type": "string"},
									"http_method": map[string]any{"type": "string"},
									"http_url":    map[string]any{"type": "string"},
								},
							},
						},
						"task_spec": map[string]any{
							"type":        "object",
							"description": "setup_blueprint 时可选：配套定时任务定义（name, type, cron_expr, run_at, prompt, model, allow_paid_fallback, max_paid_calls_per_day）",
							"properties": map[string]any{
								"name":                   map[string]any{"type": "string"},
								"type":                   map[string]any{"type": "string"},
								"cron_expr":              map[string]any{"type": "string"},
								"run_at":                 map[string]any{"type": "string"},
								"prompt":                 map[string]any{"type": "string"},
								"model":                  map[string]any{"type": "string"},
								"allow_paid_fallback":    map[string]any{"type": "boolean"},
								"max_paid_calls_per_day": map[string]any{"type": "integer"},
							},
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在编排工作区 Bot 与群聊...",
		DomainKeywords: []string{"创建bot", "新建bot", "建一个bot", "拉群", "建群", "群聊", "加人", "踢人", "成员", "管家"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageBots(ctx, cleanArgs)
		},
	}
}

func genManagedBotID(isGroup bool) string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	prefix := "bot_"
	if isGroup {
		prefix = "group_"
	}
	return prefix + hex.EncodeToString(b)
}

func normalizeManageAvatarStyle(raw, botMode string, isGroup bool) string {
	valid := map[string]bool{
		"butler:gold":     true,
		"sentinel:indigo": true,
		"radar:emerald":   true,
		"swarm:violet":    true,
		"scholar:cyan":    true,
		"critic:rose":     true,
	}
	s := strings.TrimSpace(raw)
	if valid[s] {
		return s
	}
	if isGroup {
		return "swarm:violet"
	}
	switch botMode {
	case store.BotModeCompanion:
		return "critic:rose"
	case store.BotModeWorker:
		return "scholar:cyan"
	default:
		return "radar:emerald"
	}
}

func normalizeManageGroupMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sequential", "parallel", "mention_only":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "smart"
	}
}

func resolveAllowedToolsForNewBot(raw []string, botMode string) []string {
	assistantPreset := []string{
		"manage_todos", "manage_notes", "manage_stash", "manage_tasks",
		"workspace_briefing", "send_notification", "web_search", "read_url",
		"calculate_eval", "datetime_calc", "remember_fact", "recall_memory",
		"manage_memory", "consult_bot",
	}
	companionPreset := []string{"remember_fact", "recall_memory"}
	workerPreset := []string{
		"web_search", "read_url", "calculate_eval", "datetime_calc",
		"manage_notes", "manage_todos", "manage_stash",
		"remember_fact", "recall_memory", "manage_memory",
	}

	if len(raw) == 0 {
		switch botMode {
		case store.BotModeCompanion:
			return companionPreset
		case store.BotModeWorker:
			return workerPreset
		default:
			return assistantPreset
		}
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(raw))
	addOne := func(id string) {
		clean := strings.TrimSpace(id)
		if clean == "" || seen[clean] {
			return
		}
		if _, ok := LookupToolSpec(clean); ok {
			seen[clean] = true
			out = append(out, clean)
		}
	}

	for _, item := range raw {
		switch strings.ToLower(strings.TrimSpace(item)) {
		case "all":
			for _, td := range BuiltinTools() {
				addOne(td.Function.Name)
			}
		case "assistant":
			for _, id := range assistantPreset {
				addOne(id)
			}
		case "companion":
			for _, id := range companionPreset {
				addOne(id)
			}
		case "worker":
			for _, id := range workerPreset {
				addOne(id)
			}
		case "minimal", "none":
			// leave empty unless other tools explicitly included
		default:
			addOne(item)
		}
	}
	return out
}

func findBotByIDOrName(allBots []store.Bot, wantID, wantName string, onlyGroup *bool) *store.Bot {
	id := strings.TrimSpace(wantID)
	name := strings.TrimSpace(wantName)

	matchesKind := func(b store.Bot) bool {
		if onlyGroup == nil {
			return true
		}
		return b.IsGroup == *onlyGroup
	}

	if id != "" {
		for i := range allBots {
			if allBots[i].ID == id && matchesKind(allBots[i]) {
				return &allBots[i]
			}
		}
	}
	if name != "" {
		for i := range allBots {
			if strings.EqualFold(allBots[i].Name, name) && matchesKind(allBots[i]) {
				return &allBots[i]
			}
		}
		lower := strings.ToLower(name)
		var partial *store.Bot
		matches := 0
		for i := range allBots {
			if !matchesKind(allBots[i]) {
				continue
			}
			if strings.Contains(strings.ToLower(allBots[i].Name), lower) || strings.Contains(strings.ToLower(allBots[i].ID), lower) {
				partial = &allBots[i]
				matches++
			}
		}
		if matches == 1 {
			return partial
		}
	}
	return nil
}

func resolveMemberBots(allBots []store.Bot, memberIDs, memberNames []string, includeSelf bool, selfBotID string) ([]store.Bot, []string) {
	isNotGroup := false
	seen := make(map[string]bool)
	resolved := make([]store.Bot, 0, len(memberIDs)+len(memberNames)+1)
	unresolved := make([]string, 0)

	addBot := func(b *store.Bot) {
		if b == nil || b.IsGroup || seen[b.ID] {
			return
		}
		seen[b.ID] = true
		resolved = append(resolved, *b)
	}

	if includeSelf && strings.TrimSpace(selfBotID) != "" {
		if b := findBotByIDOrName(allBots, selfBotID, "", &isNotGroup); b != nil {
			addBot(b)
		}
	}
	for _, rawID := range memberIDs {
		trimmed := strings.TrimSpace(rawID)
		if trimmed == "" {
			continue
		}
		if b := findBotByIDOrName(allBots, trimmed, trimmed, &isNotGroup); b != nil {
			addBot(b)
		} else {
			unresolved = append(unresolved, trimmed)
		}
	}
	for _, rawName := range memberNames {
		trimmed := strings.TrimSpace(rawName)
		if trimmed == "" {
			continue
		}
		if b := findBotByIDOrName(allBots, trimmed, trimmed, &isNotGroup); b != nil {
			addBot(b)
		} else {
			unresolved = append(unresolved, trimmed)
		}
	}
	return resolved, unresolved
}

func deriveGroupCapabilities(members []store.Bot) (allowedToolsJSON, skillIDsJSON, botMode string, enableThinking bool) {
	toolSet := make(map[string]bool)
	skillSet := make(map[string]bool)
	allCompanion := len(members) > 0
	for _, m := range members {
		if store.NormalizeBotMode(m.BotMode) != store.BotModeCompanion {
			allCompanion = false
		}
		for _, t := range m.AllowedToolList() {
			if strings.TrimSpace(t) != "" {
				toolSet[strings.TrimSpace(t)] = true
			}
		}
		for _, sk := range m.SkillIDList() {
			if strings.TrimSpace(sk) != "" {
				skillSet[strings.TrimSpace(sk)] = true
			}
		}
	}
	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}
	skills := make([]string, 0, len(skillSet))
	for sk := range skillSet {
		skills = append(skills, sk)
	}
	tb, _ := json.Marshal(tools)
	sb, _ := json.Marshal(skills)
	mode := store.BotModeHybrid
	if allCompanion {
		mode = store.BotModeCompanion
	}
	return string(tb), string(sb), mode, !allCompanion
}

func (e *Engine) publishBotThreadEvent(botID, action string) {
	if e != nil && e.hub != nil {
		e.hub.PublishEvent("thread_updated", map[string]any{
			"bot_id": botID,
			"action": action,
			"time":   store.NowShanghai(),
		})
	}
}

func (e *Engine) toolManageBots(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action            string   `json:"action"`
		BotID             string   `json:"bot_id"`
		BotName           string   `json:"bot_name"`
		GroupID           string   `json:"group_id"`
		GroupName         string   `json:"group_name"`
		Name              string   `json:"name"`
		RoleTitle         string   `json:"role_title"`
		AvatarStyle       string   `json:"avatar_style"`
		BotMode           string   `json:"bot_mode"`
		MemoryPolicy      string   `json:"memory_policy"`
		Traits            string   `json:"traits"`
		SystemPrompt      string   `json:"system_prompt"`
		Model             string   `json:"model"`
		AllowPaidFallback *bool    `json:"allow_paid_fallback"`
		EnableThinking    *bool    `json:"enable_thinking"`
		AllowedTools      []string `json:"allowed_tools"`
		IsPinned          *bool    `json:"is_pinned"`
		MemberIDs         []string `json:"member_ids"`
		MemberNames       []string `json:"member_names"`
		IncludeSelf       bool     `json:"include_self"`
		GroupMode         string   `json:"group_mode"`
		Announcement      string   `json:"announcement"`
		SkillIDs          []string `json:"skill_ids"`
		SkillSpecs        []skillUpsertInput `json:"skill_specs"`
		TaskSpec          *struct {
			Name               string `json:"name"`
			Type               string `json:"type"`
			CronExpr           string `json:"cron_expr"`
			RunAt              string `json:"run_at"`
			Prompt             string `json:"prompt"`
			Model              string `json:"model"`
			AllowPaidFallback  *bool  `json:"allow_paid_fallback"`
			MaxPaidCallsPerDay int    `json:"max_paid_calls_per_day"`
		} `json:"task_spec"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_bots 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	allBots, err := e.store.ListBots()
	if err != nil {
		return "", fmt.Errorf("读取 Bot 列表失败: %w", err)
	}
	botByID := make(map[string]store.Bot, len(allBots))
	for _, b := range allBots {
		botByID[b.ID] = b
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		items := make([]map[string]any, 0, len(allBots))
		for _, b := range allBots {
			mNames := make([]string, 0)
			if b.IsGroup {
				for _, mid := range b.MemberIDList() {
					if mb, ok := botByID[mid]; ok {
						mNames = append(mNames, mb.Name)
					} else {
						mNames = append(mNames, mid)
					}
				}
			}
			items = append(items, map[string]any{
				"id":                  b.ID,
				"name":                b.Name,
				"role_title":          b.RoleTitle,
				"is_group":            b.IsGroup,
				"bot_mode":            b.BotMode,
				"group_mode":          b.GroupMode,
				"member_ids":          b.MemberIDList(),
				"member_names":        mNames,
				"model":               b.Model,
				"allow_paid_fallback": b.AllowPaidFallback,
				"is_pinned":           b.IsPinned,
				"allowed_tools_count": len(b.AllowedToolList()),
			})
		}
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"count": len(items),
			"bots":  items,
		})
		return string(out), nil

	case "create_bot":
		name := notifier.StripEmoji(strings.TrimSpace(args.Name))
		if name == "" {
			name = notifier.StripEmoji(strings.TrimSpace(args.BotName))
		}
		if name == "" {
			return "", fmt.Errorf("create_bot 需要提供 name（Bot 名称）")
		}
		botMode := store.NormalizeBotMode(args.BotMode)
		if strings.TrimSpace(args.BotMode) == "" {
			botMode = store.BotModeHybrid
		}
		memPolicy := store.MemoryPolicyIsolated
		if strings.TrimSpace(args.MemoryPolicy) != "" {
			memPolicy = store.NormalizeMemoryPolicy(args.MemoryPolicy)
		}
		roleTitle := notifier.StripEmoji(strings.TrimSpace(args.RoleTitle))
		if roleTitle == "" {
			if botMode == store.BotModeCompanion {
				roleTitle = "专属生活伙伴"
			} else {
				roleTitle = "专属智能助理"
			}
		}
		traits := notifier.StripEmoji(strings.TrimSpace(args.Traits))
		sysPrompt := notifier.StripEmoji(strings.TrimSpace(args.SystemPrompt))
		if sysPrompt == "" {
			sysPrompt = fmt.Sprintf("你是用户的专属 Bot「%s」（%s）。%s保持真实准确，严禁使用 Emoji。",
				name, roleTitle, func() string {
					if traits != "" {
						return "性格与说话风格：" + traits + "。"
					}
					return ""
				}())
		}

		id := strings.TrimSpace(args.BotID)
		if id == "" || botByID[id].ID != "" {
			id = genManagedBotID(false)
		}
		model := strings.TrimSpace(args.Model)
		if model == "" {
			model = "auto:free"
		}
		allowPaid := false
		if args.AllowPaidFallback != nil {
			allowPaid = *args.AllowPaidFallback
		}
		enableThinking := botMode != store.BotModeCompanion
		if args.EnableThinking != nil {
			enableThinking = *args.EnableThinking
		}
		isPinned := false
		if args.IsPinned != nil {
			isPinned = *args.IsPinned
		}
		tools := resolveAllowedToolsForNewBot(args.AllowedTools, botMode)
		toolsJSON, _ := json.Marshal(tools)

		newBot := store.Bot{
			ID:                id,
			Name:              name,
			RoleTitle:         roleTitle,
			AvatarStyle:       normalizeManageAvatarStyle(args.AvatarStyle, botMode, false),
			SystemPrompt:      sysPrompt,
			BotMode:           botMode,
			MemoryPolicy:      memPolicy,
			Traits:            traits,
			Model:             model,
			AllowPaidFallback: allowPaid,
			EnableThinking:    enableThinking,
			AllowedTools:      string(toolsJSON),
			SkillIDs:          "[]",
			State:             store.BotStateIdle,
			IsPinned:          isPinned,
			IsGroup:           false,
			MemberIDs:         "[]",
			GroupMode:         "smart",
			SortOrder:         100,
		}
		if err := e.store.SaveBot(newBot); err != nil {
			return "", fmt.Errorf("创建 Bot 失败: %w", err)
		}
		e.publishAssistantUpdate("bots")
		out, _ := json.Marshal(map[string]any{
			"ok":            true,
			"action":        "create_bot",
			"bot_id":        newBot.ID,
			"name":          newBot.Name,
			"role_title":    newBot.RoleTitle,
			"bot_mode":      newBot.BotMode,
			"model":         newBot.Model,
			"allowed_tools": tools,
		})
		return string(out), nil

	case "update_bot", "update_group":
		wantID := strings.TrimSpace(args.BotID)
		if wantID == "" {
			wantID = strings.TrimSpace(args.GroupID)
		}
		wantName := strings.TrimSpace(args.BotName)
		if wantName == "" {
			wantName = strings.TrimSpace(args.GroupName)
		}
		if wantName == "" && wantID == "" {
			wantName = strings.TrimSpace(args.Name)
		}
		var onlyGroup *bool
		if action == "update_group" {
			v := true
			onlyGroup = &v
		}
		target := findBotByIDOrName(allBots, wantID, wantName, onlyGroup)
		if target == nil {
			return "", fmt.Errorf("未找到要更新的目标 Bot 或群聊 (id=%q, name=%q)", wantID, wantName)
		}
		updated := *target
		if v := notifier.StripEmoji(strings.TrimSpace(args.Name)); v != "" {
			updated.Name = v
		}
		if v := notifier.StripEmoji(strings.TrimSpace(args.RoleTitle)); v != "" {
			updated.RoleTitle = v
		}
		if strings.TrimSpace(args.AvatarStyle) != "" {
			updated.AvatarStyle = normalizeManageAvatarStyle(args.AvatarStyle, updated.BotMode, updated.IsGroup)
		}
		if strings.TrimSpace(args.BotMode) != "" {
			updated.BotMode = store.NormalizeBotMode(args.BotMode)
		}
		if strings.TrimSpace(args.MemoryPolicy) != "" {
			updated.MemoryPolicy = store.NormalizeMemoryPolicy(args.MemoryPolicy)
		}
		if v := notifier.StripEmoji(strings.TrimSpace(args.Traits)); v != "" {
			updated.Traits = v
		}
		if v := notifier.StripEmoji(strings.TrimSpace(args.SystemPrompt)); v != "" {
			updated.SystemPrompt = v
		}
		if strings.TrimSpace(args.Model) != "" {
			updated.Model = strings.TrimSpace(args.Model)
		}
		if args.AllowPaidFallback != nil {
			updated.AllowPaidFallback = *args.AllowPaidFallback
		}
		if args.EnableThinking != nil {
			updated.EnableThinking = *args.EnableThinking
		}
		if args.IsPinned != nil {
			updated.IsPinned = *args.IsPinned
		}
		if len(args.AllowedTools) > 0 {
			tools := resolveAllowedToolsForNewBot(args.AllowedTools, updated.BotMode)
			tb, _ := json.Marshal(tools)
			updated.AllowedTools = string(tb)
		}
		if updated.IsGroup {
			if strings.TrimSpace(args.GroupMode) != "" {
				updated.GroupMode = normalizeManageGroupMode(args.GroupMode)
			}
			if strings.TrimSpace(args.Announcement) != "" {
				updated.Announcement = notifier.StripEmoji(strings.TrimSpace(args.Announcement))
			}
			if len(args.MemberIDs) > 0 || len(args.MemberNames) > 0 {
				members, unresolved := resolveMemberBots(allBots, args.MemberIDs, args.MemberNames, args.IncludeSelf, sc.BotID)
				if len(members) == 0 {
					return "", fmt.Errorf("未匹配到有效群成员 (未识别: %v)", unresolved)
				}
				mIDs := make([]string, 0, len(members))
				for _, m := range members {
					mIDs = append(mIDs, m.ID)
				}
				mb, _ := json.Marshal(mIDs)
				updated.MemberIDs = string(mb)
				toolsJSON, skillsJSON, _, _ := deriveGroupCapabilities(members)
				updated.AllowedTools = toolsJSON
				updated.SkillIDs = skillsJSON
			}
		}
		if err := e.store.SaveBot(updated); err != nil {
			return "", fmt.Errorf("更新 Bot 失败: %w", err)
		}
		e.publishAssistantUpdate("bots")
		if updated.IsGroup {
			e.publishBotThreadEvent(updated.ID, "updated")
		}
		out, _ := json.Marshal(map[string]any{
			"ok":         true,
			"action":     action,
			"bot_id":     updated.ID,
			"name":       updated.Name,
			"is_group":   updated.IsGroup,
			"model":      updated.Model,
			"group_mode": updated.GroupMode,
		})
		return string(out), nil

	case "delete_bot":
		wantID := strings.TrimSpace(args.BotID)
		if wantID == "" {
			wantID = strings.TrimSpace(args.GroupID)
		}
		wantName := strings.TrimSpace(args.BotName)
		if wantName == "" {
			wantName = strings.TrimSpace(args.GroupName)
		}
		if wantName == "" {
			wantName = strings.TrimSpace(args.Name)
		}
		target := findBotByIDOrName(allBots, wantID, wantName, nil)
		if target == nil {
			return "", fmt.Errorf("未找到要删除的 Bot 或群聊 (id=%q, name=%q)", wantID, wantName)
		}
		if target.ID == store.DefaultConciergeBotID || target.ID == sc.BotID {
			return "", fmt.Errorf("为防止误删主控管家，请直接在界面中操作删除「%s」", target.Name)
		}
		if err := e.store.DeleteBot(target.ID); err != nil {
			return "", fmt.Errorf("删除失败: %w", err)
		}
		e.publishAssistantUpdate("bots")
		out, _ := json.Marshal(map[string]any{
			"ok":       true,
			"action":   "delete_bot",
			"deleted":  target.ID,
			"name":     target.Name,
			"is_group": target.IsGroup,
		})
		return string(out), nil

	case "create_group":
		members, unresolved := resolveMemberBots(allBots, args.MemberIDs, args.MemberNames, args.IncludeSelf, sc.BotID)
		if len(members) == 0 {
			existingNames := make([]string, 0)
			for _, b := range allBots {
				if !b.IsGroup {
					existingNames = append(existingNames, fmt.Sprintf("%s(%s)", b.Name, b.ID))
				}
			}
			return "", fmt.Errorf("拉群至少需要匹配 1 位已有非群聊 Bot 成员（未匹配：%v；当前可选成员：%s）。若需新角色请先调用 create_bot 创建", unresolved, strings.Join(existingNames, "、"))
		}

		memberIDs := make([]string, 0, len(members))
		memberNames := make([]string, 0, len(members))
		for _, m := range members {
			memberIDs = append(memberIDs, m.ID)
			memberNames = append(memberNames, m.Name)
		}

		groupName := notifier.StripEmoji(strings.TrimSpace(args.Name))
		if groupName == "" {
			groupName = notifier.StripEmoji(strings.TrimSpace(args.GroupName))
		}
		if groupName == "" {
			if len(memberNames) <= 2 {
				groupName = strings.Join(memberNames, " · ") + " 协同群"
			} else {
				groupName = fmt.Sprintf("%s 等 %d 人群聊", strings.Join(memberNames[:2], "、"), len(memberNames))
			}
		}

		roleTitle := notifier.StripEmoji(strings.TrimSpace(args.RoleTitle))
		if roleTitle == "" {
			roleTitle = fmt.Sprintf("多 Bot 协同群 (%d 成员)", len(members))
		}
		groupMode := normalizeManageGroupMode(args.GroupMode)
		toolsJSON, skillsJSON, derivedMode, derivedThink := deriveGroupCapabilities(members)
		if args.EnableThinking != nil {
			derivedThink = *args.EnableThinking
		}
		model := strings.TrimSpace(args.Model)
		if model == "" {
			model = "auto:free"
		}
		allowPaid := false
		if args.AllowPaidFallback != nil {
			allowPaid = *args.AllowPaidFallback
		}
		isPinned := false
		if args.IsPinned != nil {
			isPinned = *args.IsPinned
		}
		announcement := notifier.StripEmoji(strings.TrimSpace(args.Announcement))
		sysPrompt := notifier.StripEmoji(strings.TrimSpace(args.SystemPrompt))
		if sysPrompt == "" {
			sysPrompt = "根据群内成员性格与职责自然交流协同，避免重复客套，严禁使用 Emoji。"
		}

		memberIDsJSON, _ := json.Marshal(memberIDs)
		groupBot := store.Bot{
			ID:                genManagedBotID(true),
			Name:              groupName,
			RoleTitle:         roleTitle,
			AvatarStyle:       normalizeManageAvatarStyle(args.AvatarStyle, derivedMode, true),
			SystemPrompt:      sysPrompt,
			BotMode:           derivedMode,
			MemoryPolicy:      store.MemoryPolicyIsolated,
			Model:             model,
			AllowPaidFallback: allowPaid,
			EnableThinking:    derivedThink,
			AllowedTools:      toolsJSON,
			SkillIDs:          skillsJSON,
			State:             store.BotStateIdle,
			IsPinned:          isPinned,
			IsGroup:           true,
			MemberIDs:         string(memberIDsJSON),
			GroupMode:         groupMode,
			Announcement:      announcement,
			SortOrder:         50,
			LastSummary:       "群聊已组建，成员：" + strings.Join(memberNames, "、"),
		}
		if err := e.store.SaveBot(groupBot); err != nil {
			return "", fmt.Errorf("创建群聊失败: %w", err)
		}
		_, _ = e.store.AppendAgentMessage(store.AgentMessage{
			SessionID: "bot:" + groupBot.ID,
			BotID:     groupBot.ID,
			MsgKind:   "system",
			Role:      "system",
			Content:   fmt.Sprintf("已组建群聊「%s」，当前成员：%s", groupBot.Name, strings.Join(memberNames, "、")),
		})
		e.publishAssistantUpdate("bots")
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"action":       "create_group",
			"group_id":     groupBot.ID,
			"name":         groupBot.Name,
			"group_mode":   groupBot.GroupMode,
			"member_ids":   memberIDs,
			"member_names": memberNames,
			"unresolved":   unresolved,
		})
		return string(out), nil

	case "add_group_members", "remove_group_members":
		onlyGroup := true
		wantID := strings.TrimSpace(args.GroupID)
		if wantID == "" {
			wantID = strings.TrimSpace(args.BotID)
		}
		wantName := strings.TrimSpace(args.GroupName)
		if wantName == "" {
			wantName = strings.TrimSpace(args.BotName)
		}
		if wantName == "" {
			wantName = strings.TrimSpace(args.Name)
		}
		group := findBotByIDOrName(allBots, wantID, wantName, &onlyGroup)
		if group == nil {
			return "", fmt.Errorf("未找到目标群聊 (group_id=%q, group_name=%q)", wantID, wantName)
		}
		deltaMembers, unresolved := resolveMemberBots(allBots, args.MemberIDs, args.MemberNames, args.IncludeSelf, sc.BotID)
		if len(deltaMembers) == 0 {
			return "", fmt.Errorf("未匹配到要调整的成员 Bot (未识别: %v)", unresolved)
		}

		curIDs := group.MemberIDList()
		curSet := make(map[string]bool, len(curIDs))
		for _, id := range curIDs {
			curSet[id] = true
		}

		var nextIDs []string
		changedNames := make([]string, 0, len(deltaMembers))
		if action == "add_group_members" {
			nextIDs = append(nextIDs, curIDs...)
			for _, m := range deltaMembers {
				if !curSet[m.ID] {
					curSet[m.ID] = true
					nextIDs = append(nextIDs, m.ID)
					changedNames = append(changedNames, m.Name)
				}
			}
		} else {
			rmSet := make(map[string]bool, len(deltaMembers))
			for _, m := range deltaMembers {
				rmSet[m.ID] = true
			}
			for _, id := range curIDs {
				if rmSet[id] {
					if b, ok := botByID[id]; ok {
						changedNames = append(changedNames, b.Name)
					} else {
						changedNames = append(changedNames, id)
					}
					continue
				}
				nextIDs = append(nextIDs, id)
			}
			if len(nextIDs) == 0 {
				return "", fmt.Errorf("群聊至少需要保留 1 位成员，无法全部移出；如需解散请调用 delete_bot")
			}
		}

		nextMembers := make([]store.Bot, 0, len(nextIDs))
		nextNames := make([]string, 0, len(nextIDs))
		for _, id := range nextIDs {
			if b, ok := botByID[id]; ok {
				nextMembers = append(nextMembers, b)
				nextNames = append(nextNames, b.Name)
			}
		}
		mb, _ := json.Marshal(nextIDs)
		updated := *group
		updated.MemberIDs = string(mb)
		toolsJSON, skillsJSON, _, _ := deriveGroupCapabilities(nextMembers)
		updated.AllowedTools = toolsJSON
		updated.SkillIDs = skillsJSON
		if err := e.store.SaveBot(updated); err != nil {
			return "", fmt.Errorf("更新群成员失败: %w", err)
		}

		if len(changedNames) > 0 {
			verb := "邀请了"
			suffix := "加入群聊"
			if action == "remove_group_members" {
				verb = "将"
				suffix = "移出了群聊"
			}
			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID: "bot:" + updated.ID,
				BotID:     updated.ID,
				MsgKind:   "system",
				Role:      "system",
				Content:   fmt.Sprintf("已%s %s %s", verb, strings.Join(changedNames, "、"), suffix),
			})
		}
		e.publishAssistantUpdate("bots")
		e.publishBotThreadEvent(updated.ID, "members_updated")
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"action":       action,
			"group_id":     updated.ID,
			"group_name":   updated.Name,
			"changed":      changedNames,
			"member_ids":   nextIDs,
			"member_names": nextNames,
			"unresolved":   unresolved,
		})
		return string(out), nil

	case "setup_blueprint":
		botName := notifier.StripEmoji(strings.TrimSpace(args.Name))
		if botName == "" {
			botName = notifier.StripEmoji(strings.TrimSpace(args.BotName))
		}
		if botName == "" && strings.TrimSpace(args.BotID) == "" {
			return "", fmt.Errorf("setup_blueprint 需要提供 name（Bot 名称）或 bot_id")
		}

		// 1. Upsert skill_specs + resolve existing skill_ids/slugs
		boundSkillSet := make(map[string]bool)
		boundSkillIDs := make([]string, 0, len(args.SkillSpecs)+len(args.SkillIDs))
		boundSkills := make([]store.Skill, 0, len(args.SkillSpecs))
		addSkillID := func(id string) {
			if id != "" && !boundSkillSet[id] {
				boundSkillSet[id] = true
				boundSkillIDs = append(boundSkillIDs, id)
			}
		}
		for _, spec := range args.SkillSpecs {
			sk, sErr := e.upsertSkillFromParams(spec)
			if sErr != nil {
				return "", fmt.Errorf("创建套装技能失败: %w", sErr)
			}
			addSkillID(sk.ID)
			boundSkills = append(boundSkills, *sk)
		}
		for _, rawSk := range args.SkillIDs {
			if found := e.findSkillByMatch(rawSk, rawSk, rawSk); found != nil {
				addSkillID(found.ID)
			}
		}

		// 2. Create or update Bot (strictly default allow_paid_fallback = false)
		var onlySingle bool
		existingBot := findBotByIDOrName(allBots, args.BotID, botName, &onlySingle)
		botMode := store.NormalizeBotMode(args.BotMode)
		if strings.TrimSpace(args.BotMode) == "" {
			botMode = store.BotModeHybrid
		}
		var targetBot store.Bot
		if existingBot != nil {
			targetBot = *existingBot
			for _, curSk := range existingBot.SkillIDList() {
				addSkillID(curSk)
			}
			if len(args.AllowedTools) > 0 {
				tList := resolveAllowedToolsForNewBot(args.AllowedTools, targetBot.BotMode)
				tb, _ := json.Marshal(tList)
				targetBot.AllowedTools = string(tb)
			}
		} else {
			roleTitle := notifier.StripEmoji(strings.TrimSpace(args.RoleTitle))
			if roleTitle == "" {
				roleTitle = "专属自动化助手"
			}
			traits := notifier.StripEmoji(strings.TrimSpace(args.Traits))
			sysPrompt := notifier.StripEmoji(strings.TrimSpace(args.SystemPrompt))
			if sysPrompt == "" {
				sysPrompt = fmt.Sprintf("你是用户的专属 Bot「%s」（%s）。优先调动工具与技能核查真实数据并直接落实任务，严禁使用 Emoji。", botName, roleTitle)
			}
			tools := resolveAllowedToolsForNewBot(args.AllowedTools, botMode)
			tb, _ := json.Marshal(tools)
			model := strings.TrimSpace(args.Model)
			if model == "" {
				model = "auto:free"
			}
			allowPaid := false
			if args.AllowPaidFallback != nil {
				allowPaid = *args.AllowPaidFallback
			}
			enableThinking := botMode != store.BotModeCompanion
			if args.EnableThinking != nil {
				enableThinking = *args.EnableThinking
			}
			targetBot = store.Bot{
				ID:                genManagedBotID(false),
				Name:              botName,
				RoleTitle:         roleTitle,
				AvatarStyle:       normalizeManageAvatarStyle(args.AvatarStyle, botMode, false),
				SystemPrompt:      sysPrompt,
				BotMode:           botMode,
				MemoryPolicy:      store.MemoryPolicyIsolated,
				Traits:            traits,
				Model:             model,
				AllowPaidFallback: allowPaid,
				EnableThinking:    enableThinking,
				AllowedTools:      string(tb),
				State:             store.BotStateIdle,
				IsPinned:          args.IsPinned != nil && *args.IsPinned,
				IsGroup:           false,
				MemberIDs:         "[]",
				GroupMode:         "smart",
				SortOrder:         100,
			}
		}
		if args.AllowPaidFallback != nil {
			targetBot.AllowPaidFallback = *args.AllowPaidFallback
		}
		skBytes, _ := json.Marshal(boundSkillIDs)
		targetBot.SkillIDs = string(skBytes)
		if err := e.store.SaveBot(targetBot); err != nil {
			return "", fmt.Errorf("保存套装 Bot 失败: %w", err)
		}

		// 3. Optionally create bound scheduled task (strictly default allow_paid_fallback = false)
		var createdTask *store.Task
		if args.TaskSpec != nil && strings.TrimSpace(args.TaskSpec.Name) != "" {
			tType := strings.TrimSpace(args.TaskSpec.Type)
			if tType == "" {
				tType = "bot_prompt"
			}
			tCron := strings.TrimSpace(args.TaskSpec.CronExpr)
			tRunAt := strings.TrimSpace(args.TaskSpec.RunAt)
			if tRunAt != "" {
				if _, pErr := scheduler.ParseRunAt(tRunAt, nil); pErr != nil {
					return "", fmt.Errorf("task_spec.run_at 格式错误: %w", pErr)
				}
				tCron = ""
			} else {
				if tCron == "" {
					tCron = "0 9 * * *"
				}
				if _, cErr := cron.ParseStandard(tCron); cErr != nil {
					return "", fmt.Errorf("task_spec.cron_expr 无效: %w", cErr)
				}
			}
			paramsJSON := "{}"
			cleanPrompt := notifier.StripEmoji(strings.TrimSpace(args.TaskSpec.Prompt))
			if cleanPrompt != "" {
				pb, _ := json.Marshal(map[string]string{"prompt": cleanPrompt})
				paramsJSON = string(pb)
			}
			tAllowPaid := false
			if args.TaskSpec.AllowPaidFallback != nil {
				tAllowPaid = *args.TaskSpec.AllowPaidFallback
			}
			buf := make([]byte, 4)
			_, _ = rand.Read(buf)
			task := store.Task{
				ID:                 "task_" + hex.EncodeToString(buf),
				Name:               notifier.StripEmoji(strings.TrimSpace(args.TaskSpec.Name)),
				Type:               tType,
				BotID:              targetBot.ID,
				Model:              strings.TrimSpace(args.TaskSpec.Model),
				AllowPaidFallback:  tAllowPaid,
				MaxPaidCallsPerDay: args.TaskSpec.MaxPaidCallsPerDay,
				Description:        truncateTitle(cleanPrompt),
				CronExpr:           tCron,
				RunAt:              tRunAt,
				Enabled:            true,
				Params:             paramsJSON,
			}
			if err := e.store.SaveTask(task); err != nil {
				return "", fmt.Errorf("创建套装定时任务失败: %w", err)
			}
			if e.scheduler != nil {
				_ = e.scheduler.Reload()
			}
			createdTask = &task
			e.publishAssistantUpdate("tasks")
		}
		e.publishAssistantUpdate("bots")
		out, _ := json.Marshal(map[string]any{
			"ok":                  true,
			"action":              "setup_blueprint",
			"bot_id":              targetBot.ID,
			"bot_name":            targetBot.Name,
			"allow_paid_fallback": targetBot.AllowPaidFallback,
			"skill_ids":           boundSkillIDs,
			"skills":              boundSkills,
			"task":                createdTask,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_bots action: %s", action)
	}
}
