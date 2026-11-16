package store

import (
	"database/sql"
	"errors"
)

type Task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "morning", "radar", "asset_watcher", etc.
	BotID       string `json:"bot_id"`
	Description string `json:"description"`
	CronExpr    string `json:"cron_expr"`
	Enabled     bool   `json:"enabled"`
	Params      string `json:"params"` // JSON string
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, name, type, COALESCE(bot_id, 'butler'), description, cron_expr, enabled, params, created_at, updated_at FROM tasks ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var en int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.BotID, &t.Description, &t.CronExpr, &en, &t.Params, &t.CreatedAt, &t.UpdatedAt); err == nil {
			t.Enabled = en == 1
			tasks = append(tasks, t)
		}
	}
	return tasks, rows.Err()
}

func (s *Store) GetTask(id string) (*Task, error) {
	var t Task
	var en int
	err := s.db.QueryRow(`SELECT id, name, type, COALESCE(bot_id, 'butler'), description, cron_expr, enabled, params, created_at, updated_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Type, &t.BotID, &t.Description, &t.CronExpr, &en, &t.Params, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Enabled = en == 1
	return &t, nil
}

func (s *Store) SaveTask(t Task) error {
	now := NowShanghai()
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.BotID == "" {
		t.BotID = "butler"
	}
	en := 0
	if t.Enabled {
		en = 1
	}

	_, err := s.db.Exec(`INSERT INTO tasks (id, name, type, bot_id, description, cron_expr, enabled, params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			bot_id = excluded.bot_id,
			description = excluded.description,
			cron_expr = excluded.cron_expr,
			enabled = excluded.enabled,
			params = excluded.params,
			updated_at = excluded.updated_at`,
		t.ID, t.Name, t.Type, t.BotID, t.Description, t.CronExpr, en, t.Params, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}
