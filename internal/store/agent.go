package store

import (
	"database/sql"
	"errors"
	"strings"
)

type AgentSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Persona   string `json:"persona"`
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AgentMessage struct {
	ID               int64  `json:"id"`
	SessionID        string `json:"session_id"`
	BotID            string `json:"bot_id,omitempty"`
	SenderBotID      string `json:"sender_bot_id,omitempty"`
	MsgKind          string `json:"msg_kind,omitempty"` // "chat" | "routine" | "system"
	Role             string `json:"role"`               // "user", "assistant", "tool", "system"
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCalls        string `json:"tool_calls,omitempty"`   // JSON array of tool calls
	ToolCallID       string `json:"tool_call_id,omitempty"` // For role="tool"
	ReplyToID        int64  `json:"reply_to_id,omitempty"`
	ReplyPreview     string `json:"reply_preview,omitempty"`
	CreatedAt        string `json:"created_at"`
}

type AgentMemory struct {
	ID        int64  `json:"id"`
	Scope     string `json:"scope,omitempty"` // "global" or "bot:<id>"
	Category  string `json:"category"`        // e.g. "preference", "infra", "habit", "general"
	FactKey   string `json:"fact_key"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) ListAgentSessions(limit int) ([]AgentSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT id, title, persona, model, created_at, updated_at FROM agent_sessions ORDER BY updated_at DESC, created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AgentSession
	for rows.Next() {
		var item AgentSession
		if err := rows.Scan(&item.ID, &item.Title, &item.Persona, &item.Model, &item.CreatedAt, &item.UpdatedAt); err == nil {
			list = append(list, item)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetAgentSession(id string) (*AgentSession, error) {
	var item AgentSession
	err := s.db.QueryRow(`SELECT id, title, persona, model, created_at, updated_at FROM agent_sessions WHERE id = ?`, id).
		Scan(&item.ID, &item.Title, &item.Persona, &item.Model, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) SaveAgentSession(sess AgentSession) error {
	now := NowShanghai()
	if sess.CreatedAt == "" {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	if sess.Persona == "" {
		sess.Persona = DefaultActivePersona
	}
	if sess.Model == "" {
		sess.Model = DefaultAIModel
	}
	if strings.TrimSpace(sess.Title) == "" {
		sess.Title = "新对话"
	}

	_, err := s.db.Exec(`INSERT INTO agent_sessions (id, title, persona, model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			persona = excluded.persona,
			model = excluded.model,
			updated_at = excluded.updated_at`,
		sess.ID, sess.Title, sess.Persona, sess.Model, sess.CreatedAt, sess.UpdatedAt)
	return err
}

func (s *Store) DeleteAgentSession(id string) error {
	_, _ = s.db.Exec(`DELETE FROM agent_messages WHERE session_id = ?`, id)
	_, err := s.db.Exec(`DELETE FROM agent_sessions WHERE id = ?`, id)
	return err
}

func (s *Store) AppendAgentMessage(msg AgentMessage) (int64, error) {
	now := NowShanghai()
	if msg.CreatedAt == "" {
		msg.CreatedAt = now
	}
	if msg.MsgKind == "" {
		msg.MsgKind = "chat"
	}
	res, err := s.db.Exec(`INSERT INTO agent_messages (session_id, bot_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.SessionID, msg.BotID, msg.SenderBotID, msg.MsgKind, msg.Role, msg.Content, msg.ReasoningContent, msg.ToolCalls, msg.ToolCallID, msg.ReplyToID, msg.ReplyPreview, msg.CreatedAt)
	if err != nil {
		return 0, err
	}
	if msg.SessionID != "" {
		_, _ = s.db.Exec(`UPDATE agent_sessions SET updated_at = ? WHERE id = ?`, now, msg.SessionID)
	}
	return res.LastInsertId()
}

func (s *Store) ListAgentMessages(sessionID string, limit int) ([]AgentMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, COALESCE(bot_id,''), COALESCE(sender_bot_id,''), COALESCE(msg_kind,'chat'), role, content, reasoning_content, tool_calls, tool_call_id, COALESCE(reply_to_id,0), COALESCE(reply_preview,''), created_at
		FROM (
			SELECT id, session_id, bot_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, created_at
			FROM agent_messages WHERE session_id = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AgentMessage
	for rows.Next() {
		var m AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.BotID, &m.SenderBotID, &m.MsgKind, &m.Role, &m.Content, &m.ReasoningContent, &m.ToolCalls, &m.ToolCallID, &m.ReplyToID, &m.ReplyPreview, &m.CreatedAt); err == nil {
			list = append(list, m)
		}
	}
	return list, rows.Err()
}

func (s *Store) ListBotMessages(botID string, limit int) ([]AgentMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, COALESCE(bot_id,''), COALESCE(sender_bot_id,''), COALESCE(msg_kind,'chat'), role, content, reasoning_content, tool_calls, tool_call_id, COALESCE(reply_to_id,0), COALESCE(reply_preview,''), created_at
		FROM (
			SELECT id, session_id, bot_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, created_at
			FROM agent_messages WHERE bot_id = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`, botID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AgentMessage
	for rows.Next() {
		var m AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.BotID, &m.SenderBotID, &m.MsgKind, &m.Role, &m.Content, &m.ReasoningContent, &m.ToolCalls, &m.ToolCallID, &m.ReplyToID, &m.ReplyPreview, &m.CreatedAt); err == nil {
			list = append(list, m)
		}
	}
	return list, rows.Err()
}

func (s *Store) DeleteBotMessage(botID string, msgID int64) error {
	_, err := s.db.Exec(`DELETE FROM agent_messages WHERE bot_id = ? AND id = ?`, botID, msgID)
	return err
}

func (s *Store) ClearBotMessages(botID string) error {
	_, err := s.db.Exec(`DELETE FROM agent_messages WHERE bot_id = ?`, botID)
	return err
}

func (s *Store) SaveAgentMemory(category, factKey, content string) error {
	return s.SaveAgentMemoryWithScope("global", category, factKey, content)
}

func (s *Store) SaveAgentMemoryWithScope(scope, category, factKey, content string) error {
	now := NowShanghai()
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "global"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "general"
	}
	factKey = strings.TrimSpace(factKey)
	content = strings.TrimSpace(content)
	if factKey == "" || content == "" {
		return errors.New("fact_key and content are required")
	}

	_, err := s.db.Exec(`INSERT INTO agent_memories (scope, category, fact_key, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(fact_key) DO UPDATE SET
			scope = excluded.scope,
			category = excluded.category,
			content = excluded.content,
			updated_at = excluded.updated_at`,
		scope, category, factKey, content, now, now)
	return err
}

func (s *Store) ListAgentMemories(query string, limit int) ([]AgentMemory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := strings.TrimSpace(query)
	var rows *sql.Rows
	var err error
	if q == "" {
		rows, err = s.db.Query(`SELECT id, COALESCE(scope,'global'), category, fact_key, content, created_at, updated_at FROM agent_memories ORDER BY updated_at DESC, id DESC LIMIT ?`, limit)
	} else {
		like := "%" + q + "%"
		rows, err = s.db.Query(`SELECT id, COALESCE(scope,'global'), category, fact_key, content, created_at, updated_at FROM agent_memories WHERE fact_key LIKE ? OR content LIKE ? OR category LIKE ? OR scope LIKE ? ORDER BY updated_at DESC, id DESC LIMIT ?`,
			like, like, like, like, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AgentMemory
	for rows.Next() {
		var mem AgentMemory
		if err := rows.Scan(&mem.ID, &mem.Scope, &mem.Category, &mem.FactKey, &mem.Content, &mem.CreatedAt, &mem.UpdatedAt); err == nil {
			list = append(list, mem)
		}
	}
	return list, rows.Err()
}

func (s *Store) DeleteAgentMemory(id int64) error {
	_, err := s.db.Exec(`DELETE FROM agent_memories WHERE id = ?`, id)
	return err
}
