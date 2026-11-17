package store

import (
	"path/filepath"
	"testing"
)

func TestTodosNotesAndTaskNoHardcodedButler(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "assistant_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// 1. Tasks without bot_id must stay empty instead of silently defaulting to "butler"
	if err := st.SaveTask(Task{
		ID:       "global_check",
		Name:     "全局巡检",
		Type:     "asset_watcher",
		CronExpr: "0 3 * * *",
		RunAt:    "",
		Enabled:  true,
		Params:   "{}",
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}
	gotTask, err := st.GetTask("global_check")
	if err != nil || gotTask == nil {
		t.Fatalf("get task failed: %v", err)
	}
	if gotTask.BotID != "" {
		t.Fatalf("expected empty BotID without hardcoded fallback, got %q", gotTask.BotID)
	}

	// 2. Todos CRUD & status/priority ordering
	tdLow, err := st.SaveTodo(TodoItem{
		BotID:    "bot_muse",
		Title:    "整理参考书单",
		Priority: TodoPriorityLow,
	})
	if err != nil {
		t.Fatalf("save low todo: %v", err)
	}
	tdHigh, err := st.SaveTodo(TodoItem{
		BotID:    "bot_muse",
		Title:    "续费主力域名",
		Priority: TodoPriorityHigh,
		DueAt:    "2026-11-18",
	})
	if err != nil {
		t.Fatalf("save high todo: %v", err)
	}
	openTodos, err := st.ListTodos("open", "", 20)
	if err != nil || len(openTodos) != 2 {
		t.Fatalf("expected 2 open todos, got %d (%v)", len(openTodos), err)
	}
	if openTodos[0].ID != tdHigh.ID {
		t.Fatalf("expected high priority todo ordered first, got %+v", openTodos)
	}

	// Mark high todo done -> completed_at populated & clear completed removes only done
	tdHigh.Status = TodoStatusDone
	tdHighDone, err := st.SaveTodo(tdHigh)
	if err != nil || tdHighDone.CompletedAt == "" {
		t.Fatalf("expected completed_at populated on done todo, err=%v item=%+v", err, tdHighDone)
	}
	cleared, err := st.ClearCompletedTodos("")
	if err != nil || cleared != 1 {
		t.Fatalf("expected 1 cleared todo, got %d (%v)", cleared, err)
	}
	remainingTodos, _ := st.ListTodos("all", "", 20)
	if len(remainingTodos) != 1 || remainingTodos[0].ID != tdLow.ID {
		t.Fatalf("expected only low todo remaining, got %+v", remainingTodos)
	}

	// 3. WorkNotes CRUD & search
	n1, err := st.SaveNote(WorkNote{
		BotID:    "bot_muse",
		Title:    "本周工作周报",
		Category: "deliverable",
		Content:  "# 总结\n完成个人助手核心升级",
		Pinned:   true,
	})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	notes, err := st.ListNotes("个人助手", "", 10)
	if err != nil || len(notes) != 1 || notes[0].ID != n1.ID {
		t.Fatalf("expected matching note found, got %+v (%v)", notes, err)
	}
	if ok, _ := st.DeleteNote(n1.ID); !ok {
		t.Fatalf("expected DeleteNote to return true")
	}
}
