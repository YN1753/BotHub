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

// ModelProviderTemplate is strictly an optional UI creation template (never auto-inserted into user configs).
type ModelProviderTemplate struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	DocsHint string `json:"docs_hint"`
}

func ModelProviderTemplates() []ModelProviderTemplate {
	return []ModelProviderTemplate{
		{
			ID:       "google_ai_studio",
			Name:     "Google AI Studio (Gemini)",
			Provider: "google",
			BaseURL:  "https://generativelanguage.googleapis.com/v1beta/openai",
			DocsHint: "填入 Google AI Studio 免费 API Key 后，点击「拉取实时模型」动态获取可用 Gemini 列表",
		},
		{
			ID:       "siliconflow",
			Name:     "硅基流动 SiliconFlow",
			Provider: "siliconflow",
			BaseURL:  "https://api.siliconflow.cn/v1",
			DocsHint: "国内直连无需代理；填入 Token 后可实时拉取全量模型清单并筛选免费可用项",
		},
		{
			ID:       "zhipu",
			Name:     "智谱开放平台 BigModel",
			Provider: "zhipu",
			BaseURL:  "https://open.bigmodel.cn/api/paas/v4",
			DocsHint: "支持 GLM 免费系列及 OpenAI 兼容协议，填入 Key 后可直接探测与拉取模型",
		},
		{
			ID:       "groq",
			Name:     "Groq Cloud",
			Provider: "groq",
			BaseURL:  "https://api.groq.com/openai/v1",
			DocsHint: "高速推理免费额度平台，填入 gsk_ 密钥后实时拉取可用模型",
		},
		{
			ID:       "openrouter",
			Name:     "OpenRouter",
			Provider: "openrouter",
			BaseURL:  "https://openrouter.ai/api/v1",
			DocsHint: "聚合多厂商 :free 免费模型，填入 sk-or- 密钥后可按实时列表一键挑选",
		},
		{
			ID:       "custom_relay",
			Name:     "自定义中转站 / OpenAI 兼容",
			Provider: "custom",
			BaseURL:  "",
			DocsHint: "适用于任意 NewAPI / OneAPI 中转站或自建免费公益反代，填入地址与 Key 后自动拉取模型",
		},
	}
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
