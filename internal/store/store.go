package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const DefaultPushPlusToken = "f99b5ca6eee44e16ad94d1721bfc20f0"

// Config keys stored in SQLite.
const (
	KeyMorningEnabled      = "morning_enabled"
	KeyMorningCron         = "morning_cron"
	KeyMorningCity         = "morning_city"
	KeyNezhaURL            = "nezha_url"
	KeyRadarEnabled        = "radar_enabled"
	KeyRadarCron           = "radar_cron"
	KeyRadarHNMinScore     = "radar_hn_min_score"
	KeyRadarLimit          = "radar_limit"
	KeyAssetWatcherEnabled = "asset_watcher_enabled"
	KeyAssetWatcherCron    = "asset_watcher_cron"
	KeyCFAPIToken          = "cf_api_token"
	KeyPushPlusToken       = "pushplus_token"
	KeyAdminPassHash       = "admin_pass_hash"
	KeySessionSecret       = "session_secret"
)

type Store struct {
	db *sql.DB
}

type ModuleRun struct {
	ID          int64  `json:"id"`
	ModuleID    string `json:"module_id"`
	ModuleName  string `json:"module_name"`
	TriggerType string `json:"trigger_type"` // "cron" or "manual"
	Status      string `json:"status"`       // "running", "success", "error"
	Summary     string `json:"summary"`
	ItemsPushed int    `json:"items_pushed"`
	DurationMs  int64  `json:"duration_ms"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
}

type PushLog struct {
	ID          int64  `json:"id"`
	ModuleID    string `json:"module_id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Status      string `json:"status"` // "sent", "failed", "dry_run"
	ResponseMsg string `json:"response_msg"`
	CreatedAt   string `json:"created_at"`
}

type Asset struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"` // compute, domain, life
	Spec          string `json:"spec"`
	Price         string `json:"price"`
	Status        string `json:"status"`
	DueDate       string `json:"due_date"` // RFC3339 string
	AutoRenew     bool   `json:"auto_renew"`
	WarnDays      string `json:"warn_days"` // JSON array e.g. "[7, 3, 1]"
	LastCheckedAt string `json:"last_checked_at"`
	ExtraConfig   string `json:"extra_config,omitempty"` // JSON string
}

type PublicAsset struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Spec      string `json:"spec"`
	Price     string `json:"price"`
	Status    string `json:"status"`
	DueDate   string `json:"due_date"`
	AutoRenew bool   `json:"auto_renew"`
	DaysLeft  int    `json:"days_left"`
	WarnFlag  bool   `json:"warn_flag"`
}

func Open(dbPath string) (*Store, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
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

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS configs (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

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
CREATE INDEX IF NOT EXISTS idx_module_runs_module_id ON module_runs(module_id, id DESC);

CREATE TABLE IF NOT EXISTS push_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	module_id TEXT NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	status TEXT NOT NULL,
	response_msg TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_push_logs_created ON push_logs(id DESC);

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
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	return nil
}

func (s *Store) seedDefaults() error {
	pushToken := strings.TrimSpace(os.Getenv("BOTHUB_PUSHPLUS_TOKEN"))
	if pushToken == "" {
		pushToken = DefaultPushPlusToken
	}

	adminToken := strings.TrimSpace(os.Getenv("BOTHUB_ADMIN_TOKEN"))
	if adminToken == "" {
		adminToken = "vesper-bothub"
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return fmt.Errorf("generate session secret: %w", err)
	}

	defaults := map[string]string{
		KeyMorningEnabled:      "true",
		KeyMorningCron:         "0 8 * * *",
		KeyMorningCity:         "Shanghai",
		KeyNezhaURL:            "https://dash.starryheavens.uk",
		KeyRadarEnabled:        "true",
		KeyRadarCron:           "0 12,20 * * *",
		KeyRadarHNMinScore:     "120",
		KeyRadarLimit:          "8",
		KeyAssetWatcherEnabled: "true",
		KeyAssetWatcherCron:    "0 3 * * *",
		KeyCFAPIToken:          "",
		KeyPushPlusToken:       pushToken,
		KeyAdminPassHash:       HashPassword(adminToken),
		KeySessionSecret:       hex.EncodeToString(secretBytes),
	}

	now := NowShanghai()
	for k, v := range defaults {
		if _, err := s.db.Exec(
			`INSERT OR IGNORE INTO configs (key, value, updated_at) VALUES (?, ?, ?)`,
			k, v, now,
		); err != nil {
			return fmt.Errorf("seed config %s: %w", k, err)
		}
	}

	// If BOTHUB_ADMIN_TOKEN is explicitly provided in environment, keep DB hash synced to it on boot.
	if envPass := strings.TrimSpace(os.Getenv("BOTHUB_ADMIN_TOKEN")); envPass != "" {
		if err := s.SetConfig(KeyAdminPassHash, HashPassword(envPass)); err != nil {
			return err
		}
	}

	if err := s.seedInitialAssets(); err != nil {
		return fmt.Errorf("seed initial assets: %w", err)
	}
	return nil
}

func (s *Store) seedInitialAssets() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM assets`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := NowShanghai()
	initialAssets := []Asset{
		{
			ID:          "cf_starryheavens",
			Name:        "starryheavens.uk",
			Category:    "domain",
			Spec:        "顶级主域 · Cloudflare Registrar",
			Price:       "年付",
			Status:      "运行中",
			DueDate:     "2027-05-18T10:00:00+08:00",
			AutoRenew:   true,
			WarnDays:    "[30, 15, 7, 3, 1]",
			ExtraConfig: `{"account_id":"0c248525a82b6da799e178614786b970","domain":"starryheavens.uk"}`,
		},
		{
			ID:          "cf_looseleaf",
			Name:        "looseleaf.sarl",
			Category:    "domain",
			Spec:        "备用主域 · Cloudflare Registrar",
			Price:       "年付",
			Status:      "运行中",
			DueDate:     "2027-05-18T10:00:00+08:00",
			AutoRenew:   true,
			WarnDays:    "[30, 15, 7, 3, 1]",
			ExtraConfig: `{"account_id":"0c248525a82b6da799e178614786b970","domain":"looseleaf.sarl"}`,
		},
		{
			ID:          "bilibili",
			Name:        "Bilibili",
			Category:    "life",
			Spec:        "大会员",
			Price:       "年付",
			Status:      "会员有效",
			DueDate:     "2027-01-15T23:59:59+08:00",
			AutoRenew:   false,
			WarnDays:    "[14, 7, 3, 1]",
			ExtraConfig: `{"sessdata":""}`,
		},
		{
			ID:          "qq_music",
			Name:        "QQ 音乐",
			Category:    "life",
			Spec:        "豪华绿钻",
			Price:       "活动月卡",
			Status:      "活动体验中",
			DueDate:     "2026-11-20T23:59:59+08:00",
			AutoRenew:   false,
			WarnDays:    "[7, 3, 1]",
			ExtraConfig: `{"uin":"","qm_keyst":"","is_trial":true}`,
		},
		{
			ID:          "rak_sv",
			Name:        "RakSmart SV",
			Category:    "compute",
			Spec:        "1C 1G · 50G SSD · 1TB VIP",
			Price:       "$1.75 / 月",
			Status:      "运行中",
			DueDate:     "2026-12-12T00:00:00+08:00",
			AutoRenew:   true,
			WarnDays:    "[7, 3, 1]",
			ExtraConfig: `{"anchor_day":12}`,
		},
		{
			ID:          "cc_la",
			Name:        "CloudCone LA",
			Category:    "compute",
			Spec:        "2C 2G · 110G SSD",
			Price:       "$21.66 / 年",
			Status:      "待购入",
			DueDate:     "2027-11-13T00:00:00+08:00",
			AutoRenew:   false,
			WarnDays:    "[14, 7, 3, 1]",
			ExtraConfig: `{}`,
		},
	}

	for _, a := range initialAssets {
		autoRenewInt := 0
		if a.AutoRenew {
			autoRenewInt = 1
		}
		if _, err := s.db.Exec(`
INSERT INTO assets (id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, a.ID, a.Name, a.Category, a.Spec, a.Price, a.Status, a.DueDate, autoRenewInt, a.WarnDays, now, a.ExtraConfig); err != nil {
			return err
		}
	}
	return nil
}

func HashPassword(raw string) string {
	sum := sha256.Sum256([]byte("vesper-bothub::" + raw))
	return hex.EncodeToString(sum[:])
}

func NowShanghai() string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Now().UTC().Add(8 * time.Hour).Format("2006-01-02 15:04:05")
	}
	return time.Now().In(loc).Format("2006-01-02 15:04:05")
}

func (s *Store) GetConfig(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM configs WHERE key = ?`, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s *Store) GetAllConfigs() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		res[k] = v
	}
	return res, rows.Err()
}

func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO configs (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, NowShanghai(),
	)
	return err
}

func (s *Store) StartModuleRun(moduleID, moduleName, triggerType string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO module_runs (module_id, module_name, trigger_type, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		moduleID, moduleName, triggerType, NowShanghai(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishModuleRun(id int64, status, summary string, itemsPushed int, durationMs int64) error {
	_, err := s.db.Exec(
		`UPDATE module_runs SET status = ?, summary = ?, items_pushed = ?, duration_ms = ?, finished_at = ? WHERE id = ?`,
		status, summary, itemsPushed, durationMs, NowShanghai(), id,
	)
	return err
}

func (s *Store) LatestModuleRuns() (map[string]ModuleRun, error) {
	rows, err := s.db.Query(`
SELECT r.id, r.module_id, r.module_name, r.trigger_type, r.status, r.summary, r.items_pushed, r.duration_ms, r.started_at, r.finished_at
FROM module_runs r
INNER JOIN (
	SELECT module_id, MAX(id) AS max_id
	FROM module_runs
	GROUP BY module_id
) latest ON r.id = latest.max_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]ModuleRun)
	for rows.Next() {
		var r ModuleRun
		if err := rows.Scan(&r.ID, &r.ModuleID, &r.ModuleName, &r.TriggerType, &r.Status, &r.Summary, &r.ItemsPushed, &r.DurationMs, &r.StartedAt, &r.FinishedAt); err != nil {
			return nil, err
		}
		res[r.ModuleID] = r
	}
	return res, rows.Err()
}

func (s *Store) AddPushLog(moduleID, title, content, status, responseMsg string) error {
	_, err := s.db.Exec(
		`INSERT INTO push_logs (module_id, title, content, status, response_msg, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		moduleID, title, content, status, responseMsg, NowShanghai(),
	)
	return err
}

func (s *Store) ListPushLogs(limit int) ([]PushLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, module_id, title, content, status, response_msg, created_at FROM push_logs ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []PushLog
	for rows.Next() {
		var l PushLog
		if err := rows.Scan(&l.ID, &l.ModuleID, &l.Title, &l.Content, &l.Status, &l.ResponseMsg, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (s *Store) IsSeen(source, itemKey string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM seen_items WHERE source = ? AND item_key = ? LIMIT 1`, source, itemKey).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) MarkSeen(source, itemKey, title, url string) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO seen_items (source, item_key, title, url, first_seen_at) VALUES (?, ?, ?, ?, ?)`,
		source, itemKey, title, url, NowShanghai(),
	)
	return err
}

// Asset CRUD and Query methods

func (s *Store) ListAssets() ([]Asset, error) {
	rows, err := s.db.Query(`
SELECT id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config
FROM assets ORDER BY 
	CASE category
		WHEN 'compute' THEN 1
		WHEN 'domain' THEN 2
		WHEN 'life' THEN 3
		ELSE 4
	END, id ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Asset
	for rows.Next() {
		var a Asset
		var autoRenewInt int
		if err := rows.Scan(&a.ID, &a.Name, &a.Category, &a.Spec, &a.Price, &a.Status, &a.DueDate, &autoRenewInt, &a.WarnDays, &a.LastCheckedAt, &a.ExtraConfig); err != nil {
			return nil, err
		}
		a.AutoRenew = (autoRenewInt != 0)
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *Store) GetAsset(id string) (*Asset, error) {
	var a Asset
	var autoRenewInt int
	err := s.db.QueryRow(`
SELECT id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config
FROM assets WHERE id = ?
`, id).Scan(&a.ID, &a.Name, &a.Category, &a.Spec, &a.Price, &a.Status, &a.DueDate, &autoRenewInt, &a.WarnDays, &a.LastCheckedAt, &a.ExtraConfig)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.AutoRenew = (autoRenewInt != 0)
	return &a, nil
}

func (s *Store) SaveAsset(a Asset) error {
	autoRenewInt := 0
	if a.AutoRenew {
		autoRenewInt = 1
	}
	if a.WarnDays == "" {
		a.WarnDays = "[7, 3, 1]"
	}
	if a.ExtraConfig == "" {
		a.ExtraConfig = "{}"
	}

	_, err := s.db.Exec(`
INSERT INTO assets (id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	category = excluded.category,
	spec = excluded.spec,
	price = excluded.price,
	status = excluded.status,
	due_date = excluded.due_date,
	auto_renew = excluded.auto_renew,
	warn_days = excluded.warn_days,
	last_checked_at = excluded.last_checked_at,
	extra_config = excluded.extra_config
`, a.ID, a.Name, a.Category, a.Spec, a.Price, a.Status, a.DueDate, autoRenewInt, a.WarnDays, a.LastCheckedAt, a.ExtraConfig)
	return err
}

func (s *Store) UpdateAssetSyncResult(id, dueDate, status string, autoRenew bool, lastChecked string) error {
	autoRenewInt := 0
	if autoRenew {
		autoRenewInt = 1
	}
	_, err := s.db.Exec(`
UPDATE assets SET due_date = ?, status = ?, auto_renew = ?, last_checked_at = ? WHERE id = ?
`, dueDate, status, autoRenewInt, lastChecked, id)
	return err
}

// CalculateDaysLeft calculates days left until due date relative to Asia/Shanghai.
func CalculateDaysLeft(dueDateStr string) (int, error) {
	if strings.TrimSpace(dueDateStr) == "" {
		return 9999, nil
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}

	var t time.Time
	var err error
	// Support both RFC3339 ("2026-11-20T23:59:59+08:00") and standard date-time formats
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err = time.ParseInLocation(f, dueDateStr, loc)
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0, err
	}

	now := time.Now().In(loc)
	diff := t.Sub(now)
	days := int(math.Ceil(diff.Hours() / 24.0))
	return days, nil
}

// ParseWarnDays parses warn days from JSON array string like "[7, 3, 1]".
func ParseWarnDays(warnDaysStr string) []int {
	var days []int
	if err := json.Unmarshal([]byte(warnDaysStr), &days); err == nil && len(days) > 0 {
		return days
	}
	return []int{7, 3, 1}
}

// GetPublicAssets returns 100% sanitized assets for external API.
func (s *Store) GetPublicAssets() ([]PublicAsset, error) {
	list, err := s.ListAssets()
	if err != nil {
		return nil, err
	}

	out := make([]PublicAsset, 0, len(list))
	for _, a := range list {
		days, _ := CalculateDaysLeft(a.DueDate)
		warnFlag := (days >= 0 && days <= 7) || (a.Status == "即将到期" || a.Status == "已到期")
		out = append(out, PublicAsset{
			ID:        a.ID,
			Name:      a.Name,
			Category:  a.Category,
			Spec:      a.Spec,
			Price:     a.Price,
			Status:    a.Status,
			DueDate:   a.DueDate,
			AutoRenew: a.AutoRenew,
			DaysLeft:  days,
			WarnFlag:  warnFlag,
		})
	}
	return out, nil
}

// GetExpiringAssets returns assets expiring within threshold days.
func (s *Store) GetExpiringAssets(thresholdDays int) ([]PublicAsset, error) {
	publics, err := s.GetPublicAssets()
	if err != nil {
		return nil, err
	}
	var expiring []PublicAsset
	for _, p := range publics {
		if p.DaysLeft >= 0 && p.DaysLeft <= thresholdDays {
			expiring = append(expiring, p)
		}
	}
	return expiring, nil
}

func (s *Store) CountStats() (map[string]int, error) {
	var totalPushes, sentPushes, seenItems, totalAssets int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM push_logs`).Scan(&totalPushes)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM push_logs WHERE status = 'sent'`).Scan(&sentPushes)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM seen_items`).Scan(&seenItems)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM assets`).Scan(&totalAssets)
	return map[string]int{
		"total_pushes": totalPushes,
		"sent_pushes":  sentPushes,
		"seen_items":   seenItems,
		"total_assets": totalAssets,
	}, nil
}
