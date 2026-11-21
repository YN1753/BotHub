package store

import (
	"encoding/json"
	"sort"
	"strings"
)

const (
	ModelTierFree = "free"
	ModelTierPaid = "paid"

	ProviderProtocolOpenAI = "openai"
	ProviderProtocolGemini = "gemini"

	PoolStrategyPriority      = "priority"
	PoolStrategyRoundRobin    = "round_robin"
	PoolStrategyLowestLatency = "lowest_latency"
)

// NormalizeModelTier constrains pool tier to "free" or "paid" (default "free").
func NormalizeModelTier(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), ModelTierPaid) {
		return ModelTierPaid
	}
	return ModelTierFree
}

// NormalizeProviderProtocol constrains protocol to "openai" or "gemini" (default "openai").
func NormalizeProviderProtocol(raw, baseURL string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == ProviderProtocolGemini || strings.Contains(strings.ToLower(baseURL), "generativelanguage.googleapis.com") {
		return ProviderProtocolGemini
	}
	return ProviderProtocolOpenAI
}

// NormalizePoolStrategy constrains intra-pool scheduling strategy.
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
	AliasIDs []string `json:"alias_ids,omitempty"` // preserved legacy ModelPoolEntry IDs when merged during migration
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

// PooledModel represents Layer 2: a specific model assigned to either the Free Pool or Paid Pool.
type PooledModel struct {
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	DisplayName      string `json:"display_name,omitempty"`
	Tier             string `json:"tier"` // free | paid
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	SupportsTools    *bool  `json:"supports_tools,omitempty"`
	SupportsThinking *bool  `json:"supports_thinking,omitempty"`
	LastLatencyMs    int64  `json:"last_latency_ms,omitempty"`
	LastVerifiedAt   string `json:"last_verified_at,omitempty"`
}

// CanonicalTargetID returns the unified, per-model target identifier used for routing and health telemetry.
func (m PooledModel) CanonicalTargetID() string {
	return "pool:" + strings.TrimSpace(m.ProviderID) + "::" + strings.TrimSpace(m.ModelID)
}

type PoolStrategyConfig struct {
	Free string `json:"free"` // priority | round_robin | lowest_latency
	Paid string `json:"paid"` // priority | round_robin | lowest_latency
}

type ModelPoolEntry struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Tier     string   `json:"tier"`     // free | paid
	Provider string   `json:"provider"` // openai | gemini | deepseek | custom
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	APIKeys  []string `json:"api_keys,omitempty"`
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
		e.BaseURL = CleanProviderBaseURL(e.BaseURL)
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
	if err := s.SetConfig(KeyFreeModelPool, string(b)); err != nil {
		return err
	}
	// Reset decoupled keys so legacy SaveModelPool callers & tests immediately reflect in GetModelConfig()
	_ = s.SetConfig(KeyModelProviders, "")
	_ = s.SetConfig(KeyPooledModels, "")
	return nil
}

// GetPoolStrategies returns the configured intra-pool scheduling strategies for Free & Paid pools.
func (s *Store) GetPoolStrategies() PoolStrategyConfig {
	freeRaw, _ := s.GetConfig(KeyFreePoolStrategy)
	paidRaw, _ := s.GetConfig(KeyPaidPoolStrategy)
	return PoolStrategyConfig{
		Free: NormalizePoolStrategy(freeRaw),
		Paid: NormalizePoolStrategy(paidRaw),
	}
}

// GetModelConfig loads the two-layer model configuration (Providers + PooledModels + PoolStrategies).
// When the new keys are empty, it performs a lossless in-memory migration from legacy free_model_pool
// and deepseek_api_key without requiring any re-entry of API keys.
func (s *Store) GetModelConfig() ([]ModelProvider, []PooledModel, PoolStrategyConfig) {
	strategies := s.GetPoolStrategies()
	if s == nil {
		return []ModelProvider{}, []PooledModel{}, strategies
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
			models[i].Tier = NormalizeModelTier(models[i].Tier)
		}
		sortPooledModels(models)
		if providers == nil {
			providers = []ModelProvider{}
		}
		if models == nil {
			models = []PooledModel{}
		}
		return providers, models, strategies
	}

	return s.migrateLegacyModelConfig(strategies)
}

func (s *Store) migrateLegacyModelConfig(strategies PoolStrategyConfig) ([]ModelProvider, []PooledModel, PoolStrategyConfig) {
	legacyEntries := s.GetModelPool()
	providers := make([]ModelProvider, 0, len(legacyEntries)+1)
	models := make([]PooledModel, 0, len(legacyEntries)*2+1)

	type dedupeKey struct {
		baseURL  string
		apiKey   string
		protocol string
	}
	providerIdxByKey := make(map[dedupeKey]int)
	seenPoolModel := make(map[string]bool)
	hasPaid := false

	for _, e := range legacyEntries {
		baseURL := CleanProviderBaseURL(e.BaseURL)
		if e.ID == "" || baseURL == "" {
			continue
		}
		proto := NormalizeProviderProtocol(e.Provider, baseURL)
		key := dedupeKey{baseURL: strings.ToLower(baseURL), apiKey: strings.TrimSpace(e.APIKey), protocol: proto}

		var providerID string
		if idx, exists := providerIdxByKey[key]; exists {
			providerID = providers[idx].ID
			if e.ID != providerID {
				providers[idx].AliasIDs = appendUniqueString(providers[idx].AliasIDs, e.ID)
			}
			if e.Enabled {
				providers[idx].Enabled = true
			}
		} else {
			providerID = e.ID
			name := strings.TrimSpace(e.Name)
			if name == "" {
				name = providerID
			}
			providers = append(providers, ModelProvider{
				ID:       providerID,
				Name:     name,
				Protocol: proto,
				BaseURL:  baseURL,
				APIKey:   strings.TrimSpace(e.APIKey),
				Enabled:  e.Enabled,
				Note:     strings.TrimSpace(e.Note),
			})
			providerIdxByKey[key] = len(providers) - 1
		}

		tier := NormalizeModelTier(e.Tier)
		if tier == ModelTierPaid && e.Enabled && strings.TrimSpace(e.APIKey) != "" {
			hasPaid = true
		}
		eff := e.EffectiveModels()
		for mIdx, mid := range eff {
			dedupeModelKey := providerID + "::" + mid
			if seenPoolModel[dedupeModelKey] {
				continue
			}
			seenPoolModel[dedupeModelKey] = true
			prio := e.Priority
			if prio <= 0 {
				prio = (len(models) + 1) * 10
			} else {
				prio = prio*10 + mIdx
			}
			models = append(models, PooledModel{
				ProviderID: providerID,
				ModelID:    mid,
				Tier:       tier,
				Enabled:    e.Enabled,
				Priority:   prio,
			})
		}
	}

	// Losslessly surface legacy deepseek_api_key as a Paid channel if no paid pool entry exists yet
	if !hasPaid {
		dsKey, _ := s.GetConfig(KeyDeepSeekAPIKey)
		dsKey = strings.TrimSpace(dsKey)
		if dsKey != "" {
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
			pid := "legacy_deepseek"
			key := dedupeKey{baseURL: strings.ToLower(dsBase), apiKey: dsKey, protocol: proto}
			if idx, exists := providerIdxByKey[key]; exists {
				pid = providers[idx].ID
			} else {
				providers = append(providers, ModelProvider{
					ID:       pid,
					Name:     "DeepSeek 官方",
					Protocol: proto,
					BaseURL:  dsBase,
					APIKey:   dsKey,
					Enabled:  true,
					Note:     "自动迁移自历史主力配置",
				})
			}
			dedupeModelKey := pid + "::" + dsModel
			if !seenPoolModel[dedupeModelKey] {
				models = append(models, PooledModel{
					ProviderID: pid,
					ModelID:    dsModel,
					Tier:       ModelTierPaid,
					Enabled:    true,
					Priority:   10,
				})
			}
		}
	}

	sortPooledModels(models)
	return providers, models, strategies
}

// SaveModelConfig persists the decoupled Providers, PooledModels, and PoolStrategies.
// Masked API keys ("****") automatically retain their existing raw secret from storage.
// It also writes a synchronized mirror into KeyFreeModelPool for backward compatibility.
func (s *Store) SaveModelConfig(providers []ModelProvider, models []PooledModel, strategies PoolStrategyConfig) error {
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
		m.Tier = NormalizeModelTier(m.Tier)
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
		// Preserve persisted probe telemetry if caller omitted them
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

	strategies.Free = NormalizePoolStrategy(strategies.Free)
	strategies.Paid = NormalizePoolStrategy(strategies.Paid)

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
	if err := s.SetConfig(KeyFreePoolStrategy, strategies.Free); err != nil {
		return err
	}
	if err := s.SetConfig(KeyPaidPoolStrategy, strategies.Paid); err != nil {
		return err
	}

	// Keep legacy KeyFreeModelPool synchronized for older clients & backward-compatible readers
	legacyMirror := buildLegacyPoolMirror(cleanedProviders, cleanedModels)
	if lb, err := json.Marshal(legacyMirror); err == nil {
		_ = s.SetConfig(KeyFreeModelPool, string(lb))
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
	providers, models, strategies := s.GetModelConfig()
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
	return s.SaveModelConfig(providers, models, strategies)
}

func buildLegacyPoolMirror(providers []ModelProvider, models []PooledModel) []ModelPoolEntry {
	pMap := make(map[string]ModelProvider, len(providers))
	for _, p := range providers {
		pMap[p.ID] = p
	}
	type groupKey struct {
		providerID string
		tier       string
	}
	order := make([]groupKey, 0)
	grouped := make(map[groupKey][]PooledModel)
	for _, m := range models {
		gk := groupKey{providerID: m.ProviderID, tier: m.Tier}
		if _, ok := grouped[gk]; !ok {
			order = append(order, gk)
		}
		grouped[gk] = append(grouped[gk], m)
	}

	out := make([]ModelPoolEntry, 0, len(order))
	for idx, gk := range order {
		p, ok := pMap[gk.providerID]
		if !ok {
			continue
		}
		list := grouped[gk]
		mids := make([]string, 0, len(list))
		anyEnabled := false
		minPrio := list[0].Priority
		for _, m := range list {
			mids = append(mids, m.ModelID)
			if m.Enabled {
				anyEnabled = true
			}
			if m.Priority < minPrio {
				minPrio = m.Priority
			}
		}
		entryID := p.ID
		if gk.tier == ModelTierPaid {
			// Avoid ID collision if the same provider has both Free and Paid models in legacy view
			if _, hasFree := grouped[groupKey{providerID: p.ID, tier: ModelTierFree}]; hasFree {
				entryID = p.ID + "_paid"
			}
		}
		primaryModel := ""
		if len(mids) > 0 {
			primaryModel = mids[0]
		}
		out = append(out, ModelPoolEntry{
			ID:       entryID,
			Name:     p.Name,
			Tier:     gk.tier,
			Provider: p.Protocol,
			BaseURL:  p.BaseURL,
			APIKey:   p.APIKey,
			Model:    primaryModel,
			Models:   mids,
			Enabled:  p.Enabled && anyEnabled,
			Priority: minPrio + idx,
			Note:     p.Note,
		})
	}
	return out
}

func sortPooledModels(models []PooledModel) {
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Tier != models[j].Tier {
			return models[i].Tier == ModelTierFree
		}
		if models[i].Priority != models[j].Priority {
			return models[i].Priority < models[j].Priority
		}
		return models[i].ModelID < models[j].ModelID
	})
}

func appendUniqueString(list []string, val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return list
	}
	for _, existing := range list {
		if existing == val {
			return list
		}
	}
	return append(list, val)
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

func MaskModelPool(entries []ModelPoolEntry) []ModelPoolEntry {
	out := make([]ModelPoolEntry, 0, len(entries))
	for _, e := range entries {
		copyEntry := e
		copyEntry.APIKey = MaskSecretValue(e.APIKey)
		copyEntry.APIKeys = MaskAPIKeys(e.APIKeys)
		out = append(out, copyEntry)
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
