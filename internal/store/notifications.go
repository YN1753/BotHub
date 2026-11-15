package store

import (
	"fmt"
	"strings"
)

type Notification struct {
	ID           int64  `json:"id"`
	ModuleID     string `json:"module_id"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	MarkdownBody string `json:"markdown_body"`
	Priority     string `json:"priority"`
	CreatedAt    string `json:"created_at"`
	ReadAt       string `json:"read_at"`
}

func (s *Store) CreateNotification(moduleID, title, summary, markdownBody, priority string) (Notification, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		moduleID = "system"
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Vesper BotHub 通知"
	}
	priority = strings.ToLower(strings.TrimSpace(priority))
	if priority != "high" && priority != "critical" {
		priority = "normal"
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		clean := strings.ReplaceAll(markdownBody, "\n", " ")
		clean = strings.ReplaceAll(clean, "#", "")
		clean = strings.ReplaceAll(clean, "*", "")
		clean = strings.TrimSpace(clean)
		runes := []rune(clean)
		if len(runes) > 120 {
			summary = string(runes[:120]) + "..."
		} else {
			summary = clean
		}
	}

	now := NowShanghai()
	res, err := s.db.Exec(
		`INSERT INTO notifications (module_id, title, summary, markdown_body, priority, created_at, read_at) VALUES (?, ?, ?, ?, ?, ?, '')`,
		moduleID, title, summary, markdownBody, priority, now,
	)
	if err != nil {
		return Notification{}, fmt.Errorf("insert notification failed: %w", err)
	}
	id, _ := res.LastInsertId()

	// Keep inbox bounded to latest 1000 items
	_, _ = s.db.Exec(`DELETE FROM notifications WHERE id NOT IN (SELECT id FROM notifications ORDER BY id DESC LIMIT 1000)`)

	return Notification{
		ID:           id,
		ModuleID:     moduleID,
		Title:        title,
		Summary:      summary,
		MarkdownBody: markdownBody,
		Priority:     priority,
		CreatedAt:    now,
		ReadAt:       "",
	}, nil
}

// ListNotifications returns notifications.
// If afterID > 0, it returns incremental notifications with id > afterID in ascending order (for offline catch-up).
// If afterID <= 0, it returns the latest notifications in descending order.
func (s *Store) ListNotifications(afterID int64, limit int, unreadOnly bool) ([]Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var query string
	var args []any

	if afterID > 0 {
		if unreadOnly {
			query = `SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
			         FROM notifications WHERE id > ? AND read_at = '' ORDER BY id ASC LIMIT ?`
		} else {
			query = `SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
			         FROM notifications WHERE id > ? ORDER BY id ASC LIMIT ?`
		}
		args = []any{afterID, limit}
	} else {
		if unreadOnly {
			query = `SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
			         FROM notifications WHERE read_at = '' ORDER BY id DESC LIMIT ?`
		} else {
			query = `SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
			         FROM notifications ORDER BY id DESC LIMIT ?`
		}
		args = []any{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.ModuleID, &n.Title, &n.Summary, &n.MarkdownBody, &n.Priority, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

func (s *Store) LatestNotificationID() int64 {
	var id int64
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM notifications`).Scan(&id)
	return id
}

func (s *Store) CountUnreadNotifications() int {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM notifications WHERE read_at = ''`).Scan(&count)
	return count
}

func (s *Store) MarkNotificationRead(id int64) error {
	now := NowShanghai()
	_, err := s.db.Exec(`UPDATE notifications SET read_at = ? WHERE id = ? AND read_at = ''`, now, id)
	return err
}

func (s *Store) MarkAllNotificationsRead() error {
	now := NowShanghai()
	_, err := s.db.Exec(`UPDATE notifications SET read_at = ? WHERE read_at = ''`, now)
	return err
}
