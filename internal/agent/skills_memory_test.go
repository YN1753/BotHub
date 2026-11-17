package agent

import (
	"context"
	"encoding/json"
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
	prompt := buildBotSystemPrompt(&companionBot, []store.Skill{playbook}, nil, false)
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
	workerPrompt := buildBotSystemPrompt(&workerBot, nil, nil, false)
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
	if !eng.isReadOnlyTool("skill_html_weather") || !eng.isReadOnlyTool("web_search") || eng.isReadOnlyTool("manage_memory") {
		t.Fatalf("read-only tool classification mismatch")
	}
}
