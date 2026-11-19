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
	if b.Model != "auto:free" || b.AllowPaidFallback {
		t.Fatalf("expected default butler to use auto:free with allow_paid_fallback=false, got model=%q paid=%v", b.Model, b.AllowPaidFallback)
	}
	if !strings.Contains(b.AllowedTools, "manage_bots") {
		t.Fatalf("expected default butler to have manage_bots enabled, got %s", b.AllowedTools)
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
	updRes, err := eng.ExecuteTool(ctx, "manage_bots", `{"action":"update_bot","bot_name":"代码审查官","traits":"直接指出边界风险","allow_paid_fallback":true}`)
	if err != nil || !strings.Contains(updRes, `"ok":true`) {
		t.Fatalf("update_bot failed: err=%v out=%s", err, updRes)
	}
	gotBot1, _ := st.GetBot(created1.BotID)
	if gotBot1 == nil || !gotBot1.AllowPaidFallback || !strings.Contains(gotBot1.Traits, "边界风险") {
		t.Fatalf("expected updated traits and allow_paid_fallback=true, got %+v", gotBot1)
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
