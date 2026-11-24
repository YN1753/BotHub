package store

import (
	"encoding/json"
	"sort"
	"strings"
)

const (
	ProviderProtocolOpenAI = "openai"
	ProviderProtocolGemini = "gemini"

	PoolStrategyPriority      = "priority"
	PoolStrategyRoundRobin    = "round_robin"
	PoolStrategyLowestLatency = "lowest_latency"
)

// NormalizeProviderProtocol constrains protocol to "openai" or "gemini" (default "openai").
func NormalizeProviderProtocol(raw, baseURL string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == ProviderProtocolGemini || strings.Contains(strings.ToLower(baseURL), "generativelanguage.googleapis.com") {
		return ProviderProtocolGemini
	}
	return ProviderProtocolOpenAI
}

// NormalizePoolStrategy constrains pool scheduling strategy.
func NormalizePoolStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case PoolStrategyRoundRobin:
		return PoolStrategyRoundRobin
	case PoolStrategyLowestLatency:
		return PoolStrategyLowestLatency
	default:
		return PoolStrategyPriority
	}
}

// CleanProviderBaseURL strips trailing slashes and accidental /chat/completions suffixes.
func CleanProviderBaseURL(raw string) string {
	u := strings.TrimSpace(raw)
	u = strings.TrimRight(u, "/")
	for _, suffix := range []string{"/chat/completions", "/completions", "/models"} {
		if strings.HasSuffix(strings.ToLower(u), suffix) {
			u = u[:len(u)-len(suffix)]
			u = strings.TrimRight(u, "/")
		}
	}
	return u
}

// ModelProvider represents Layer 1: a reusable API provider/channel (credentials + endpoint + protocol).
type ModelProvider struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol string   `json:"protocol"` // openai | gemini
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	APIKeys  []string `json:"api_keys,omitempty"`
	Enabled  bool     `json:"enabled"`
	Note     string   `json:"note,omitempty"`
	AliasIDs []string `json:"alias_ids,omitempty"`
}

// SplitAPIKeys splits raw key strings (supporting newline, comma, or semicolon separators)
// and returns a deduplicated ordered slice of non-empty API keys.
func SplitAPIKeys(rawKeys ...string) []string {
	out := make([]string, 0, len(rawKeys))
	seen := make(map[string]bool, len(rawKeys))
	for _, raw := range rawKeys {
		parts := strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';'
		})
		for _, part := range parts {
			k := strings.TrimSpace(part)
			if k != "" && !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}

// EffectiveAPIKeys returns all configured API keys for this provider in priority order.
func (p ModelProvider) EffectiveAPIKeys() []string {
	if len(p.APIKeys) > 0 {
		merged := make([]string, 0, len(p.APIKeys)+1)
		merged = append(merged, p.APIKeys...)
		if strings.TrimSpace(p.APIKey) != "" {
			merged = append(merged, p.APIKey)
		}
		return SplitAPIKeys(merged...)
	}
	return SplitAPIKeys(p.APIKey)
}

// PooledModel represents Layer 2: a specific model in the unified Model Pool.
type PooledModel struct {
	ProviderID       string  `json:"provider_id"`
	ModelID          string  `json:"model_id"`
	DisplayName      string  `json:"display_name,omitempty"`
	Enabled          bool    `json:"enabled"`
	Priority         int     `json:"priority"`
	IsLight          bool    `json:"is_light,omitempty"`           // Optional fast/background model marker
	InputPricePerM   float64 `json:"input_price_per_m,omitempty"`  // Optional USD / 1M input tokens override (0 = built-in table)
	OutputPricePerM  float64 `json:"output_price_per_m,omitempty"` // Optional USD / 1M output tokens override (0 = built-in table)
	SupportsTools    *bool   `json:"supports_tools,omitempty"`
	SupportsThinking *bool   `json:"supports_thinking,omitempty"`
	LastLatencyMs    int64   `json:"last_latency_ms,omitempty"`
	LastVerifiedAt   string  `json:"last_verified_at,omitempty"`
}

// CanonicalTargetID returns the unified, per-model target identifier used for routing and health telemetry.
func (m PooledModel) CanonicalTargetID() string {
	return "pool:" + strings.TrimSpace(m.ProviderID) + "::" + strings.TrimSpace(m.ModelID)
}

// GetPoolStrategy returns the configured scheduling strategy for the unified model pool.
func (s *Store) GetPoolStrategy() string {
	if s == nil {
		return PoolStrategyPriority
	}
	raw, _ := s.GetConfig(KeyPoolStrategy)
	return NormalizePoolStrategy(raw)
}

// GetModelConfig loads the unified two-layer model configuration (Providers + PooledModels + PoolStrategy).
func (s *Store) GetModelConfig() ([]ModelProvider, []PooledModel, string) {
	strategy := s.GetPoolStrategy()
	if s == nil {
		return []ModelProvider{}, []PooledModel{}, strategy
	}
	rawProviders, _ := s.GetConfig(KeyModelProviders)
	rawModels, _ := s.GetConfig(KeyPooledModels)
	if strings.TrimSpace(rawProviders) != "" || strings.TrimSpace(rawModels) != "" {
		var providers []ModelProvider
		var models []PooledModel
		if strings.TrimSpace(rawProviders) != "" {
			_ = json.Unmarshal([]byte(rawProviders), &providers)
		}
		if strings.TrimSpace(rawModels) != "" {
			_ = json.Unmarshal([]byte(rawModels), &models)
		}
		for i := range providers {
			providers[i].ID = strings.TrimSpace(providers[i].ID)
			providers[i].BaseURL = CleanProviderBaseURL(providers[i].BaseURL)
			providers[i].Protocol = NormalizeProviderProtocol(providers[i].Protocol, providers[i].BaseURL)
		}
		for i := range models {
			models[i].ProviderID = strings.TrimSpace(models[i].ProviderID)
			models[i].ModelID = strings.TrimSpace(models[i].ModelID)
		}
		sortPooledModels(models)
		if providers == nil {
			providers = []ModelProvider{}
		}
		if models == nil {
			models = []PooledModel{}
		}
		return providers, models, strategy
	}

	return s.fallbackSinglePrimaryConfig(strategy)
}

func (s *Store) fallbackSinglePrimaryConfig(strategy string) ([]ModelProvider, []PooledModel, string) {
	dsKey, _ := s.GetConfig(KeyDeepSeekAPIKey)
	dsKey = strings.TrimSpace(dsKey)
	if dsKey == "" {
		return []ModelProvider{}, []PooledModel{}, strategy
	}
	dsBase, _ := s.GetConfig(KeyDeepSeekBaseURL)
	dsBase = CleanProviderBaseURL(dsBase)
	if dsBase == "" {
		dsBase = DefaultDeepSeekBaseURL
	}
	dsModel, _ := s.GetConfig(KeyAIModel)
	dsModel = strings.TrimSpace(dsModel)
	if dsModel == "" {
		dsModel = DefaultAIModel
	}
	proto := NormalizeProviderProtocol("", dsBase)
	providers := []ModelProvider{{
		ID:       "legacy_deepseek",
		Name:     "DeepSeek 官方",
		Protocol: proto,
		BaseURL:  dsBase,
		APIKey:   dsKey,
		Enabled:  true,
		Note:     "自动承接主力配置",
	}}
	models := []PooledModel{{
		ProviderID: "legacy_deepseek",
		ModelID:    dsModel,
		Enabled:    true,
		Priority:   10,
	}}
	return providers, models, strategy
}

// purgeLegacyFreePoolModelsOnce removes legacy free-tier models when paid models already exist,
// and strips obsolete tier fields cleanly during the v2.89 single-pool migration.
func (s *Store) purgeLegacyFreePoolModelsOnce() {
	rawModels, _ := s.GetConfig(KeyPooledModels)
	if strings.TrimSpace(rawModels) == "" {
		return
	}
	type legacyModel struct {
		PooledModel
		Tier string `json:"tier"`
	}
	var legacy []legacyModel
	if err := json.Unmarshal([]byte(rawModels), &legacy); err != nil || len(legacy) == 0 {
		return
	}
	var paidOnly []PooledModel
	var allClean []PooledModel
	for _, item := range legacy {
		m := item.PooledModel
		allClean = append(allClean, m)
		lower := strings.ToLower(m.ModelID)
		isFree := strings.EqualFold(strings.TrimSpace(item.Tier), "free") || strings.Contains(lower, ":free")
		if !isFree {
			paidOnly = append(paidOnly, m)
		}
	}
	chosen := allClean
	if len(paidOnly) > 0 {
		chosen = paidOnly
	}
	sortPooledModels(chosen)
	for i := range chosen {
		chosen[i].Priority = (i + 1) * 10
	}
	if b, err := json.Marshal(chosen); err == nil {
		_ = s.SetConfig(KeyPooledModels, string(b))
	}
}

// SaveModelConfig persists the unified Providers, PooledModels, and PoolStrategy.
// Masked API keys ("****") automatically retain their existing raw secret from storage.
func (s *Store) SaveModelConfig(providers []ModelProvider, models []PooledModel, strategy string) error {
	existingProviders, existingModels, _ := s.GetModelConfig()
	existingKeys := make(map[string]string, len(existingProviders)+2)
	existingKeyLists := make(map[string][]string, len(existingProviders)+2)
	existingAliases := make(map[string][]string, len(existingProviders))
	for _, p := range existingProviders {
		effKeys := p.EffectiveAPIKeys()
		primaryKey := p.APIKey
		if primaryKey == "" && len(effKeys) > 0 {
			primaryKey = effKeys[0]
		}
		existingKeys[p.ID] = primaryKey
		existingKeyLists[p.ID] = effKeys
		if len(p.AliasIDs) > 0 {
			existingAliases[p.ID] = p.AliasIDs
		}
		for _, alias := range p.AliasIDs {
			existingKeys[alias] = primaryKey
			existingKeyLists[alias] = effKeys
		}
	}
	if dsKey, _ := s.GetConfig(KeyDeepSeekAPIKey); strings.TrimSpace(dsKey) != "" {
		if _, ok := existingKeys["legacy_deepseek"]; !ok {
			existingKeys["legacy_deepseek"] = strings.TrimSpace(dsKey)
			existingKeyLists["legacy_deepseek"] = []string{strings.TrimSpace(dsKey)}
		}
	}

	resolveMaskedKey := func(providerID, candidate string, idx int) string {
		c := strings.TrimSpace(candidate)
		if !strings.Contains(c, "****") {
			return c
		}
		stored := existingKeyLists[providerID]
		for _, raw := range stored {
			if MaskSecretValue(raw) == c {
				return raw
			}
		}
		if idx >= 0 && idx < len(stored) {
			return stored[idx]
		}
		if existingKeys[providerID] != "" {
			return existingKeys[providerID]
		}
		return ""
	}

	existingProbeMeta := make(map[string]PooledModel, len(existingModels))
	for _, em := range existingModels {
		existingProbeMeta[em.CanonicalTargetID()] = em
	}

	cleanedProviders := make([]ModelProvider, 0, len(providers))
	providerMap := make(map[string]ModelProvider, len(providers))
	for _, p := range providers {
		p.ID = strings.TrimSpace(p.ID)
		p.Name = strings.TrimSpace(p.Name)
		p.BaseURL = CleanProviderBaseURL(p.BaseURL)
		p.Protocol = NormalizeProviderProtocol(p.Protocol, p.BaseURL)
		p.Note = strings.TrimSpace(p.Note)
		if p.ID == "" || p.BaseURL == "" {
			continue
		}
		if p.Name == "" {
			p.Name = p.ID
		}
		var rawSubmitted []string
		if len(p.APIKeys) > 0 {
			rawSubmitted = SplitAPIKeys(p.APIKeys...)
		} else {
			rawSubmitted = SplitAPIKeys(p.APIKey)
		}
		resolvedKeys := make([]string, 0, len(rawSubmitted))
		seenResolved := make(map[string]bool, len(rawSubmitted))
		for idx, k := range rawSubmitted {
			rk := resolveMaskedKey(p.ID, k, idx)
			if rk != "" && !seenResolved[rk] {
				seenResolved[rk] = true
				resolvedKeys = append(resolvedKeys, rk)
			}
		}
		if len(resolvedKeys) > 0 {
			p.APIKeys = resolvedKeys
			p.APIKey = resolvedKeys[0]
		} else {
			p.APIKeys = nil
			p.APIKey = ""
		}
		if len(p.AliasIDs) == 0 && len(existingAliases[p.ID]) > 0 {
			p.AliasIDs = existingAliases[p.ID]
		}
		if _, dup := providerMap[p.ID]; dup {
			continue
		}
		providerMap[p.ID] = p
		cleanedProviders = append(cleanedProviders, p)
	}

	cleanedModels := make([]PooledModel, 0, len(models))
	seenModel := make(map[string]bool, len(models))
	for idx, m := range models {
		m.ProviderID = strings.TrimSpace(m.ProviderID)
		m.ModelID = strings.TrimSpace(m.ModelID)
		m.DisplayName = strings.TrimSpace(m.DisplayName)
		if m.ProviderID == "" || m.ModelID == "" {
			continue
		}
		if _, exists := providerMap[m.ProviderID]; !exists {
			continue
		}
		key := m.CanonicalTargetID()
		if seenModel[key] {
			continue
		}
		seenModel[key] = true
		if m.Priority <= 0 {
			m.Priority = (idx + 1) * 10
		}
		if m.InputPricePerM < 0 {
			m.InputPricePerM = 0
		}
		if m.OutputPricePerM < 0 {
			m.OutputPricePerM = 0
		}
		if prev, ok := existingProbeMeta[key]; ok {
			if m.SupportsTools == nil {
				m.SupportsTools = prev.SupportsTools
			}
			if m.SupportsThinking == nil {
				m.SupportsThinking = prev.SupportsThinking
			}
			if m.LastLatencyMs == 0 {
				m.LastLatencyMs = prev.LastLatencyMs
			}
			if m.LastVerifiedAt == "" {
				m.LastVerifiedAt = prev.LastVerifiedAt
			}
		}
		cleanedModels = append(cleanedModels, m)
	}
	sortPooledModels(cleanedModels)

	normStrategy := NormalizePoolStrategy(strategy)

	pb, err := json.Marshal(cleanedProviders)
	if err != nil {
		return err
	}
	mb, err := json.Marshal(cleanedModels)
	if err != nil {
		return err
	}
	if err := s.SetConfig(KeyModelProviders, string(pb)); err != nil {
		return err
	}
	if err := s.SetConfig(KeyPooledModels, string(mb)); err != nil {
		return err
	}
	if err := s.SetConfig(KeyPoolStrategy, normStrategy); err != nil {
		return err
	}
	return nil
}

// UpdatePooledModelProbe persists probe capability flags and latency onto a pooled model.
func (s *Store) UpdatePooledModelProbe(providerID, modelID string, supportsTools, supportsThinking *bool, latencyMs int64) error {
	if s == nil {
		return nil
	}
	pid := strings.TrimSpace(providerID)
	mid := strings.TrimSpace(modelID)
	if pid == "" || mid == "" {
		return nil
	}
	providers, models, strategy := s.GetModelConfig()
	updated := false
	now := NowShanghai()
	for i := range models {
		if models[i].ProviderID == pid && models[i].ModelID == mid {
			if supportsTools != nil {
				v := *supportsTools
				models[i].SupportsTools = &v
			}
			if supportsThinking != nil {
				v := *supportsThinking
				models[i].SupportsThinking = &v
			}
			if latencyMs > 0 {
				models[i].LastLatencyMs = latencyMs
			}
			models[i].LastVerifiedAt = now
			updated = true
			break
		}
	}
	if !updated {
		return nil
	}
	return s.SaveModelConfig(providers, models, strategy)
}

func sortPooledModels(models []PooledModel) {
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Priority != models[j].Priority {
			return models[i].Priority < models[j].Priority
		}
		return models[i].ModelID < models[j].ModelID
	})
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

func MaskAPIKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if m := MaskSecretValue(k); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func MaskModelProviders(providers []ModelProvider) []ModelProvider {
	out := make([]ModelProvider, 0, len(providers))
	for _, p := range providers {
		cp := p
		eff := p.EffectiveAPIKeys()
		if len(eff) > 0 {
			cp.APIKey = MaskSecretValue(eff[0])
			cp.APIKeys = MaskAPIKeys(eff)
		} else {
			cp.APIKey = MaskSecretValue(p.APIKey)
			cp.APIKeys = nil
		}
		out = append(out, cp)
	}
	return out
}
