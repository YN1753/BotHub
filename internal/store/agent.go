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
	ThreadID         string `json:"thread_id,omitempty"`
	SenderBotID      string `json:"sender_bot_id,omitempty"`
	MsgKind          string `json:"msg_kind,omitempty"` // "chat" | "routine" | "system"
	Role             string `json:"role"`               // "user", "assistant", "tool", "system"
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCalls        string `json:"tool_calls,omitempty"`   // JSON array of tool calls
	ToolCallID       string `json:"tool_call_id,omitempty"` // For role="tool"
	ReplyToID        int64  `json:"reply_to_id,omitempty"`
	ReplyPreview     string `json:"reply_preview,omitempty"`
	ModelUsed        string `json:"model_used,omitempty"`
	CreatedAt        string `json:"created_at"`
}

type AgentMemory struct {
	ID             int64  `json:"id"`
	Scope          string `json:"scope,omitempty"` // "global" or "bot:<id>"
	Category       string `json:"category"`        // e.g. "preference", "infra", "relation", "schedule", "insight", "general"
	FactKey        string `json:"fact_key"`
	Content        string `json:"content"`
	Importance     int    `json:"importance"`
	Pinned         bool   `json:"pinned"`
	RecallCount    int    `json:"recall_count"`
	LastRecalledAt string `json:"last_recalled_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type BotThread struct {
	ID        string `json:"id"`
	BotID     string `json:"bot_id"`
	Title     string `json:"title"`
	Summary   string `json:"summary,omitempty"`
	Archived  bool   `json:"archived"`
	MsgCount  int    `json:"msg_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type BotExecutionLog struct {
	ID            int64  `json:"id"`
	BotID         string `json:"bot_id"`
	ThreadID      string `json:"thread_id,omitempty"`
	ExecType      string `json:"exec_type"` // "tool" | "skill" | "routine"
	Name          string `json:"name"`
	InputSummary  string `json:"input_summary,omitempty"`
	OutputSummary string `json:"output_summary,omitempty"`
	Status        string `json:"status"`
	DurationMs    int64  `json:"duration_ms"`
	CreatedAt     string `json:"created_at"`
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

	list := make([]AgentSession, 0)
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
	res, err := s.db.Exec(`INSERT INTO agent_messages (session_id, bot_id, thread_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, model_used, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.SessionID, msg.BotID, msg.ThreadID, msg.SenderBotID, msg.MsgKind, msg.Role, msg.Content, msg.ReasoningContent, msg.ToolCalls, msg.ToolCallID, msg.ReplyToID, msg.ReplyPreview, msg.ModelUsed, msg.CreatedAt)
	if err != nil {
		return 0, err
	}
	if msg.SessionID != "" {
		_, _ = s.db.Exec(`UPDATE agent_sessions SET updated_at = ? WHERE id = ?`, now, msg.SessionID)
	}
	if msg.ThreadID != "" {
		_, _ = s.db.Exec(`UPDATE bot_threads SET updated_at = ? WHERE id = ?`, now, msg.ThreadID)
	}
	return res.LastInsertId()
}

func (s *Store) ListAgentMessages(sessionID string, limit int) ([]AgentMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, COALESCE(bot_id,''), COALESCE(thread_id,''), COALESCE(sender_bot_id,''), COALESCE(msg_kind,'chat'), role, content, reasoning_content, tool_calls, tool_call_id, COALESCE(reply_to_id,0), COALESCE(reply_preview,''), COALESCE(model_used,''), created_at
		FROM (
			SELECT id, session_id, bot_id, thread_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, model_used, created_at
			FROM agent_messages WHERE session_id = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AgentMessage, 0)
	for rows.Next() {
		var m AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.BotID, &m.ThreadID, &m.SenderBotID, &m.MsgKind, &m.Role, &m.Content, &m.ReasoningContent, &m.ToolCalls, &m.ToolCallID, &m.ReplyToID, &m.ReplyPreview, &m.ModelUsed, &m.CreatedAt); err == nil {
			list = append(list, m)
		}
	}
	return list, rows.Err()
}

func (s *Store) ListBotMessages(botID string, limit int) ([]AgentMessage, error) {
	return s.ListBotMessagesByThread(botID, "", limit)
}

func (s *Store) ListBotMessagesByThread(botID, threadID string, limit int) ([]AgentMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 80
	}
	threadID = strings.TrimSpace(threadID)
	rows, err := s.db.Query(`
		SELECT id, session_id, COALESCE(bot_id,''), COALESCE(thread_id,''), COALESCE(sender_bot_id,''), COALESCE(msg_kind,'chat'), role, content, reasoning_content, tool_calls, tool_call_id, COALESCE(reply_to_id,0), COALESCE(reply_preview,''), COALESCE(model_used,''), created_at
		FROM (
			SELECT id, session_id, bot_id, thread_id, sender_bot_id, msg_kind, role, content, reasoning_content, tool_calls, tool_call_id, reply_to_id, reply_preview, model_used, created_at
			FROM agent_messages WHERE bot_id = ? AND COALESCE(thread_id,'') = ? ORDER BY id DESC LIMIT ?
		) sub ORDER BY id ASC`, botID, threadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AgentMessage, 0)
	for rows.Next() {
		var m AgentMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.BotID, &m.ThreadID, &m.SenderBotID, &m.MsgKind, &m.Role, &m.Content, &m.ReasoningContent, &m.ToolCalls, &m.ToolCallID, &m.ReplyToID, &m.ReplyPreview, &m.ModelUsed, &m.CreatedAt); err == nil {
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

func (s *Store) ClearBotMessagesInThread(botID, threadID string) error {
	_, err := s.db.Exec(`DELETE FROM agent_messages WHERE bot_id = ? AND COALESCE(thread_id,'') = ?`, botID, strings.TrimSpace(threadID))
	return err
}

// BotThread CRUD (isolated multi-topic conversation archives strictly scoped to a Bot or Group)

func (s *Store) ListBotThreads(botID string) ([]BotThread, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.bot_id, t.title, COALESCE(t.summary,''), COALESCE(t.archived,0), t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM agent_messages m WHERE m.bot_id = t.bot_id AND COALESCE(m.thread_id,'') = t.id) AS msg_count
		FROM bot_threads t WHERE t.bot_id = ? ORDER BY t.archived ASC, t.updated_at DESC`, strings.TrimSpace(botID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]BotThread, 0)
	for rows.Next() {
		var item BotThread
		var arch int
		if err := rows.Scan(&item.ID, &item.BotID, &item.Title, &item.Summary, &arch, &item.CreatedAt, &item.UpdatedAt, &item.MsgCount); err == nil {
			item.Archived = arch == 1
			list = append(list, item)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetBotThread(botID, threadID string) (*BotThread, error) {
	var item BotThread
	var arch int
	err := s.db.QueryRow(`
		SELECT t.id, t.bot_id, t.title, COALESCE(t.summary,''), COALESCE(t.archived,0), t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM agent_messages m WHERE m.bot_id = t.bot_id AND COALESCE(m.thread_id,'') = t.id) AS msg_count
		FROM bot_threads t WHERE t.bot_id = ? AND t.id = ?`, strings.TrimSpace(botID), strings.TrimSpace(threadID)).
		Scan(&item.ID, &item.BotID, &item.Title, &item.Summary, &arch, &item.CreatedAt, &item.UpdatedAt, &item.MsgCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.Archived = arch == 1
	return &item, nil
}

func (s *Store) SaveBotThread(t BotThread) error {
	now := NowShanghai()
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if strings.TrimSpace(t.Title) == "" {
		t.Title = "新话题"
	}
	arch := 0
	if t.Archived {
		arch = 1
	}
	_, err := s.db.Exec(`INSERT INTO bot_threads (id, bot_id, title, summary, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			summary = excluded.summary,
			archived = excluded.archived,
			updated_at = excluded.updated_at`,
		t.ID, t.BotID, t.Title, t.Summary, arch, t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *Store) DeleteBotThread(botID, threadID string) error {
	tid := strings.TrimSpace(threadID)
	bid := strings.TrimSpace(botID)
	if tid == "" {
		return errors.New("cannot delete default main timeline without thread_id")
	}
	_, _ = s.db.Exec(`DELETE FROM agent_messages WHERE bot_id = ? AND COALESCE(thread_id,'') = ?`, bid, tid)
	_, err := s.db.Exec(`DELETE FROM bot_threads WHERE bot_id = ? AND id = ?`, bid, tid)
	return err
}

func (s *Store) SetBotActiveThread(botID, threadID string) error {
	_, err := s.db.Exec(`UPDATE bots SET active_thread_id = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(threadID), NowShanghai(), strings.TrimSpace(botID))
	return err
}

func (s *Store) DeleteAgentMemoryByScopeKey(scope, factKey string) (bool, error) {
	sc := strings.TrimSpace(scope)
	if sc == "" {
		sc = "global"
	}
	res, err := s.db.Exec(`DELETE FROM agent_memories WHERE COALESCE(NULLIF(TRIM(scope),''),'global') = ? AND fact_key = ?`,
		sc, strings.TrimSpace(factKey))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) PruneColdMemories(scope string, maxKeep int) (int64, error) {
	if maxKeep <= 0 {
		maxKeep = 30
	}
	sc := strings.TrimSpace(scope)
	if sc == "" {
		return 0, errors.New("scope is required")
	}
	res, err := s.db.Exec(`DELETE FROM agent_memories
		WHERE COALESCE(NULLIF(TRIM(scope),''),'global') = ?
		AND COALESCE(pinned, 0) = 0
		AND id NOT IN (
			SELECT id FROM agent_memories
			WHERE COALESCE(NULLIF(TRIM(scope),''),'global') = ?
			ORDER BY COALESCE(pinned,0) DESC, COALESCE(importance,3) DESC, COALESCE(recall_count,0) DESC, updated_at DESC
			LIMIT ?
		)`, sc, sc, maxKeep)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Scoped Memory CRUD

func (s *Store) SaveAgentMemory(category, factKey, content string) error {
	return s.SaveAgentMemoryWithScope("global", category, factKey, content)
}

func (s *Store) SaveAgentMemoryWithScope(scope, category, factKey, content string) error {
	return s.SaveAgentMemoryDetailed(scope, category, factKey, content, 3, false)
}

// NormalizeMemoryKey canonicalizes memory fact keys so trivial casing/spacing variants merge cleanly.
func NormalizeMemoryKey(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.Join(strings.Fields(s), "_")
	s = strings.Trim(s, "_")
	runes := []rune(s)
	if len(runes) > 80 {
		s = string(runes[:80])
	}
	return s
}

func (s *Store) SaveAgentMemoryDetailed(scope, category, factKey, content string, importance int, pinned bool) error {
	now := NowShanghai()
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "global"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "general"
	}
	factKey = NormalizeMemoryKey(factKey)
	content = strings.TrimSpace(content)
	if cRunes := []rune(content); len(cRunes) > 1200 {
		content = string(cRunes[:1200]) + "..."
	}
	if factKey == "" || content == "" {
		return errors.New("fact_key and content are required")
	}
	if importance < 1 {
		importance = 1
	} else if importance > 5 {
		importance = 5
	}
	pinInt := 0
	if pinned {
		pinInt = 1
	}

	_, err := s.db.Exec(`INSERT INTO agent_memories (scope, category, fact_key, content, importance, pinned, recall_count, last_recalled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, '', ?, ?)
		ON CONFLICT(scope, fact_key) DO UPDATE SET
			category = excluded.category,
			content = excluded.content,
			importance = CASE WHEN excluded.importance != 3 THEN excluded.importance ELSE agent_memories.importance END,
			pinned = CASE WHEN excluded.pinned = 1 THEN 1 ELSE agent_memories.pinned END,
			updated_at = excluded.updated_at`,
		scope, category, factKey, content, importance, pinInt, now, now)
	return err
}

func (s *Store) GetAgentMemory(id int64) (*AgentMemory, error) {
	var mem AgentMemory
	var pinInt int
	err := s.db.QueryRow(`SELECT id, COALESCE(NULLIF(TRIM(scope),''),'global'), category, fact_key, content,
		COALESCE(importance, 3), COALESCE(pinned, 0), COALESCE(recall_count, 0), COALESCE(last_recalled_at, ''),
		created_at, updated_at
		FROM agent_memories WHERE id = ?`, id).
		Scan(&mem.ID, &mem.Scope, &mem.Category, &mem.FactKey, &mem.Content, &mem.Importance, &pinInt, &mem.RecallCount, &mem.LastRecalledAt, &mem.CreatedAt, &mem.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	mem.Pinned = pinInt == 1
	return &mem, nil
}

func (s *Store) UpdateAgentMemory(id int64, scope, category, factKey, content string, importance int, pinned bool) error {
	now := NowShanghai()
	if importance < 1 {
		importance = 1
	} else if importance > 5 {
		importance = 5
	}
	pinInt := 0
	if pinned {
		pinInt = 1
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "global"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "general"
	}
	_, err := s.db.Exec(`UPDATE agent_memories SET scope = ?, category = ?, fact_key = ?, content = ?, importance = ?, pinned = ?, updated_at = ? WHERE id = ?`,
		scope, category, strings.TrimSpace(factKey), strings.TrimSpace(content), importance, pinInt, now, id)
	return err
}

func (s *Store) ListAgentMemories(query string, limit int) ([]AgentMemory, error) {
	return s.ListScopedMemories(nil, query, limit)
}

func (s *Store) ListScopedMemories(scopes []string, query string, limit int) ([]AgentMemory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var conds []string
	var args []any

	if len(scopes) > 0 {
		phs := make([]string, 0, len(scopes))
		for _, sc := range scopes {
			trimmed := strings.TrimSpace(sc)
			if trimmed != "" {
				phs = append(phs, "?")
				args = append(args, trimmed)
			}
		}
		if len(phs) > 0 {
			conds = append(conds, "COALESCE(NULLIF(TRIM(scope),''),'global') IN ("+strings.Join(phs, ",")+")")
		}
	}

	q := strings.TrimSpace(query)
	if q != "" {
		like := "%" + q + "%"
		conds = append(conds, "(fact_key LIKE ? OR content LIKE ? OR category LIKE ?)")
		args = append(args, like, like, like)
	}

	whereClause := ""
	if len(conds) > 0 {
		whereClause = "WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)

	sqlStr := `SELECT id, COALESCE(NULLIF(TRIM(scope),''),'global'), category, fact_key, content,
		COALESCE(importance, 3), COALESCE(pinned, 0), COALESCE(recall_count, 0), COALESCE(last_recalled_at, ''),
		created_at, updated_at
		FROM agent_memories ` + whereClause + ` ORDER BY COALESCE(pinned,0) DESC, COALESCE(importance,3) DESC, updated_at DESC, id DESC LIMIT ?`

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AgentMemory, 0)
	for rows.Next() {
		var mem AgentMemory
		var pinInt int
		if err := rows.Scan(&mem.ID, &mem.Scope, &mem.Category, &mem.FactKey, &mem.Content, &mem.Importance, &pinInt, &mem.RecallCount, &mem.LastRecalledAt, &mem.CreatedAt, &mem.UpdatedAt); err == nil {
			mem.Pinned = pinInt == 1
			list = append(list, mem)
		}
	}
	return list, rows.Err()
}

func (s *Store) TouchRecalledMemories(ids []int64) {
	if len(ids) == 0 {
		return
	}
	now := NowShanghai()
	for _, id := range ids {
		if id > 0 {
			_, _ = s.db.Exec(`UPDATE agent_memories SET recall_count = recall_count + 1, last_recalled_at = ? WHERE id = ?`, now, id)
		}
	}
}

func (s *Store) DeleteAgentMemory(id int64) error {
	_, err := s.db.Exec(`DELETE FROM agent_memories WHERE id = ?`, id)
	return err
}

// Per-Bot Execution Audit Logs (strictly scoped by bot_id)

func (s *Store) RecordBotExecution(entry BotExecutionLog) {
	if strings.TrimSpace(entry.BotID) == "" {
		return
	}
	if entry.CreatedAt == "" {
		entry.CreatedAt = NowShanghai()
	}
	if entry.Status == "" {
		entry.Status = "ok"
	}
	_, _ = s.db.Exec(`INSERT INTO bot_execution_logs (bot_id, thread_id, exec_type, name, input_summary, output_summary, status, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.BotID, entry.ThreadID, entry.ExecType, entry.Name, entry.InputSummary, entry.OutputSummary, entry.Status, entry.DurationMs, entry.CreatedAt)
}

func (s *Store) ListBotExecutions(botID string, limit int) ([]BotExecutionLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.Query(`SELECT id, bot_id, COALESCE(thread_id,''), exec_type, name, COALESCE(input_summary,''), COALESCE(output_summary,''), status, COALESCE(duration_ms,0), created_at
		FROM bot_execution_logs WHERE bot_id = ? ORDER BY id DESC LIMIT ?`, strings.TrimSpace(botID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]BotExecutionLog, 0)
	for rows.Next() {
		var l BotExecutionLog
		if err := rows.Scan(&l.ID, &l.BotID, &l.ThreadID, &l.ExecType, &l.Name, &l.InputSummary, &l.OutputSummary, &l.Status, &l.DurationMs, &l.CreatedAt); err == nil {
			list = append(list, l)
		}
	}
	return list, rows.Err()
}
