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
