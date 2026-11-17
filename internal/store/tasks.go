package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type Task struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "morning", "radar", "asset_watcher", "bot_prompt", etc.
	BotID       string `json:"bot_id"`
	Description string `json:"description"`
	CronExpr    string `json:"cron_expr"`
	RunAt       string `json:"run_at,omitempty"` // Optional one-shot Shanghai timestamp ("2006-01-02 15:04")
	Enabled     bool   `json:"enabled"`
	Params      string `json:"params"` // JSON string
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *Store) ListTasks() ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, name, type, COALESCE(bot_id, ''), description, cron_expr, COALESCE(run_at, ''), enabled, params, created_at, updated_at FROM tasks ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var en int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.BotID, &t.Description, &t.CronExpr, &t.RunAt, &en, &t.Params, &t.CreatedAt, &t.UpdatedAt); err == nil {
			t.Enabled = en == 1
			tasks = append(tasks, t)
		}
	}
	return tasks, rows.Err()
}

func (s *Store) GetTask(id string) (*Task, error) {
	var t Task
	var en int
	err := s.db.QueryRow(`SELECT id, name, type, COALESCE(bot_id, ''), description, cron_expr, COALESCE(run_at, ''), enabled, params, created_at, updated_at FROM tasks WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Type, &t.BotID, &t.Description, &t.CronExpr, &t.RunAt, &en, &t.Params, &t.CreatedAt, &t.UpdatedAt)
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
	if strings.TrimSpace(t.CreatedAt) == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	t.ID = strings.TrimSpace(t.ID)
	t.Name = strings.TrimSpace(t.Name)
	t.Type = strings.TrimSpace(t.Type)
	t.BotID = strings.TrimSpace(t.BotID)
	t.Description = strings.TrimSpace(t.Description)
	t.CronExpr = strings.TrimSpace(t.CronExpr)
	t.RunAt = strings.TrimSpace(t.RunAt)
	if strings.TrimSpace(t.Params) == "" || !json.Valid([]byte(t.Params)) {
		t.Params = "{}"
	}
	en := 0
	if t.Enabled {
		en = 1
	}

	_, err := s.db.Exec(`INSERT INTO tasks (id, name, type, bot_id, description, cron_expr, run_at, enabled, params, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			bot_id = excluded.bot_id,
			description = excluded.description,
			cron_expr = excluded.cron_expr,
			run_at = excluded.run_at,
			enabled = excluded.enabled,
			params = excluded.params,
			updated_at = excluded.updated_at`,
		t.ID, t.Name, t.Type, t.BotID, t.Description, t.CronExpr, t.RunAt, en, t.Params, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) DeleteTask(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, trimmed)
	return err
}
