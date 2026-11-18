package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

const (
	BotStateIdle     = "idle"
	BotStateThinking = "thinking"
	BotStateWorking  = "working"
	BotStateBlocked  = "blocked"
	BotStateDone     = "done"

	BotModeWorker    = "worker"
	BotModeCompanion = "companion"
	BotModeHybrid    = "hybrid"

	MemoryPolicyIsolated = "isolated"
	MemoryPolicyStandard = "standard"
)

type Bot struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RoleTitle      string `json:"role_title"`
	AvatarStyle    string `json:"avatar_style"`
	SystemPrompt   string `json:"system_prompt"`
	BotMode        string `json:"bot_mode"`      // worker | companion | hybrid
	MemoryPolicy   string `json:"memory_policy"` // isolated | standard
	Traits            string `json:"traits"`        // tone / speaking style nuances
	Model             string `json:"model"`
	AllowPaidFallback bool   `json:"allow_paid_fallback"`
	EnableThinking    bool   `json:"enable_thinking"`
	AllowedTools   string `json:"allowed_tools"` // JSON string array
	SkillIDs       string `json:"skill_ids"`     // JSON string array of Skill IDs
	State          string `json:"state"`         // idle | thinking | working | blocked | done
	UnreadCount    int    `json:"unread_count"`
	IsPinned       bool   `json:"is_pinned"`
	IsBuiltin      bool   `json:"is_builtin"`
	IsGroup        bool   `json:"is_group"`
	MemberIDs      string `json:"member_ids"`   // JSON string array of Bot IDs when IsGroup=true
	GroupMode      string `json:"group_mode"`   // smart | sequential | parallel | mention_only
	Announcement   string `json:"announcement"` // Pinned group notice banner
	ActiveThreadID string `json:"active_thread_id"`
	SortOrder      int    `json:"sort_order"`
	LastSummary    string `json:"last_summary"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func NormalizeBotMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case BotModeWorker:
		return BotModeWorker
	case BotModeCompanion:
		return BotModeCompanion
	default:
		return BotModeHybrid
	}
}

func NormalizeMemoryPolicy(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case MemoryPolicyIsolated:
		return MemoryPolicyIsolated
	default:
		return MemoryPolicyStandard
	}
}

func (b Bot) AllowedToolList() []string {
	var list []string
	_ = json.Unmarshal([]byte(b.AllowedTools), &list)
	return list
}

func (b Bot) SkillIDList() []string {
	var list []string
	_ = json.Unmarshal([]byte(b.SkillIDs), &list)
	return list
}

func (b Bot) MemberIDList() []string {
	var list []string
	_ = json.Unmarshal([]byte(b.MemberIDs), &list)
	return list
}

const botSelectCols = `id, name, role_title, avatar_style, system_prompt,
		COALESCE(bot_mode,'hybrid'), COALESCE(memory_policy,'standard'), COALESCE(traits,''),
		model, COALESCE(allow_paid_fallback,0), enable_thinking, allowed_tools, COALESCE(skill_ids,'[]'),
		state, unread_count, is_pinned, is_builtin, is_group, member_ids,
		COALESCE(group_mode,'smart'), COALESCE(announcement,''), COALESCE(active_thread_id,''),
		sort_order, last_summary, created_at, updated_at`

func normalizeLoadedBot(b *Bot, paidFallback, et, pin, isGroup int) {
	b.AllowPaidFallback = paidFallback == 1
	b.EnableThinking = et == 1
	b.IsPinned = pin == 1
	b.IsBuiltin = false
	b.IsGroup = isGroup == 1
	b.BotMode = NormalizeBotMode(b.BotMode)
	b.MemoryPolicy = NormalizeMemoryPolicy(b.MemoryPolicy)
	if strings.TrimSpace(b.GroupMode) == "" {
		b.GroupMode = "smart"
	}
}

func (s *Store) ListBots() ([]Bot, error) {
	rows, err := s.db.Query(`SELECT ` + botSelectCols + ` FROM bots ORDER BY is_pinned DESC, sort_order ASC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Bot, 0)
	for rows.Next() {
		var b Bot
		var paidFallback, et, pin, isBuiltin, isGroup int
		if err := rows.Scan(
			&b.ID, &b.Name, &b.RoleTitle, &b.AvatarStyle, &b.SystemPrompt,
			&b.BotMode, &b.MemoryPolicy, &b.Traits,
			&b.Model, &paidFallback, &et, &b.AllowedTools, &b.SkillIDs,
			&b.State, &b.UnreadCount, &pin, &isBuiltin, &isGroup, &b.MemberIDs,
			&b.GroupMode, &b.Announcement, &b.ActiveThreadID,
			&b.SortOrder, &b.LastSummary, &b.CreatedAt, &b.UpdatedAt,
		); err == nil {
			normalizeLoadedBot(&b, paidFallback, et, pin, isGroup)
			list = append(list, b)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetBot(id string) (*Bot, error) {
	var b Bot
	var paidFallback, et, pin, isBuiltin, isGroup int
	err := s.db.QueryRow(`SELECT `+botSelectCols+` FROM bots WHERE id = ?`, strings.TrimSpace(id)).
		Scan(
			&b.ID, &b.Name, &b.RoleTitle, &b.AvatarStyle, &b.SystemPrompt,
			&b.BotMode, &b.MemoryPolicy, &b.Traits,
			&b.Model, &paidFallback, &et, &b.AllowedTools, &b.SkillIDs,
			&b.State, &b.UnreadCount, &pin, &isBuiltin, &isGroup, &b.MemberIDs,
			&b.GroupMode, &b.Announcement, &b.ActiveThreadID,
			&b.SortOrder, &b.LastSummary, &b.CreatedAt, &b.UpdatedAt,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	normalizeLoadedBot(&b, paidFallback, et, pin, isGroup)
	return &b, nil
}

func (s *Store) SaveBot(b Bot) error {
	now := NowShanghai()
	if b.CreatedAt == "" {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	if b.AvatarStyle == "" {
		b.AvatarStyle = "butler:gold"
	}
	b.BotMode = NormalizeBotMode(b.BotMode)
	b.MemoryPolicy = NormalizeMemoryPolicy(b.MemoryPolicy)
	if b.AllowedTools == "" {
		b.AllowedTools = "[]"
	}
	if strings.TrimSpace(b.SkillIDs) == "" {
		b.SkillIDs = "[]"
	}
	if strings.TrimSpace(b.MemberIDs) == "" {
		b.MemberIDs = "[]"
	}
	if strings.TrimSpace(b.GroupMode) == "" {
		b.GroupMode = "smart"
	}
	if b.State == "" {
		b.State = BotStateIdle
	}
	paidFallback := 0
	if b.AllowPaidFallback {
		paidFallback = 1
	}
	et := 0
	if b.EnableThinking {
		et = 1
	}
	pin := 0
	if b.IsPinned {
		pin = 1
	}
	builtin := 0
	if b.IsBuiltin {
		builtin = 1
	}
	isGroup := 0
	if b.IsGroup {
		isGroup = 1
	}
	_, err := s.db.Exec(`INSERT INTO bots (id, name, role_title, avatar_style, system_prompt, bot_mode, memory_policy, traits, model, allow_paid_fallback, enable_thinking, allowed_tools, skill_ids, state, unread_count, is_pinned, is_builtin, is_group, member_ids, group_mode, announcement, active_thread_id, sort_order, last_summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			role_title = excluded.role_title,
			avatar_style = excluded.avatar_style,
			system_prompt = excluded.system_prompt,
			bot_mode = excluded.bot_mode,
			memory_policy = excluded.memory_policy,
			traits = excluded.traits,
			model = excluded.model,
			allow_paid_fallback = excluded.allow_paid_fallback,
			enable_thinking = excluded.enable_thinking,
			allowed_tools = excluded.allowed_tools,
			skill_ids = excluded.skill_ids,
			state = excluded.state,
			is_pinned = excluded.is_pinned,
			is_group = excluded.is_group,
			member_ids = excluded.member_ids,
			group_mode = excluded.group_mode,
			announcement = excluded.announcement,
			active_thread_id = excluded.active_thread_id,
			sort_order = excluded.sort_order,
			last_summary = excluded.last_summary,
			updated_at = excluded.updated_at`,
		b.ID, b.Name, b.RoleTitle, b.AvatarStyle, b.SystemPrompt, b.BotMode, b.MemoryPolicy, b.Traits, b.Model, paidFallback, et, b.AllowedTools, b.SkillIDs, b.State, b.UnreadCount, pin, builtin, isGroup, b.MemberIDs, b.GroupMode, b.Announcement, b.ActiveThreadID, b.SortOrder, b.LastSummary, b.CreatedAt, b.UpdatedAt)
	return err
}

func (s *Store) DeleteBot(id string) error {
	trimmed := strings.TrimSpace(id)
	bots, _ := s.ListBots()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec(`DELETE FROM agent_messages WHERE bot_id = ?`, trimmed)
	_, _ = tx.Exec(`DELETE FROM bot_threads WHERE bot_id = ?`, trimmed)
	_, _ = tx.Exec(`DELETE FROM bot_execution_logs WHERE bot_id = ?`, trimmed)
	_, _ = tx.Exec(`DELETE FROM agent_memories WHERE scope = ?`, "bot:"+trimmed)
	_, _ = tx.Exec(`UPDATE tasks SET bot_id = '' WHERE bot_id = ?`, trimmed)
	_, _ = tx.Exec(`UPDATE todos SET bot_id = '' WHERE bot_id = ?`, trimmed)
	_, _ = tx.Exec(`UPDATE notes SET bot_id = '' WHERE bot_id = ?`, trimmed)
	for _, b := range bots {
		if !b.IsGroup {
			continue
		}
		cur := b.MemberIDList()
		next := make([]string, 0, len(cur))
		changed := false
		for _, mid := range cur {
			if mid == trimmed {
				changed = true
				continue
			}
			next = append(next, mid)
		}
		if changed {
			raw, _ := json.Marshal(next)
			_, _ = tx.Exec(`UPDATE bots SET member_ids = ?, updated_at = ? WHERE id = ?`, string(raw), NowShanghai(), b.ID)
		}
	}
	if _, err := tx.Exec(`DELETE FROM bots WHERE id = ?`, trimmed); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetBotState(id, state, lastSummary string) error {
	now := NowShanghai()
	if strings.TrimSpace(lastSummary) != "" {
		_, err := s.db.Exec(`UPDATE bots SET state = ?, last_summary = ?, updated_at = ? WHERE id = ?`, state, lastSummary, now, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE bots SET state = ?, updated_at = ? WHERE id = ?`, state, now, id)
	return err
}

func (s *Store) IncrementBotUnread(id, lastSummary string) error {
	now := NowShanghai()
	if strings.TrimSpace(lastSummary) != "" {
		_, err := s.db.Exec(`UPDATE bots SET unread_count = unread_count + 1, last_summary = ?, updated_at = ? WHERE id = ?`, lastSummary, now, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE bots SET unread_count = unread_count + 1, updated_at = ? WHERE id = ?`, now, id)
	return err
}

func (s *Store) ClearBotUnread(id string) error {
	_, err := s.db.Exec(`UPDATE bots SET unread_count = 0, state = CASE WHEN state = 'done' THEN 'idle' ELSE state END WHERE id = ?`, id)
	return err
}
