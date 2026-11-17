package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestV2SkillsThreadsScopedMemoriesAndBackup(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()
	s.push = notifier.NewClient(s.store)
	s.agent.SetHub(s.push.Hub())
	s.scheduler = scheduler.NewWithRegistry(s.store, modules.NewDefaultRegistry(s.store, s.push))
	s.scheduler.SetHub(s.push.Hub())
	s.scheduler.SetBotRoutineExecutor(s.agent)

	// 1. Create a Bot with isolated memory policy and companion mode
	botPayload := `{
		"name": "小暖",
		"role_title": "日常陪伴伙伴",
		"bot_mode": "companion",
		"memory_policy": "isolated",
		"traits": "温柔细腻、像微信老朋友一样说话",
		"allowed_tools": "[]"
	}`
	reqBot := httptest.NewRequest(http.MethodPost, "/api/v2/bots", strings.NewReader(botPayload))
	recBot := httptest.NewRecorder()
	s.handleCreateBot(recBot, reqBot)
	if recBot.Code != http.StatusOK {
		t.Fatalf("create bot failed (%d): %s", recBot.Code, recBot.Body.String())
	}
	var botResp struct {
		OK  bool      `json:"ok"`
		Bot store.Bot `json:"bot"`
	}
	_ = json.Unmarshal(recBot.Body.Bytes(), &botResp)
	botID := botResp.Bot.ID
	if botID == "" || botResp.Bot.BotMode != store.BotModeCompanion || botResp.Bot.MemoryPolicy != store.MemoryPolicyIsolated {
		t.Fatalf("unexpected bot fields: %+v", botResp.Bot)
	}

	// 2. Create two topic threads and verify isolated message retrieval per thread
	reqTh1 := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+botID+"/threads", strings.NewReader(`{"title":"深夜闲聊"}`))
	reqTh1.SetPathValue("id", botID)
	recTh1 := httptest.NewRecorder()
	s.handleCreateBotThread(recTh1, reqTh1)
	if recTh1.Code != http.StatusOK {
		t.Fatalf("create thread failed (%d): %s", recTh1.Code, recTh1.Body.String())
	}
	var th1Resp struct {
		Thread store.BotThread `json:"thread"`
	}
	_ = json.Unmarshal(recTh1.Body.Bytes(), &th1Resp)
	th1ID := th1Resp.Thread.ID
	if th1ID == "" {
		t.Fatalf("expected non-empty thread id")
	}

	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		ThreadID:  "",
		Role:      "user",
		Content:   "主时间线消息",
	})
	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		ThreadID:  th1ID,
		Role:      "user",
		Content:   "深夜闲聊专属消息",
	})

	reqMsgsMain := httptest.NewRequest(http.MethodGet, "/api/v2/bots/"+botID+"/messages?thread_id=", nil)
	reqMsgsMain.SetPathValue("id", botID)
	recMsgsMain := httptest.NewRecorder()
	s.handleGetBotMessages(recMsgsMain, reqMsgsMain)
	if !strings.Contains(recMsgsMain.Body.String(), "主时间线消息") || strings.Contains(recMsgsMain.Body.String(), "深夜闲聊专属消息") {
		t.Fatalf("main timeline thread isolation failed: %s", recMsgsMain.Body.String())
	}

	reqMsgsTh1 := httptest.NewRequest(http.MethodGet, "/api/v2/bots/"+botID+"/messages?thread_id="+th1ID, nil)
	reqMsgsTh1.SetPathValue("id", botID)
	recMsgsTh1 := httptest.NewRecorder()
	s.handleGetBotMessages(recMsgsTh1, reqMsgsTh1)
	if !strings.Contains(recMsgsTh1.Body.String(), "深夜闲聊专属消息") || strings.Contains(recMsgsTh1.Body.String(), "主时间线消息") {
		t.Fatalf("topic thread isolation failed: %s", recMsgsTh1.Body.String())
	}

	// Rename thread
	reqRen := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+botID+"/threads/"+th1ID, strings.NewReader(`{"title":"凌晨漫谈"}`))
	reqRen.SetPathValue("id", botID)
	reqRen.SetPathValue("threadId", th1ID)
	recRen := httptest.NewRecorder()
	s.handleUpdateBotThread(recRen, reqRen)
	if recRen.Code != http.StatusOK || !strings.Contains(recRen.Body.String(), "凌晨漫谈") {
		t.Fatalf("rename thread failed: %d %s", recRen.Code, recRen.Body.String())
	}

	// 3. Create, update, and test a custom prompt skill
	reqSk := httptest.NewRequest(http.MethodPost, "/api/v2/skills", strings.NewReader(`{
		"name": "睡眠提醒",
		"kind": "prompt",
		"description": "温和关心作息",
		"instruction": "提醒对方早点休息"
	}`))
	recSk := httptest.NewRecorder()
	s.handleCreateSkill(recSk, reqSk)
	if recSk.Code != http.StatusOK {
		t.Fatalf("create skill failed (%d): %s", recSk.Code, recSk.Body.String())
	}
	var skResp struct {
		Skill store.Skill `json:"skill"`
	}
	_ = json.Unmarshal(recSk.Body.Bytes(), &skResp)
	skID := skResp.Skill.ID

	reqTestSk := httptest.NewRequest(http.MethodPost, "/api/v2/skills/"+skID+"/test", strings.NewReader(`{}`))
	reqTestSk.SetPathValue("id", skID)
	recTestSk := httptest.NewRecorder()
	s.handleTestSkill(recTestSk, reqTestSk)
	if recTestSk.Code != http.StatusOK || !strings.Contains(recTestSk.Body.String(), "就绪") {
		t.Fatalf("test prompt skill failed: %d %s", recTestSk.Code, recTestSk.Body.String())
	}

	// 4. Scoped Memory Save, Filter & Update
	memPayload := fmt.Sprintf(`{"scope":"bot:%s","category":"preference","fact_key":"bedtime","content":"通常一点半睡","importance":4,"pinned":true}`, botID)
	reqMem := httptest.NewRequest(http.MethodPost, "/api/agent/memories", strings.NewReader(memPayload))
	recMem := httptest.NewRecorder()
	s.handleSaveAgentMemory(recMem, reqMem)
	if recMem.Code != http.StatusOK {
		t.Fatalf("save scoped memory failed (%d): %s", recMem.Code, recMem.Body.String())
	}

	reqListMem0 := httptest.NewRequest(http.MethodGet, "/api/agent/memories?scope=bot:"+botID, nil)
	recListMem0 := httptest.NewRecorder()
	s.handleListAgentMemories(recListMem0, reqListMem0)
	var memListResp struct {
		Memories []store.AgentMemory `json:"memories"`
	}
	_ = json.Unmarshal(recListMem0.Body.Bytes(), &memListResp)
	if len(memListResp.Memories) != 1 || !memListResp.Memories[0].Pinned {
		t.Fatalf("expected 1 pinned memory for bot scope, got %+v", memListResp.Memories)
	}

	memID := memListResp.Memories[0].ID
	reqUpdMem := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/agent/memories/%d", memID), strings.NewReader(`{"content":"现在改十二点半睡","importance":5,"pinned":true}`))
	reqUpdMem.SetPathValue("id", fmt.Sprintf("%d", memID))
	recUpdMem := httptest.NewRecorder()
	s.handleUpdateAgentMemory(recUpdMem, reqUpdMem)
	if recUpdMem.Code != http.StatusOK || !strings.Contains(recUpdMem.Body.String(), "十二点半睡") {
		t.Fatalf("update memory failed: %d %s", recUpdMem.Code, recUpdMem.Body.String())
	}

	reqListMem := httptest.NewRequest(http.MethodGet, "/api/agent/memories?scope=bot:"+botID, nil)
	recListMem := httptest.NewRecorder()
	s.handleListAgentMemories(recListMem, reqListMem)
	if !strings.Contains(recListMem.Body.String(), "十二点半睡") {
		t.Fatalf("expected scoped memory filter to return updated memory, got %s", recListMem.Body.String())
	}

	// 5. Export backup & verify skills and scoped memories roundtrip cleanly
	reqExp := httptest.NewRequest(http.MethodGet, "/api/backup/export", nil)
	recExp := httptest.NewRecorder()
	s.handleExportBackup(recExp, reqExp)
	if recExp.Code != http.StatusOK {
		t.Fatalf("export backup failed: %d", recExp.Code)
	}
	var backup BackupData
	if err := json.Unmarshal(recExp.Body.Bytes(), &backup); err != nil {
		t.Fatalf("decode backup failed: %v", err)
	}
	if len(backup.Skills) == 0 || len(backup.Bots) == 0 || len(backup.AgentMemories) == 0 {
		t.Fatalf("expected skills, bots, and memories in backup, got skills=%d bots=%d mems=%d", len(backup.Skills), len(backup.Bots), len(backup.AgentMemories))
	}

	// Re-import into a fresh server
	s2, cleanup2 := setupTestServer(t)
	defer cleanup2()
	rawBackup, _ := json.Marshal(backup)
	reqImp := httptest.NewRequest(http.MethodPost, "/api/backup/import", bytes.NewReader(rawBackup))
	recImp := httptest.NewRecorder()
	s2.handleImportBackup(recImp, reqImp)
	if recImp.Code != http.StatusOK || !strings.Contains(recImp.Body.String(), `"skills_restored":1`) {
		t.Fatalf("import backup failed (%d): %s", recImp.Code, recImp.Body.String())
	}

	// 6. Summarize thread (deterministic fallback when no LLM configured) & prune cold memories
	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:" + botID,
		BotID:     botID,
		ThreadID:  th1ID,
		Role:      "assistant",
		Content:   "记得早点休息，明天还要开早会。",
	})
	reqSum := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+botID+"/threads/"+th1ID+"/summarize", strings.NewReader(`{"save_to_memory":true}`))
	reqSum.SetPathValue("id", botID)
	reqSum.SetPathValue("threadId", th1ID)
	recSum := httptest.NewRecorder()
	s.handleSummarizeBotThread(recSum, reqSum)
	if recSum.Code != http.StatusOK || !strings.Contains(recSum.Body.String(), `"summary"`) {
		t.Fatalf("summarize thread failed: %d %s", recSum.Code, recSum.Body.String())
	}

	// Add cold memories + prune keeping top 2
	for i := 1; i <= 4; i++ {
		_ = s.store.SaveAgentMemoryDetailed("bot:"+botID, "general", fmt.Sprintf("cold_%d", i), fmt.Sprintf("琐碎记录 %d", i), 1, false)
	}
	reqPrune := httptest.NewRequest(http.MethodPost, "/api/agent/memories/prune", strings.NewReader(fmt.Sprintf(`{"scope":"bot:%s","max_keep":2}`, botID)))
	recPrune := httptest.NewRecorder()
	s.handlePruneAgentMemories(recPrune, reqPrune)
	if recPrune.Code != http.StatusOK || !strings.Contains(recPrune.Body.String(), `"removed"`) {
		t.Fatalf("prune memories failed: %d %s", recPrune.Code, recPrune.Body.String())
	}
	remaining, _ := s.store.ListScopedMemories([]string{"bot:" + botID}, "", 20)
	if len(remaining) > 2 {
		t.Fatalf("expected <=2 memories after prune (keeping pinned + top weighted), got %d", len(remaining))
	}

	// 7. Fork thread preserves conversation branch cleanly
	reqFork := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+botID+"/threads/fork", strings.NewReader(fmt.Sprintf(`{"source_thread_id":"%s","title":"早会支线"}`, th1ID)))
	reqFork.SetPathValue("id", botID)
	recFork := httptest.NewRecorder()
	s.handleForkBotThread(recFork, reqFork)
	if recFork.Code != http.StatusOK || !strings.Contains(recFork.Body.String(), "早会支线") || !strings.Contains(recFork.Body.String(), "明天还要开早会") {
		t.Fatalf("fork thread failed (%d): %s", recFork.Code, recFork.Body.String())
	}

	// 8. bot_prompt template registered and scheduled routine runs cleanly with mock upstream
	tpls := s.scheduler.TaskTemplates()
	hasBotPromptTpl := false
	for _, tpl := range tpls {
		if tpl.Type == "bot_prompt" {
			hasBotPromptTpl = true
			break
		}
	}
	if !hasBotPromptTpl {
		t.Fatalf("expected bot_prompt template registered in scheduler")
	}

	// Execute bot_prompt task end-to-end via scheduler.RunNow with mock upstream AI server
	mockAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintln(w, `data: {"choices":[{"delta":{"role":"assistant","content":"早安，记得多喝温水，今天也要顺顺利利。"}}]}`)
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, `data: [DONE]`)
		_, _ = fmt.Fprintln(w)
	}))
	defer mockAI.Close()
	_ = s.store.SetConfig(store.KeyDeepSeekBaseURL, mockAI.URL)
	_ = s.store.SetConfig(store.KeyDeepSeekAPIKey, "sk-routine-test")
	_ = s.store.SetConfig(store.KeyAIModel, "mock-model")

	if err := s.store.SaveTask(store.Task{
		ID:       "routine_morning_warm",
		Name:     "早安问候",
		Type:     "bot_prompt",
		BotID:    botID,
		CronExpr: "0 9 * * *",
		Enabled:  true,
		Params:   `{"prompt":"跟用户打个温柔的早安招呼","notify":"true"}`,
	}); err != nil {
		t.Fatalf("save bot_prompt task failed: %v", err)
	}
	resRun, runErr := s.scheduler.RunNow(t.Context(), "routine_morning_warm")
	if runErr != nil || !strings.Contains(resRun.Summary, "多喝温水") || resRun.ItemsPushed != 1 {
		t.Fatalf("expected bot_prompt routine execution + inbox push, err=%v res=%+v", runErr, resRun)
	}
	execLogs, _ := s.store.ListBotExecutions(botID, 10)
	if len(execLogs) == 0 || execLogs[0].ExecType != "routine" || execLogs[0].Status != "success" {
		t.Fatalf("expected routine recorded in bot_execution_logs, got %+v", execLogs)
	}

	// 9. Personal Assistant Todos & Notes endpoints + backup persistence
	reqTodo := httptest.NewRequest(http.MethodPost, "/api/v2/todos", strings.NewReader(`{"title":"复盘本周监控","priority":"high","due_at":"2026-11-22"}`))
	recTodo := httptest.NewRecorder()
	s.handleCreateTodo(recTodo, reqTodo)
	if recTodo.Code != http.StatusOK || !strings.Contains(recTodo.Body.String(), "复盘本周监控") {
		t.Fatalf("create todo failed: %d %s", recTodo.Code, recTodo.Body.String())
	}

	reqNote := httptest.NewRequest(http.MethodPost, "/api/v2/notes", strings.NewReader(`{"title":"个人助手规范","content":"保持务实","category":"plan"}`))
	recNote := httptest.NewRecorder()
	s.handleCreateNote(recNote, reqNote)
	if recNote.Code != http.StatusOK || !strings.Contains(recNote.Body.String(), "个人助手规范") {
		t.Fatalf("create note failed: %d %s", recNote.Code, recNote.Body.String())
	}

	reqDash := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	recDash := httptest.NewRecorder()
	s.handleDashboard(recDash, reqDash)
	if recDash.Code != http.StatusOK || !strings.Contains(recDash.Body.String(), `"todos"`) || !strings.Contains(recDash.Body.String(), `"notes"`) {
		t.Fatalf("expected todos and notes in dashboard payload, got: %s", recDash.Body.String())
	}

	reqBrief := httptest.NewRequest(http.MethodGet, "/api/v2/briefing", nil)
	recBrief := httptest.NewRecorder()
	s.handleGetBriefing(recBrief, reqBrief)
	if recBrief.Code != http.StatusOK || !strings.Contains(recBrief.Body.String(), `"open_todo_count"`) {
		t.Fatalf("expected briefing payload, got (%d): %s", recBrief.Code, recBrief.Body.String())
	}

	// 10. Delete thread removes thread & its messages
	reqDelTh := httptest.NewRequest(http.MethodDelete, "/api/v2/bots/"+botID+"/threads/"+th1ID, nil)
	reqDelTh.SetPathValue("id", botID)
	reqDelTh.SetPathValue("threadId", th1ID)
	recDelTh := httptest.NewRecorder()
	s.handleDeleteBotThread(recDelTh, reqDelTh)
	if recDelTh.Code != http.StatusOK {
		t.Fatalf("delete thread failed: %d", recDelTh.Code)
	}
}
