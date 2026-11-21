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
		title = "BotHub 通知"
	}
	priority = strings.ToLower(strings.TrimSpace(priority))
	if priority != "high" && priority != "critical" {
		priority = "normal"
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = summarizeMarkdownPreview(markdownBody, 120)
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

// ListRecentNotificationsByModule returns the most recent notifications produced by a specific task/module ID.
func (s *Store) ListRecentNotificationsByModule(moduleID string, limit int) ([]Notification, error) {
	mid := strings.TrimSpace(moduleID)
	if mid == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 30 {
		limit = 7
	}
	rows, err := s.db.Query(
		`SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
		 FROM notifications WHERE module_id = ? ORDER BY id DESC LIMIT ?`,
		mid, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Notification, 0, limit)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.ModuleID, &n.Title, &n.Summary, &n.MarkdownBody, &n.Priority, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
}

// ListRecentNotificationsByTitlePrefix returns recent notifications matching a title prefix (fallback when taskID is unavailable).
func (s *Store) ListRecentNotificationsByTitlePrefix(prefix string, limit int) ([]Notification, error) {
	p := strings.TrimSpace(prefix)
	if p == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 30 {
		limit = 7
	}
	rows, err := s.db.Query(
		`SELECT id, module_id, title, summary, markdown_body, priority, created_at, read_at
		 FROM notifications WHERE title LIKE ? ORDER BY id DESC LIMIT ?`,
		p+"%", limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Notification, 0, limit)
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.ModuleID, &n.Title, &n.Summary, &n.MarkdownBody, &n.Priority, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		list = append(list, n)
	}
	return list, rows.Err()
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

func (s *Store) ClearReadNotifications() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM notifications WHERE read_at != ''`)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func (s *Store) DeleteNotification(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM notifications WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func summarizeMarkdownPreview(markdownBody string, maxRunes int) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "#", "", "*", "", "`", "")
	clean := strings.Join(strings.Fields(replacer.Replace(markdownBody)), " ")
	runes := []rune(clean)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return clean
}
