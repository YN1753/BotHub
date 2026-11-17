package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TodoStatusPending = "pending"
	TodoStatusDoing   = "doing"
	TodoStatusDone    = "done"

	TodoPriorityHigh   = "high"
	TodoPriorityNormal = "normal"
	TodoPriorityLow    = "low"
)

type TodoItem struct {
	ID          string `json:"id"`
	BotID       string `json:"bot_id,omitempty"`
	ThreadID    string `json:"thread_id,omitempty"`
	Title       string `json:"title"`
	Notes       string `json:"notes,omitempty"`
	Status      string `json:"status"`   // pending | doing | done
	Priority    string `json:"priority"` // high | normal | low
	DueAt       string `json:"due_at,omitempty"`
	Tags        string `json:"tags,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type WorkNote struct {
	ID        string `json:"id"`
	BotID     string `json:"bot_id,omitempty"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Category  string `json:"category"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func newAssistantID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func NormalizeTodoStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case TodoStatusDoing, "in_progress":
		return TodoStatusDoing
	case TodoStatusDone, "completed":
		return TodoStatusDone
	default:
		return TodoStatusPending
	}
}

func NormalizeTodoPriority(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case TodoPriorityHigh, "urgent":
		return TodoPriorityHigh
	case TodoPriorityLow:
		return TodoPriorityLow
	default:
		return TodoPriorityNormal
	}
}

func (s *Store) ListTodos(statusFilter, botID string, limit int) ([]TodoItem, error) {
	if limit <= 0 || limit > 300 {
		limit = 120
	}
	var conds []string
	var args []any

	sf := strings.ToLower(strings.TrimSpace(statusFilter))
	if sf == "open" || sf == "active" {
		conds = append(conds, "status != 'done'")
	} else if sf != "" && sf != "all" {
		conds = append(conds, "status = ?")
		args = append(args, NormalizeTodoStatus(sf))
	}

	if bid := strings.TrimSpace(botID); bid != "" {
		conds = append(conds, "bot_id = ?")
		args = append(args, bid)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)

	sqlStr := `SELECT id, COALESCE(bot_id,''), COALESCE(thread_id,''), title, COALESCE(notes,''),
		status, priority, COALESCE(due_at,''), COALESCE(tags,''), created_at, updated_at, COALESCE(completed_at,'')
		FROM todos ` + where + `
		ORDER BY CASE status WHEN 'doing' THEN 0 WHEN 'pending' THEN 1 ELSE 2 END,
			CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
			CASE WHEN due_at = '' THEN 1 ELSE 0 END,
			due_at ASC,
			updated_at DESC
		LIMIT ?`

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]TodoItem, 0)
	for rows.Next() {
		var item TodoItem
		if err := rows.Scan(
			&item.ID, &item.BotID, &item.ThreadID, &item.Title, &item.Notes,
			&item.Status, &item.Priority, &item.DueAt, &item.Tags,
			&item.CreatedAt, &item.UpdatedAt, &item.CompletedAt,
		); err == nil {
			list = append(list, item)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetTodo(id string) (*TodoItem, error) {
	var item TodoItem
	err := s.db.QueryRow(`SELECT id, COALESCE(bot_id,''), COALESCE(thread_id,''), title, COALESCE(notes,''),
		status, priority, COALESCE(due_at,''), COALESCE(tags,''), created_at, updated_at, COALESCE(completed_at,'')
		FROM todos WHERE id = ?`, strings.TrimSpace(id)).
		Scan(&item.ID, &item.BotID, &item.ThreadID, &item.Title, &item.Notes,
			&item.Status, &item.Priority, &item.DueAt, &item.Tags,
			&item.CreatedAt, &item.UpdatedAt, &item.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) SaveTodo(item TodoItem) (TodoItem, error) {
	now := NowShanghai()
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" {
		item.ID = newAssistantID("td")
	}
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" {
		return item, errors.New("todo title is required")
	}
	item.Status = NormalizeTodoStatus(item.Status)
	item.Priority = NormalizeTodoPriority(item.Priority)
	item.DueAt = strings.TrimSpace(item.DueAt)
	item.Tags = strings.TrimSpace(item.Tags)
	item.BotID = strings.TrimSpace(item.BotID)
	item.ThreadID = strings.TrimSpace(item.ThreadID)
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	if item.Status == TodoStatusDone {
		if item.CompletedAt == "" {
			item.CompletedAt = now
		}
	} else {
		item.CompletedAt = ""
	}

	_, err := s.db.Exec(`INSERT INTO todos (id, bot_id, thread_id, title, notes, status, priority, due_at, tags, created_at, updated_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			bot_id = excluded.bot_id,
			thread_id = excluded.thread_id,
			title = excluded.title,
			notes = excluded.notes,
			status = excluded.status,
			priority = excluded.priority,
			due_at = excluded.due_at,
			tags = excluded.tags,
			updated_at = excluded.updated_at,
			completed_at = excluded.completed_at`,
		item.ID, item.BotID, item.ThreadID, item.Title, item.Notes, item.Status, item.Priority, item.DueAt, item.Tags, item.CreatedAt, item.UpdatedAt, item.CompletedAt)
	return item, err
}

func (s *Store) DeleteTodo(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) ClearCompletedTodos(botID string) (int64, error) {
	bid := strings.TrimSpace(botID)
	var res sql.Result
	var err error
	if bid != "" {
		res, err = s.db.Exec(`DELETE FROM todos WHERE status = 'done' AND bot_id = ?`, bid)
	} else {
		res, err = s.db.Exec(`DELETE FROM todos WHERE status = 'done'`)
	}
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Workspace Deliverables / Notes CRUD

func (s *Store) ListNotes(query, botID string, limit int) ([]WorkNote, error) {
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	var conds []string
	var args []any

	if bid := strings.TrimSpace(botID); bid != "" {
		conds = append(conds, "bot_id = ?")
		args = append(args, bid)
	}
	if q := strings.TrimSpace(query); q != "" {
		like := "%" + q + "%"
		conds = append(conds, "(title LIKE ? OR content LIKE ? OR category LIKE ?)")
		args = append(args, like, like, like)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)

	sqlStr := `SELECT id, COALESCE(bot_id,''), title, COALESCE(content,''), COALESCE(category,'general'),
		COALESCE(pinned,0), created_at, updated_at
		FROM notes ` + where + ` ORDER BY COALESCE(pinned,0) DESC, updated_at DESC LIMIT ?`

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]WorkNote, 0)
	for rows.Next() {
		var n WorkNote
		var pinInt int
		if err := rows.Scan(&n.ID, &n.BotID, &n.Title, &n.Content, &n.Category, &pinInt, &n.CreatedAt, &n.UpdatedAt); err == nil {
			n.Pinned = pinInt == 1
			list = append(list, n)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetNote(id string) (*WorkNote, error) {
	var n WorkNote
	var pinInt int
	err := s.db.QueryRow(`SELECT id, COALESCE(bot_id,''), title, COALESCE(content,''), COALESCE(category,'general'),
		COALESCE(pinned,0), created_at, updated_at
		FROM notes WHERE id = ?`, strings.TrimSpace(id)).
		Scan(&n.ID, &n.BotID, &n.Title, &n.Content, &n.Category, &pinInt, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n.Pinned = pinInt == 1
	return &n, nil
}

func (s *Store) SaveNote(n WorkNote) (WorkNote, error) {
	now := NowShanghai()
	n.ID = strings.TrimSpace(n.ID)
	if n.ID == "" {
		n.ID = newAssistantID("note")
	}
	n.Title = strings.TrimSpace(n.Title)
	if n.Title == "" {
		return n, errors.New("note title is required")
	}
	n.BotID = strings.TrimSpace(n.BotID)
	n.Category = strings.TrimSpace(n.Category)
	if n.Category == "" {
		n.Category = "general"
	}
	if n.CreatedAt == "" {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	pinInt := 0
	if n.Pinned {
		pinInt = 1
	}

	_, err := s.db.Exec(`INSERT INTO notes (id, bot_id, title, content, category, pinned, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			bot_id = excluded.bot_id,
			title = excluded.title,
			content = excluded.content,
			category = excluded.category,
			pinned = excluded.pinned,
			updated_at = excluded.updated_at`,
		n.ID, n.BotID, n.Title, n.Content, n.Category, pinInt, n.CreatedAt, n.UpdatedAt)
	return n, err
}

func (s *Store) DeleteNote(id string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

type WorkspaceBriefing struct {
	GeneratedAt       string         `json:"generated_at"`
	Today             string         `json:"today"`
	Weekday           string         `json:"weekday"`
	OpenTodoCount     int            `json:"open_todo_count"`
	OverdueTodos      []TodoItem     `json:"overdue_todos"`
	DueTodayTodos     []TodoItem     `json:"due_today_todos"`
	HighPriorityTodos []TodoItem     `json:"high_priority_todos"`
	ExpiringAssets    []Asset        `json:"expiring_assets"`
	EnabledTasks      []Task         `json:"enabled_tasks"`
	UnreadAlerts      []Notification `json:"unread_alerts"`
	UnreadCount       int            `json:"unread_count"`
	RecentNotes       []WorkNote     `json:"recent_notes"`
	FocusHints        []string       `json:"focus_hints,omitempty"`
}

// BuildWorkspaceBriefing aggregates live personal todos, expiring assets, active schedules,
// unread alerts, and recent work deliverables from SQLite as a single source of truth.
func (s *Store) BuildWorkspaceBriefing() (*WorkspaceBriefing, error) {
	now := time.Now().In(ShanghaiLocation())
	today := now.Format("2006-01-02")
	weekdays := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

	openTodos, err := s.ListTodos("open", "", 150)
	if err != nil {
		return nil, err
	}
	overdue := make([]TodoItem, 0)
	dueToday := make([]TodoItem, 0)
	highPrio := make([]TodoItem, 0)
	for _, td := range openTodos {
		dueDay := ""
		if len(td.DueAt) >= 10 {
			dueDay = td.DueAt[:10]
		}
		if dueDay != "" {
			if dueDay < today {
				overdue = append(overdue, td)
			} else if dueDay == today {
				dueToday = append(dueToday, td)
			}
		}
		if td.Priority == TodoPriorityHigh {
			highPrio = append(highPrio, td)
		}
	}

	allAssets, _ := s.ListAssets()
	expiring := make([]Asset, 0)
	for _, a := range allAssets {
		if NormalizeAssetStatus(a.Status) == "已停用" {
			continue
		}
		if a.DueDate == "" || strings.HasPrefix(a.DueDate, "2099") {
			continue
		}
		if a.DaysLeft <= 14 {
			expiring = append(expiring, a)
		}
	}

	allTasks, _ := s.ListTasks()
	enabledTasks := make([]Task, 0)
	for _, t := range allTasks {
		if t.Enabled {
			enabledTasks = append(enabledTasks, t)
		}
	}

	unreadAlerts, _ := s.ListNotifications(0, 12, true)
	recentNotes, _ := s.ListNotes("", "", 6)
	unreadCount := s.CountUnreadNotifications()

	var hints []string
	if len(overdue) > 0 {
		hints = append(hints, fmt.Sprintf("存在 %d 项已逾期待办，建议最优先处理或重排截止日期", len(overdue)))
	}
	if len(dueToday) > 0 {
		hints = append(hints, fmt.Sprintf("今日有 %d 项待办到期", len(dueToday)))
	}
	urgentAssets := 0
	for _, a := range expiring {
		if a.DaysLeft <= 3 {
			urgentAssets++
		}
	}
	if urgentAssets > 0 {
		hints = append(hints, fmt.Sprintf("有 %d 项资产将在 3 天内到期，需核实是否续费", urgentAssets))
	}
	if unreadCount > 0 {
		hints = append(hints, fmt.Sprintf("收件箱尚有 %d 条未读通知待查看", unreadCount))
	}

	return &WorkspaceBriefing{
		GeneratedAt:       now.Format("2006-01-02 15:04:05"),
		Today:             today,
		Weekday:           weekdays[int(now.Weekday())%7],
		OpenTodoCount:     len(openTodos),
		OverdueTodos:      overdue,
		DueTodayTodos:     dueToday,
		HighPriorityTodos: highPrio,
		ExpiringAssets:    expiring,
		EnabledTasks:      enabledTasks,
		UnreadAlerts:      unreadAlerts,
		UnreadCount:       unreadCount,
		RecentNotes:       recentNotes,
		FocusHints:        hints,
	}, nil
}
