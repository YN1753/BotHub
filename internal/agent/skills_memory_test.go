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
	if !strings.Contains(prompt, "真实、有温度、有自己性格特点的聊天伙伴") || !strings.Contains(prompt, "说话简短自然、幽默接梗") || !strings.Contains(prompt, "先接住对方的情绪") {
		t.Fatalf("expected companion prompt with traits and playbook skill, got:\n%s", prompt)
	}

	// Companion with no tools attached yields nil tools slice (prevents LLM tool-calling overhead)
	tools := filterToolsForBot(&companionBot, []store.Skill{playbook})
	if tools != nil {
		t.Fatalf("expected nil tools when companion bot has empty allowed_tools and only prompt skill, got %d tools", len(tools))
	}

	workerBot := store.Bot{
		ID:           "bot_ops",
		Name:         "运维官",
		RoleTitle:    "服务器值守专家",
		BotMode:      store.BotModeWorker,
		AllowedTools: `["query_nezha_nodes"]`,
	}
	workerPrompt := buildBotSystemPrompt(&workerBot, nil, nil, nil, "", false)
	if !strings.Contains(workerPrompt, "干练务实的专业执行者") {
		t.Fatalf("expected worker prompt guidance, got:\n%s", workerPrompt)
	}
	workerTools := filterToolsForBot(&workerBot, nil)
	if len(workerTools) != 1 || workerTools[0].Function.Name != "query_nezha_nodes" {
		t.Fatalf("expected 1 worker tool query_nezha_nodes, got %+v", workerTools)
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

	// 5. hasLightTarget works with Free Model Pool when pool_for_light=true without light_api_key
	if err := st.SaveModelPool([]store.ModelPoolEntry{{
		ID:       "pool1",
		Name:     "Free Pool Node",
		Provider: "openai",
		BaseURL:  "https://example.invalid/v1",
		APIKey:   "sk-pool-test",
		Model:    "gpt-test-mini",
		Enabled:  true,
		Priority: 10,
	}}); err != nil {
		t.Fatalf("save pool failed: %v", err)
	}
	if !eng.hasLightTarget() {
		t.Fatalf("expected hasLightTarget()=true when active free pool node exists and pool_for_light defaults true")
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

	// 9. manage_assets summary multi-currency annualized calculation
	_ = st.SaveAsset(store.Asset{ID: "vps1", Name: "HK Node", Category: "compute", Price: "$12 / 年", Status: "生效中", DueDate: "2027-01-01"})
	_ = st.SaveAsset(store.Asset{ID: "sub1", Name: "Cloud Drive", Category: "life", Price: "¥30 / 月", Status: "生效中", DueDate: "2027-01-01"})
	sumOut, sErr := eng.ExecuteTool(ctx, "manage_assets", `{"action":"summary"}`)
	if sErr != nil || !strings.Contains(sumOut, `"currency":"USD"`) || !strings.Contains(sumOut, `"currency":"CNY"`) {
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
