package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	SkillKindPrompt     = "prompt"
	SkillKindWebhook    = "webhook"
	SkillKindMCP        = "mcp"
	SkillKindBuiltinMCP = "builtin_mcp"

	BuiltinMCPWorkbenchID = "mcp_workbench"
	BuiltinMCPAssetsID    = "mcp_assets"
	BuiltinMCPNezhaID     = "mcp_nezha"
	BuiltinMCPOpsID       = "mcp_ops"
)

type SkillParamField struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Type        string `json:"type"` // string | integer | boolean
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// MCPToolMeta describes a tool exposed by a built-in or remote MCP server.
type MCPToolMeta struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type Skill struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Kind         string `json:"kind"` // builtin_mcp | mcp | webhook | prompt
	Description  string `json:"description"`
	Instruction  string `json:"instruction"`
	HTTPMethod   string `json:"http_method"`
	HTTPURL      string `json:"http_url"`
	HTTPHeaders  string `json:"http_headers"`  // JSON object string
	ParamsSchema string `json:"params_schema"` // JSON array of SkillParamField (webhook) or MCPToolMeta (builtin_mcp / mcp)
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func NormalizeSkillKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SkillKindBuiltinMCP:
		return SkillKindBuiltinMCP
	case SkillKindMCP, "mcp_http", "mcp_sse", "remote_mcp":
		return SkillKindMCP
	case SkillKindWebhook, "http":
		return SkillKindWebhook
	default:
		return SkillKindPrompt
	}
}

func (sk Skill) IsBuiltinMCP() bool {
	return sk.Kind == SkillKindBuiltinMCP || strings.HasPrefix(sk.ID, "mcp_") && (sk.ID == BuiltinMCPWorkbenchID || sk.ID == BuiltinMCPAssetsID || sk.ID == BuiltinMCPNezhaID || sk.ID == BuiltinMCPOpsID)
}

func (sk Skill) Params() []SkillParamField {
	var fields []SkillParamField
	_ = json.Unmarshal([]byte(sk.ParamsSchema), &fields)
	return fields
}

func (sk Skill) MCPTools() []MCPToolMeta {
	var tools []MCPToolMeta
	if err := json.Unmarshal([]byte(sk.ParamsSchema), &tools); err == nil && len(tools) > 0 {
		out := make([]MCPToolMeta, 0, len(tools))
		for _, t := range tools {
			if strings.TrimSpace(t.Name) != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func (sk Skill) BuiltinMCPToolNames() []string {
	if !sk.IsBuiltinMCP() {
		return nil
	}
	tools := sk.MCPTools()
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		if name := strings.TrimSpace(t.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func (sk Skill) TriggerKeywords() []string {
	raw := strings.TrimSpace(sk.Instruction)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '|'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.ToLower(strings.TrimSpace(p)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (sk Skill) HeadersMap() map[string]string {
	var m map[string]string
	_ = json.Unmarshal([]byte(sk.HTTPHeaders), &m)
	return m
}

func mustMarshalMCPTools(items []MCPToolMeta) string {
	b, _ := json.Marshal(items)
	return string(b)
}

// BuiltinMCPServers returns the 4 modular built-in domain MCP servers.
func BuiltinMCPServers() []Skill {
	return []Skill{
		{
			ID:          BuiltinMCPWorkbenchID,
			Name:        "个人工作台 MCP",
			Slug:        "workbench",
			Kind:        SkillKindBuiltinMCP,
			Description: "待办事项清单(Todos)、长文笔记(Notes)、闪念与网页收藏匣(Stash)及工作台今日简报",
			Instruction: "待办,todo,提醒我,记一下,待办清单,日程,笔记,note,写篇文档,备忘,纪要,收藏匣,闪念,稍后读,书签,stash,工作台,简报",
			HTTPMethod:  "LOCAL",
			HTTPURL:     "mcp://builtin/workbench",
			HTTPHeaders: "{}",
			ParamsSchema: mustMarshalMCPTools([]MCPToolMeta{
				{Name: "manage_todos", Description: "待办事项增删改查、完成勾选与截止时间管理"},
				{Name: "manage_notes", Description: "长文笔记与知识库文档的创建、检索与修订"},
				{Name: "manage_stash", Description: "网页链接收藏匣与随手闪念的录入、检索与 AI 提炼"},
				{Name: "workspace_briefing", Description: "获取工作台待办、资产与今日日程的全局简报"},
			}),
			Enabled: true,
		},
		{
			ID:          BuiltinMCPAssetsID,
			Name:        "数字资产管家 MCP",
			Slug:        "assets",
			Kind:        SkillKindBuiltinMCP,
			Description: "VPS 服务器、域名、SSL 证书与软件订阅的续费账单、到期日与成本核算管理",
			Instruction: "数字资产,资产台账,vps,域名,ssl证书,服务器账单,续费,到期,订阅开销,主机台账,买鸡",
			HTTPMethod:  "LOCAL",
			HTTPURL:     "mcp://builtin/assets",
			HTTPHeaders: "{}",
			ParamsSchema: mustMarshalMCPTools([]MCPToolMeta{
				{Name: "manage_assets", Description: "管理 VPS、域名与订阅资产的到期时间、续费金额与台账状态"},
			}),
			Enabled: true,
		},
		{
			ID:          BuiltinMCPNezhaID,
			Name:        "哪吒探针监控 MCP",
			Slug:        "nezha",
			Kind:        SkillKindBuiltinMCP,
			Description: "哪吒监控面板服务器节点在线状态、CPU/内存/磁盘/负载与实时上下行流量遥测",
			Instruction: "哪吒,探针,服务器节点,节点状态,在线率,节点离线,服务器负载,cpu占用,内存占用,磁盘空间,网卡流量,宕机",
			HTTPMethod:  "LOCAL",
			HTTPURL:     "mcp://builtin/nezha",
			HTTPHeaders: "{}",
			ParamsSchema: mustMarshalMCPTools([]MCPToolMeta{
				{Name: "query_nezha_nodes", Description: "实时查询哪吒监控面板各服务器节点的在线状态与硬件负载遥测"},
			}),
			Enabled: true,
		},
		{
			ID:          BuiltinMCPOpsID,
			Name:        "自动化与系统管控 MCP",
			Slug:        "ops",
			Kind:        SkillKindBuiltinMCP,
			Description: "Cron 定时例程任务编排、服务通知即时推送、Bot 成员与群聊管理、模型池与 MCP 扩展编排",
			Instruction: "定时任务,定时推送,cron,例程,计划任务,发通知,推送通知,创建bot,新建bot,新建群聊,拉个群,模型池,冷却状态,创建技能,创建mcp,挂载mcp",
			HTTPMethod:  "LOCAL",
			HTTPURL:     "mcp://builtin/ops",
			HTTPHeaders: "{}",
			ParamsSchema: mustMarshalMCPTools([]MCPToolMeta{
				{Name: "manage_tasks", Description: "创建、修改、启停或立即触发 Cron 定时任务与单次提醒"},
				{Name: "send_notification", Description: "向服务通知中心即时发送结构化推送通知"},
				{Name: "manage_bots", Description: "创建或配置专属 Bot 与多智能体协同群聊"},
				{Name: "manage_models", Description: "查看 AI 模型池健康度、解除冷却与调整免费/付费调度策略"},
				{Name: "manage_skills", Description: "管理、创建或为 Bot 绑定 MCP 服务与自定义技能"},
			}),
			Enabled: true,
		},
	}
}

func (s *Store) SeedBuiltinMCPs() error {
	now := NowShanghai()
	for _, mcp := range BuiltinMCPServers() {
		existing, _ := s.GetSkill(mcp.ID)
		if existing != nil {
			// Refresh built-in MCP schema/description while preserving user's enabled toggle
			mcp.Enabled = existing.Enabled
			mcp.CreatedAt = existing.CreatedAt
		} else {
			mcp.CreatedAt = now
		}
		if err := s.SaveSkill(mcp); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListSkills() ([]Skill, error) {
	rows, err := s.db.Query(`SELECT id, name, slug, kind, description, instruction, http_method, http_url, http_headers, params_schema, enabled, created_at, updated_at
		FROM skills ORDER BY CASE WHEN kind = 'builtin_mcp' THEN 0 WHEN kind = 'mcp' THEN 1 ELSE 2 END ASC, updated_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]Skill, 0)
	for rows.Next() {
		var sk Skill
		var en int
		if err := rows.Scan(&sk.ID, &sk.Name, &sk.Slug, &sk.Kind, &sk.Description, &sk.Instruction, &sk.HTTPMethod, &sk.HTTPURL, &sk.HTTPHeaders, &sk.ParamsSchema, &en, &sk.CreatedAt, &sk.UpdatedAt); err == nil {
			sk.Enabled = en == 1
			list = append(list, sk)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetSkill(id string) (*Skill, error) {
	var sk Skill
	var en int
	err := s.db.QueryRow(`SELECT id, name, slug, kind, description, instruction, http_method, http_url, http_headers, params_schema, enabled, created_at, updated_at
		FROM skills WHERE id = ?`, strings.TrimSpace(id)).
		Scan(&sk.ID, &sk.Name, &sk.Slug, &sk.Kind, &sk.Description, &sk.Instruction, &sk.HTTPMethod, &sk.HTTPURL, &sk.HTTPHeaders, &sk.ParamsSchema, &en, &sk.CreatedAt, &sk.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sk.Enabled = en == 1
	return &sk, nil
}

func (s *Store) GetSkillBySlug(slug string) (*Skill, error) {
	var sk Skill
	var en int
	err := s.db.QueryRow(`SELECT id, name, slug, kind, description, instruction, http_method, http_url, http_headers, params_schema, enabled, created_at, updated_at
		FROM skills WHERE slug = ?`, strings.TrimSpace(slug)).
		Scan(&sk.ID, &sk.Name, &sk.Slug, &sk.Kind, &sk.Description, &sk.Instruction, &sk.HTTPMethod, &sk.HTTPURL, &sk.HTTPHeaders, &sk.ParamsSchema, &en, &sk.CreatedAt, &sk.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sk.Enabled = en == 1
	return &sk, nil
}

func (s *Store) GetSkillsByIDs(ids []string) ([]Skill, error) {
	if len(ids) == 0 {
		return []Skill{}, nil
	}
	all, err := s.ListSkills()
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			want[trimmed] = true
		}
	}
	out := make([]Skill, 0, len(want))
	for _, sk := range all {
		if sk.Enabled && want[sk.ID] {
			out = append(out, sk)
		}
	}
	return out, nil
}

func (s *Store) SaveSkill(sk Skill) error {
	now := NowShanghai()
	if strings.TrimSpace(sk.CreatedAt) == "" {
		sk.CreatedAt = now
	}
	sk.UpdatedAt = now
	sk.ID = strings.TrimSpace(sk.ID)
	sk.Name = strings.TrimSpace(sk.Name)
	sk.Slug = strings.TrimSpace(sk.Slug)
	sk.HTTPURL = strings.TrimSpace(sk.HTTPURL)
	sk.Kind = NormalizeSkillKind(sk.Kind)
	sk.HTTPMethod = strings.ToUpper(strings.TrimSpace(sk.HTTPMethod))
	if sk.HTTPMethod == "" {
		if sk.Kind == SkillKindMCP {
			sk.HTTPMethod = "POST"
		} else if sk.Kind == SkillKindBuiltinMCP {
			sk.HTTPMethod = "LOCAL"
		} else {
			sk.HTTPMethod = "GET"
		}
	}
	if strings.TrimSpace(sk.HTTPHeaders) == "" || !json.Valid([]byte(sk.HTTPHeaders)) {
		sk.HTTPHeaders = "{}"
	}
	if strings.TrimSpace(sk.ParamsSchema) == "" || !json.Valid([]byte(sk.ParamsSchema)) {
		sk.ParamsSchema = "[]"
	}
	en := 0
	if sk.Enabled {
		en = 1
	}
	_, err := s.db.Exec(`INSERT INTO skills (id, name, slug, kind, description, instruction, http_method, http_url, http_headers, params_schema, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			slug = excluded.slug,
			kind = excluded.kind,
			description = excluded.description,
			instruction = excluded.instruction,
			http_method = excluded.http_method,
			http_url = excluded.http_url,
			http_headers = excluded.http_headers,
			params_schema = excluded.params_schema,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`,
		sk.ID, sk.Name, sk.Slug, sk.Kind, sk.Description, sk.Instruction, sk.HTTPMethod, sk.HTTPURL, sk.HTTPHeaders, sk.ParamsSchema, en, sk.CreatedAt, sk.UpdatedAt)
	return err
}

func (s *Store) DeleteSkill(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	if existing, _ := s.GetSkill(trimmed); existing != nil && existing.IsBuiltinMCP() {
		return fmt.Errorf("系统内置 MCP 服务「%s」不支持删除，如无需使用可将其停用或从 Bot 解绑", existing.Name)
	}
	_, err := s.db.Exec(`DELETE FROM skills WHERE id = ?`, trimmed)
	if err != nil {
		return err
	}
	// Clean dangling skill IDs from bots.skill_ids
	bots, err := s.ListBots()
	if err != nil {
		return nil
	}
	for _, b := range bots {
		cur := b.SkillIDList()
		changed := false
		next := make([]string, 0, len(cur))
		for _, sid := range cur {
			if sid == trimmed {
				changed = true
				continue
			}
			next = append(next, sid)
		}
		if changed {
			raw, _ := json.Marshal(next)
			b.SkillIDs = string(raw)
			_ = s.SaveBot(b)
		}
	}
	return nil
}
