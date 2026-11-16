package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

type SkillParamField struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Type        string `json:"type"` // string | integer | boolean
	Required    bool   `json:"required"`
}

type Skill struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Kind         string `json:"kind"` // prompt | webhook
	Description  string `json:"description"`
	Instruction  string `json:"instruction"`
	HTTPMethod   string `json:"http_method"`
	HTTPURL      string `json:"http_url"`
	HTTPHeaders  string `json:"http_headers"`  // JSON object string
	ParamsSchema string `json:"params_schema"` // JSON array of SkillParamField
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func (sk Skill) Params() []SkillParamField {
	var fields []SkillParamField
	_ = json.Unmarshal([]byte(sk.ParamsSchema), &fields)
	return fields
}

func (sk Skill) HeadersMap() map[string]string {
	var m map[string]string
	_ = json.Unmarshal([]byte(sk.HTTPHeaders), &m)
	return m
}

func (s *Store) ListSkills() ([]Skill, error) {
	rows, err := s.db.Query(`SELECT id, name, slug, kind, description, instruction, http_method, http_url, http_headers, params_schema, enabled, created_at, updated_at
		FROM skills ORDER BY updated_at DESC, id ASC`)
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
	if sk.CreatedAt == "" {
		sk.CreatedAt = now
	}
	sk.UpdatedAt = now
	if sk.Kind != "webhook" {
		sk.Kind = "prompt"
	}
	if strings.TrimSpace(sk.HTTPMethod) == "" {
		sk.HTTPMethod = "GET"
	}
	if strings.TrimSpace(sk.HTTPHeaders) == "" {
		sk.HTTPHeaders = "{}"
	}
	if strings.TrimSpace(sk.ParamsSchema) == "" {
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
		sk.ID, sk.Name, sk.Slug, sk.Kind, sk.Description, sk.Instruction, strings.ToUpper(sk.HTTPMethod), sk.HTTPURL, sk.HTTPHeaders, sk.ParamsSchema, en, sk.CreatedAt, sk.UpdatedAt)
	return err
}

func (s *Store) DeleteSkill(id string) error {
	_, err := s.db.Exec(`DELETE FROM skills WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	// Clean dangling skill IDs from bots.skill_ids
	bots, err := s.ListBots()
	if err == nil {
		for _, b := range bots {
			cur := b.SkillIDList()
			changed := false
			next := make([]string, 0, len(cur))
			for _, sid := range cur {
				if sid == id {
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
	}
	return nil
}
