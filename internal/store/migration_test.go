package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenMigratesLegacyV212DatabaseWithoutErrors(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy_v212.db")
	rawDB, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)", dbPath))
	if err != nil {
		t.Fatalf("open raw sqlite failed: %v", err)
	}

	// Exact v2.12.0 table definitions prior to v2.13.0 schema additions
	legacySQL := `
CREATE TABLE configs (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE tasks (id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', cron_expr TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 1, params TEXT NOT NULL DEFAULT '{}', bot_id TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE agent_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', reasoning_content TEXT NOT NULL DEFAULT '', tool_calls TEXT NOT NULL DEFAULT '', tool_call_id TEXT NOT NULL DEFAULT '', bot_id TEXT NOT NULL DEFAULT '', sender_bot_id TEXT NOT NULL DEFAULT '', msg_kind TEXT NOT NULL DEFAULT 'chat', reply_to_id INTEGER NOT NULL DEFAULT 0, reply_preview TEXT NOT NULL DEFAULT '', model_used TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL);
CREATE TABLE agent_memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category TEXT NOT NULL DEFAULT 'general',
	fact_key TEXT NOT NULL UNIQUE,
	content TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	scope TEXT NOT NULL DEFAULT 'global'
);
CREATE TABLE bots (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	role_title TEXT NOT NULL,
	avatar_style TEXT NOT NULL DEFAULT 'cyan',
	system_prompt TEXT NOT NULL,
	model TEXT NOT NULL DEFAULT 'deepseek-flash',
	enable_thinking INTEGER NOT NULL DEFAULT 1,
	allowed_tools TEXT NOT NULL DEFAULT '[]',
	state TEXT NOT NULL DEFAULT 'idle',
	unread_count INTEGER NOT NULL DEFAULT 0,
	is_pinned INTEGER NOT NULL DEFAULT 0,
	is_builtin INTEGER NOT NULL DEFAULT 0,
	is_group INTEGER NOT NULL DEFAULT 0,
	member_ids TEXT NOT NULL DEFAULT '[]',
	group_mode TEXT NOT NULL DEFAULT 'smart',
	announcement TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 100,
	last_summary TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
INSERT INTO agent_memories (category, fact_key, content, created_at, updated_at, scope)
VALUES ('infra', 'primary_vps', 'Tokyo Node', '2026-11-15 10:00:00', '2026-11-15 10:00:00', 'bot:existing');
INSERT INTO bots (id, name, role_title, state, system_prompt, created_at, updated_at)
VALUES ('existing', 'LegacyBot', 'Ops', 'working', 'Hello', '2026-11-15 10:00:00', '2026-11-15 10:00:00');
`
	if _, err := rawDB.Exec(legacySQL); err != nil {
		rawDB.Close()
		t.Fatalf("seed legacy v2.12 schema failed: %v", err)
	}
	rawDB.Close()

	// Upgrading with store.Open MUST succeed cleanly (previously crashed with "no such column: pinned")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("migrating legacy v2.12 database failed: %v", err)
	}
	defer st.Close()

	// Existing memory must survive AND new bots must be able to reuse same fact_key in different scopes
	if err := st.SaveAgentMemoryDetailed("bot:other", "infra", "primary_vps", "Singapore Node", 5, true); err != nil {
		t.Fatalf("saving same fact_key in new scope after migration failed: %v", err)
	}

	mems, err := st.ListScopedMemories([]string{"bot:existing", "bot:other"}, "", 20)
	if err != nil || len(mems) != 2 {
		t.Fatalf("expected 2 migrated/scoped memories, err=%v mems=%+v", err, mems)
	}
	bot, err := st.GetBot("existing")
	if err != nil || bot == nil || bot.State != "idle" {
		t.Fatalf("expected legacy bot to load and reset crashed working->idle after migration, err=%v bot=%+v", err, bot)
	}
}

func TestBotMessagesQueryPlanUsesCompositeDescendingIndex(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "plan.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	rows, err := st.db.Query(`EXPLAIN QUERY PLAN SELECT id, content FROM agent_messages WHERE bot_id = 'butler' AND thread_id = '' ORDER BY id DESC LIMIT 60`)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()

	var planDetails []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err == nil {
			planDetails = append(planDetails, detail)
		}
	}
	joined := strings.Join(planDetails, " | ")
	if !strings.Contains(joined, "idx_agent_messages_bot_thread_desc") {
		t.Fatalf("expected query plan to use idx_agent_messages_bot_thread_desc, got: %s", joined)
	}
	if strings.Contains(joined, "USE TEMP B-TREE") {
		t.Fatalf("expected zero TEMP B-TREE sort on bot message lookup, got: %s", joined)
	}
}
