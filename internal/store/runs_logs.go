package store

import (
	"database/sql"
	"errors"
)

type ModuleRun struct {
	ID          int64  `json:"id"`
	ModuleID    string `json:"module_id"`
	ModuleName  string `json:"module_name"`
	TriggerType string `json:"trigger_type"`
	Status      string `json:"status"`
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
	Status      string `json:"status"`
	ResponseMsg string `json:"response_msg"`
	CreatedAt   string `json:"created_at"`
}

type SeenItem struct {
	Source  string
	ItemKey string
	Title   string
	URL     string
}

func (s *Store) StartModuleRun(moduleID, moduleName, triggerType string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO module_runs (module_id, module_name, trigger_type, status, started_at) VALUES (?, ?, ?, 'running', ?)`,
		moduleID, moduleName, triggerType, NowShanghai())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishModuleRun(id int64, status, summary string, itemsPushed int, durationMs int64) error {
	_, err := s.db.Exec(`UPDATE module_runs SET status = ?, summary = ?, items_pushed = ?, duration_ms = ?, finished_at = ? WHERE id = ?`,
		status, summary, itemsPushed, durationMs, NowShanghai(), id)
	return err
}

func (s *Store) LatestModuleRuns() (map[string]ModuleRun, error) {
	rows, err := s.db.Query(`
SELECT r.id, r.module_id, r.module_name, r.trigger_type, r.status, r.summary, r.items_pushed, r.duration_ms, r.started_at, r.finished_at
FROM module_runs r INNER JOIN (SELECT module_id, MAX(id) AS max_id FROM module_runs GROUP BY module_id) latest ON r.id = latest.max_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]ModuleRun)
	for rows.Next() {
		var r ModuleRun
		if err := rows.Scan(&r.ID, &r.ModuleID, &r.ModuleName, &r.TriggerType, &r.Status, &r.Summary, &r.ItemsPushed, &r.DurationMs, &r.StartedAt, &r.FinishedAt); err == nil {
			res[r.ModuleID] = r
		}
	}
	return res, rows.Err()
}

func (s *Store) AddPushLog(moduleID, title, content, status, responseMsg string) error {
	_, err := s.db.Exec(`INSERT INTO push_logs (module_id, title, content, status, response_msg, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		moduleID, title, content, status, responseMsg, NowShanghai())
	return err
}

func (s *Store) ListPushLogs(limit int) ([]PushLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 40
	}
	rows, err := s.db.Query(`SELECT id, module_id, title, content, status, response_msg, created_at FROM push_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []PushLog
	for rows.Next() {
		var l PushLog
		if err := rows.Scan(&l.ID, &l.ModuleID, &l.Title, &l.Content, &l.Status, &l.ResponseMsg, &l.CreatedAt); err == nil {
			logs = append(logs, l)
		}
	}
	return logs, rows.Err()
}

func (s *Store) IsSeen(source, itemKey string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM seen_items WHERE source = ? AND item_key = ? LIMIT 1`, source, itemKey).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return exists == 1, err
}

func (s *Store) MarkSeen(source, itemKey, title, url string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO seen_items (source, item_key, title, url, first_seen_at) VALUES (?, ?, ?, ?, ?)`,
		source, itemKey, title, url, NowShanghai())
	return err
}

func (s *Store) BatchMarkSeen(items []SeenItem) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO seen_items (source, item_key, title, url, first_seen_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := NowShanghai()
	for _, it := range items {
		if _, err := stmt.Exec(it.Source, it.ItemKey, it.Title, it.URL, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PruneOldRecords() error {
	// Retain latest 500 push logs
	_, _ = s.db.Exec(`DELETE FROM push_logs WHERE id NOT IN (SELECT id FROM push_logs ORDER BY id DESC LIMIT 500)`)
	// Retain module runs within 60 days
	_, _ = s.db.Exec(`DELETE FROM module_runs WHERE started_at < datetime('now', '-60 days')`)
	// Retain seen items within 90 days
	_, _ = s.db.Exec(`DELETE FROM seen_items WHERE first_seen_at < datetime('now', '-90 days')`)
	// Retain read notifications within 90 days
	_, _ = s.db.Exec(`DELETE FROM notifications WHERE read_at != '' AND created_at < datetime('now', '-90 days')`)
	// Retain latest 2000 bot execution logs
	_, _ = s.db.Exec(`DELETE FROM bot_execution_logs WHERE id NOT IN (SELECT id FROM bot_execution_logs ORDER BY id DESC LIMIT 2000)`)
	// Non-blocking WAL checkpoint to keep db file size lean
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(PASSIVE)`)
	return nil
}

func (s *Store) CountStats() (map[string]int, error) {
	var totalPushes, sentPushes, seenItems, totalAssets, totalMemories, totalNotifications, unreadNotifications, openTodos, totalBots int
	_ = s.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM push_logs),
		(SELECT COUNT(*) FROM push_logs WHERE status = 'sent'),
		(SELECT COUNT(*) FROM seen_items),
		(SELECT COUNT(*) FROM assets),
		(SELECT COUNT(*) FROM agent_memories),
		(SELECT COUNT(*) FROM notifications),
		(SELECT COUNT(*) FROM notifications WHERE read_at = ''),
		(SELECT COUNT(*) FROM todos WHERE status != 'done'),
		(SELECT COUNT(*) FROM bots)`).
		Scan(&totalPushes, &sentPushes, &seenItems, &totalAssets, &totalMemories, &totalNotifications, &unreadNotifications, &openTodos, &totalBots)
	return map[string]int{
		"total_pushes":         totalPushes,
		"sent_pushes":          sentPushes,
		"seen_items":           seenItems,
		"total_assets":         totalAssets,
		"total_memories":       totalMemories,
		"total_notifications":  totalNotifications,
		"unread_notifications": unreadNotifications,
		"open_todos":           openTodos,
		"total_bots":           totalBots,
	}, nil
}
