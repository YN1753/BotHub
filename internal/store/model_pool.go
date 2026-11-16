package store

import (
	"encoding/json"
	"sort"
	"strings"
)

type ModelPoolEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // openai | google | custom
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
	Note     string `json:"note"`
}

func (s *Store) GetModelPool() []ModelPoolEntry {
	raw, err := s.GetConfig(KeyFreeModelPool)
	if err != nil || strings.TrimSpace(raw) == "" {
		return []ModelPoolEntry{}
	}
	var list []ModelPoolEntry
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []ModelPoolEntry{}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Priority < list[j].Priority
	})
	return list
}

func (s *Store) SaveModelPool(entries []ModelPoolEntry) error {
	existing := s.GetModelPool()
	existingKeys := make(map[string]string, len(existing))
	for _, item := range existing {
		existingKeys[item.ID] = item.APIKey
	}

	cleaned := make([]ModelPoolEntry, 0, len(entries))
	for idx, e := range entries {
		e.ID = strings.TrimSpace(e.ID)
		e.Name = strings.TrimSpace(e.Name)
		e.Provider = strings.TrimSpace(e.Provider)
		e.BaseURL = strings.TrimRight(strings.TrimSpace(e.BaseURL), "/")
		e.Model = strings.TrimSpace(e.Model)
		e.APIKey = strings.TrimSpace(e.APIKey)
		if e.ID == "" || e.BaseURL == "" {
			continue
		}
		if e.Name == "" {
			e.Name = e.Model
		}
		if strings.Contains(e.APIKey, "****") {
			e.APIKey = existingKeys[e.ID]
		}
		if e.Priority == 0 {
			e.Priority = (idx + 1) * 10
		}
		cleaned = append(cleaned, e)
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return err
	}
	return s.SetConfig(KeyFreeModelPool, string(b))
}

func MaskSecretValue(val string) string {
	v := strings.TrimSpace(val)
	if v == "" {
		return ""
	}
	runes := []rune(v)
	if len(runes) <= 8 {
		return "****"
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}

func MaskModelPool(entries []ModelPoolEntry) []ModelPoolEntry {
	out := make([]ModelPoolEntry, 0, len(entries))
	for _, e := range entries {
		copyEntry := e
		copyEntry.APIKey = MaskSecretValue(e.APIKey)
		out = append(out, copyEntry)
	}
	return out
}
