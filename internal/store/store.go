package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
	DefaultAIModel         = "deepseek-chat"
	DefaultActivePersona   = "grok"

	KeyPushChannel       = "push_channel"
	KeyMorningCity       = "morning_city"
	KeyNezhaURL          = "nezha_url"
	KeyNezhaDBPath       = "nezha_db_path"
	KeyRadarHNMinScore   = "radar_hn_min_score"
	KeyRadarLimit        = "radar_limit"
	KeyCFAPIToken        = "cf_api_token"
	KeyBiliSessdata      = "bili_sessdata"
	KeyDeepSeekAPIKey    = "deepseek_api_key"
	KeyDeepSeekBaseURL   = "deepseek_base_url"
	KeyAIModel           = "ai_model"
	KeyActivePersona     = "active_persona"
	KeyLightAPIKey       = "light_api_key"
	KeyLightBaseURL      = "light_base_url"
	KeyLightModel        = "light_model"
	KeyFreeModelPool     = "free_model_pool"
	KeyPoolForLight      = "pool_for_light"
	KeyPoolFallback      = "pool_fallback"
	KeyAppBearerToken    = "app_bearer_token"
	KeyNezhaWebhookToken = "nezha_webhook_token"
	KeyAdminPassHash     = "admin_pass_hash"
	KeySessionSecret     = "session_secret"

	KeyOIDCEnabled      = "oidc_enabled"
	KeyOIDCIssuer       = "oidc_issuer"
	KeyOIDCClientID     = "oidc_client_id"
	KeyOIDCClientSecret = "oidc_client_secret"
	KeyOIDCAllowedSub   = "oidc_allowed_sub"
)

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_pragma=cache_size(-16000)&_pragma=mmap_size(67108864)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seedDefaults(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS configs (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS module_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	module_id TEXT NOT NULL,
	module_name TEXT NOT NULL,
	trigger_type TEXT NOT NULL,
	status TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	items_pushed INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_module_runs_module ON module_runs(module_id, id DESC);

CREATE TABLE IF NOT EXISTS push_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	module_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	status TEXT NOT NULL,
	response_msg TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_push_logs_id ON push_logs(id DESC);

CREATE TABLE IF NOT EXISTS seen_items (
	source TEXT NOT NULL,
	item_key TEXT NOT NULL,
	title TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	first_seen_at TEXT NOT NULL,
	PRIMARY KEY (source, item_key)
);

CREATE TABLE IF NOT EXISTS assets (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	category TEXT NOT NULL,
	spec TEXT NOT NULL DEFAULT '',
	price TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	due_date TEXT NOT NULL DEFAULT '',
	auto_renew INTEGER NOT NULL DEFAULT 0,
	warn_days TEXT NOT NULL DEFAULT '[7, 3, 1]',
	last_checked_at TEXT NOT NULL DEFAULT '',
	extra_config TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	cron_expr TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	params TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_sessions (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	persona TEXT NOT NULL DEFAULT 'grok',
	model TEXT NOT NULL DEFAULT 'deepseek-chat',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	reasoning_content TEXT NOT NULL DEFAULT '',
	tool_calls TEXT NOT NULL DEFAULT '',
	tool_call_id TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON agent_messages(session_id, id ASC);

CREATE TABLE IF NOT EXISTS agent_memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scope TEXT NOT NULL DEFAULT 'global',
	category TEXT NOT NULL DEFAULT 'general',
	fact_key TEXT NOT NULL,
	content TEXT NOT NULL,
	importance INTEGER NOT NULL DEFAULT 3,
	pinned INTEGER NOT NULL DEFAULT 0,
	recall_count INTEGER NOT NULL DEFAULT 0,
	last_recalled_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(scope, fact_key)
);
CREATE INDEX IF NOT EXISTS idx_agent_memories_cat ON agent_memories(category, updated_at DESC);

CREATE TABLE IF NOT EXISTS bot_threads (
	id TEXT PRIMARY KEY,
	bot_id TEXT NOT NULL,
	title TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	archived INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bot_threads_bot ON bot_threads(bot_id, archived, updated_at DESC);

CREATE TABLE IF NOT EXISTS skills (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL DEFAULT 'prompt',
	description TEXT NOT NULL DEFAULT '',
	instruction TEXT NOT NULL DEFAULT '',
	http_method TEXT NOT NULL DEFAULT 'GET',
	http_url TEXT NOT NULL DEFAULT '',
	http_headers TEXT NOT NULL DEFAULT '{}',
	params_schema TEXT NOT NULL DEFAULT '[]',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bot_execution_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	bot_id TEXT NOT NULL,
	thread_id TEXT NOT NULL DEFAULT '',
	exec_type TEXT NOT NULL,
	name TEXT NOT NULL,
	input_summary TEXT NOT NULL DEFAULT '',
	output_summary TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'ok',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_bot_exec_logs_bot ON bot_execution_logs(bot_id, id DESC);

CREATE TABLE IF NOT EXISTS notifications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	module_id TEXT NOT NULL DEFAULT 'system',
	title TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	markdown_body TEXT NOT NULL,
	priority TEXT NOT NULL DEFAULT 'normal',
	created_at TEXT NOT NULL,
	read_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_notifications_id ON notifications(id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(read_at, id DESC);

CREATE TABLE IF NOT EXISTS bots (
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

CREATE TABLE IF NOT EXISTS todos (
	id TEXT PRIMARY KEY,
	bot_id TEXT NOT NULL DEFAULT '',
	thread_id TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	notes TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	priority TEXT NOT NULL DEFAULT 'normal',
	due_at TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status, priority, updated_at DESC);

CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	bot_id TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	category TEXT NOT NULL DEFAULT 'general',
	pinned INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notes_updated ON notes(pinned DESC, updated_at DESC);

CREATE TABLE IF NOT EXISTS stash_items (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL DEFAULT 'link',
	url TEXT NOT NULL DEFAULT '',
	domain TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	raw_input TEXT NOT NULL DEFAULT '',
	user_note TEXT NOT NULL DEFAULT '',
	ai_tldr TEXT NOT NULL DEFAULT '',
	ai_points TEXT NOT NULL DEFAULT '[]',
	content_excerpt TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	error_msg TEXT NOT NULL DEFAULT '',
	pinned INTEGER NOT NULL DEFAULT 0,
	read_count INTEGER NOT NULL DEFAULT 0,
	last_reviewed_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_stash_status_updated ON stash_items(status, pinned DESC, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_stash_url ON stash_items(url);
CREATE INDEX IF NOT EXISTS idx_stash_review ON stash_items(status, read_count, last_reviewed_at, created_at);

CREATE VIRTUAL TABLE IF NOT EXISTS stash_fts USING fts5(
	title,
	ai_tldr,
	ai_points,
	tags,
	user_note,
	content_excerpt,
	content='stash_items',
	content_rowid='rowid',
	tokenize='trigram'
);

CREATE TRIGGER IF NOT EXISTS stash_items_ai AFTER INSERT ON stash_items BEGIN
	INSERT INTO stash_fts(rowid, title, ai_tldr, ai_points, tags, user_note, content_excerpt)
	VALUES (new.rowid, new.title, new.ai_tldr, new.ai_points, new.tags, new.user_note, new.content_excerpt);
END;

CREATE TRIGGER IF NOT EXISTS stash_items_ad AFTER DELETE ON stash_items BEGIN
	INSERT INTO stash_fts(stash_fts, rowid, title, ai_tldr, ai_points, tags, user_note, content_excerpt)
	VALUES ('delete', old.rowid, old.title, old.ai_tldr, old.ai_points, old.tags, old.user_note, old.content_excerpt);
END;

CREATE TRIGGER IF NOT EXISTS stash_items_au AFTER UPDATE ON stash_items BEGIN
	INSERT INTO stash_fts(stash_fts, rowid, title, ai_tldr, ai_points, tags, user_note, content_excerpt)
	VALUES ('delete', old.rowid, old.title, old.ai_tldr, old.ai_points, old.tags, old.user_note, old.content_excerpt);
	INSERT INTO stash_fts(rowid, title, ai_tldr, ai_points, tags, user_note, content_excerpt)
	VALUES (new.rowid, new.title, new.ai_tldr, new.ai_points, new.tags, new.user_note, new.content_excerpt);
END;`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Safe incremental column migrations for existing databases
	alters := []string{
		`ALTER TABLE tasks ADD COLUMN bot_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN bot_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN sender_bot_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN msg_kind TEXT NOT NULL DEFAULT 'chat'`,
		`ALTER TABLE agent_memories ADD COLUMN scope TEXT NOT NULL DEFAULT 'global'`,
		`ALTER TABLE bots ADD COLUMN is_group INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE bots ADD COLUMN member_ids TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE bots ADD COLUMN group_mode TEXT NOT NULL DEFAULT 'smart'`,
		`ALTER TABLE bots ADD COLUMN announcement TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN reply_to_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE agent_messages ADD COLUMN reply_preview TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN model_used TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE agent_messages ADD COLUMN thread_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE bots ADD COLUMN bot_mode TEXT NOT NULL DEFAULT 'hybrid'`,
		`ALTER TABLE bots ADD COLUMN memory_policy TEXT NOT NULL DEFAULT 'standard'`,
		`ALTER TABLE bots ADD COLUMN skill_ids TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE bots ADD COLUMN traits TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE bots ADD COLUMN active_thread_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tasks ADD COLUMN run_at TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range alters {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	if err := s.migrateScopedMemoriesTable(); err != nil {
		return err
	}
	_, _ = s.db.Exec(`UPDATE agent_messages SET thread_id = '' WHERE thread_id IS NULL`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_messages_bot ON agent_messages(bot_id, id ASC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_messages_bot_thread ON agent_messages(bot_id, thread_id, id ASC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_messages_bot_thread_desc ON agent_messages(bot_id, thread_id, id DESC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_bots_list ON bots(is_pinned DESC, sort_order ASC, updated_at DESC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_status_due ON todos(status, due_at)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_todos_bot ON todos(bot_id, updated_at DESC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_bot ON notes(bot_id, updated_at DESC)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_push_logs_status ON push_logs(status)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_seen_items_first_seen ON seen_items(first_seen_at)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notifications_mod ON notifications(module_id, id DESC)`)
	_, _ = s.db.Exec(`UPDATE bots SET is_builtin = 0 WHERE is_builtin = 1`)
	_, _ = s.db.Exec(`UPDATE bots SET is_group = 1 WHERE id = 'team_room' AND is_group = 0`)
	_, _ = s.db.Exec(`UPDATE bots SET state = 'idle' WHERE state = 'working'`)
	_, _ = s.db.Exec(`UPDATE module_runs SET status = 'error', summary = '服务重启中断，已自动收敛状态', finished_at = ? WHERE status = 'running'`, NowShanghai())
	_, _ = s.db.Exec(`UPDATE stash_items SET status = CASE WHEN ai_tldr != '' THEN 'ready' ELSE 'failed' END, error_msg = CASE WHEN ai_tldr != '' THEN '' ELSE '服务重启中断后台提炼，可点击重提炼恢复' END, updated_at = ? WHERE status = 'processing'`, NowShanghai())
	_, _ = s.db.Exec(`DELETE FROM configs WHERE key IN ('wecom_corpid', 'wecom_agentid', 'wecom_secret', 'wecom_touser', 'pushplus_token', 'morning_enabled', 'morning_cron', 'radar_enabled', 'radar_cron', 'asset_watcher_enabled', 'asset_watcher_cron')`)
	_, _ = s.db.Exec(`UPDATE configs SET value = 'native' WHERE key = 'push_channel' AND value != 'native'`)
	if err := s.migratePasskeysTable(); err != nil {
		return err
	}
	s.migrateStashToolOnce()
	s.cleanupNoisyRoutineLogs()
	return nil
}

func (s *Store) cleanupNoisyRoutineLogs() {
	_, _ = s.db.Exec(`DELETE FROM agent_messages
		WHERE msg_kind = 'routine'
		  AND content LIKE '### 例程主动汇报:%'
		  AND content LIKE '%- **执行状态**: success%'`)

	_, _ = s.db.Exec(`UPDATE bots SET
		unread_count = 0,
		state = 'idle',
		last_summary = COALESCE((
			SELECT SUBSTR(TRIM(REPLACE(m.content, CHAR(10), ' ')), 1, 64)
			FROM agent_messages m
			WHERE m.bot_id = bots.id
			  AND m.role IN ('user', 'assistant')
			  AND COALESCE(m.thread_id, '') = COALESCE(bots.active_thread_id, '')
			ORDER BY m.id DESC LIMIT 1
		), '')
		WHERE last_summary LIKE '巡检 %发送 0 项临期强提醒%'
		   OR last_summary = '抓取完成，暂无新增条目'
		   OR last_summary LIKE '正在执行例程:%'
		   OR last_summary LIKE '已推送 %（城市:%'`)
}

func (s *Store) migrateScopedMemoriesTable() error {
	var createSQL string
	err := s.db.QueryRow(`SELECT COALESCE(sql, '') FROM sqlite_master WHERE type='table' AND name='agent_memories'`).Scan(&createSQL)
	if err != nil {
		return nil
	}
	normalizedSQL := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(createSQL, "\n", ""), " ", ""))
	needsTableRebuild := !strings.Contains(normalizedSQL, "unique(scope,fact_key)")
	if needsTableRebuild {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		rebuildStmts := []string{
			`CREATE TABLE agent_memories_v2 (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				scope TEXT NOT NULL DEFAULT 'global',
				category TEXT NOT NULL DEFAULT 'general',
				fact_key TEXT NOT NULL,
				content TEXT NOT NULL,
				importance INTEGER NOT NULL DEFAULT 3,
				pinned INTEGER NOT NULL DEFAULT 0,
				recall_count INTEGER NOT NULL DEFAULT 0,
				last_recalled_at TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL,
				UNIQUE(scope, fact_key)
			)`,
			`INSERT OR IGNORE INTO agent_memories_v2 (id, scope, category, fact_key, content, created_at, updated_at)
			 SELECT id, COALESCE(NULLIF(TRIM(scope), ''), 'global'), category, fact_key, content, created_at, updated_at FROM agent_memories`,
			`DROP TABLE agent_memories`,
			`ALTER TABLE agent_memories_v2 RENAME TO agent_memories`,
			`CREATE INDEX IF NOT EXISTS idx_agent_memories_cat ON agent_memories(category, updated_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_agent_memories_scope ON agent_memories(scope, pinned DESC, importance DESC, updated_at DESC)`,
		}
		for _, q := range rebuildStmts {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return tx.Commit()
	}

	memAlters := []string{
		`ALTER TABLE agent_memories ADD COLUMN importance INTEGER NOT NULL DEFAULT 3`,
		`ALTER TABLE agent_memories ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE agent_memories ADD COLUMN recall_count INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE agent_memories ADD COLUMN last_recalled_at TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range memAlters {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_memories_scope ON agent_memories(scope, pinned DESC, importance DESC, updated_at DESC)`)
	return nil
}

func (s *Store) seedDefaults() error {
	pushChannel := "native"
	adminToken := strings.TrimSpace(os.Getenv("BOTHUB_ADMIN_TOKEN"))
	if adminToken == "" {
		adminToken = "vesper-bothub"
	}
	secretBytes := make([]byte, 32)
	_, _ = rand.Read(secretBytes)

	deepseekKey := strings.TrimSpace(os.Getenv("BOTHUB_DEEPSEEK_API_KEY"))
	deepseekBaseURL := strings.TrimSpace(os.Getenv("BOTHUB_DEEPSEEK_BASE_URL"))
	if deepseekBaseURL == "" {
		deepseekBaseURL = DefaultDeepSeekBaseURL
	}
	aiModel := strings.TrimSpace(os.Getenv("BOTHUB_AI_MODEL"))
	if aiModel == "" {
		aiModel = DefaultAIModel
	}
	activePersona := strings.TrimSpace(os.Getenv("BOTHUB_ACTIVE_PERSONA"))
	if activePersona == "" {
		activePersona = DefaultActivePersona
	}
	nezhaDBPath := strings.TrimSpace(os.Getenv("BOTHUB_NEZHA_DB_PATH"))

	lightKey := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_API_KEY"))
	lightBaseURL := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_BASE_URL"))
	lightModel := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_MODEL"))

	appToken := strings.TrimSpace(os.Getenv("BOTHUB_APP_TOKEN"))
	if appToken == "" {
		appToken, _ = GenerateAppBearerToken()
	}

	now := NowShanghai()
	defaults := map[string]string{
		KeyPushChannel:       pushChannel,
		KeyMorningCity:       "Shanghai",
		KeyNezhaURL:          "",
		KeyNezhaDBPath:       nezhaDBPath,
		KeyRadarHNMinScore:   "120",
		KeyRadarLimit:        "8",
		KeyCFAPIToken:        "",
		KeyBiliSessdata:      "",
		KeyDeepSeekAPIKey:    deepseekKey,
		KeyDeepSeekBaseURL:   deepseekBaseURL,
		KeyAIModel:           aiModel,
		KeyActivePersona:     activePersona,
		KeyLightAPIKey:       lightKey,
		KeyLightBaseURL:      lightBaseURL,
		KeyLightModel:        lightModel,
		KeyFreeModelPool:     "[]",
		KeyPoolForLight:      "true",
		KeyPoolFallback:      "true",
		KeyAppBearerToken:    appToken,
		KeyNezhaWebhookToken: "",
		KeyAdminPassHash:     HashPassword(adminToken),
		KeySessionSecret:     hex.EncodeToString(secretBytes),
	}

	for k, v := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO configs (key, value, updated_at) VALUES (?, ?, ?)`, k, v, now); err != nil {
			return err
		}
	}

	// Clean up retired placeholder deepseek-v4-pro without touching real live model IDs like deepseek-chat / deepseek-reasoner / deepseek-flash
	_, _ = s.db.Exec(`UPDATE configs SET value = 'deepseek-chat', updated_at = ? WHERE key = 'ai_model' AND value = 'deepseek-v4-pro'`, now)
	_, _ = s.db.Exec(`UPDATE bots SET model = 'deepseek-chat', updated_at = ? WHERE model = 'deepseek-v4-pro'`, now)
	_, _ = s.db.Exec(`UPDATE agent_sessions SET model = 'deepseek-chat' WHERE model = 'deepseek-v4-pro'`)
	if envPass := strings.TrimSpace(os.Getenv("BOTHUB_ADMIN_TOKEN")); envPass != "" {
		_ = s.SetConfig(KeyAdminPassHash, HashPassword(envPass))
	}

	return nil
}

var shanghaiLocOnce = sync.OnceValue(func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
})

func ShanghaiLocation() *time.Location {
	return shanghaiLocOnce()
}

func NowShanghai() string {
	return time.Now().In(ShanghaiLocation()).Format("2006-01-02 15:04:05")
}
