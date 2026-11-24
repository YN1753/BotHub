package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func manageModelsToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_models",
				Description: "巡检与调优 AI 模型渠道与统一模型池（大管家核心能力，不暴露任何明文 API Key）。支持 status（查看各渠道、模型池列表、健康冷却状态与近 7 日调用及预估费用统计）、clear_cooldown（一键解封冷却模型，target_id 留空则全部解封）、set_strategy（切换模型池调度策略：priority / round_robin / lowest_latency）、toggle_model（启用或停用池内模型）、reorder（调整池内模型优先级 priority）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"status", "clear_cooldown", "set_strategy", "toggle_model", "reorder"},
							"description": "操作类型：status | clear_cooldown | set_strategy | toggle_model | reorder",
						},
						"target_id": map[string]any{
							"type":        "string",
							"description": "可选：目标节点 ID（如 pool:<provider_id>::<model_id>，clear_cooldown 时留空表示解封全部）",
						},
						"provider_id": map[string]any{
							"type":        "string",
							"description": "可选：目标渠道 ID（toggle_model / reorder 时可配合 model_id 精确匹配）",
						},
						"model_id": map[string]any{
							"type":        "string",
							"description": "目标模型 ID（如 claude-sonnet-4、deepseek-chat 等）",
						},
						"strategy": map[string]any{
							"type":        "string",
							"enum":        []string{"priority", "round_robin", "lowest_latency"},
							"description": "set_strategy 时的池内调度策略：priority（按优先级）| round_robin（轮询均摊）| lowest_latency（最快延迟）",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "toggle_model 时设置是否启用该模型",
						},
						"priority": map[string]any{
							"type":        "integer",
							"description": "reorder 时设置的新优先级数值（越小越优先，如 10, 20, 30）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在巡检与调优 AI 模型池...",
		DomainKeywords: []string{"模型池", "模型", "渠道", "冷却", "解封", "轮询", "token", "调用量", "费用", "测速"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageModels(ctx, cleanArgs)
		},
	}
}

func manageSkillsToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_skills",
				Description: "管理自定义技能工坊（Prompt 提示词技能与 Webhook 接口技能）及 Bot 技能挂载。支持 list（查看全部自定义技能）、upsert（创建或更新自定义技能）、delete（删除技能）、bind_bot（将技能挂载到指定 Bot 或从 Bot 卸载）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "upsert", "delete", "bind_bot"},
							"description": "操作类型：list | upsert | delete | bind_bot",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "技能 ID（更新、删除或绑定时使用，也可通过 slug 或 name 匹配）",
						},
						"slug": map[string]any{
							"type":        "string",
							"description": "技能英文蛇形标识（如 daily_digest，新建未填时自动生成）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "技能显示名称",
						},
						"kind": map[string]any{
							"type":        "string",
							"enum":        []string{"prompt", "webhook"},
							"description": "技能类型：prompt（提示词增强，默认）| webhook（HTTP 接口调用）",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "技能用途简述",
						},
						"instruction": map[string]any{
							"type":        "string",
							"description": "prompt 技能的具体行为指令，或 webhook 技能的补充说明",
						},
						"http_method": map[string]any{
							"type":        "string",
							"description": "webhook 技能的 HTTP 方法（GET / POST，默认 GET）",
						},
						"http_url": map[string]any{
							"type":        "string",
							"description": "webhook 技能的请求 URL（支持 {{param}} 占位符）",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "是否启用该技能（默认 true）",
						},
						"bot_id": map[string]any{
							"type":        "string",
							"description": "bind_bot 或 upsert 时要挂载的目标 Bot ID（可与 bot_name 互替）",
						},
						"bot_name": map[string]any{
							"type":        "string",
							"description": "bind_bot 或 upsert 时要挂载的目标 Bot 名称",
						},
						"unbind": map[string]any{
							"type":        "boolean",
							"description": "bind_bot 时设为 true 表示从目标 Bot 卸载该技能（默认 false 表示挂载）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在配置自定义技能工坊...",
		DomainKeywords: []string{"技能", "skill", "webhook", "工坊", "挂载技能"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageSkills(ctx, cleanArgs)
		},
	}
}

func (e *Engine) toolManageModels(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Action     string `json:"action"`
		TargetID   string `json:"target_id"`
		ProviderID string `json:"provider_id"`
		ModelID    string `json:"model_id"`
		Strategy   string `json:"strategy"`
		Enabled    *bool  `json:"enabled"`
		Priority   *int   `json:"priority"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_models 参数失败: %w", err)
	}

	providers, models, strategy := e.store.GetModelConfig()
	action := strings.ToLower(strings.TrimSpace(args.Action))

	type safeProviderView struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
		BaseURL  string `json:"base_url"`
		HasKey   bool   `json:"has_key"`
		Enabled  bool   `json:"enabled"`
		Note     string `json:"note,omitempty"`
	}
	buildStatusPayload := func(msg string) string {
		pList, mList, strat := e.store.GetModelConfig()
		safeProvs := make([]safeProviderView, 0, len(pList))
		for _, p := range pList {
			safeProvs = append(safeProvs, safeProviderView{
				ID:       p.ID,
				Name:     p.Name,
				Protocol: p.Protocol,
				BaseURL:  p.BaseURL,
				HasKey:   strings.TrimSpace(p.APIKey) != "",
				Enabled:  p.Enabled,
				Note:     p.Note,
			})
		}
		usage, _ := e.store.SummarizeModelUsage(7)
		payload := map[string]any{
			"ok":            true,
			"providers":     safeProvs,
			"pooled_models": mList,
			"strategy":      strat,
			"model_health":  e.GetModelHealth(),
			"usage_7d":      usage,
		}
		if msg != "" {
			payload["message"] = msg
		}
		b, _ := json.Marshal(payload)
		return string(b)
	}

	matchModelIndex := func() int {
		tid := strings.TrimSpace(args.TargetID)
		pid := strings.TrimSpace(args.ProviderID)
		mid := strings.TrimSpace(args.ModelID)
		if strings.HasPrefix(tid, "pool:") {
			spec := strings.TrimPrefix(tid, "pool:")
			if sep := strings.Index(spec, "::"); sep >= 0 {
				if pid == "" {
					pid = spec[:sep]
				}
				if mid == "" {
					mid = spec[sep+2:]
				}
			}
		}
		for i, m := range models {
			if pid != "" && m.ProviderID == pid && strings.EqualFold(m.ModelID, mid) {
				return i
			}
		}
		if mid != "" {
			for i, m := range models {
				if strings.EqualFold(m.ModelID, mid) {
					return i
				}
			}
			lower := strings.ToLower(mid)
			for i, m := range models {
				if strings.Contains(strings.ToLower(m.ModelID), lower) {
					return i
				}
			}
		}
		return -1
	}

	switch action {
	case "status":
		return buildStatusPayload(""), nil

	case "clear_cooldown":
		tid := strings.TrimSpace(args.TargetID)
		if tid == "" && strings.TrimSpace(args.ModelID) != "" {
			if idx := matchModelIndex(); idx >= 0 {
				tid = models[idx].CanonicalTargetID()
			}
		}
		e.ClearModelCooldown(tid)
		e.publishAssistantUpdate("models")
		e.publishAssistantUpdate("model_health")
		if tid == "" {
			return buildStatusPayload("已解除所有模型的冷却状态"), nil
		}
		return buildStatusPayload("已解除模型 " + tid + " 的冷却状态"), nil

	case "set_strategy":
		if s := strings.TrimSpace(args.Strategy); s != "" {
			strategy = store.NormalizePoolStrategy(s)
		}
		if err := e.store.SaveModelConfig(providers, models, strategy); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("models")
		return buildStatusPayload(fmt.Sprintf("已更新模型池调度策略：%s", strategy)), nil

	case "toggle_model":
		idx := matchModelIndex()
		if idx < 0 {
			return "", fmt.Errorf("未在当前模型池中找到模型 %q", args.ModelID)
		}
		nextEnabled := !models[idx].Enabled
		if args.Enabled != nil {
			nextEnabled = *args.Enabled
		}
		models[idx].Enabled = nextEnabled
		if err := e.store.SaveModelConfig(providers, models, strategy); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("models")
		return buildStatusPayload(fmt.Sprintf("已将模型 %s 设为 enabled=%v", models[idx].ModelID, nextEnabled)), nil

	case "reorder":
		idx := matchModelIndex()
		if idx < 0 {
			return "", fmt.Errorf("未在当前模型池中找到模型 %q", args.ModelID)
		}
		if args.Priority == nil || *args.Priority <= 0 {
			return "", fmt.Errorf("reorder 时需提供大于 0 的 priority 参数")
		}
		models[idx].Priority = *args.Priority
		if err := e.store.SaveModelConfig(providers, models, strategy); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("models")
		return buildStatusPayload(fmt.Sprintf("已将模型 %s 优先级调整为 %d", models[idx].ModelID, *args.Priority)), nil

	default:
		return "", fmt.Errorf("不支持的 manage_models action: %s", action)
	}
}

func normalizeSkillSlug(raw, fallbackName string) string {
	src := strings.ToLower(strings.TrimSpace(raw))
	if src == "" {
		src = strings.ToLower(strings.TrimSpace(fallbackName))
	}
	var b strings.Builder
	prevUnderscore := false
	for _, r := range src {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			prevUnderscore = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		buf := make([]byte, 3)
		_, _ = rand.Read(buf)
		res = "custom_" + hex.EncodeToString(buf)
	}
	return res
}

type skillUpsertInput struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Instruction string `json:"instruction"`
	HTTPMethod  string `json:"http_method"`
	HTTPURL     string `json:"http_url"`
	Enabled     *bool  `json:"enabled"`
}

func (e *Engine) findSkillByMatch(id, slug, name string) *store.Skill {
	all, _ := e.store.ListSkills()
	wantID := strings.TrimSpace(id)
	wantSlug := strings.TrimSpace(slug)
	wantName := strings.TrimSpace(name)
	for i := range all {
		if (wantID != "" && all[i].ID == wantID) ||
			(wantSlug != "" && strings.EqualFold(all[i].Slug, wantSlug)) ||
			(wantName != "" && strings.EqualFold(all[i].Name, wantName)) {
			return &all[i]
		}
	}
	return nil
}

func (e *Engine) upsertSkillFromParams(in skillUpsertInput) (*store.Skill, error) {
	existing := e.findSkillByMatch(in.ID, in.Slug, in.Name)
	var sk store.Skill
	if existing != nil {
		sk = *existing
	} else {
		buf := make([]byte, 4)
		_, _ = rand.Read(buf)
		sk = store.Skill{
			ID:           "sk_" + hex.EncodeToString(buf),
			Kind:         "prompt",
			HTTPMethod:   "GET",
			HTTPHeaders:  "{}",
			ParamsSchema: "[]",
			Enabled:      true,
		}
	}
	if v := notifier.StripEmoji(strings.TrimSpace(in.Name)); v != "" {
		sk.Name = v
	}
	if sk.Name == "" {
		return nil, fmt.Errorf("技能名称 name 不能为空")
	}
	if v := strings.TrimSpace(in.Slug); v != "" || sk.Slug == "" {
		sk.Slug = normalizeSkillSlug(v, sk.Name)
	}
	if v := strings.ToLower(strings.TrimSpace(in.Kind)); v == "webhook" || v == "prompt" || v == "mcp" {
		sk.Kind = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(in.Description)); v != "" {
		sk.Description = v
	}
	if v := notifier.StripEmoji(strings.TrimSpace(in.Instruction)); v != "" {
		sk.Instruction = v
	}
	if v := strings.TrimSpace(in.HTTPMethod); v != "" {
		sk.HTTPMethod = strings.ToUpper(v)
	}
	if v := strings.TrimSpace(in.HTTPURL); v != "" {
		sk.HTTPURL = v
	}
	if in.Enabled != nil {
		sk.Enabled = *in.Enabled
	}
	if err := e.store.SaveSkill(sk); err != nil {
		return nil, err
	}
	e.publishAssistantUpdate("skills")
	return &sk, nil
}

func (e *Engine) toolManageSkills(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		Action      string `json:"action"`
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Kind        string `json:"kind"`
		Description string `json:"description"`
		Instruction string `json:"instruction"`
		HTTPMethod  string `json:"http_method"`
		HTTPURL     string `json:"http_url"`
		Enabled     *bool  `json:"enabled"`
		BotID       string `json:"bot_id"`
		BotName     string `json:"bot_name"`
		Unbind      bool   `json:"unbind"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_skills 参数失败: %w", err)
	}

	findSkill := func() *store.Skill {
		return e.findSkillByMatch(args.ID, args.Slug, args.Name)
	}

	bindSkillToBot := func(skillID, botID, botName string, unbind bool) (*store.Bot, error) {
		allBots, _ := e.store.ListBots()
		var target *store.Bot
		for i := range allBots {
			if (botID != "" && allBots[i].ID == botID) ||
				(botName != "" && strings.EqualFold(allBots[i].Name, botName)) {
				target = &allBots[i]
				break
			}
		}
		if target == nil && botName != "" {
			needle := strings.ToLower(botName)
			for i := range allBots {
				if strings.Contains(strings.ToLower(allBots[i].Name), needle) {
					target = &allBots[i]
					break
				}
			}
		}
		if target == nil {
			return nil, fmt.Errorf("未找到目标 Bot（id=%q name=%q）", botID, botName)
		}
		cur := target.SkillIDList()
		next := make([]string, 0, len(cur)+1)
		found := false
		for _, id := range cur {
			if id == skillID {
				found = true
				if unbind {
					continue
				}
			}
			next = append(next, id)
		}
		if !unbind && !found {
			next = append(next, skillID)
		}
		raw, _ := json.Marshal(next)
		target.SkillIDs = string(raw)
		if err := e.store.SaveBot(*target); err != nil {
			return nil, err
		}
		e.publishAssistantUpdate("bots")
		return target, nil
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		skills, err := e.store.ListSkills()
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{
			"ok":     true,
			"count":  len(skills),
			"skills": store.MaskSkillsForPublic(skills),
		})
		return string(b), nil

	case "upsert":
		sk, err := e.upsertSkillFromParams(skillUpsertInput{
			ID:          args.ID,
			Slug:        args.Slug,
			Name:        args.Name,
			Kind:        args.Kind,
			Description: args.Description,
			Instruction: args.Instruction,
			HTTPMethod:  args.HTTPMethod,
			HTTPURL:     args.HTTPURL,
			Enabled:     args.Enabled,
		})
		if err != nil {
			return "", err
		}

		var boundBotName string
		if strings.TrimSpace(args.BotID) != "" || strings.TrimSpace(args.BotName) != "" {
			if b, bErr := bindSkillToBot(sk.ID, strings.TrimSpace(args.BotID), strings.TrimSpace(args.BotName), false); bErr == nil && b != nil {
				boundBotName = b.Name
			}
		}
		out, _ := json.Marshal(map[string]any{
			"ok":        true,
			"skill":     sk,
			"bound_bot": boundBotName,
		})
		return string(out), nil

	case "delete":
		existing := findSkill()
		if existing == nil {
			return "", fmt.Errorf("未找到要删除的自定义技能")
		}
		if err := e.store.DeleteSkill(existing.ID); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("skills")
		e.publishAssistantUpdate("bots")
		out, _ := json.Marshal(map[string]any{
			"ok":         true,
			"deleted_id": existing.ID,
			"name":       existing.Name,
		})
		return string(out), nil

	case "bind_bot":
		existing := findSkill()
		if existing == nil {
			return "", fmt.Errorf("未找到要挂载的技能（请提供 id、slug 或 name）")
		}
		bot, err := bindSkillToBot(existing.ID, strings.TrimSpace(args.BotID), strings.TrimSpace(args.BotName), args.Unbind)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":       true,
			"skill_id": existing.ID,
			"skill":    existing.Name,
			"bot_id":   bot.ID,
			"bot_name": bot.Name,
			"unbound":  args.Unbind,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_skills action: %s", action)
	}
}
