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

func DefaultBots(now string) []Bot {
	allTools := `["read_url","web_search","query_nezha_nodes","manage_assets","manage_tasks","remember_fact","recall_memory"]`
	return []Bot{
		{
			ID:             "team_room",
			Name:           "中枢指挥群",
			RoleTitle:      "多 Bot 协同与 @接力频道",
			AvatarStyle:    "swarm:indigo",
			SystemPrompt:   "你是 Vesper BotHub 中枢指挥群的协调官。擅长调度多位专属 Bot 协同排查问题，回答务必结构化、高信息密度，严禁使用任何 Emoji。",
			Model:          DefaultAIModel,
			EnableThinking: true,
			AllowedTools:   allTools,
			State:          BotStateIdle,
			IsPinned:       true,
			IsBuiltin:      true,
			IsGroup:        true,
			MemberIDs:      `["butler","sentinel","radar","grok"]`,
			GroupMode:      "smart",
			Announcement:   "欢迎来到中枢指挥群：支持 @指定 Bot 接力、右侧一键拉人调整成员，或直接下达指令由协同官智能调度。",
			SortOrder:      1,
			LastSummary:    "输入 @Bot名称 可点名接力协同",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "butler",
			Name:           "长庚执更",
			RoleTitle:      "资产守望与晨间简报管家",
			AvatarStyle:    "cyan",
			SystemPrompt:   "你是长庚执更管家，负责维护数字资产台账、VPS 与域名到期提醒、每日晨报与定时调度。语气沉稳可靠、条理清晰，严禁使用任何 Emoji。",
			Model:          DefaultAIModel,
			EnableThinking: true,
			AllowedTools:   `["manage_assets","manage_tasks","remember_fact","recall_memory","read_url"]`,
			State:          BotStateIdle,
			IsPinned:       true,
			IsBuiltin:      true,
			SortOrder:      2,
			LastSummary:    "负责每日晨报与资产到期守望",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "sentinel",
			Name:           "探针哨兵",
			RoleTitle:      "哪吒节点巡检与故障排查",
			AvatarStyle:    "amber",
			SystemPrompt:   "你是探针哨兵 Bot，专注于哪吒探针节点状态直连查询、VPS 负载与掉线根因分析。遇到告警直接给出 3 条以内硬核排查指令，严禁使用任何 Emoji。",
			Model:          DefaultAIModel,
			EnableThinking: true,
			AllowedTools:   `["query_nezha_nodes","read_url","recall_memory","remember_fact"]`,
			State:          BotStateIdle,
			IsPinned:       true,
			IsBuiltin:      true,
			SortOrder:      3,
			LastSummary:    "实时守望哪吒探针与 Webhook 告警",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "radar",
			Name:           "开源雷达",
			RoleTitle:      "全网前沿技术与趋势猎手",
			AvatarStyle:    "emerald",
			SystemPrompt:   "你是开源雷达 Bot，专注 GitHub Trending、Hacker News、Go/Cloud/Android 前沿工程趋势检索与深度提炼，结合用户技术栈输出高价值洞察，严禁使用任何 Emoji。",
			Model:          DefaultAIModel,
			EnableThinking: false,
			AllowedTools:   `["web_search","read_url","remember_fact","recall_memory"]`,
			State:          BotStateIdle,
			IsPinned:       false,
			IsBuiltin:      true,
			SortOrder:      4,
			LastSummary:    "定时追踪 GitHub 与 HN 前沿开源动态",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "grok",
			Name:           "Grok 锐评官",
			RoleTitle:      "高密度深度思考与架构顾问",
			AvatarStyle:    "violet",
			SystemPrompt:   "你是 Grok 锐评官，直击本质、拒绝废话、擅长第一性原理系统设计与毒辣犀利的方案评审。可直接调用全部工具查证事实后再结论，严禁使用任何 Emoji。",
			Model:          DefaultAIModel,
			EnableThinking: true,
			AllowedTools:   allTools,
			State:          BotStateIdle,
			IsPinned:       false,
			IsBuiltin:      true,
			SortOrder:      5,
			LastSummary:    "全局深度推理与系统架构评审",
			CreatedAt:      now,
			UpdatedAt:      now,
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
			b.IsBuiltin = builtin == 1
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
	b.IsBuiltin = builtin == 1
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
	_, _ = s.db.Exec(`UPDATE tasks SET bot_id = 'butler' WHERE bot_id = ?`, id)
	_, err := s.db.Exec(`DELETE FROM bots WHERE id = ? AND is_builtin = 0`, id)
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
