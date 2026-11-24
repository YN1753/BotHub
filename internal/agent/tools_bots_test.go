package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestDefaultButlerSeededCleanAndIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "butler_seed.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bots, err := st.ListBots()
	if err != nil {
		t.Fatalf("list bots: %v", err)
	}
	if len(bots) != 1 {
		t.Fatalf("expected 1 default concierge bot on fresh DB, got %d: %+v", len(bots), bots)
	}
	b := bots[0]
	if b.ID != store.DefaultConciergeBotID || b.Name != "大管家" || !b.IsPinned {
		t.Fatalf("unexpected default butler bot: %+v", b)
	}
	if b.Model != "auto" || !b.EnableThinking {
		t.Fatalf("expected default butler to use auto with enable_thinking=true, got model=%q thinking=%v", b.Model, b.EnableThinking)
	}
	if b.AllowedTools != "[]" || b.SkillIDs != "[]" {
		t.Fatalf("expected default butler to start with pure Native Core (AllowedTools=[], SkillIDs=[]), got tools=%s skills=%s", b.AllowedTools, b.SkillIDs)
	}
	// Ensure zero hardcoded personal user tech stack assumptions in default prompt
	for _, forbidden := range []string{"CloudCone", "Golang", "Kubernetes", "Android 开发"} {
		if strings.Contains(b.SystemPrompt, forbidden) {
			t.Fatalf("default butler prompt must stay neutral without hardcoded personal fact %q", forbidden)
		}
	}

	// Deleting butler and reopening DB must NOT resurrect it once seeded flag is recorded
	if err := st.DeleteBot(store.DefaultConciergeBotID); err != nil {
		t.Fatalf("delete butler: %v", err)
	}
	_ = st.Close()

	st2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer st2.Close()

	botsAfter, _ := st2.ListBots()
	if len(botsAfter) != 0 {
		t.Fatalf("expected deleted butler to remain deleted across restarts, got %+v", botsAfter)
	}
}

func TestManageBotsLifecycleCreateBotAndGroup(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "manage_bots.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         store.DefaultConciergeBotID,
		TimelineBotID: store.DefaultConciergeBotID,
	})

	// 1. Create two specialist bots via manage_bots
	res1, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"create_bot","name":"代码审查官","role_title":"架构与代码审阅","bot_mode":"worker","traits":"严谨克制","allowed_tools":["web_search","manage_notes"]}`)
	if err != nil || !strings.Contains(res1, `"ok":true`) {
		t.Fatalf("create_bot 1 failed: err=%v out=%s", err, res1)
	}
	var created1 struct {
		BotID string `json:"bot_id"`
	}
	_ = json.Unmarshal([]byte(res1), &created1)
	if created1.BotID == "" {
		t.Fatalf("expected non-empty bot_id from create_bot 1, got %s", res1)
	}

	res2, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"create_bot","name":"测试工程师","role_title":"自动化测试与回归","bot_mode":"worker"}`)
	if err != nil || !strings.Contains(res2, `"ok":true`) {
		t.Fatalf("create_bot 2 failed: err=%v out=%s", err, res2)
	}

	// 2. Update bot by name
	updRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"update_bot","bot_name":"代码审查官","traits":"直接指出边界风险"}`)
	if err != nil || !strings.Contains(updRes, `"ok":true`) {
		t.Fatalf("update_bot failed: err=%v out=%s", err, updRes)
	}
	gotBot1, _ := st.GetBot(created1.BotID)
	if gotBot1 == nil || !strings.Contains(gotBot1.Traits, "边界风险") {
		t.Fatalf("expected updated traits, got %+v", gotBot1)
	}

	// 3. Create group resolving members by names + include_self
	grpRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"create_group","name":"研发协同群","member_names":["代码审查官","测试工程师"],"include_self":true,"group_mode":"smart","announcement":"先评审再合并"}`)
	if err != nil || !strings.Contains(grpRes, `"ok":true`) {
		t.Fatalf("create_group failed: err=%v out=%s", err, grpRes)
	}
	var createdGrp struct {
		GroupID     string   `json:"group_id"`
		MemberIDs   []string `json:"member_ids"`
		MemberNames []string `json:"member_names"`
	}
	_ = json.Unmarshal([]byte(grpRes), &createdGrp)
	if createdGrp.GroupID == "" || len(createdGrp.MemberIDs) != 3 {
		t.Fatalf("expected group with 3 members (butler + 2 bots), got %+v (%s)", createdGrp, grpRes)
	}

	// 4. Create a 3rd bot, add to group, then remove from group
	_, err = eng.ExecuteTool(ctx, "manage_bots", `{"action":"create_bot","name":"文档秘书","role_title":"整理纪要","bot_mode":"hybrid"}`)
	if err != nil {
		t.Fatalf("create_bot 3 failed: %v", err)
	}
	addRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"add_group_members","group_name":"研发协同群","member_names":["文档秘书"]}`)
	if err != nil || !strings.Contains(addRes, "文档秘书") {
		t.Fatalf("add_group_members failed: err=%v out=%s", err, addRes)
	}
	rmRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"remove_group_members","group_id":"`+createdGrp.GroupID+`","member_names":["测试工程师"]}`)
	if err != nil || !strings.Contains(rmRes, "测试工程师") {
		t.Fatalf("remove_group_members failed: err=%v out=%s", err, rmRes)
	}
	grpAfter, _ := st.GetBot(createdGrp.GroupID)
	if grpAfter == nil || len(grpAfter.MemberIDList()) != 3 {
		t.Fatalf("expected 3 members after +1 -1, got %+v", grpAfter)
	}

	// 5. Guardrail: refusing to delete default concierge butler via tool
	if _, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"delete_bot","bot_id":"concierge"}`); err == nil {
		t.Fatalf("expected manage_bots delete_bot to refuse deleting concierge butler")
	}

	// 6. Deleting a normal bot succeeds
	delRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"delete_bot","bot_name":"文档秘书"}`)
	if err != nil || !strings.Contains(delRes, `"ok":true`) {
		t.Fatalf("delete_bot failed: err=%v out=%s", err, delRes)
	}

	// 7. Listing roster reflects current bots and groups
	listRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"list"}`)
	if err != nil || !strings.Contains(listRes, "大管家") || !strings.Contains(listRes, "研发协同群") {
		t.Fatalf("list failed: err=%v out=%s", err, listRes)
	}
}

func TestManageModelsAndManageSkillsConciergeTools(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "concierge_tools.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Verify concierge seeded with pure Native Core and zero pre-mounted MCPs
	concierge, _ := st.GetBot(store.DefaultConciergeBotID)
	if concierge == nil || concierge.AllowedTools != "[]" || concierge.SkillIDs != "[]" {
		t.Fatalf("expected concierge to start with pure Native Core and zero pre-mounted MCPs, got %+v", concierge)
	}

	_ = st.SaveModelConfig(
		[]store.ModelProvider{
			{ID: "or_1", Name: "OpenRouter", Protocol: "openai", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-top-secret-never-leak", Enabled: true},
		},
		[]store.PooledModel{
			{ProviderID: "or_1", ModelID: "anthropic/claude-sonnet-4", Enabled: true, Priority: 10},
			{ProviderID: "or_1", ModelID: "google/gemini-2.5-flash", Enabled: true, Priority: 20, IsLight: true},
		},
		store.PoolStrategyPriority,
	)

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         store.DefaultConciergeBotID,
		TimelineBotID: store.DefaultConciergeBotID,
	})

	// 1. manage_models status must NEVER leak raw API keys
	statusOut, err := eng.ExecuteTool(ctx, "manage_models", `{"action":"status"}`)
	if err != nil || !strings.Contains(statusOut, `"ok":true`) {
		t.Fatalf("manage_models status failed: err=%v out=%s", err, statusOut)
	}
	if strings.Contains(statusOut, "sk-top-secret-never-leak") {
		t.Fatalf("manage_models leaked raw API key: %s", statusOut)
	}

	// 2. manage_models set_strategy + clear_cooldown
	eng.health.recordFailure("pool:or_1::anthropic/claude-sonnet-4", "HTTP 429", true)
	_, err = eng.ExecuteTool(ctx, "manage_models", `{"action":"clear_cooldown","model_id":"anthropic/claude-sonnet-4"}`)
	if err != nil || eng.health.isCooling("pool:or_1::anthropic/claude-sonnet-4") {
		t.Fatalf("expected manage_models clear_cooldown to unblock model, err=%v", err)
	}
	_, err = eng.ExecuteTool(ctx, "manage_models", `{"action":"set_strategy","strategy":"round_robin"}`)
	if err != nil || st.GetPoolStrategy() != store.PoolStrategyRoundRobin {
		t.Fatalf("expected manage_models set_strategy to set round_robin, err=%v", err)
	}

	// 3. manage_skills upsert + bind_bot + list + delete
	skOut, err := eng.ExecuteTool(ctx, "manage_skills", `{"action":"upsert","name":"日报提炼","kind":"prompt","description":"结构化整理当日进展","instruction":"按三段式输出今日成果、阻塞与明日计划","bot_id":"concierge"}`)
	if err != nil || !strings.Contains(skOut, "日报提炼") {
		t.Fatalf("manage_skills upsert failed: err=%v out=%s", err, skOut)
	}
	updatedConcierge, _ := st.GetBot(store.DefaultConciergeBotID)
	if updatedConcierge == nil || len(updatedConcierge.SkillIDList()) != 1 {
		t.Fatalf("expected skill bound to concierge, got %+v", updatedConcierge)
	}
	delOut, err := eng.ExecuteTool(ctx, "manage_skills", `{"action":"delete","name":"日报提炼"}`)
	if err != nil || !strings.Contains(delOut, `"ok":true`) {
		t.Fatalf("manage_skills delete failed: err=%v out=%s", err, delOut)
	}
}

func TestManageBotsSetupBlueprintAndUsageCallers(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "blueprint.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         store.DefaultConciergeBotID,
		TimelineBotID: store.DefaultConciergeBotID,
	})

	bpOut, err := eng.ExecuteTool(ctx, "manage_bots", `{
		"action": "setup_blueprint",
		"name": "运维晨报官",
		"role_title": "每日服务器与资产巡检助手",
		"bot_mode": "worker",
		"allowed_tools": ["manage_assets", "query_nezha_nodes"],
		"skill_specs": [{
			"slug": "ops_checklist",
			"name": "巡检清单",
			"kind": "prompt",
			"description": "汇总节点负载和即将到期资产",
			"instruction": "输出高危异常并给出处置建议"
		}],
		"task_spec": {
			"name": "每日晨间巡检",
			"type": "bot_prompt",
			"cron_expr": "0 8 * * *",
			"prompt": "请巡检全部节点和 14 天内到期资产"
		}
	}`)
	if err != nil || !strings.Contains(bpOut, `"action":"setup_blueprint"`) {
		t.Fatalf("setup_blueprint failed: err=%v out=%s", err, bpOut)
	}

	var parsed struct {
		OK       bool     `json:"ok"`
		BotID    string   `json:"bot_id"`
		SkillIDs []string `json:"skill_ids"`
		Task     struct {
			ID    string `json:"id"`
			BotID string `json:"bot_id"`
		} `json:"task"`
	}
	if err := json.Unmarshal([]byte(bpOut), &parsed); err != nil {
		t.Fatalf("unmarshal blueprint output: %v", err)
	}
	if !parsed.OK || parsed.BotID == "" {
		t.Fatalf("expected valid bot from blueprint, got %+v", parsed)
	}
	if len(parsed.SkillIDs) != 1 || parsed.Task.ID == "" || parsed.Task.BotID != parsed.BotID {
		t.Fatalf("expected bound skill and task from blueprint, got %+v", parsed)
	}

	_ = st.RecordModelUsage(store.ModelUsageRecord{
		TargetID:         "pool:or_1::anthropic/claude-sonnet-4",
		ProviderID:       "or_1",
		ModelID:          "anthropic/claude-sonnet-4",
		Source:           "chat",
		BotID:            parsed.BotID,
		PromptTokens:     1200,
		CachedTokens:     800,
		CompletionTokens: 200,
		CostUSD:          0.0045,
		LatencyMs:        420,
		Success:          true,
	})
	sum, err := st.SummarizeModelUsage(7)
	if err != nil || len(sum.TopCallers) == 0 || sum.TopCallers[0].BotID != parsed.BotID || sum.TotalCachedTokens != 800 || sum.TotalCostUSD <= 0 {
		t.Fatalf("expected top_callers + cost + cache in usage summary, err=%v sum=%+v", err, sum)
	}
}

func TestCreateBotAutoForgeAndUpgradeBot(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "forge_bots.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         store.DefaultConciergeBotID,
		TimelineBotID: store.DefaultConciergeBotID,
	})

	// 1. Creating a companion bot with NO system_prompt and NO allowed_tools must:
	//    - Default to auto, EnableThinking=true, MemoryPolicy=standard
	//    - Keep AllowedTools="[]" (full Native Core including web_search & mount_mcp)
	//    - Auto-forge a rich high-IQ SystemPrompt (>= 160 runes)
	res, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"create_bot","name":"毒舌影评人","role_title":"电影与叙事结构分析","bot_mode":"companion","traits":"犀利、有审美、不客套"}`)
	if err != nil || !strings.Contains(res, `"ok":true`) {
		t.Fatalf("create_bot failed: err=%v out=%s", err, res)
	}
	var created struct {
		BotID string `json:"bot_id"`
	}
	_ = json.Unmarshal([]byte(res), &created)
	bot, err := st.GetBot(created.BotID)
	if err != nil || bot == nil {
		t.Fatalf("get created bot: %v", err)
	}
	if bot.Model != "auto" || !bot.EnableThinking || bot.MemoryPolicy != "standard" {
		t.Fatalf("expected unified auto high-IQ defaults, got %+v", bot)
	}
	if bot.AllowedTools != "[]" {
		t.Fatalf("expected empty AllowedTools ([]) to preserve full Native Core + mount_mcp, got %s", bot.AllowedTools)
	}
	if len([]rune(bot.SystemPrompt)) < 160 {
		t.Fatalf("expected auto-forged rich SystemPrompt (>=160 runes), got %d runes: %s", len([]rune(bot.SystemPrompt)), bot.SystemPrompt)
	}

	// 2. Seed an old dumb bot (thin prompt, restricted allowed_tools) and upgrade via manage_bots(action="upgrade_bot")
	if err := st.SaveBot(store.Bot{
		ID:             "bot_legacy_dumb",
		Name:           "旧版小助手",
		RoleTitle:      "查资料和写周报",
		BotMode:        "worker",
		MemoryPolicy:   "isolated",
		Model:          "auto",
		EnableThinking: false,
		AllowedTools:   `["remember_fact","recall_memory"]`,
		SystemPrompt:   "你是旧版小助手。",
		SkillIDs:       "[]",
	}); err != nil {
		t.Fatalf("seed legacy dumb bot: %v", err)
	}

	upgOut, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"upgrade_bot","bot_id":"bot_legacy_dumb","goal":"擅长技术调研和结构化周报总结"}`)
	if err != nil || !strings.Contains(upgOut, `"action":"upgrade_bot"`) {
		t.Fatalf("upgrade_bot failed: err=%v out=%s", err, upgOut)
	}
	upgraded, _ := st.GetBot("bot_legacy_dumb")
	if upgraded == nil {
		t.Fatalf("expected upgraded bot to exist")
	}
	if upgraded.Model != "auto" || !upgraded.EnableThinking || upgraded.AllowedTools != "[]" || upgraded.MemoryPolicy != "standard" {
		t.Fatalf("expected upgraded bot to use auto with full native core, got %+v", upgraded)
	}
	if len([]rune(upgraded.SystemPrompt)) < 100 || strings.Contains(upgraded.SystemPrompt, "web_search") {
		t.Fatalf("expected upgraded SystemPrompt to be forged cleanly without built-in tool boilerplate (>=100 runes), got %d runes: %s", len([]rune(upgraded.SystemPrompt)), upgraded.SystemPrompt)
	}
}
