package store

import (
	"encoding/json"
	"sort"
	"strings"
)

const (
	ModelTierFree = "free"
	ModelTierPaid = "paid"
)

// NormalizeModelTier constrains pool tier to "free" or "paid" (default "free").
func NormalizeModelTier(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), ModelTierPaid) {
		return ModelTierPaid
	}
	return ModelTierFree
}

type ModelPoolEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Tier     string   `json:"tier"`     // free | paid
	Provider string   `json:"provider"` // openai | gemini | deepseek | custom
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	Model    string   `json:"model"`            // primary / default model on this endpoint
	Models   []string `json:"models,omitempty"` // multiple checked model IDs sharing BaseURL+APIKey
	Enabled  bool     `json:"enabled"`
	Priority int      `json:"priority"`
	Note     string   `json:"note"`
}

// EffectiveModels returns the deduplicated list of enabled model IDs configured on this provider entry.
func (e ModelPoolEntry) EffectiveModels() []string {
	out := make([]string, 0, len(e.Models)+1)
	seen := make(map[string]bool, len(e.Models)+1)
	for _, raw := range e.Models {
		m := strings.TrimSpace(raw)
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	if primary := strings.TrimSpace(e.Model); primary != "" && !seen[primary] {
		out = append([]string{primary}, out...)
	}
	return out
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
	for i := range list {
		list[i].Tier = NormalizeModelTier(list[i].Tier)
		eff := list[i].EffectiveModels()
		if len(eff) > 0 {
			list[i].Model = eff[0]
			list[i].Models = eff
		} else {
			list[i].Models = nil
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Tier != list[j].Tier {
			return list[i].Tier == ModelTierFree
		}
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
		e.Tier = NormalizeModelTier(e.Tier)
		e.Provider = strings.TrimSpace(e.Provider)
		e.BaseURL = strings.TrimRight(strings.TrimSpace(e.BaseURL), "/")
		e.Model = strings.TrimSpace(e.Model)
		e.APIKey = strings.TrimSpace(e.APIKey)
		if e.ID == "" || e.BaseURL == "" {
			continue
		}
		eff := e.EffectiveModels()
		if len(eff) > 0 {
			e.Model = eff[0]
			e.Models = eff
		} else {
			e.Models = nil
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
