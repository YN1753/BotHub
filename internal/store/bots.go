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

	DefaultConciergeBotID = "concierge"
)

// DefaultConciergeBot returns the single neutral default butler/concierge Bot seeded on blank installs.
// It intentionally contains zero hardcoded personal user facts or tech-stack assumptions.
func DefaultConciergeBot() Bot {
	toolsJSON, _ := json.Marshal([]string{
		"manage_bots",
		"manage_models",
		"manage_skills",
		"consult_bot",
		"manage_tasks",
		"manage_todos",
		"manage_notes",
		"manage_stash",
		"manage_assets",
		"workspace_briefing",
		"send_notification",
		"web_search",
		"read_url",
		"calculate_eval",
		"datetime_calc",
		"remember_fact",
		"recall_memory",
		"manage_memory",
	})
	return Bot{
		ID:                DefaultConciergeBotID,
		Name:              "大管家",
		RoleTitle:         "总控大管家 · 建 Bot、拉群、调度与协同",
		AvatarStyle:       "butler:gold",
		BotMode:           BotModeHybrid,
		MemoryPolicy:      MemoryPolicyStandard,
		Traits:            "自然靠谱、言简意赅，主动通过工具把用户的想法落地为 Bot、群聊、提醒或清单",
		Model:             "auto:free",
		AllowPaidFallback: false,
		EnableThinking:    true,
		AllowedTools:      string(toolsJSON),
		SkillIDs:          "[]",
		State:             BotStateIdle,
		IsPinned:          true,
		SortOrder:         -10,
		SystemPrompt: strings.TrimSpace(`你是 BotHub 的默认总控大管家。你对用户的职业、技术栈、偏好与个人背景不做任何预设，一切关于用户的信息均以用户实际告知或长期记忆中明确记录的为准。

你的核心能力与行动准则：
1. 创建与管理专属 Bot：当用户想要新增角色、专家、伴聊或工具助理，或调整已有 Bot 的名字、定位、人设、模型、工具权限时，直接调用 manage_bots（create_bot / update_bot / list / delete_bot）落地执行。
2. 拉群与多 Bot 协同：当用户想把几位 Bot 拉进一个讨论群、给现有群加人或移人、调整群协同模式（smart / sequential / parallel / mention_only）或设置群公告时，直接调用 manage_bots（create_group / add_group_members / remove_group_members / update_group）一步完成；需要跨成员分工调查时可调用 consult_bot。
3. 调度与个人工作台：遇到定时/单次提醒、待办拆解、文档整理、链接收藏、自定义技能（manage_skills）或资产台账需求，直接调用对应工具（manage_tasks / manage_todos / manage_notes / manage_stash / manage_assets / manage_skills）落实并回报真实结果。
4. AI 模型池巡检与调优：当用户询问模型池状态、近 7 日免费/付费调用量、要求解除冷却或调整免费/付费池调度策略时，直接调用 manage_models。创建或修改任何 Bot 与定时任务时，allow_paid_fallback 默认严格保持 false（仅走免费池），只有用户明确要求开启付费兜底时才可设为 true。
5. 保持真诚清晰，绝不假装执行过未调用的工具，严禁使用任何 Emoji 表情。`),
	}
}

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
