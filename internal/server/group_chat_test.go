package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestGroupMembershipAnnouncementAndAdhocGroup(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Seed 1v1 conversation history on butler
	_, _ = s.store.AppendAgentMessage(store.AgentMessage{
		SessionID: "bot:butler",
		BotID:     "butler",
		MsgKind:   "chat",
		Role:      "user",
		Content:   "帮我核查一下东京节点下个月续费是否划算",
	})
	id2, _ := s.store.AppendAgentMessage(store.AgentMessage{
		SessionID:   "bot:butler",
		BotID:       "butler",
		SenderBotID: "butler",
		MsgKind:     "chat",
		Role:        "assistant",
		Content:     "建议拉探针哨兵一起看看最近 CPU 与延迟走势再决定是否续费。",
	})

	// 2. Partial update (toggle is_pinned only) must preserve enable_thinking and allowed_tools
	pinPayload, _ := json.Marshal(map[string]any{"is_pinned": false})
	reqPin := httptest.NewRequest(http.MethodPost, "/api/v2/bots/butler", bytes.NewReader(pinPayload))
	reqPin.SetPathValue("id", "butler")
	recPin := httptest.NewRecorder()
	s.handleUpdateBot(recPin, reqPin)
	if recPin.Code != http.StatusOK {
		t.Fatalf("partial pin update failed: %d: %s", recPin.Code, recPin.Body.String())
	}
	butlerAfter, _ := s.store.GetBot("butler")
	if !butlerAfter.EnableThinking || butlerAfter.IsPinned {
		t.Fatalf("expected EnableThinking=true preserved and IsPinned=false, got thinking=%v pinned=%v", butlerAfter.EnableThinking, butlerAfter.IsPinned)
	}

	// 3. Create an ad-hoc group from butler DM inviting sentinel with carry_context=true
	adhocBody, _ := json.Marshal(map[string]any{
		"source_bot_id": "butler",
		"member_ids":    []string{"sentinel"},
		"group_mode":    "mention_only",
		"carry_context": true,
	})
	reqAdhoc := httptest.NewRequest(http.MethodPost, "/api/v2/bots/adhoc-group", bytes.NewReader(adhocBody))
	recAdhoc := httptest.NewRecorder()
	s.handleCreateAdhocGroup(recAdhoc, reqAdhoc)
	if recAdhoc.Code != http.StatusOK {
		t.Fatalf("adhoc-group failed: %d: %s", recAdhoc.Code, recAdhoc.Body.String())
	}
	var adhocResp struct {
		OK  bool      `json:"ok"`
		Bot store.Bot `json:"bot"`
	}
	if err := json.Unmarshal(recAdhoc.Body.Bytes(), &adhocResp); err != nil {
		t.Fatalf("unmarshal adhocResp: %v", err)
	}
	groupID := adhocResp.Bot.ID
	if !adhocResp.Bot.IsGroup || len(adhocResp.Bot.MemberIDList()) != 2 || adhocResp.Bot.GroupMode != "mention_only" {
		t.Fatalf("unexpected adhoc group created: %+v", adhocResp.Bot)
	}

	msgs, _ := s.store.ListBotMessages(groupID, 20)
	if len(msgs) < 2 || !strings.Contains(msgs[1].Content, "已同步") {
		t.Fatalf("expected system bridge context notice in adhoc group, got: %+v", msgs)
	}

	// 4. Invite radar & remove sentinel dynamically via /members
	addBody, _ := json.Marshal(map[string]any{
		"action":  "add",
		"bot_ids": []string{"radar"},
	})
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+groupID+"/members", bytes.NewReader(addBody))
	reqAdd.SetPathValue("id", groupID)
	recAdd := httptest.NewRecorder()
	s.handleUpdateGroupMembers(recAdd, reqAdd)
	if recAdd.Code != http.StatusOK || !strings.Contains(recAdd.Body.String(), "开源雷达") {
		t.Fatalf("add member failed: %d: %s", recAdd.Code, recAdd.Body.String())
	}

	rmBody, _ := json.Marshal(map[string]any{
		"action":  "remove",
		"bot_ids": []string{"sentinel"},
	})
	reqRm := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+groupID+"/members", bytes.NewReader(rmBody))
	reqRm.SetPathValue("id", groupID)
	recRm := httptest.NewRecorder()
	s.handleUpdateGroupMembers(recRm, reqRm)
	if recRm.Code != http.StatusOK || !strings.Contains(recRm.Body.String(), "移出群聊") {
		t.Fatalf("remove member failed: %d: %s", recRm.Code, recRm.Body.String())
	}

	groupUpdated, _ := s.store.GetBot(groupID)
	gotMembers := strings.Join(groupUpdated.MemberIDList(), ",")
	if gotMembers != "butler,radar" {
		t.Fatalf("expected remaining members butler,radar, got %q", gotMembers)
	}

	// 5. Update group announcement & verify system notice generated
	annBody, _ := json.Marshal(map[string]any{
		"announcement": "先查资产续费周期再对比开源替代品",
		"group_mode":   "smart",
	})
	reqAnn := httptest.NewRequest(http.MethodPost, "/api/v2/bots/"+groupID, bytes.NewReader(annBody))
	reqAnn.SetPathValue("id", groupID)
	recAnn := httptest.NewRecorder()
	s.handleUpdateBot(recAnn, reqAnn)
	if recAnn.Code != http.StatusOK {
		t.Fatalf("update announcement failed: %d", recAnn.Code)
	}
	msgsAfterAnn, _ := s.store.ListBotMessages(groupID, 30)
	if !strings.Contains(msgsAfterAnn[len(msgsAfterAnn)-1].Content, "群公告已更新") {
		t.Fatalf("expected announcement system notice, got last msg: %+v", msgsAfterAnn[len(msgsAfterAnn)-1])
	}

	// 6. Single message deletion on butler
	reqDelMsg := httptest.NewRequest(http.MethodDelete, "/api/v2/bots/butler/messages/"+strconv.FormatInt(id2, 10), nil)
	reqDelMsg.SetPathValue("id", "butler")
	reqDelMsg.SetPathValue("msgId", strconv.FormatInt(id2, 10))
	recDelMsg := httptest.NewRecorder()
	s.handleDeleteBotMessage(recDelMsg, reqDelMsg)
	if recDelMsg.Code != http.StatusOK {
		t.Fatalf("delete single message failed: %d: %s", recDelMsg.Code, recDelMsg.Body.String())
	}
	butlerMsgs, _ := s.store.ListBotMessages("butler", 10)
	if len(butlerMsgs) != 1 || butlerMsgs[0].Role != "user" {
		t.Fatalf("expected only user message remaining on butler, got: %+v", butlerMsgs)
	}
}

func TestForwardMessageAndBackupBotsRoundTrip(t *testing.T) {
	s, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Forward a reference message from sentinel into butler
	fwdBody, _ := json.Marshal(map[string]any{
		"from_name": "探针哨兵",
		"content":   "CloudCone LA 内存使用率达到 92%，建议迁移轻负载容器。",
		"note":      "评估是否要降配省钱",
	})
	reqFwd := httptest.NewRequest(http.MethodPost, "/api/v2/bots/butler/forward", bytes.NewReader(fwdBody))
	reqFwd.SetPathValue("id", "butler")
	recFwd := httptest.NewRecorder()
	s.handleForwardBotMessage(recFwd, reqFwd)
	if recFwd.Code != http.StatusOK {
		t.Fatalf("forward failed: %d: %s", recFwd.Code, recFwd.Body.String())
	}
	butlerAfterFwd, _ := s.store.GetBot("butler")
	if butlerAfterFwd.UnreadCount < 1 || !strings.Contains(butlerAfterFwd.LastSummary, "转发") {
		t.Fatalf("expected unread count >= 1 and forward summary on butler, got %+v", butlerAfterFwd)
	}

	// 2. Create custom group + scoped memory, export backup JSON, reset DB, import backup
	grp := store.Bot{
		ID:           "group_backup_test",
		Name:         "灾备应急群",
		RoleTitle:    "备份验证",
		AvatarStyle:  "swarm:gold",
		SystemPrompt: "协同官",
		IsGroup:      true,
		MemberIDs:    `["butler","sentinel"]`,
		GroupMode:    "mention_only",
		Announcement: "每周五检查冷备份",
	}
	if err := s.store.SaveBot(grp); err != nil {
		t.Fatalf("save custom group: %v", err)
	}
	if err := s.store.SaveAgentMemoryWithScope("bot:butler", "schedule", "backup_day", "Friday 03:00"); err != nil {
		t.Fatalf("save scoped memory: %v", err)
	}

	reqExp := httptest.NewRequest(http.MethodGet, "/api/system/export", nil)
	recExp := httptest.NewRecorder()
	s.handleExportBackup(recExp, reqExp)
	if recExp.Code != http.StatusOK {
		t.Fatalf("export backup failed: %d", recExp.Code)
	}

	var exported BackupData
	if err := json.Unmarshal(recExp.Body.Bytes(), &exported); err != nil {
		t.Fatalf("unmarshal backup: %v", err)
	}
	if len(exported.Bots) == 0 {
		t.Fatalf("expected Bots included in backup export")
	}

	// Re-import into a fresh server
	s2, cleanup2 := setupTestServer(t)
	defer cleanup2()
	reqImp := httptest.NewRequest(http.MethodPost, "/api/system/import", bytes.NewReader(recExp.Body.Bytes()))
	recImp := httptest.NewRecorder()
	s2.handleImportBackup(recImp, reqImp)
	if recImp.Code != http.StatusOK {
		t.Fatalf("import backup failed: %d: %s", recImp.Code, recImp.Body.String())
	}

	restoredGrp, _ := s2.store.GetBot("group_backup_test")
	if restoredGrp == nil || !restoredGrp.IsGroup || restoredGrp.Announcement != "每周五检查冷备份" || restoredGrp.GroupMode != "mention_only" {
		t.Fatalf("expected custom group restored completely, got %+v", restoredGrp)
	}
	restoredMems, _ := s2.store.ListAgentMemories("backup_day", 5)
	if len(restoredMems) != 1 || restoredMems[0].Scope != "bot:butler" {
		t.Fatalf("expected scoped memory restored with bot:butler scope, got %+v", restoredMems)
	}
}

