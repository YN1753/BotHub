package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestScopedMemoryCollisionImmunityAndRecallCount(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "mem_scope.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	// Two bots writing the exact same fact_key must stay strictly isolated per scope
	if err := st.SaveAgentMemoryDetailed("bot:alpha", "preference", "communication_style", "喜欢直接发短句", 5, true); err != nil {
		t.Fatalf("save alpha memory failed: %v", err)
	}
	if err := st.SaveAgentMemoryDetailed("bot:beta", "preference", "communication_style", "喜欢详尽 Markdown 表格", 4, false); err != nil {
		t.Fatalf("save beta memory failed: %v", err)
	}
	if err := st.SaveAgentMemoryDetailed("global", "infra", "home_region", "Tokyo", 3, false); err != nil {
		t.Fatalf("save global memory failed: %v", err)
	}

	// Isolated policy on alpha only loads bot:alpha memories (ignores global and bot:beta)
	isolatedMems, err := st.ListScopedMemories([]string{"bot:alpha"}, "", 10)
	if err != nil {
		t.Fatalf("list isolated memories failed: %v", err)
	}
	if len(isolatedMems) != 1 || isolatedMems[0].Content != "喜欢直接发短句" || !isolatedMems[0].Pinned {
		t.Fatalf("expected only alpha pinned memory in isolated mode, got %+v", isolatedMems)
	}

	// Standard policy on alpha loads both bot:alpha and global, but never bot:beta
	standardMems, err := st.ListScopedMemories([]string{"bot:alpha", "global"}, "", 10)
	if err != nil {
		t.Fatalf("list standard memories failed: %v", err)
	}
	if len(standardMems) != 2 {
		t.Fatalf("expected 2 memories (bot:alpha + global), got %d: %+v", len(standardMems), standardMems)
	}
	for _, m := range standardMems {
		if m.Scope == "bot:beta" {
			t.Fatalf("memory isolation violated: bot:alpha saw bot:beta memory")
		}
	}

	// Tool remember_fact / recall_memory scoped execution & recall_count increment
	eng := NewEngine(st, nil)
	ctxAlpha := WithToolScope(context.Background(), ToolScope{
		BotID:         "alpha",
		TimelineBotID: "alpha",
		MemoryPolicy:  store.MemoryPolicyIsolated,
	})
	resRemember, execErr := eng.ExecuteTool(ctxAlpha, "remember_fact", `{"category":"insight","fact_key":"coffee","content":"只喝冰美式"}`)
	if execErr != nil || strings.Contains(resRemember, `"error"`) {
		t.Fatalf("scoped remember_fact failed: err=%v res=%s", execErr, resRemember)
	}

	resRecall, execErr := eng.ExecuteTool(ctxAlpha, "recall_memory", `{"query":"冰美式"}`)
	if execErr != nil || !strings.Contains(resRecall, "只喝冰美式") {
		t.Fatalf("expected recall_memory to return scoped fact, err=%v res=%s", execErr, resRecall)
	}

	ctxBeta := WithToolScope(context.Background(), ToolScope{
		BotID:         "beta",
		TimelineBotID: "beta",
		MemoryPolicy:  store.MemoryPolicyIsolated,
	})
	resRecallBeta, _ := eng.ExecuteTool(ctxBeta, "recall_memory", `{"query":"冰美式"}`)
	if strings.Contains(resRecallBeta, "只喝冰美式") {
		t.Fatalf("bot:beta unexpectedly recalled bot:alpha private memory: %s", resRecallBeta)
	}

	rechecked, _ := st.ListScopedMemories([]string{"bot:alpha"}, "", 10)
	var recalledCoffee *store.AgentMemory
	for i := range rechecked {
		if rechecked[i].FactKey == "coffee" {
			recalledCoffee = &rechecked[i]
		}
	}
	if recalledCoffee == nil || recalledCoffee.RecallCount < 1 {
		t.Fatalf("expected recall_count >= 1 after recall_memory, got %+v", recalledCoffee)
	}
}

func TestWebhookSkillCompileExecuteAndSSRF(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Auth") != "secret-xyz" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("missing header"))
			return
		}
		q := r.URL.Query().Get("city")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"city":   q,
			"status": "clear",
		})
	}))
	defer srv.Close()

	sk := store.Skill{
		ID:           "sk_weather",
		Name:         "Weather Probe",
		Slug:         "weather_probe",
		Kind:         "webhook",
		Description:  "查询指定城市的即时状态",
		HTTPMethod:   "GET",
		HTTPURL:      srv.URL + "?city={{city}}",
		HTTPHeaders:  `{"X-Custom-Auth":"{{token}}"}`,
		ParamsSchema: `[{"key":"city","type":"string","description":"城市","required":true},{"key":"token","type":"string","description":"令牌","required":true}]`,
		Enabled:      true,
	}
	if err := st.SaveSkill(sk); err != nil {
		t.Fatalf("save skill failed: %v", err)
	}

	toolDef := CompileSkillTool(sk)
	if toolDef.Function.Name != "skill_weather_probe" {
		t.Fatalf("expected compiled name skill_weather_probe, got %s", toolDef.Function.Name)
	}

	// SSRF guard must block 127.0.0.1 when allowLocalhostForTest is false
	engBlocked := NewEngine(st, nil)
	if _, _, err := engBlocked.ExecuteWebhookSkill(context.Background(), sk, map[string]any{"city": "tokyo", "token": "secret-xyz"}); err == nil {
		t.Fatalf("expected SSRF check to block loopback skill execution in production mode")
	}

	// Allowed in test mode: execute via tool dispatch using skill_<slug>
	eng := NewEngine(st, nil)
	eng.allowLocalhostForTest = true
	res, execErr := eng.ExecuteTool(context.Background(), "skill_weather_probe", `{"city":"shanghai","token":"secret-xyz"}`)
	if execErr != nil || !strings.Contains(res, "shanghai") || !strings.Contains(res, "clear") {
		t.Fatalf("expected interpolated JSON output from webhook skill, err=%v res=%s", execErr, res)
	}
}

func TestCompanionVsWorkerPromptAndSkills(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "prompts.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	playbook := store.Skill{
		ID:          "sk_warm_talk",
		Name:        "夜聊共情规则",
		Slug:        "warm_talk",
		Kind:        "prompt",
		Description: "倾听优先，避免说教",
		Instruction: "先接住对方的情绪，再轻松接两句关心，不要像客服模板。",
		Enabled:     true,
	}
	_ = st.SaveSkill(playbook)

	companionBot := store.Bot{
		ID:           "bot_friend",
		Name:         "阿澈",
		RoleTitle:    "毒舌又靠谱的老朋友",
		BotMode:      store.BotModeCompanion,
		MemoryPolicy: store.MemoryPolicyIsolated,
		Traits:       "说话简短自然、幽默接梗",
		SkillIDs:     `["sk_warm_talk"]`,
		SystemPrompt: "平时像微信聊天一样互动。",
	}
	prompt := buildBotSystemPrompt(&companionBot, []store.Skill{playbook}, nil, nil, "", false)
	if !strings.Contains(prompt, "真实、有温度的老朋友") || !strings.Contains(prompt, "说话简短自然、幽默接梗") || !strings.Contains(prompt, "先接住对方的情绪") {
		t.Fatalf("expected companion prompt with traits and playbook skill, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "原生联网与核心能力声明") {
		t.Fatalf("expected redundant native web_search boilerplate removed from prompt, got:\n%s", prompt)
	}

	// Companion with default empty allowed_tools receives the 9 Native Core tools (including web_search, read_url, mount_mcp)
	tools := filterToolsForBot(&companionBot, []store.Skill{playbook})
	if len(tools) != len(NativeCoreTools()) || !hasToolNamed(tools, "web_search") || !hasToolNamed(tools, "mount_mcp") {
		t.Fatalf("expected %d Native Core tools for companion bot, got %d tools", len(NativeCoreTools()), len(tools))
	}

	workerBot := store.Bot{
		ID:           "bot_ops",
		Name:         "运维官",
		RoleTitle:    "服务器值守专家",
		BotMode:      store.BotModeWorker,
		AllowedTools: `["query_nezha_nodes"]`,
	}
	workerPrompt := buildBotSystemPrompt(&workerBot, nil, nil, nil, "", false)
	if !strings.Contains(workerPrompt, "干练务实") || !strings.Contains(workerPrompt, "结构化展示准则") {
		t.Fatalf("expected worker prompt guidance with structured display hint, got:\n%s", workerPrompt)
	}
	workerTools := filterToolsForBot(&workerBot, nil)
	if len(workerTools) != 1 || workerTools[0].Function.Name != "query_nezha_nodes" {
		t.Fatalf("expected 1 worker tool query_nezha_nodes, got %+v", workerTools)
	}

	// Non-companion Bot with default empty allowed_tools receives Native Core tools + any mounted MCP tools
	defaultWorker := store.Bot{
		ID:           "bot_default_worker",
		Name:         "默认助理",
		BotMode:      store.BotModeHybrid,
		AllowedTools: "[]",
	}
	defaultTools := filterToolsForBot(&defaultWorker, nil)
	if len(defaultTools) != len(NativeCoreTools()) {
		t.Fatalf("expected %d Native Core tools by default when zero MCPs are mounted, got %d", len(NativeCoreTools()), len(defaultTools))
	}
	// Mounting built-in Workbench MCP expands its 4 domain tools on top of Native Core
	workbenchMCP, _ := st.GetSkill(store.BuiltinMCPWorkbenchID)
	if workbenchMCP == nil {
		t.Fatalf("expected seeded builtin mcp_workbench")
	}
	mountedTools := filterToolsForBot(&defaultWorker, []store.Skill{*workbenchMCP})
	if !hasToolNamed(mountedTools, "manage_todos") || !hasToolNamed(mountedTools, "manage_notes") || !hasToolNamed(mountedTools, "manage_stash") {
		t.Fatalf("expected mounted mcp_workbench to expose manage_todos/manage_notes/manage_stash, got %+v", mountedTools)
	}
}

func TestSearchTokenizationGroupRoutingAndManageMemoryTool(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "route_mem.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	// 1. CJK bigram tokenizer handles space-free Chinese queries accurately
	tokens := extractMatchTokens("帮我用食谱技能查查今晚吃什么")
	if len(tokens) == 0 {
		t.Fatalf("expected non-empty CJK tokens")
	}

	chefSkill := store.Skill{
		ID:          "sk_recipe",
		Name:        "每日食谱管家",
		Slug:        "daily_recipe",
		Kind:        "prompt",
		Description: "统筹晚餐食谱和营养搭配",
		Enabled:     true,
	}
	botA := store.Bot{
		ID:        "bot_ops_2",
		Name:      "运维官",
		RoleTitle: "监控与资产巡检",
		BotMode:   store.BotModeWorker,
	}
	botB := store.Bot{
		ID:        "bot_life",
		Name:      "阿暖",
		RoleTitle: "生活陪伴朋友",
		Traits:    "热爱美食烹饪与下厨",
		BotMode:   store.BotModeCompanion,
		SkillIDs:  `["sk_recipe"]`,
	}
	chosen := selectSmartGroupMembersWithSkills(
		[]store.Bot{botA, botB},
		map[string]store.Skill{"sk_recipe": chefSkill},
		"帮我用食谱查查今晚吃什么",
	)
	if len(chosen) == 0 || chosen[0].ID != "bot_life" {
		t.Fatalf("expected bot_life ranked first for CJK cooking/skill query, got %+v", chosen)
	}

	// 2. @Mention order preservation
	eng := NewEngine(st, nil)
	mentioned := eng.resolveMentionedBots("先让 @阿暖 说一句，再请 @运维官 补充", []store.Bot{botA, botB})
	if len(mentioned) != 2 || mentioned[0].ID != "bot_life" || mentioned[1].ID != "bot_ops_2" {
		t.Fatalf("expected mention order [bot_life, bot_ops_2], got %+v", mentioned)
	}

	// 3. Ranked memory selection puts query-relevant and pinned memories on top
	mems := []store.AgentMemory{
		{ID: 1, FactKey: "dns_provider", Category: "infra", Content: "使用 Cloudflare 解析域名", Importance: 3},
		{ID: 2, FactKey: "diet_rule", Category: "preference", Content: "晚上不吃香菜也不喝冰奶茶", Importance: 4},
		{ID: 3, FactKey: "core_alias", Category: "general", Content: "叫我阿星即可", Importance: 2, Pinned: true},
	}
	picked := selectRelevantMemories(mems, "今晚奶茶喝什么", 2)
	if len(picked) != 2 || picked[0].FactKey != "core_alias" || picked[1].FactKey != "diet_rule" {
		t.Fatalf("expected [core_alias (pinned), diet_rule (matched)], got %+v", picked)
	}

	// 4. manage_memory tool + fenced JSON args normalization
	ctxLife := WithToolScope(context.Background(), ToolScope{
		BotID:         "bot_life",
		TimelineBotID: "bot_life",
		MemoryPolicy:  store.MemoryPolicyIsolated,
	})
	upRes, err := eng.ExecuteTool(ctxLife, "manage_memory", "```json\n{\"action\":\"upsert\",\"fact_key\":\"tea_pref\",\"content\":\"喜欢热乌龙茶\",\"category\":\"preference\",\"importance\":5}\n```")
	if err != nil || !strings.Contains(upRes, `"ok":true`) {
		t.Fatalf("manage_memory upsert failed: err=%v res=%s", err, upRes)
	}
	pinRes, err := eng.ExecuteTool(ctxLife, "manage_memory", `{"action":"pin","fact_key":"tea_pref","pinned":true}`)
	if err != nil || !strings.Contains(pinRes, `"pinned":true`) {
		t.Fatalf("manage_memory pin failed: err=%v res=%s", err, pinRes)
	}
	forgetRes, err := eng.ExecuteTool(ctxLife, "manage_memory", `{"action":"forget","fact_key":"tea_pref"}`)
	if err != nil || !strings.Contains(forgetRes, `"forgotten":true`) {
		t.Fatalf("manage_memory forget failed: err=%v res=%s", err, forgetRes)
	}
	listAfter, _ := st.ListScopedMemories([]string{"bot:bot_life"}, "", 10)
	if len(listAfter) != 0 {
		t.Fatalf("expected forgotten memory removed from bot:bot_life scope, got %+v", listAfter)
	}

	// 5. hasLightTarget works with Unified Model Pool without light_api_key
	if err := st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov1",
			Name:     "Unified Pool Provider",
			Protocol: "openai",
			BaseURL:  "https://example.invalid/v1",
			APIKey:   "sk-pool-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov1",
			ModelID:    "gpt-test-mini",
			Enabled:    true,
			Priority:   10,
			IsLight:    true,
		}},
		store.PoolStrategyPriority,
	); err != nil {
		t.Fatalf("save model config failed: %v", err)
	}
	if !eng.hasLightTarget() {
		t.Fatalf("expected hasLightTarget()=true when active model pool node exists")
	}
}

func TestWebhookSkillDefaultsRequiredValidationHTMLStripAndBatchConcurrency(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "skill_v215.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		city := r.URL.Query().Get("city")
		unit := r.URL.Query().Get("unit")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><style>body{}</style><script>alert(1)</script></head><body><h1>" + city + " Weather</h1><p>Temp 22 " + unit + "</p></body></html>"))
	}))
	defer srv.Close()

	sk := store.Skill{
		ID:           "sk_html_weather",
		Name:         "HTML Weather",
		Slug:         "html_weather",
		Kind:         "webhook",
		Description:  "获取天气",
		HTTPMethod:   "GET",
		HTTPURL:      srv.URL + "?city={{city}}&unit={{unit}}",
		ParamsSchema: `[{"key":"city","type":"string","description":"城市","required":true},{"key":"unit","type":"string","description":"单位","default":"Celsius"}]`,
		Enabled:      true,
	}
	_ = st.SaveSkill(sk)

	eng := NewEngine(st, nil)
	eng.allowLocalhostForTest = true

	// 1. Missing required param fails fast cleanly without HTTP call
	_, _, missErr := eng.ExecuteWebhookSkill(context.Background(), sk, map[string]any{})
	if missErr == nil || !strings.Contains(missErr.Error(), "缺少必填参数: city") {
		t.Fatalf("expected missing required parameter error, got: %v", missErr)
	}

	// 2. Omitted optional param uses Default ("Celsius") + HTML stripped + trailing comma JSON repaired
	out, execErr := eng.ExecuteTool(context.Background(), "skill_html_weather", `{"city":"Hangzhou",}`)
	if execErr != nil {
		t.Fatalf("execute skill with default & trailing comma failed: %v", execErr)
	}
	if !strings.Contains(out, "Hangzhou Weather") || !strings.Contains(out, "Celsius") || strings.Contains(out, "<script>") {
		t.Fatalf("expected stripped HTML + Celsius default, got: %s", out)
	}

	// 3. Read-only tool batching check
	if !eng.isReadOnlyTool("skill_html_weather") || !eng.isReadOnlyTool("web_search") || !eng.isReadOnlyTool("datetime_calc") || eng.isReadOnlyTool("manage_memory") {
		t.Fatalf("read-only tool classification mismatch")
	}
}

func TestAssistantToolsTodosNotesNotificationConsultAndDatetime(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "assistant_tools.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webhook" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			_, _ = w.Write([]byte(`{"echo":"` + fmt.Sprint(body["msg"]) + `"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"role":"assistant","content":"已核查节点全部健康运行，平均负载 0.12。"}}]}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, `data: [DONE]`)
		_, _ = fmt.Fprintln(w)
	}))
	defer mockSrv.Close()

	_ = st.SetConfig(store.KeyDeepSeekBaseURL, mockSrv.URL)
	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-consult-test")
	_ = st.SetConfig(store.KeyAIModel, "mock-model")

	_ = st.SaveBot(store.Bot{
		ID:        "bot_muse",
		Name:      "Muse",
		RoleTitle: "个人总助",
		BotMode:   store.BotModeHybrid,
	})
	_ = st.SaveBot(store.Bot{
		ID:        "bot_ops",
		Name:      "Ops专员",
		RoleTitle: "运维工程师",
		BotMode:   store.BotModeWorker,
	})

	eng := NewEngine(st, nil)
	eng.allowLocalhostForTest = true

	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         "bot_muse",
		TimelineBotID: "bot_muse",
		MemoryPolicy:  store.MemoryPolicyIsolated,
	})

	// 1. manage_todos add / list / complete
	addRes, err := eng.ExecuteTool(ctx, "manage_todos", `{"action":"add","title":"整理周报并发送","priority":"high","due_at":"2026-11-20"}`)
	if err != nil || !strings.Contains(addRes, `"ok":true`) {
		t.Fatalf("manage_todos add failed: %v %s", err, addRes)
	}
	compRes, err := eng.ExecuteTool(ctx, "manage_todos", `{"action":"complete","title":"整理周报"}`)
	if err != nil || !strings.Contains(compRes, `"status":"done"`) {
		t.Fatalf("manage_todos complete by title failed: %v %s", err, compRes)
	}

	// 2. manage_notes upsert / get
	noteRes, err := eng.ExecuteTool(ctx, "manage_notes", `{"action":"upsert","title":"杭州周末行程","content":"- 西湖骑行\n- 龙井品茶","category":"plan","pinned":true}`)
	if err != nil || !strings.Contains(noteRes, `"ok":true`) {
		t.Fatalf("manage_notes upsert failed: %v %s", err, noteRes)
	}
	getNoteRes, err := eng.ExecuteTool(ctx, "manage_notes", `{"action":"get","title":"杭州周末"}`)
	if err != nil || !strings.Contains(getNoteRes, "西湖骑行") {
		t.Fatalf("manage_notes get failed: %v %s", err, getNoteRes)
	}

	// 3. send_notification creates Inbox entry
	notifRes, err := eng.ExecuteTool(ctx, "send_notification", `{"title":"服务器续费提醒","markdown_body":"主力节点将于明日到期","priority":"high"}`)
	if err != nil || !strings.Contains(notifRes, `"priority":"high"`) {
		t.Fatalf("send_notification failed: %v %s", err, notifRes)
	}
	notifs, _ := st.ListNotifications(0, 10, false)
	if len(notifs) == 0 || notifs[0].ModuleID != "bot:bot_muse" || notifs[0].Priority != "high" {
		t.Fatalf("expected bot notification saved with module_id bot:bot_muse, got %+v", notifs)
	}

	// 4. datetime_calc add & cron_next
	dtRes, err := eng.ExecuteTool(ctx, "datetime_calc", `{"action":"add","base_time":"2026-11-17 10:00","delta_minutes":45}`)
	if err != nil || !strings.Contains(dtRes, "2026-11-17 10:45") {
		t.Fatalf("datetime_calc add failed: %v %s", err, dtRes)
	}
	cronRes, err := eng.ExecuteTool(ctx, "datetime_calc", `{"action":"cron_next","cron_expr":"30 9 * * *"}`)
	if err != nil || !strings.Contains(cronRes, "09:30") {
		t.Fatalf("datetime_calc cron_next failed: %v %s", err, cronRes)
	}
	diffRes, err := eng.ExecuteTool(ctx, "datetime_calc", `{"action":"diff","base_time":"2026-11-17","target_time":"2026-11-20"}`)
	if err != nil || !strings.Contains(diffRes, `"diff_days":3`) || !strings.Contains(diffRes, `"direction":"future"`) {
		t.Fatalf("datetime_calc diff failed: %v %s", err, diffRes)
	}

	// 5. consult_bot runs target bot cleanly without polluting target's DM history, and blocks cycles
	consultRes, err := eng.ExecuteTool(ctx, "consult_bot", `{"bot_name":"Ops专员","instruction":"汇报当前所有节点状态"}`)
	if err != nil || !strings.Contains(consultRes, "平均负载 0.12") || !strings.Contains(consultRes, `"bot_id":"bot_ops"`) {
		t.Fatalf("consult_bot failed: %v %s", err, consultRes)
	}
	opsMsgs, _ := st.ListBotMessages("bot_ops", 10)
	if len(opsMsgs) != 0 {
		t.Fatalf("expected internal consult_bot not to pollute target bot DM history, got %d messages", len(opsMsgs))
	}
	execLogs, _ := st.ListBotExecutions("bot_muse", 10)
	if len(execLogs) == 0 || execLogs[0].ExecType != "delegate" {
		t.Fatalf("expected delegate audit log on caller bot_muse, got %+v", execLogs)
	}

	// Cycle guard check (Depth >= 2)
	deepCtx := WithToolScope(context.Background(), ToolScope{BotID: "bot_muse", Depth: 2})
	if _, cErr := eng.ExecuteTool(deepCtx, "consult_bot", `{"bot_id":"bot_ops","instruction":"nested"}`); cErr == nil || !strings.Contains(cErr.Error(), "上限") {
		t.Fatalf("expected depth guard error, got: %v", cErr)
	}

	// 6. Webhook JSON string escaping when parameter contains quotes
	postSkill := store.Skill{
		ID:           "sk_json_post",
		Name:         "JSON Echo",
		Slug:         "json_echo",
		Kind:         "webhook",
		HTTPMethod:   "POST",
		HTTPURL:      mockSrv.URL + "/webhook",
		Instruction:  `{"msg":"{{msg}}"}`,
		ParamsSchema: `[{"key":"msg","type":"string","required":true}]`,
		Enabled:      true,
	}
	_, bodyOut, postErr := eng.ExecuteWebhookSkill(context.Background(), postSkill, map[string]any{"msg": `hello "muse"`})
	if postErr != nil || !strings.Contains(bodyOut, `hello \"muse\"`) && !strings.Contains(bodyOut, `hello "muse"`) {
		t.Fatalf("expected escaped JSON webhook body roundtrip, err=%v out=%s", postErr, bodyOut)
	}

	// 7. Webhook SSRF html title check + recency memory scoring
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "general", "old_fact", "旧事实", 3, false)
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "general", "fresh_fact", "刚聊过的新约定", 3, false)
	allMems, _ := st.ListScopedMemories([]string{"bot:bot_muse"}, "", 10)
	selected := selectRelevantMemories(allMems, "完全不相关查询", 1)
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected memory, got %d", len(selected))
	}

	// 8. calculate_eval precision math, percent & error guards
	calcOut, cErr := eng.ExecuteTool(ctx, "calculate_eval", `{"expression":"round((198.5 * 12) * 85%, 2)"}`)
	if cErr != nil || !strings.Contains(calcOut, `"formatted":"2024.7"`) {
		t.Fatalf("calculate_eval failed: err=%v out=%s", cErr, calcOut)
	}
	if _, zErr := eng.ExecuteTool(ctx, "calculate_eval", `{"expression":"10 / 0"}`); zErr == nil {
		t.Fatalf("expected divide-by-zero error")
	}

	// 9. manage_assets summary unified CNY calculation (with live/cached FX conversion)
	_ = st.SaveAsset(store.Asset{ID: "vps1", Name: "HK Node", Category: "compute", Price: "$12 / 年", Status: "生效中", DueDate: "2027-01-01"})
	_ = st.SaveAsset(store.Asset{ID: "sub1", Name: "Cloud Drive", Category: "life", Price: "¥30 / 月", Status: "生效中", DueDate: "2027-01-01"})
	sumOut, sErr := eng.ExecuteTool(ctx, "manage_assets", `{"action":"summary"}`)
	if sErr != nil || !strings.Contains(sumOut, `"currency":"CNY"`) || !strings.Contains(sumOut, `"monthly_cny":`) || !strings.Contains(sumOut, `"priced_items":2`) {
		t.Fatalf("manage_assets summary failed: err=%v out=%s", sErr, sumOut)
	}

	// 10. workspace_briefing & parallel consult_bot targets fan-out + read-only classification
	briefOut, bErr := eng.ExecuteTool(ctx, "workspace_briefing", `{}`)
	if bErr != nil || !strings.Contains(briefOut, `"generated_at"`) {
		t.Fatalf("workspace_briefing failed: err=%v out=%s", bErr, briefOut)
	}
	if !eng.isReadOnlyTool("workspace_briefing") || !eng.isReadOnlyTool("calculate_eval") {
		t.Fatalf("expected workspace_briefing and calculate_eval marked read-only")
	}

	_ = st.SaveBot(store.Bot{ID: "bot_research", Name: "情报官", RoleTitle: "调研助手", BotMode: store.BotModeWorker})
	parOut, pErr := eng.ExecuteTool(ctx, "consult_bot", `{"targets":[{"bot_name":"Ops专员","instruction":"查节点"},{"bot_name":"情报官","instruction":"查资讯"}]}`)
	if pErr != nil || !strings.Contains(parOut, `"parallel_count":2`) {
		t.Fatalf("parallel consult_bot failed: err=%v out=%s", pErr, parOut)
	}

	// 11. batch_add todos + append notes + working context snapshot
	batchTodoOut, btErr := eng.ExecuteTool(ctx, "manage_todos", `{"action":"batch_add","items":[{"title":"准备周五上线评审","priority":"high","due_at":"2026-11-20"},{"title":"同步资产续费清单","priority":"normal"}]}`)
	if btErr != nil || !strings.Contains(batchTodoOut, `"count":2`) {
		t.Fatalf("batch_add todos failed: err=%v out=%s", btErr, batchTodoOut)
	}
	museBot := store.Bot{ID: "bot_muse", Name: "Muse", RoleTitle: "个人助手", BotMode: store.BotModeHybrid}
	workCtx := eng.buildWorkingContextBlock(&museBot, BuiltinTools())
	if !strings.Contains(workCtx, "准备周五上线评审") {
		t.Fatalf("expected live working context snapshot to include open todo, got:\n%s", workCtx)
	}

	appNoteOut, anErr := eng.ExecuteTool(ctx, "manage_notes", `{"action":"append","title":"杭州周末行程","content":"## 补充雨天备选\n- 参观浙江省博物馆"}`)
	if anErr != nil || !strings.Contains(appNoteOut, `"appended":true`) || !strings.Contains(appNoteOut, "龙井品茶") || !strings.Contains(appNoteOut, "博物馆") {
		t.Fatalf("manage_notes append failed: err=%v out=%s", anErr, appNoteOut)
	}

	// 12. recall_memory category/pinned filtering + manage_memory consolidate deduplication
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "preference", "drink_copy_a", "每天早晨习惯喝一杯冰美式不加糖", 4, false)
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "preference", "drink_copy_b", "每天早晨习惯喝一杯冰美式不加糖。", 5, true)
	conOut, cmErr := eng.ExecuteTool(ctx, "manage_memory", `{"action":"consolidate"}`)
	if cmErr != nil || !strings.Contains(conOut, `"merged":1`) {
		t.Fatalf("manage_memory consolidate failed: err=%v out=%s", cmErr, conOut)
	}
	pinRecallOut, prErr := eng.ExecuteTool(ctx, "recall_memory", `{"category":"preference","pinned_only":true,"query":"冰美式"}`)
	if prErr != nil || !strings.Contains(pinRecallOut, `"count":1`) || !strings.Contains(pinRecallOut, "冰美式") {
		t.Fatalf("recall_memory category/pinned filter failed: err=%v out=%s", prErr, pinRecallOut)
	}

	// 13. manage_tasks name lookup + next_runs preview + structured error hint
	_ = st.SaveTask(store.Task{ID: "t_daily_report", Name: "每日工作晚报", Type: "bot_prompt", BotID: "bot_muse", CronExpr: "0 21 * * *", Enabled: true, Params: "{}"})
	upTaskOut, utErr := eng.ExecuteTool(ctx, "manage_tasks", `{"action":"upsert","name":"每日工作晚报","cron_expr":"30 20 * * *"}`)
	if utErr != nil || !strings.Contains(upTaskOut, `"updated":true`) || !strings.Contains(upTaskOut, `"next_runs"`) {
		t.Fatalf("manage_tasks name upsert + next_runs failed: err=%v out=%s", utErr, upTaskOut)
	}
	errPayload := toolErrorPayload("manage_todos", fmt.Errorf("未找到目标待办条目"))
	if !strings.Contains(errPayload, `"hint"`) || !strings.Contains(errPayload, `"retryable":false`) {
		t.Fatalf("unexpected toolErrorPayload output: %s", errPayload)
	}

	// 14. trimContextMessages preserves system prompt + valid user turn boundary, canonicalSearchURL normalizes duplicates
	hist := []apiMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "", ToolCalls: []apiToolCall{{ID: "c1"}}},
		{Role: "tool", Content: "res1", ToolCallID: "c1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
	}
	trimmed, ok := trimContextMessages(hist)
	if !ok || trimmed[0].Role != "system" || trimmed[1].Role != "user" {
		t.Fatalf("expected clean trim starting with system+user, ok=%v trimmed=%+v", ok, trimmed)
	}
	if canonicalSearchURL("https://www.example.com/docs/") != canonicalSearchURL("http://example.com/docs") {
		t.Fatalf("expected canonicalSearchURL to normalize scheme/www/trailing slash")
	}
}

func TestDynamicMCPAutoMountAndRemoteMCPProtocol(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "mcp_dynamic.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	eng := NewEngine(st, nil)
	eng.allowLocalhostForTest = true

	// 1. Bot starts with 0 mounted MCPs
	bot := store.Bot{
		ID:           "bot_dynamic",
		Name:         "Muse",
		RoleTitle:    "全能助手",
		BotMode:      store.BotModeHybrid,
		AllowedTools: "[]",
		SkillIDs:     "[]",
	}
	if err := st.SaveBot(bot); err != nil {
		t.Fatalf("save bot: %v", err)
	}

	// 2. Chat mentions待办 & 哪吒探针 -> autoMountRelevantMCPs mounts mcp_workbench & mcp_nezha permanently
	mounted := eng.autoMountRelevantMCPs(&bot, "帮我记个待办明天排查一下哪吒探针服务器负载")
	if len(mounted) != 2 {
		t.Fatalf("expected 2 auto-mounted MCPs (workbench + nezha), got %+v", mounted)
	}
	reloadedBot, _ := st.GetBot("bot_dynamic")
	if reloadedBot == nil || !strings.Contains(reloadedBot.SkillIDs, store.BuiltinMCPWorkbenchID) || !strings.Contains(reloadedBot.SkillIDs, store.BuiltinMCPNezhaID) {
		t.Fatalf("expected permanent persistence of mcp_workbench and mcp_nezha on bot, got %s", reloadedBot.SkillIDs)
	}

	// 3. Model-invoked mount_mcp permanently mounts mcp_assets
	ctx := WithToolScope(context.Background(), ToolScope{
		BotID:         "bot_dynamic",
		TimelineBotID: "bot_dynamic",
	})
	mountOut, mErr := eng.ExecuteTool(ctx, "mount_mcp", `{"mcp_id":"mcp_assets"}`)
	if mErr != nil || !strings.Contains(mountOut, "manage_assets") {
		t.Fatalf("mount_mcp failed: err=%v out=%s", mErr, mountOut)
	}
	reloadedBot2, _ := st.GetBot("bot_dynamic")
	if !strings.Contains(reloadedBot2.SkillIDs, store.BuiltinMCPAssetsID) {
		t.Fatalf("expected mcp_assets persisted on bot, got %s", reloadedBot2.SkillIDs)
	}

	// 4. Mock standard Remote MCP Server (JSON-RPC 2.0 initialize -> tools/list -> tools/call)
	mcpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc mcpJSONRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&rpc)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sess_test_123")
		switch rpc.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"search_docs","description":"检索 DeepWiki 架构文档","inputSchema":{"type":"object","properties":{"repo":{"type":"string"}},"required":["repo"]}}]}}`))
		case "tools/call":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"DeepWiki 文档摘要：架构正常"}],"isError":false}}`))
		}
	}))
	defer mcpSrv.Close()

	parsedMCPs, pErr := ParseMCPServersJSON(fmt.Sprintf(`{"mcpServers":{"deepwiki":{"url":%q,"description":"DeepWiki 仓库知识库"}}}`, mcpSrv.URL))
	if pErr != nil || len(parsedMCPs) != 1 {
		t.Fatalf("ParseMCPServersJSON failed: err=%v out=%+v", pErr, parsedMCPs)
	}
	remoteMCP := parsedMCPs[0]
	discovered, dErr := eng.DiscoverMCPServerTools(ctx, remoteMCP)
	if dErr != nil || len(discovered) != 1 || discovered[0].Name != "search_docs" {
		t.Fatalf("DiscoverMCPServerTools failed: err=%v tools=%+v", dErr, discovered)
	}
	rawSchema, _ := json.Marshal(discovered)
	remoteMCP.ParamsSchema = string(rawSchema)
	if err := st.SaveSkill(remoteMCP); err != nil {
		t.Fatalf("save remote MCP: %v", err)
	}

	// Execute remote MCP tool via ExecuteTool
	callOut, cErr := eng.ExecuteTool(ctx, "mcp_deepwiki__search_docs", `{"repo":"vesper/bothub"}`)
	if cErr != nil || !strings.Contains(callOut, "DeepWiki 文档摘要：架构正常") {
		t.Fatalf("ExecuteRemoteMCPTool failed: err=%v out=%s", cErr, callOut)
	}

	// 5. No-internet excuse detection
	if !containsNoInternetExcuse("抱歉，我目前没有联网方式，无法查询今日新闻。") {
		t.Fatalf("expected containsNoInternetExcuse to detect excuse")
	}

	// 6. Connected Apps (GitHub / Cloudflare / RSS / WebDAV / Email) auto-mount + execution + secret masking
	mockAppSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/gh/user/repos"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"full_name":"YN1753/BotHub","description":"Personal AI BotHub","stargazers_count":42,"open_issues_count":1,"language":"Go","html_url":"https://github.com/YN1753/BotHub"}]`))
		case strings.HasPrefix(r.URL.Path, "/gh/repos/YN1753/BotHub/issues"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"number":88,"title":"Add App Connectors","html_url":"https://github.com/YN1753/BotHub/issues/88"}`))
		case strings.HasPrefix(r.URL.Path, "/cf/zones"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"zone_123456789012345678901234567890","name":"starryheavens.uk","status":"active"}]}`))
		case strings.HasPrefix(r.URL.Path, "/rss/feed.xml"):
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Tech Radar</title><item><title>Go 1.26 Released</title><link>https://go.dev/blog/go1.26</link><description>Fast and clean.</description></item></channel></rss>`))
		case strings.HasPrefix(r.URL.Path, "/dav/"):
			if r.Method == "PROPFIND" {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusMultiStatus)
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/dav/notes.md</D:href><D:propstat><D:prop><D:displayname>notes.md</D:displayname><D:getcontentlength>128</D:getcontentlength></D:prop></D:propstat></D:response></D:multistatus>`))
			} else if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusCreated)
			} else if r.Method == http.MethodGet {
				_, _ = w.Write([]byte("# Cloud Note\nHello WebDAV"))
			}
		case strings.HasPrefix(r.URL.Path, "/space/api/v1/bridge/overview"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vesper-space","stats":{"totalWorks":9,"publicWorks":8,"privateWorks":1,"totalComments":4},"storage":{"alistMountPath":"/space"}}`))
		case strings.HasPrefix(r.URL.Path, "/space/api/v1/works"):
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"work":{"id":"vs-test-1","title":"Sunset Coast","category":"Travel","visibility":"public"},"imagebed":{"url":"https://space.starryheavens.uk/api/v1/media/2026/11/vs-test-1.jpg","markdown":"![Sunset Coast](https://space.starryheavens.uk/api/v1/media/2026/11/vs-test-1.jpg)","alistPath":"/space/2026/11/vs-test-1.jpg"}}`))
			} else if r.Method == http.MethodPatch {
				_, _ = w.Write([]byte(`{"work":{"id":"vs-test-1","title":"Sunset Coast Revised","category":"Travel","visibility":"public","curator_note":"Golden hour"}}`))
			} else {
				_, _ = w.Write([]byte(`{"works":[{"id":"vs-test-1","title":"Sunset Coast","category":"Travel"}],"total":1}`))
			}
		}
	}))
	defer mockAppSrv.Close()

	// Enable GitHub, Cloudflare, RSS, and WebDAV connectors
	ghSk, _ := st.GetSkill(store.BuiltinMCPAppGitHubID)
	ghSk.Enabled = true
	ghSk.HTTPHeaders = fmt.Sprintf(`{"token":"ghp_secret1234567890","default_owner":"YN1753","api_base":%q}`, mockAppSrv.URL+"/gh")
	_ = st.SaveSkill(*ghSk)

	// Verify secret masking & preservation
	maskedGH := ghSk.MaskedForPublic()
	if strings.Contains(maskedGH.HTTPHeaders, "ghp_secret1234567890") || !strings.Contains(maskedGH.HTTPHeaders, "****") {
		t.Fatalf("expected token to be masked in MaskedForPublic, got %s", maskedGH.HTTPHeaders)
	}
	mergedHeaders := store.MergePreservingMaskedSecrets(ghSk.HTTPHeaders, maskedGH.HTTPHeaders)
	if !strings.Contains(mergedHeaders, "ghp_secret1234567890") {
		t.Fatalf("expected MergePreservingMaskedSecrets to preserve raw token, got %s", mergedHeaders)
	}

	// Auto-mount GitHub connector when chat mentions GitHub PR / Issue
	mountedGH := eng.autoMountRelevantMCPs(&bot, "帮我看看我 GitHub 仓库的 Issue 和 PR")
	if len(mountedGH) == 0 || mountedGH[0].ID != store.BuiltinMCPAppGitHubID {
		t.Fatalf("expected mcp_app_github to auto-mount on GitHub mention, got %+v", mountedGH)
	}

	ghReposOut, gErr := eng.ExecuteTool(ctx, "github_connector", `{"action":"list_repos"}`)
	if gErr != nil || !strings.Contains(ghReposOut, "YN1753/BotHub") {
		t.Fatalf("github_connector list_repos failed: err=%v out=%s", gErr, ghReposOut)
	}
	ghIssueOut, giErr := eng.ExecuteTool(ctx, "github_connector", `{"action":"create_issue","repo":"BotHub","title":"Add App Connectors","body":"Support GitHub/Gmail/CF"}`)
	if giErr != nil || !strings.Contains(ghIssueOut, `"number":88`) {
		t.Fatalf("github_connector create_issue failed: err=%v out=%s", giErr, ghIssueOut)
	}

	// Cloudflare connector test
	cfSk, _ := st.GetSkill(store.BuiltinMCPAppCloudflareID)
	cfSk.Enabled = true
	cfSk.HTTPHeaders = fmt.Sprintf(`{"api_token":"cf_tok_12345678","default_zone":"starryheavens.uk","api_base":%q}`, mockAppSrv.URL+"/cf")
	_ = st.SaveSkill(*cfSk)
	cfZonesOut, cfErr := eng.ExecuteTool(ctx, "cloudflare_connector", `{"action":"list_zones"}`)
	if cfErr != nil || !strings.Contains(cfZonesOut, "starryheavens.uk") {
		t.Fatalf("cloudflare_connector list_zones failed: err=%v out=%s", cfErr, cfZonesOut)
	}

	// RSS connector test
	rssSk, _ := st.GetSkill(store.BuiltinMCPAppRSSID)
	rssSk.Enabled = true
	rssSk.HTTPHeaders = fmt.Sprintf(`{"feeds":"TechRadar|%s/rss/feed.xml"}`, mockAppSrv.URL)
	_ = st.SaveSkill(*rssSk)
	rssOut, rErr := eng.ExecuteTool(ctx, "rss_connector", `{"action":"fetch_latest"}`)
	if rErr != nil || !strings.Contains(rssOut, "Go 1.26 Released") {
		t.Fatalf("rss_connector fetch_latest failed: err=%v out=%s", rErr, rssOut)
	}

	// WebDAV connector test
	davSk, _ := st.GetSkill(store.BuiltinMCPAppWebDAVID)
	davSk.Enabled = true
	davSk.HTTPHeaders = fmt.Sprintf(`{"base_url":%q,"username":"u","password":"p"}`, mockAppSrv.URL+"/dav/")
	_ = st.SaveSkill(*davSk)
	davListOut, dlErr := eng.ExecuteTool(ctx, "webdav_connector", `{"action":"list_files","path":"/"}`)
	if dlErr != nil || !strings.Contains(davListOut, "notes.md") {
		t.Fatalf("webdav_connector list_files failed: err=%v out=%s", dlErr, davListOut)
	}
	davWriteOut, dwErr := eng.ExecuteTool(ctx, "webdav_connector", `{"action":"write_file","path":"/notes.md","content":"# Updated"}`)
	if dwErr != nil || !strings.Contains(davWriteOut, `"ok":true`) {
		t.Fatalf("webdav_connector write_file failed: err=%v out=%s", dwErr, davWriteOut)
	}

	// Vesper Space connector test
	spSk, _ := st.GetSkill(store.BuiltinMCPAppSpaceID)
	spSk.Enabled = true
	spSk.HTTPHeaders = fmt.Sprintf(`{"base_url":%q,"bridge_token":"vs_bridge_secret_token","default_artist":"迟暮"}`, mockAppSrv.URL+"/space")
	_ = st.SaveSkill(*spSk)
	maskedSP := spSk.MaskedForPublic()
	if strings.Contains(maskedSP.HTTPHeaders, "vs_bridge_secret_token") || !strings.Contains(maskedSP.HTTPHeaders, "****") {
		t.Fatalf("expected bridge_token to be masked in MaskedForPublic, got %s", maskedSP.HTTPHeaders)
	}
	mountedSP := eng.autoMountRelevantMCPs(&bot, "帮我把这张图片上传到 Vesper Space 图床并返回 Markdown 外链")
	if len(mountedSP) == 0 || mountedSP[0].ID != store.BuiltinMCPAppSpaceID {
		t.Fatalf("expected mcp_app_space to auto-mount on Vesper Space mention, got %+v", mountedSP)
	}
	spOvOut, spOvErr := eng.ExecuteTool(ctx, "vesper_space_connector", `{"action":"overview"}`)
	if spOvErr != nil || !strings.Contains(spOvOut, `"totalWorks":9`) {
		t.Fatalf("vesper_space_connector overview failed: err=%v out=%s", spOvErr, spOvOut)
	}
	spUpOut, spUpErr := eng.ExecuteTool(ctx, "vesper_space_connector", `{"action":"upload_work","title":"Sunset Coast","src":"https://example.com/photo.jpg","category":"Travel"}`)
	if spUpErr != nil || !strings.Contains(spUpOut, "vs-test-1.jpg") {
		t.Fatalf("vesper_space_connector upload_work failed: err=%v out=%s", spUpErr, spUpOut)
	}
	spPatchOut, spPatchErr := eng.ExecuteTool(ctx, "vesper_space_connector", `{"action":"update_work","work_id":"vs-test-1","title":"Sunset Coast Revised","curator_note":"Golden hour"}`)
	if spPatchErr != nil || !strings.Contains(spPatchOut, "Sunset Coast Revised") {
		t.Fatalf("vesper_space_connector update_work failed: err=%v out=%s", spPatchErr, spPatchOut)
	}
}

func TestClaudeMemoryTwoZonePromptIndexAndAutoDream(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "claude_memory.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Seed memories across Claude's 4 canonical types + legacy categories
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "feedback", "no_emoji_push", "严禁在推送或聊天中使用任何 Emoji", 5, true)
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "user", "stack_pref", "主力后端用 Go，移动端用 Kotlin Compose", 4, false)
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "project", "bothub_v2", "正在演进 BotHub 的 MCP 与连接器体系", 4, false)
	_ = st.SaveAgentMemoryDetailed("bot:bot_muse", "general", "old_ref_tail", "长尾参考信息：旧文档地址", 2, false)

	allMems, _ := st.ListScopedMemories([]string{"bot:bot_muse"}, "", 20)
	detailed := allMems[:3] // leave the 4th item as a long-tail index pointer

	bot := store.Bot{
		ID:           "bot_muse",
		Name:         "Muse",
		RoleTitle:    "全能助手",
		BotMode:      store.BotModeHybrid,
		SystemPrompt: "你是 Muse。",
	}
	prompt := buildBotSystemPromptWithMemoryIndex(&bot, nil, detailed, allMems, nil, "架构讨论", false, "")

	// 1. Two-Zone Prompt Check: Static identity/rules MUST precede Dynamic [当前运行上下文] and [长期记忆索引 (MEMORY.md)]
	idxPersona := strings.Index(prompt, "[人格定位与交流方式]")
	idxRuntime := strings.Index(prompt, "[当前运行上下文]")
	idxMemDoc := strings.Index(prompt, "[长期记忆索引 (MEMORY.md)]")
	if idxPersona < 0 || idxRuntime < 0 || idxMemDoc < 0 || !(idxPersona < idxRuntime && idxRuntime < idxMemDoc) {
		t.Fatalf("expected Two-Zone ordering (persona < runtime < MEMORY.md), got indices %d, %d, %d", idxPersona, idxRuntime, idxMemDoc)
	}
	// 2. Layered MEMORY.md Check: feedback section before user/project, plus long-tail index pointer
	if !strings.Contains(prompt, "## 纠偏与工作纪律 (feedback)") || !strings.Contains(prompt, "## 长尾记忆索引") || !strings.Contains(prompt, "`old_ref_tail`") {
		t.Fatalf("expected layered MEMORY.md sections + tail index, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "不要说出“根据我的记忆库”") {
		t.Fatalf("expected natural memory integration boundary in prompt")
	}

	// 3. AutoDream 4-Phase Consolidation with mock LLM resolving a contradiction & normalizing a date
	dreamLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"upserts\":[{\"category\":\"feedback\",\"fact_key\":\"deploy_style\",\"content\":\"2026-11-23 起每次改完代码直接打包并推送 tag 自动部署\",\"importance\":5,\"pinned\":false}],\"forgets\":[\"old_ref_tail\"],\"summary\":\"提炼了自动发版偏好并清理了 1 条过时参考\"}"}}]}`))
	}))
	defer dreamLLM.Close()

	_ = st.SetConfig(store.KeyLightAPIKey, "sk-test-dream")
	_ = st.SetConfig(store.KeyLightBaseURL, dreamLLM.URL)
	_ = st.SetConfig(store.KeyLightModel, "test-light")

	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:bot_muse",
		BotID:     "bot_muse",
		MsgKind:   "chat",
		Role:      "user",
		Content:   "以后完成了之后自己打包并推送 tag 就行",
	})
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:bot_muse",
		BotID:     "bot_muse",
		MsgKind:   "chat",
		Role:      "assistant",
		Content:   "好的，之后完成后都会直接打包并推送 tag。",
	})

	eng := NewEngine(st, nil)
	eng.allowLocalhostForTest = true
	ctx := WithToolScope(context.Background(), ToolScope{BotID: "bot_muse", TimelineBotID: "bot_muse"})
	dreamOut, dErr := eng.ExecuteTool(ctx, "manage_memory", `{"action":"dream"}`)
	if dErr != nil || !strings.Contains(dreamOut, `"upserted":1`) || !strings.Contains(dreamOut, `"forgotten":1`) {
		t.Fatalf("manage_memory(dream) failed: err=%v out=%s", dErr, dreamOut)
	}

	afterMems, _ := st.ListScopedMemories([]string{"bot:bot_muse"}, "", 20)
	hasDeployStyle, hasOldTail := false, false
	for _, m := range afterMems {
		if m.FactKey == "deploy_style" && m.Category == "feedback" {
			hasDeployStyle = true
		}
		if m.FactKey == "old_ref_tail" {
			hasOldTail = true
		}
	}
	if !hasDeployStyle || hasOldTail {
		t.Fatalf("expected AutoDream to upsert deploy_style(feedback) and forget old_ref_tail, got %+v", afterMems)
	}
}
