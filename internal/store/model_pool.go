package store

import (
	"encoding/json"
	"sort"
	"strings"
)

type ModelPoolEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // google | siliconflow | zhipu | groq | openrouter | custom
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
	Note     string `json:"note"`
}

func DefaultModelPool() []ModelPoolEntry {
	return []ModelPoolEntry{
		{
			ID:       "google_ai_studio",
			Name:     "Google AI Studio · Gemini Flash",
			Provider: "google",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta/openai",
			Model:    "gemini-2.5-flash",
			Enabled:  true,
			Priority: 10,
			Note:     "Google AI Studio 官方 OpenAI 兼容免费额度，适合长文本摘要与日常对话",
		},
		{
			ID:       "siliconflow_free",
			Name:     "硅基流动 SiliconFlow · Qwen2.5-7B",
			Provider: "siliconflow",
			BaseURL:  "https://api.siliconflow.cn/v1",
			Model:    "Qwen/Qwen2.5-7B-Instruct",
			Enabled:  true,
			Priority: 20,
			Note:     "国内直连零代理，硅基流动免费标签模型稳定可用",
		},
		{
			ID:       "zhipu_glm4_flash",
			Name:     "智谱开放平台 · GLM-4-Flash",
			Provider: "zhipu",
			BaseURL:  "https://open.bigmodel.cn/api/paas/v4",
			Model:    "glm-4-flash",
			Enabled:  true,
			Priority: 30,
			Note:     "官方永久免费高速中文模型，支持 Function Calling",
		},
		{
			ID:       "groq_free",
			Name:     "Groq Cloud · Llama 3.3 70B",
			Provider: "groq",
			BaseURL:  "https://api.groq.com/openai/v1",
			Model:    "llama-3.3-70b-versatile",
			Enabled:  true,
			Priority: 40,
			Note:     "LPU 超高速免费推理额度",
		},
		{
			ID:       "openrouter_free",
			Name:     "OpenRouter · :free 模型池",
			Provider: "openrouter",
			BaseURL:  "https://openrouter.ai/api/v1",
			Model:    "meta-llama/llama-3.3-70b-instruct:free",
			Enabled:  true,
			Priority: 50,
			Note:     "聚合多款零成本 :free 开源大模型",
		},
	}
}

func DefaultModelPoolJSON() string {
	b, _ := json.Marshal(DefaultModelPool())
	return string(b)
}

func (s *Store) GetModelPool() []ModelPoolEntry {
	raw, err := s.GetConfig(KeyFreeModelPool)
	if err != nil || strings.TrimSpace(raw) == "" {
		return DefaultModelPool()
	}
	var list []ModelPoolEntry
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) == 0 {
		return DefaultModelPool()
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
		if e.ID == "" || e.BaseURL == "" || e.Model == "" {
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
