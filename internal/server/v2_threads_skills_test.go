package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestV2SkillsThreadsScopedMemoriesAndBackup(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

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

	// 6. Delete thread removes thread & its messages
	reqDelTh := httptest.NewRequest(http.MethodDelete, "/api/v2/bots/"+botID+"/threads/"+th1ID, nil)
	reqDelTh.SetPathValue("id", botID)
	reqDelTh.SetPathValue("threadId", th1ID)
	recDelTh := httptest.NewRecorder()
	s.handleDeleteBotThread(recDelTh, reqDelTh)
	if recDelTh.Code != http.StatusOK {
		t.Fatalf("delete thread failed: %d", recDelTh.Code)
	}
}
