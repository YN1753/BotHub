package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultPushPlusToken   = "f99b5ca6eee44e16ad94d1721bfc20f0"
	DefaultWeComCorpID     = "wwda96f68ec84b1f74"
	DefaultWeComAgentID    = "1000002"
	DefaultWeComSecret     = "P8R70brpQMVWrmysZaGlWsiTqS6QV5GJOWfBHfWvjC4"
	DefaultWeComToUser     = "@all"
	DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1"
	DefaultAIModel         = "deepseek-flash"
	DefaultActivePersona   = "grok"
	DefaultNezhaDBPath     = "/opt/nezha/dashboard/data/sqlite.db"

	KeyPushChannel         = "push_channel"
	KeyWeComCorpID         = "wecom_corpid"
	KeyWeComAgentID        = "wecom_agentid"
	KeyWeComSecret         = "wecom_secret"
	KeyWeComToUser         = "wecom_touser"
	KeyMorningEnabled      = "morning_enabled"
	KeyMorningCron         = "morning_cron"
	KeyMorningCity         = "morning_city"
	KeyNezhaURL            = "nezha_url"
	KeyNezhaDBPath         = "nezha_db_path"
	KeyRadarEnabled        = "radar_enabled"
	KeyRadarCron           = "radar_cron"
	KeyRadarHNMinScore     = "radar_hn_min_score"
	KeyRadarLimit          = "radar_limit"
	KeyAssetWatcherEnabled = "asset_watcher_enabled"
	KeyAssetWatcherCron    = "asset_watcher_cron"
	KeyCFAPIToken          = "cf_api_token"
	KeyBiliSessdata        = "bili_sessdata"
	KeyPushPlusToken       = "pushplus_token"
	KeyDeepSeekAPIKey      = "deepseek_api_key"
	KeyDeepSeekBaseURL     = "deepseek_base_url"
	KeyAIModel             = "ai_model"
	KeyActivePersona       = "active_persona"
	KeyLightAPIKey         = "light_api_key"
	KeyLightBaseURL        = "light_base_url"
	KeyLightModel          = "light_model"
	KeyAppBearerToken      = "app_bearer_token"
	KeyNezhaWebhookToken   = "nezha_webhook_token"
	KeyAdminPassHash       = "admin_pass_hash"
	KeySessionSecret       = "session_secret"
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
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath))
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
	category TEXT NOT NULL DEFAULT 'general',
	fact_key TEXT NOT NULL UNIQUE,
	content TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_memories_cat ON agent_memories(category, updated_at DESC);

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
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(read_at, id DESC);`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) seedDefaults() error {
	pushChannel := strings.TrimSpace(os.Getenv("BOTHUB_PUSH_CHANNEL"))
	if pushChannel == "" {
		pushChannel = "wecom"
	}
	wecomCorpID := strings.TrimSpace(os.Getenv("BOTHUB_WECOM_CORPID"))
	if wecomCorpID == "" {
		wecomCorpID = DefaultWeComCorpID
	}
	wecomAgentID := strings.TrimSpace(os.Getenv("BOTHUB_WECOM_AGENTID"))
	if wecomAgentID == "" {
		wecomAgentID = DefaultWeComAgentID
	}
	wecomSecret := strings.TrimSpace(os.Getenv("BOTHUB_WECOM_SECRET"))
	if wecomSecret == "" {
		wecomSecret = DefaultWeComSecret
	}
	wecomToUser := strings.TrimSpace(os.Getenv("BOTHUB_WECOM_TOUSER"))
	if wecomToUser == "" {
		wecomToUser = DefaultWeComToUser
	}

	pushToken := strings.TrimSpace(os.Getenv("BOTHUB_PUSHPLUS_TOKEN"))
	if pushToken == "" {
		pushToken = DefaultPushPlusToken
	}
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
	if nezhaDBPath == "" {
		nezhaDBPath = DefaultNezhaDBPath
	}

	lightKey := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_API_KEY"))
	lightBaseURL := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_BASE_URL"))
	lightModel := strings.TrimSpace(os.Getenv("BOTHUB_LIGHT_MODEL"))

	appToken := strings.TrimSpace(os.Getenv("BOTHUB_APP_TOKEN"))
	if appToken == "" {
		appBytes := make([]byte, 24)
		_, _ = rand.Read(appBytes)
		appToken = "bh2_" + hex.EncodeToString(appBytes)
	}

	now := NowShanghai()
	defaults := map[string]string{
		KeyPushChannel:         pushChannel,
		KeyWeComCorpID:         wecomCorpID,
		KeyWeComAgentID:        wecomAgentID,
		KeyWeComSecret:         wecomSecret,
		KeyWeComToUser:         wecomToUser,
		KeyMorningEnabled:      "true",
		KeyMorningCron:         "0 8 * * *",
		KeyMorningCity:         "Shanghai",
		KeyNezhaURL:            "https://dash.starryheavens.uk",
		KeyNezhaDBPath:         nezhaDBPath,
		KeyRadarEnabled:        "true",
		KeyRadarCron:           "0 12,20 * * *",
		KeyRadarHNMinScore:     "120",
		KeyRadarLimit:          "8",
		KeyAssetWatcherEnabled: "true",
		KeyAssetWatcherCron:    "0 3 * * *",
		KeyCFAPIToken:          "",
		KeyBiliSessdata:        "",
		KeyPushPlusToken:       pushToken,
		KeyDeepSeekAPIKey:      deepseekKey,
		KeyDeepSeekBaseURL:     deepseekBaseURL,
		KeyAIModel:             aiModel,
		KeyActivePersona:       activePersona,
		KeyLightAPIKey:         lightKey,
		KeyLightBaseURL:        lightBaseURL,
		KeyLightModel:          lightModel,
		KeyAppBearerToken:      appToken,
		KeyNezhaWebhookToken:   "",
		KeyAdminPassHash:       HashPassword(adminToken),
		KeySessionSecret:       hex.EncodeToString(secretBytes),
	}

	for k, v := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO configs (key, value, updated_at) VALUES (?, ?, ?)`, k, v, now); err != nil {
			return err
		}
	}

	// Automatically upgrade retired model IDs (deepseek-chat / deepseek-reasoner) in existing databases
	_, _ = s.db.Exec(`UPDATE configs SET value = 'deepseek-flash', updated_at = ? WHERE key = 'ai_model' AND value = 'deepseek-chat'`, now)
	_, _ = s.db.Exec(`UPDATE configs SET value = 'deepseek-v4-pro', updated_at = ? WHERE key = 'ai_model' AND value = 'deepseek-reasoner'`, now)
	_, _ = s.db.Exec(`UPDATE agent_sessions SET model = 'deepseek-flash' WHERE model = 'deepseek-chat'`)
	_, _ = s.db.Exec(`UPDATE agent_sessions SET model = 'deepseek-v4-pro' WHERE model = 'deepseek-reasoner'`)
	if envPass := strings.TrimSpace(os.Getenv("BOTHUB_ADMIN_TOKEN")); envPass != "" {
		_ = s.SetConfig(KeyAdminPassHash, HashPassword(envPass))
	}

	// Seed assets if table is empty
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM assets`).Scan(&count); err == nil && count == 0 {
		initial := []Asset{
			{ID: "cf_starryheavens", Name: "starryheavens.uk", Category: "domain", Spec: "顶级主域 · Cloudflare", Price: "年付", Status: "运行中", DueDate: "2027-05-18T10:00:00+08:00", AutoRenew: true, WarnDays: "[30, 15, 7, 3, 1]", ExtraConfig: `{"account_id":"0c248525a82b6da799e178614786b970","domain":"starryheavens.uk"}`},
			{ID: "cf_looseleaf", Name: "looseleaf.sarl", Category: "domain", Spec: "备用主域 · Cloudflare", Price: "年付", Status: "运行中", DueDate: "2027-05-18T10:00:00+08:00", AutoRenew: true, WarnDays: "[30, 15, 7, 3, 1]", ExtraConfig: `{"account_id":"0c248525a82b6da799e178614786b970","domain":"looseleaf.sarl"}`},
			{ID: "bilibili", Name: "Bilibili", Category: "life", Spec: "大会员", Price: "年付", Status: "会员有效", DueDate: "2027-01-15T23:59:59+08:00", AutoRenew: false, WarnDays: "[14, 7, 3, 1]", ExtraConfig: `{"sessdata":""}`},
			{ID: "qq_music", Name: "QQ 音乐", Category: "life", Spec: "豪华绿钻", Price: "活动月卡", Status: "活动体验中", DueDate: "2026-11-20T23:59:59+08:00", AutoRenew: false, WarnDays: "[7, 3, 1]", ExtraConfig: `{"uin":"","qm_keyst":"","is_trial":true}`},
			{ID: "rak_sv", Name: "RakSmart SV", Category: "compute", Spec: "1C 1G · 50G SSD · 1TB VIP", Price: "$1.75 / 月", Status: "运行中", DueDate: "2026-12-12T00:00:00+08:00", AutoRenew: true, WarnDays: "[7, 3, 1]", ExtraConfig: `{"anchor_day":12}`},
			{ID: "cc_la", Name: "CloudCone LA", Category: "compute", Spec: "2C 2G · 110G SSD", Price: "$21.66 / 年", Status: "待购入", DueDate: "2027-11-13T00:00:00+08:00", AutoRenew: false, WarnDays: "[14, 7, 3, 1]", ExtraConfig: `{}`},
		}
		for _, a := range initial {
			_ = s.SaveAsset(a)
		}
	}

	// Seed default tasks if tasks table is empty
	var taskCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&taskCount); err == nil && taskCount == 0 {
		initialTasks := []Task{
			{
				ID:          "morning",
				Name:        "每日晨报小秘书",
				Type:        "morning",
				Description: "汇总目标城市气象建议、每日格言、节点脉搏与临期资产",
				CronExpr:    "0 8 * * *",
				Enabled:     true,
				Params:      `{"city":"Shanghai","nezha_url":"https://dash.starryheavens.uk"}`,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "radar",
				Name:        "全网开源技术雷达",
				Type:        "radar",
				Description: "抓取 GitHub Trending 与 Hacker News 热门高分条目，自动排版去重推送",
				CronExpr:    "0 12,20 * * *",
				Enabled:     true,
				Params:      `{"hn_min_score":120,"limit":8}`,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "asset_watcher",
				Name:        "数字资产与会员守望",
				Type:        "asset_watcher",
				Description: "自动嗅探域名到期、B站大会员与VPS扣费周期，触发临期防扣款强提醒",
				CronExpr:    "0 3 * * *",
				Enabled:     true,
				Params:      `{}`,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		}
		for _, t := range initialTasks {
			_ = s.SaveTask(t)
		}
	}
	return nil
}

func ShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func NowShanghai() string {
	return time.Now().In(ShanghaiLocation()).Format("2006-01-02 15:04:05")
}
