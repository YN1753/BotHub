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
)

type Bot struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	RoleTitle      string `json:"role_title"`
	AvatarStyle    string `json:"avatar_style"`
	SystemPrompt   string `json:"system_prompt"`
	Model          string `json:"model"`
	EnableThinking bool   `json:"enable_thinking"`
	AllowedTools   string `json:"allowed_tools"` // JSON string array
	State          string `json:"state"`         // idle | thinking | working | blocked | done
	UnreadCount    int    `json:"unread_count"`
	IsPinned       bool   `json:"is_pinned"`
	IsBuiltin      bool   `json:"is_builtin"`
	IsGroup        bool   `json:"is_group"`
	MemberIDs      string `json:"member_ids"`   // JSON string array of Bot IDs when IsGroup=true
	GroupMode      string `json:"group_mode"`   // smart | sequential | mention_only
	Announcement   string `json:"announcement"` // Pinned group notice banner
	SortOrder      int    `json:"sort_order"`
	LastSummary    string `json:"last_summary"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func (b Bot) AllowedToolList() []string {
	var list []string
	_ = json.Unmarshal([]byte(b.AllowedTools), &list)
	return list
}

func (b Bot) MemberIDList() []string {
	var list []string
	_ = json.Unmarshal([]byte(b.MemberIDs), &list)
	return list
}

// BotTemplate is served strictly as an optional creation preset (never auto-seeded into the bots table).
type BotTemplate struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	RoleTitle      string   `json:"role_title"`
	AvatarStyle    string   `json:"avatar_style"`
	SystemPrompt   string   `json:"system_prompt"`
	EnableThinking bool     `json:"enable_thinking"`
	AllowedTools   []string `json:"allowed_tools"`
}

func BuiltinBotTemplates() []BotTemplate {
	return []BotTemplate{
		{
			ID:             "butler_tpl",
			Name:           "资产与晨报管家",
			RoleTitle:      "资产守望与晨间简报管家",
			AvatarStyle:    "butler:gold",
			SystemPrompt:   "你是用户的数字资产与日常简报管家，擅长追踪服务器、域名与会员订阅到期提醒、制定续费策略与日常日程编排，语气沉稳条理清晰，严禁使用 Emoji。",
			EnableThinking: true,
			AllowedTools:   []string{"manage_assets", "manage_tasks", "remember_fact", "recall_memory", "read_url"},
		},
		{
			ID:             "sentinel_tpl",
			Name:           "探针巡检哨兵",
			RoleTitle:      "哪吒节点巡检与故障排查",
			AvatarStyle:    "sentinel:amber",
			SystemPrompt:   "你是运维探针哨兵，专注直连查询节点在线状态、负载、流量健康度与故障根因分析，遇到异常直接给出可落地的排查指令，严禁使用 Emoji。",
			EnableThinking: true,
			AllowedTools:   []string{"query_nezha_nodes", "read_url", "recall_memory", "remember_fact"},
		},
		{
			ID:             "radar_tpl",
			Name:           "开源技术雷达",
			RoleTitle:      "前沿技术检索与趋势精读",
			AvatarStyle:    "radar:emerald",
			SystemPrompt:   "你是开源技术雷达助手，专注 GitHub 热门项目、工程技术趋势检索与长文章摘要提炼，输出高信息密度结论，严禁使用 Emoji。",
			EnableThinking: false,
			AllowedTools:   []string{"web_search", "read_url", "remember_fact", "recall_memory"},
		},
		{
			ID:             "architect_tpl",
			Name:           "架构锐评顾问",
			RoleTitle:      "高密度深度思考与代码评审",
			AvatarStyle:    "critic:violet",
			SystemPrompt:   "你是系统架构评审顾问，直击问题本质、拒绝空话，擅长第一性原理系统设计与并发/安全隐患剖析。优先查证工具数据后再下结论，严禁使用 Emoji。",
			EnableThinking: true,
			AllowedTools:   []string{"read_url", "web_search", "query_nezha_nodes", "manage_assets", "manage_tasks", "remember_fact", "recall_memory"},
		},
		{
			ID:             "translator_tpl",
			Name:           "技术文档编译官",
			RoleTitle:      "外文资料与 RFC 精炼编译",
			AvatarStyle:    "visor:indigo",
			SystemPrompt:   "你是技术文档编译官，擅长把英文博客、RFC 与 Release Notes 提炼为结构化中文重点与可执行迁移清单，严禁使用 Emoji。",
			EnableThinking: false,
			AllowedTools:   []string{"read_url", "web_search", "remember_fact"},
		},
	}
}

func (s *Store) ListBots() ([]Bot, error) {
	rows, err := s.db.Query(`SELECT id, name, role_title, avatar_style, system_prompt, model, enable_thinking, allowed_tools, state, unread_count, is_pinned, is_builtin, is_group, member_ids, COALESCE(group_mode,'smart'), COALESCE(announcement,''), sort_order, last_summary, created_at, updated_at
		FROM bots ORDER BY is_pinned DESC, sort_order ASC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Bot
	for rows.Next() {
		var b Bot
		var et, pin, builtin, isGroup int
		if err := rows.Scan(&b.ID, &b.Name, &b.RoleTitle, &b.AvatarStyle, &b.SystemPrompt, &b.Model, &et, &b.AllowedTools, &b.State, &b.UnreadCount, &pin, &builtin, &isGroup, &b.MemberIDs, &b.GroupMode, &b.Announcement, &b.SortOrder, &b.LastSummary, &b.CreatedAt, &b.UpdatedAt); err == nil {
			b.EnableThinking = et == 1
			b.IsPinned = pin == 1
			b.IsBuiltin = false
			b.IsGroup = isGroup == 1 || b.ID == "team_room"
			if strings.TrimSpace(b.GroupMode) == "" {
				b.GroupMode = "smart"
			}
			list = append(list, b)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetBot(id string) (*Bot, error) {
	var b Bot
	var et, pin, builtin, isGroup int
	err := s.db.QueryRow(`SELECT id, name, role_title, avatar_style, system_prompt, model, enable_thinking, allowed_tools, state, unread_count, is_pinned, is_builtin, is_group, member_ids, COALESCE(group_mode,'smart'), COALESCE(announcement,''), sort_order, last_summary, created_at, updated_at
		FROM bots WHERE id = ?`, strings.TrimSpace(id)).
		Scan(&b.ID, &b.Name, &b.RoleTitle, &b.AvatarStyle, &b.SystemPrompt, &b.Model, &et, &b.AllowedTools, &b.State, &b.UnreadCount, &pin, &builtin, &isGroup, &b.MemberIDs, &b.GroupMode, &b.Announcement, &b.SortOrder, &b.LastSummary, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.EnableThinking = et == 1
	b.IsPinned = pin == 1
	b.IsBuiltin = false
	b.IsGroup = isGroup == 1 || b.ID == "team_room"
	if strings.TrimSpace(b.GroupMode) == "" {
		b.GroupMode = "smart"
	}
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
	if b.Model == "" {
		b.Model = DefaultAIModel
	}
	if b.AllowedTools == "" {
		b.AllowedTools = "[]"
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
	_, err := s.db.Exec(`INSERT INTO bots (id, name, role_title, avatar_style, system_prompt, model, enable_thinking, allowed_tools, state, unread_count, is_pinned, is_builtin, is_group, member_ids, group_mode, announcement, sort_order, last_summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			role_title = excluded.role_title,
			avatar_style = excluded.avatar_style,
			system_prompt = excluded.system_prompt,
			model = excluded.model,
			enable_thinking = excluded.enable_thinking,
			allowed_tools = excluded.allowed_tools,
			state = excluded.state,
			is_pinned = excluded.is_pinned,
			is_group = excluded.is_group,
			member_ids = excluded.member_ids,
			group_mode = excluded.group_mode,
			announcement = excluded.announcement,
			sort_order = excluded.sort_order,
			last_summary = excluded.last_summary,
			updated_at = excluded.updated_at`,
		b.ID, b.Name, b.RoleTitle, b.AvatarStyle, b.SystemPrompt, b.Model, et, b.AllowedTools, b.State, b.UnreadCount, pin, builtin, isGroup, b.MemberIDs, b.GroupMode, b.Announcement, b.SortOrder, b.LastSummary, b.CreatedAt, b.UpdatedAt)
	return err
}

func (s *Store) DeleteBot(id string) error {
	_, _ = s.db.Exec(`DELETE FROM agent_messages WHERE bot_id = ?`, id)
	_, _ = s.db.Exec(`UPDATE tasks SET bot_id = '' WHERE bot_id = ?`, id)
	_, err := s.db.Exec(`DELETE FROM bots WHERE id = ?`, id)
	return err
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
