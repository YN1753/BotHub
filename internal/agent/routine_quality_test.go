package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestExtractPromptExamplesAndNormalizeOnCreate(t *testing.T) {
	raw := "每天早上分享一条有趣的科普冷知识（比如猫为什么喜欢钻纸箱、章鱼有三颗心脏），简洁有干货。"
	coreGoal, examples := extractPromptExamples(raw)
	if strings.Contains(coreGoal, "钻纸箱") {
		t.Fatalf("expected parenthesized examples stripped from coreGoal, got %q", coreGoal)
	}
	if len(examples) < 2 {
		t.Fatalf("expected extracted examples, got %v", examples)
	}
	foundCat := false
	for _, ex := range examples {
		if strings.Contains(ex, "猫为什么喜欢钻纸箱") {
			foundCat = true
		}
	}
	if !foundCat {
		t.Fatalf("expected cat box example extracted, got %v", examples)
	}

	norm := normalizeRoutinePromptOnCreate(raw)
	if !strings.Contains(norm, "严禁原样照搬") {
		t.Fatalf("expected anti-copy guardrail appended on create, got %q", norm)
	}
	// Idempotent check
	if norm2 := normalizeRoutinePromptOnCreate(norm); norm2 != norm {
		t.Fatalf("expected normalizeRoutinePromptOnCreate idempotent, got %q vs %q", norm2, norm)
	}

	plain := "每天 21:00 汇总今日待办完成情况并给出明日三条重点"
	if got := normalizeRoutinePromptOnCreate(plain); got != plain {
		t.Fatalf("expected prompt without examples unchanged, got %q", got)
	}
}

func TestIsMonitoringRoutineClassification(t *testing.T) {
	if !isMonitoringRoutine("服务器节点探针巡检", "检查哪吒探针服务器负载和离线异常") {
		t.Fatalf("expected server monitoring routine classified as monitoring")
	}
	if isMonitoringRoutine("每日冷知识分享", "每天推送一条反常识科普冷知识，比如物理学或生物学") {
		t.Fatalf("expected trivia routine classified as content, not monitoring")
	}
	if isMonitoringRoutine("每日技术简报", "汇总服务器和AI领域最新趋势推荐") {
		t.Fatalf("expected briefing/recommendation routine classified as content")
	}
}

func TestEvaluateRoutineDraftExampleEchoAndPastSimilarity(t *testing.T) {
	examples := []string{"猫为什么喜欢钻纸箱", "章鱼有三颗心脏"}
	past := []store.Notification{
		{
			ID:           1,
			ModuleID:     "task_trivia",
			Title:        "[Muse] 每日冷知识",
			MarkdownBody: "蜂蜜是地球上极少数几乎不会腐败的天然食物，因为极低水分活度与天然过氧化氢抑制了细菌繁殖。",
			CreatedAt:    "2026-11-20 09:00:00",
		},
	}

	// 1. Verbatim / near-verbatim echo of user's example
	echoCheck := evaluateRoutineDraft("今天的冷知识：猫为什么喜欢钻纸箱？因为封闭空间能降低猫的皮质醇压力。", past, examples)
	if echoCheck.EchoedExample == "" || !echoCheck.needsRewrite(false) {
		t.Fatalf("expected example echo detected and rewrite triggered, got %+v", echoCheck)
	}

	// 2. Cross-day near-duplicate of yesterday's push
	repeatCheck := evaluateRoutineDraft("蜂蜜是地球上极少数几乎不会变质腐败的天然食物，极低水分活度和天然过氧化氢能有效抑制细菌繁殖。", past, examples)
	if repeatCheck.MaxPastSim < 0.38 || !repeatCheck.needsRewrite(false) {
		t.Fatalf("expected cross-day similarity caught, got %+v", repeatCheck)
	}

	// 3. Fresh, non-repeating content passes cleanly
	freshCheck := evaluateRoutineDraft("深海灯塔水母在受到环境压力或衰老时，可以通过细胞转分化逆转生命周期回到水螅体阶段。", past, examples)
	if freshCheck.EchoedExample != "" || freshCheck.needsRewrite(false) {
		t.Fatalf("expected fresh content to pass cleanly, got %+v", freshCheck)
	}
}

func TestRunBotRoutineDetachesChatHistoryInjectsPastLedgerAndRewritesOnRepeat(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "routine_quality_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	var (
		mu       sync.Mutex
		reqCount int
		seenMsgs [][]apiMessage
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []apiMessage `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		reqCount++
		cur := reqCount
		seenMsgs = append(seenMsgs, body.Messages)
		mu.Unlock()

		var replyText string
		if cur == 1 {
			// Draft 1 lazily echoes the user's example
			replyText = "冷知识时间：猫为什么喜欢钻纸箱？因为纸箱能带来安全感并保温。"
		} else {
			// Draft 2 (rewrite) produces genuinely fresh content
			replyText = "南极洲冰盖下的沃斯托克湖已被厚达四公里的冰层封存超过一千五百万年，依然保持液态并存在极端嗜冷微生物。"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"role": "assistant", "content": replyText}},
			},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", string(chunk))
	}))
	defer srv.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	if err := st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov_test",
			Name:     "Mock",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  srv.URL + "/v1",
			APIKey:   "sk-routine-quality-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov_test",
			ModelID:    "mock-chat",
			Tier:       store.ModelTierFree,
			Enabled:    true,
			Priority:   10,
		}},
		store.PoolStrategyConfig{},
	); err != nil {
		t.Fatalf("SaveModelConfig: %v", err)
	}

	_ = st.SaveBot(store.Bot{
		ID:      "bot_muse",
		Name:    "Muse",
		BotMode: store.BotModeWorker,
		Model:   "auto:free",
	})

	// Seed a chat room message with a unique marker that MUST NOT leak into background routine runs
	_, _ = st.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:bot_muse",
		BotID:     "bot_muse",
		Role:      "user",
		Content:   "CHAT_ONLY_SECRET_EXAMPLE_MARKER_999 每天给我推点冷知识，比如猫为什么喜欢钻纸箱",
	})

	// Seed yesterday's notification for this task
	_, _ = st.CreateNotification(
		"task_daily_trivia",
		"[Muse] 每日冷知识",
		"昨日蜂蜜冷知识",
		"蜂蜜是地球上极少数几乎不会腐败的天然食物，因为极低水分活度抑制了细菌繁殖。",
		"normal",
	)

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := store.WithTaskQuota(context.Background(), "task_daily_trivia", 0)
	reply, err := eng.RunBotRoutine(ctx, "bot_muse", "每天推送一条硬核冷知识（比如猫为什么喜欢钻纸箱）", "每日冷知识")
	if err != nil {
		t.Fatalf("RunBotRoutine failed: %v", err)
	}
	if !strings.Contains(reply, "沃斯托克湖") {
		t.Fatalf("expected rewritten fresh reply after example-echo interception, got %q", reply)
	}

	mu.Lock()
	defer mu.Unlock()
	if reqCount != 2 {
		t.Fatalf("expected 2 LLM calls (initial draft + quality rewrite), got %d", reqCount)
	}
	// Verify Draft 1 request had zero chat-room history leakage and included past push ledger + example blocklist
	var firstTurnAllText strings.Builder
	for _, m := range seenMsgs[0] {
		firstTurnAllText.WriteString(m.Content)
		firstTurnAllText.WriteString("\n")
	}
	all1 := firstTurnAllText.String()
	if strings.Contains(all1, "CHAT_ONLY_SECRET_EXAMPLE_MARKER_999") {
		t.Fatalf("chat history leaked into RunBotRoutine context")
	}
	if !strings.Contains(all1, "历史已推送去重账本") || !strings.Contains(all1, "蜂蜜是地球上极少数") {
		t.Fatalf("expected past notification ledger injected into routine prompt, got:\n%s", all1)
	}
	if !strings.Contains(all1, "严禁原样照搬的用户举例清单") {
		t.Fatalf("expected extracted examples guard injected into routine prompt, got:\n%s", all1)
	}
	// Verify Draft 2 request included rewrite instruction
	var secondTurnAllText strings.Builder
	for _, m := range seenMsgs[1] {
		secondTurnAllText.WriteString(m.Content)
	}
	if !strings.Contains(secondTurnAllText.String(), "系统质量拦截强制重写") {
		t.Fatalf("expected rewrite directive on second call")
	}
}

func TestRunBotRoutineMonitoringSuppressesUnchangedAlert(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "routine_monitor_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	sameAlert := "告警：节点 hk-edge-01 持续离线，CPU 与网络探针均无响应，请尽快排查重启。"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{
				{"delta": map[string]any{"role": "assistant", "content": sameAlert}},
			},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", string(chunk))
	}))
	defer srv.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	_ = st.SaveModelConfig(
		[]store.ModelProvider{{
			ID:       "prov_mon",
			Name:     "Mock",
			Protocol: store.ProviderProtocolOpenAI,
			BaseURL:  srv.URL + "/v1",
			APIKey:   "sk-mon-test",
			Enabled:  true,
		}},
		[]store.PooledModel{{
			ProviderID: "prov_mon",
			ModelID:    "mock-chat",
			Tier:       store.ModelTierFree,
			Enabled:    true,
			Priority:   10,
		}},
		store.PoolStrategyConfig{},
	)
	_ = st.SaveBot(store.Bot{
		ID:      "bot_ops",
		Name:    "Ops",
		BotMode: store.BotModeWorker,
		Model:   "auto:free",
	})
	_, _ = st.CreateNotification("task_node_watch", "[Ops] 节点巡检", sameAlert, sameAlert, "high")

	eng := NewEngine(st, &fakeTaskController{st: st})
	ctx := store.WithTaskQuota(context.Background(), "task_node_watch", 0)
	reply, err := eng.RunBotRoutine(ctx, "bot_ops", "巡检服务器节点健康状态，如有异常告警", "节点巡检")
	if err != nil {
		t.Fatalf("RunBotRoutine monitoring failed: %v", err)
	}
	if strings.TrimSpace(reply) != "" {
		t.Fatalf("expected unchanged monitoring alert to be silently suppressed, got %q", reply)
	}
}
