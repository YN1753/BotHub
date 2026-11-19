package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

type ModelHealthStats struct {
	TargetID         string `json:"target_id"`
	Status           string `json:"status"` // healthy | cooling_down | degraded
	SuccessCount     int64  `json:"success_count"`
	FailCount        int64  `json:"fail_count"`
	ConsecutiveFails int64  `json:"consecutive_fails"`
	LastLatencyMs    int64  `json:"last_latency_ms"`
	LastError        string `json:"last_error,omitempty"`
	CooldownSec      int64  `json:"cooldown_sec"`
	cooldownUntil    time.Time
}

type ModelHealthTracker struct {
	mu    sync.Mutex
	stats map[string]*ModelHealthStats
}

func newModelHealthTracker() *ModelHealthTracker {
	return &ModelHealthTracker{
		stats: make(map[string]*ModelHealthStats),
	}
}

func (t *ModelHealthTracker) recordSuccess(targetID string, latencyMs int64) {
	if t == nil || targetID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.stats[targetID]
	if !ok {
		s = &ModelHealthStats{TargetID: targetID}
		t.stats[targetID] = s
	}
	s.SuccessCount++
	s.ConsecutiveFails = 0
	s.LastLatencyMs = latencyMs
	s.LastError = ""
	s.Status = "healthy"
	s.cooldownUntil = time.Time{}
	s.CooldownSec = 0
}

func (t *ModelHealthTracker) recordFailure(targetID, errMsg string, isRateLimit bool) {
	if t == nil || targetID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.stats[targetID]
	if !ok {
		s = &ModelHealthStats{TargetID: targetID}
		t.stats[targetID] = s
	}
	s.FailCount++
	s.ConsecutiveFails++
	s.LastError = strings.TrimSpace(errMsg)
	baseCool := 30 * time.Second
	if isRateLimit {
		baseCool = 60 * time.Second
	}
	shift := s.ConsecutiveFails - 1
	if shift > 3 {
		shift = 3
	}
	cool := baseCool * time.Duration(1<<shift)
	if cool > 5*time.Minute {
		cool = 5 * time.Minute
	}
	s.Status = "cooling_down"
	s.cooldownUntil = time.Now().Add(cool)
}

// ClearCooldown manually clears cooldown/degraded state for a single targetID, or all targets when targetID == "".
func (t *ModelHealthTracker) ClearCooldown(targetID string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	tid := strings.TrimSpace(targetID)
	if tid == "" {
		for _, s := range t.stats {
			s.ConsecutiveFails = 0
			s.LastError = ""
			s.Status = "healthy"
			s.cooldownUntil = time.Time{}
			s.CooldownSec = 0
		}
		return
	}
	if s, ok := t.stats[tid]; ok {
		s.ConsecutiveFails = 0
		s.LastError = ""
		s.Status = "healthy"
		s.cooldownUntil = time.Time{}
		s.CooldownSec = 0
	}
}

func (t *ModelHealthTracker) isCooling(targetID string) bool {
	if t == nil || targetID == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.stats[targetID]
	if !ok {
		return false
	}
	return time.Now().Before(s.cooldownUntil)
}

func (t *ModelHealthTracker) lastLatency(targetID string) int64 {
	if t == nil || targetID == "" {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.stats[targetID]; ok && s.LastLatencyMs > 0 {
		return s.LastLatencyMs
	}
	return 0
}

func (t *ModelHealthTracker) prioritizeHealthy(targets []ModelTarget) []ModelTarget {
	if t == nil || len(targets) <= 1 {
		return targets
	}
	healthy := make([]ModelTarget, 0, len(targets))
	cooling := make([]ModelTarget, 0)
	for _, tgt := range targets {
		if t.isCooling(tgt.ID) {
			cooling = append(cooling, tgt)
		} else {
			healthy = append(healthy, tgt)
		}
	}
	return append(healthy, cooling...)
}

func (t *ModelHealthTracker) Snapshot() map[string]ModelHealthStats {
	out := make(map[string]ModelHealthStats)
	if t == nil {
		return out
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for id, s := range t.stats {
		cp := *s
		if now.Before(s.cooldownUntil) {
			cp.CooldownSec = int64(time.Until(s.cooldownUntil).Seconds()) + 1
			cp.Status = "cooling_down"
		} else {
			cp.CooldownSec = 0
			if cp.ConsecutiveFails > 0 {
				cp.Status = "degraded"
			} else {
				cp.Status = "healthy"
			}
		}
		out[id] = cp
	}
	return out
}

func (e *Engine) GetModelHealth() map[string]ModelHealthStats {
	return e.health.Snapshot()
}

func (e *Engine) ClearModelCooldown(targetID string) {
	if e != nil && e.health != nil {
		e.health.ClearCooldown(targetID)
	}
}

type ModelTarget struct {
	ID               string
	ProviderID       string
	Tier             string // free | paid
	Label            string
	Provider         string // openai | gemini | deepseek | openrouter | custom
	BaseURL          string
	APIKey           string
	Model            string
	AllowThinking    bool
	SupportsTools    *bool
	SupportsThinking *bool
	LastLatencyMs    int64
}

type ModelResolveOptions struct {
	AllowPaidFallback bool
	NeedTools         bool
	NeedThinking      bool
}

func BuildChatCompletionsURL(baseURL string) string {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(u, "/chat/completions") {
		return u
	}
	return u + "/chat/completions"
}

func BuildModelsListURL(baseURL string) string {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u = strings.TrimSuffix(u, "/chat/completions")
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/models") {
		return u
	}
	return u + "/models"
}

func IsAllowThinkingModel(provider, model string) bool {
	if provider != "" && provider != "deepseek" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(model))
	return lower == "deepseek-chat" || strings.Contains(lower, "flash")
}

func isLikelyThinkingModel(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	if lower == "" {
		return false
	}
	for _, kw := range []string{"reasoner", "thinking", "-r1", "/r1", "qwq", "o1", "o3", "o4-mini", "gemini-2.5", "claude-3-7", "claude-sonnet-4", "deepseek-chat"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func inferRuntimeProvider(protocol, baseURL string) string {
	lowerURL := strings.ToLower(strings.TrimSpace(baseURL))
	if strings.Contains(lowerURL, "openrouter.ai") {
		return "openrouter"
	}
	if strings.Contains(lowerURL, "deepseek.com") {
		return "deepseek"
	}
	if strings.EqualFold(strings.TrimSpace(protocol), store.ProviderProtocolGemini) || strings.Contains(lowerURL, "generativelanguage.googleapis.com") {
		return store.ProviderProtocolGemini
	}
	return store.ProviderProtocolOpenAI
}

func isConfigTrue(st *store.Store, key string, defaultVal bool) bool {
	if st == nil {
		return defaultVal
	}
	v, err := st.GetConfig(key)
	if err != nil || strings.TrimSpace(v) == "" {
		return defaultVal
	}
	return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1"
}

func primaryDeepSeekTarget(st *store.Store) ModelTarget {
	apiKey, _ := st.GetConfig(store.KeyDeepSeekAPIKey)
	baseURL, _ := st.GetConfig(store.KeyDeepSeekBaseURL)
	cfgModel, _ := st.GetConfig(store.KeyAIModel)

	apiKey = strings.TrimSpace(apiKey)
	baseURL = store.CleanProviderBaseURL(baseURL)
	if baseURL == "" {
		baseURL = store.DefaultDeepSeekBaseURL
	}
	model := NormalizeModelID(cfgModel)
	if model == "" {
		model = store.DefaultAIModel
	}
	provider := "custom"
	if strings.Contains(strings.ToLower(baseURL), "deepseek.com") {
		provider = "deepseek"
	}
	return ModelTarget{
		ID:            "primary",
		ProviderID:    "legacy_deepseek",
		Tier:          store.ModelTierPaid,
		Label:         "主力 · " + model,
		Provider:      provider,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         model,
		AllowThinking: IsAllowThinkingModel(provider, model),
	}
}

type ModelOptionDTO struct {
	ID               string `json:"id"`
	ProviderID       string `json:"provider_id,omitempty"`
	Label            string `json:"label"`
	Tier             string `json:"tier"` // auto | free | paid
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	Available        bool   `json:"available"`
	SupportsTools    *bool  `json:"supports_tools,omitempty"`
	SupportsThinking *bool  `json:"supports_thinking,omitempty"`
	LastLatencyMs    int64  `json:"last_latency_ms,omitempty"`
}

func activePoolTargetsByTier(st *store.Store, wantTier string) []ModelTarget {
	if st == nil {
		return nil
	}
	providers, models, _ := st.GetModelConfig()
	pMap := make(map[string]store.ModelProvider, len(providers))
	for _, p := range providers {
		pMap[p.ID] = p
	}

	targets := make([]ModelTarget, 0, len(models))
	for _, m := range models {
		if !m.Enabled || store.NormalizeModelTier(m.Tier) != wantTier {
			continue
		}
		p, ok := pMap[m.ProviderID]
		if !ok || !p.Enabled || strings.TrimSpace(p.APIKey) == "" || strings.TrimSpace(p.BaseURL) == "" {
			continue
		}
		runtimeProv := inferRuntimeProvider(p.Protocol, p.BaseURL)
		displayName := strings.TrimSpace(m.DisplayName)
		if displayName == "" {
			displayName = m.ModelID
		}
		label := fmt.Sprintf("%s · %s", p.Name, displayName)
		targets = append(targets, ModelTarget{
			ID:               m.CanonicalTargetID(),
			ProviderID:       p.ID,
			Tier:             wantTier,
			Label:            label,
			Provider:         runtimeProv,
			BaseURL:          p.BaseURL,
			APIKey:           strings.TrimSpace(p.APIKey),
			Model:            m.ModelID,
			AllowThinking:    IsAllowThinkingModel(runtimeProv, m.ModelID),
			SupportsTools:    m.SupportsTools,
			SupportsThinking: m.SupportsThinking,
			LastLatencyMs:    m.LastLatencyMs,
		})
	}
	if wantTier == store.ModelTierPaid && len(targets) == 0 && len(providers) == 0 {
		primary := primaryDeepSeekTarget(st)
		if primary.APIKey != "" {
			targets = append(targets, primary)
		}
	}
	return targets
}

func activeFreePoolTargets(st *store.Store) []ModelTarget {
	return activePoolTargetsByTier(st, store.ModelTierFree)
}

func (e *Engine) capabilityScore(t ModelTarget, opts ModelResolveOptions) int {
	score := 0
	if opts.NeedTools {
		if t.SupportsTools != nil {
			if *t.SupportsTools {
				score += 4
			} else {
				score -= 4
			}
		}
	}
	if opts.NeedThinking {
		if t.SupportsThinking != nil {
			if *t.SupportsThinking {
				score += 2
			} else {
				score -= 2
			}
		} else if isLikelyThinkingModel(t.Model) {
			score += 1
		}
	}
	return score
}

func (e *Engine) effectiveLatencyMs(t ModelTarget) int64 {
	if e != nil && e.health != nil {
		if live := e.health.lastLatency(t.ID); live > 0 {
			return live
		}
	}
	if t.LastLatencyMs > 0 {
		return t.LastLatencyMs
	}
	return 1_000_000_000
}

// orderPoolTargets applies health filtering, capability-aware sorting, and intra-pool scheduling strategy
// (priority | round_robin | lowest_latency) for a single pool tier.
func (e *Engine) orderPoolTargets(targets []ModelTarget, tier, strategy string, opts ModelResolveOptions) []ModelTarget {
	if len(targets) <= 1 {
		return targets
	}
	healthy := make([]ModelTarget, 0, len(targets))
	cooling := make([]ModelTarget, 0)
	for _, tgt := range targets {
		if e != nil && e.health != nil && e.health.isCooling(tgt.ID) {
			cooling = append(cooling, tgt)
		} else {
			healthy = append(healthy, tgt)
		}
	}

	sortSubset := func(list []ModelTarget, allowRotate bool) []ModelTarget {
		if len(list) <= 1 {
			return list
		}
		ordered := append([]ModelTarget(nil), list...)
		switch store.NormalizePoolStrategy(strategy) {
		case store.PoolStrategyLowestLatency:
			sort.SliceStable(ordered, func(i, j int) bool {
				ci := e.capabilityScore(ordered[i], opts)
				cj := e.capabilityScore(ordered[j], opts)
				if ci != cj {
					return ci > cj
				}
				return e.effectiveLatencyMs(ordered[i]) < e.effectiveLatencyMs(ordered[j])
			})
		case store.PoolStrategyRoundRobin:
			sort.SliceStable(ordered, func(i, j int) bool {
				return e.capabilityScore(ordered[i], opts) > e.capabilityScore(ordered[j], opts)
			})
			if allowRotate && e != nil {
				topScore := e.capabilityScore(ordered[0], opts)
				topCount := 1
				for topCount < len(ordered) && e.capabilityScore(ordered[topCount], opts) == topScore {
					topCount++
				}
				if topCount > 1 {
					var seq uint64
					if tier == store.ModelTierPaid {
						seq = e.paidRR.Add(1) - 1
					} else {
						seq = e.freeRR.Add(1) - 1
					}
					shift := int(seq % uint64(topCount))
					if shift > 0 {
						rotated := make([]ModelTarget, 0, len(ordered))
						rotated = append(rotated, ordered[shift:topCount]...)
						rotated = append(rotated, ordered[:shift]...)
						rotated = append(rotated, ordered[topCount:]...)
						ordered = rotated
					}
				}
			}
		default: // priority
			if opts.NeedTools || opts.NeedThinking {
				sort.SliceStable(ordered, func(i, j int) bool {
					return e.capabilityScore(ordered[i], opts) > e.capabilityScore(ordered[j], opts)
				})
			}
		}
		return ordered
	}

	healthy = sortSubset(healthy, true)
	cooling = sortSubset(cooling, false)
	return append(healthy, cooling...)
}

// ModelOptionsPayload returns unified, tier-tagged model & pool routing options for Web & Android clients.
func (e *Engine) ModelOptionsPayload() []ModelOptionDTO {
	if e == nil || e.store == nil {
		return []ModelOptionDTO{}
	}
	providers, models, _ := e.store.GetModelConfig()
	pMap := make(map[string]store.ModelProvider, len(providers))
	for _, p := range providers {
		pMap[p.ID] = p
	}

	freeReady := 0
	paidReady := 0
	var freeOpts []ModelOptionDTO
	var paidOpts []ModelOptionDTO

	for _, m := range models {
		if !m.Enabled {
			continue
		}
		p, ok := pMap[m.ProviderID]
		if !ok || !p.Enabled {
			continue
		}
		ready := strings.TrimSpace(p.APIKey) != "" && strings.TrimSpace(p.BaseURL) != "" && strings.TrimSpace(m.ModelID) != ""
		tier := store.NormalizeModelTier(m.Tier)
		if ready {
			if tier == store.ModelTierPaid {
				paidReady++
			} else {
				freeReady++
			}
		}
		tag := "[免费]"
		if tier == store.ModelTierPaid {
			tag = "[付费]"
		}
		statusPart := "可用"
		if !ready {
			statusPart = "待配置"
		}
		display := strings.TrimSpace(m.DisplayName)
		if display == "" {
			display = m.ModelID
		}
		lat := m.LastLatencyMs
		if e.health != nil {
			if live := e.health.lastLatency(m.CanonicalTargetID()); live > 0 {
				lat = live
			}
		}
		dto := ModelOptionDTO{
			ID:               m.CanonicalTargetID(),
			ProviderID:       p.ID,
			Label:            fmt.Sprintf("%s %s · %s (%s)", tag, p.Name, display, statusPart),
			Tier:             tier,
			Provider:         inferRuntimeProvider(p.Protocol, p.BaseURL),
			Model:            m.ModelID,
			Available:        ready,
			SupportsTools:    m.SupportsTools,
			SupportsThinking: m.SupportsThinking,
			LastLatencyMs:    lat,
		}
		if tier == store.ModelTierPaid {
			paidOpts = append(paidOpts, dto)
		} else {
			freeOpts = append(freeOpts, dto)
		}
	}

	// Backward-compatible legacy Paid node if user has deepseek_api_key without any providers
	if paidReady == 0 && len(providers) == 0 {
		primary := primaryDeepSeekTarget(e.store)
		if primary.APIKey != "" {
			paidReady = 1
			paidOpts = append(paidOpts, ModelOptionDTO{
				ID:         "primary",
				ProviderID: "legacy_deepseek",
				Label:      fmt.Sprintf("[付费] 主力配置 · %s (可用)", primary.Model),
				Tier:       store.ModelTierPaid,
				Provider:   primary.Provider,
				Model:      primary.Model,
				Available:  true,
			})
		}
	}

	out := make([]ModelOptionDTO, 0, 2+len(freeOpts)+len(paidOpts))
	freeDesc := fmt.Sprintf("免费模型池 (%d 可用)", freeReady)
	if freeReady == 0 {
		freeDesc = "免费模型池 (未配置)"
	}
	out = append(out, ModelOptionDTO{
		ID:        "auto:free",
		Label:     "自动 · " + freeDesc,
		Tier:      "auto",
		Provider:  "free-pool",
		Model:     "auto:free",
		Available: freeReady > 0,
	})
	if paidReady > 0 || len(paidOpts) > 0 {
		out = append(out, ModelOptionDTO{
			ID:        "auto:paid",
			Label:     fmt.Sprintf("自动 · 付费模型池 (%d 可用)", paidReady),
			Tier:      "auto",
			Provider:  "paid-pool",
			Model:     "auto:paid",
			Available: paidReady > 0,
		})
	}
	out = append(out, freeOpts...)
	out = append(out, paidOpts...)
	return out
}

func (e *Engine) ResolveChatTargets(requestedModel string) ([]ModelTarget, error) {
	return e.ResolveChatTargetsWithContext(context.Background(), requestedModel, false)
}

func (e *Engine) ResolveChatTargetsWithContext(ctx context.Context, requestedModel string, allowPaidParam bool) ([]ModelTarget, error) {
	return e.ResolveChatTargetsWithOptions(ctx, requestedModel, ModelResolveOptions{
		AllowPaidFallback: allowPaidParam,
	})
}

func (e *Engine) checkTaskPaidQuota(ctx context.Context) error {
	if e == nil || e.store == nil {
		return nil
	}
	taskID, maxPerDay := store.TaskQuotaFromContext(ctx)
	if taskID == "" || maxPerDay <= 0 {
		return nil
	}
	used, err := e.store.CountTaskPaidCallsToday(taskID)
	if err != nil {
		return nil
	}
	if used >= maxPerDay {
		return fmt.Errorf("当前任务今日付费调用已达上限（%d/%d 次），已自动熔断拦截", used, maxPerDay)
	}
	return nil
}

// ResolveChatTargetsWithOptions builds an ordered failover slice of upstream ModelTargets
// honoring two-layer provider/pool separation, intra-pool strategies, capability preference,
// per-task/per-bot paid fallback guardrails, and per-task daily paid call quota limits.
func (e *Engine) ResolveChatTargetsWithOptions(ctx context.Context, requestedModel string, opts ModelResolveOptions) ([]ModelTarget, error) {
	reqM := strings.TrimSpace(requestedModel)
	providers, models, strategies := e.store.GetModelConfig()
	freeTargets := e.orderPoolTargets(activePoolTargetsByTier(e.store, store.ModelTierFree), store.ModelTierFree, strategies.Free, opts)
	paidTargets := e.orderPoolTargets(activePoolTargetsByTier(e.store, store.ModelTierPaid), store.ModelTierPaid, strategies.Paid, opts)
	primary := primaryDeepSeekTarget(e.store)

	allowPaid := opts.AllowPaidFallback
	ctxAllowPaid, ctxExplicit := store.PaidFallbackFromContext(ctx)
	if ctxExplicit {
		allowPaid = ctxAllowPaid
	}
	legacyAllowFallback := isConfigTrue(e.store, store.KeyPoolFallback, true)

	if reqM == "auto:free" || reqM == "free-pool" {
		if len(freeTargets) > 0 {
			out := append([]ModelTarget{}, freeTargets...)
			shouldFailoverToPaid := allowPaid || (reqM == "free-pool" && !ctxExplicit && legacyAllowFallback)
			if shouldFailoverToPaid && len(paidTargets) > 0 && e.checkTaskPaidQuota(ctx) == nil {
				out = append(out, paidTargets...)
			}
			return out, nil
		}
		shouldUsePaid := allowPaid || (reqM == "free-pool" && !ctxExplicit && len(paidTargets) > 0)
		if shouldUsePaid && len(paidTargets) > 0 {
			if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
				return nil, qErr
			}
			return paidTargets, nil
		}
		return nil, fmt.Errorf("免费模型池暂无可用节点；若当前任务或对话需要调用付费模型，请开启「允许付费兜底」或改选付费模型")
	}

	if reqM == "auto:paid" || reqM == "paid-pool" {
		if ctxExplicit && !ctxAllowPaid {
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费模型池请求")
		}
		if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
			return nil, qErr
		}
		if len(paidTargets) > 0 {
			return paidTargets, nil
		}
		return nil, fmt.Errorf("付费模型池暂无可用节点，请先在「设置 -> AI 模型配置」的付费模型池中添加渠道与模型")
	}

	if reqM == "" {
		if len(freeTargets) > 0 {
			out := append([]ModelTarget{}, freeTargets...)
			if allowPaid && len(paidTargets) > 0 && e.checkTaskPaidQuota(ctx) == nil {
				out = append(out, paidTargets...)
			}
			return out, nil
		}
		if allowPaid && len(paidTargets) > 0 {
			if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
				return nil, qErr
			}
			return paidTargets, nil
		}
		onlyLegacyPrimary := len(providers) == 1 && providers[0].ID == "legacy_deepseek" && len(e.store.GetModelPool()) == 0
		if ((len(providers) == 0 && len(models) == 0) || onlyLegacyPrimary) && primary.APIKey != "" {
			if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
				return nil, qErr
			}
			return []ModelTarget{primary}, nil
		}
		if len(paidTargets) > 0 {
			return nil, fmt.Errorf("免费模型池暂无可用节点；若当前任务或对话需要调用付费模型，请开启「允许付费兜底」或改选付费模型")
		}
		return nil, fmt.Errorf("尚未配置任何可用模型节点，请先在「设置 -> AI 模型配置」中添加渠道与免费/付费池模型")
	}

	if strings.HasPrefix(reqM, "pool:") {
		rawSpec := strings.TrimPrefix(reqM, "pool:")
		entryID := rawSpec
		pinnedModel := ""
		if idx := strings.Index(rawSpec, "::"); idx >= 0 {
			entryID = strings.TrimSpace(rawSpec[:idx])
			pinnedModel = strings.TrimSpace(rawSpec[idx+2:])
		}

		// Find provider by canonical ID or migrated alias ID
		var matchedProvider *store.ModelProvider
		for i := range providers {
			if providers[i].ID == entryID {
				matchedProvider = &providers[i]
				break
			}
			for _, alias := range providers[i].AliasIDs {
				if alias == entryID {
					matchedProvider = &providers[i]
					break
				}
			}
			if matchedProvider != nil {
				break
			}
		}
		if matchedProvider == nil {
			return nil, fmt.Errorf("未找到指定的模型渠道: %s", entryID)
		}
		if strings.TrimSpace(matchedProvider.APIKey) == "" {
			return nil, fmt.Errorf("渠道「%s」尚未填写 API Key，请先在系统设置中填入密钥", matchedProvider.Name)
		}

		// Match specific pooled model (or first pooled model of this provider when legacy pool:<id> is used)
		var matchedModel *store.PooledModel
		if pinnedModel != "" {
			for i := range models {
				if models[i].ProviderID == matchedProvider.ID && models[i].ModelID == pinnedModel {
					matchedModel = &models[i]
					break
				}
			}
		} else {
			for i := range models {
				if models[i].ProviderID == matchedProvider.ID && models[i].Enabled {
					matchedModel = &models[i]
					break
				}
			}
			if matchedModel == nil {
				for i := range models {
					if models[i].ProviderID == matchedProvider.ID {
						matchedModel = &models[i]
						break
					}
				}
			}
		}

		chosenModel := pinnedModel
		tier := store.ModelTierFree
		var supportsTools, supportsThinking *bool
		var lastLatency int64
		if matchedModel != nil {
			chosenModel = matchedModel.ModelID
			tier = store.NormalizeModelTier(matchedModel.Tier)
			supportsTools = matchedModel.SupportsTools
			supportsThinking = matchedModel.SupportsThinking
			lastLatency = matchedModel.LastLatencyMs
		}
		if chosenModel == "" {
			return nil, fmt.Errorf("渠道「%s」尚未分配具体模型 ID，请先在设置中拉取并加入模型池", matchedProvider.Name)
		}
		if tier == store.ModelTierPaid {
			if ctxExplicit && !ctxAllowPaid {
				return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费节点「%s」", matchedProvider.Name)
			}
			if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
				return nil, qErr
			}
		}
		runtimeProv := inferRuntimeProvider(matchedProvider.Protocol, matchedProvider.BaseURL)
		pinned := ModelTarget{
			ID:               "pool:" + matchedProvider.ID + "::" + chosenModel,
			ProviderID:       matchedProvider.ID,
			Tier:             tier,
			Label:            fmt.Sprintf("%s · %s", matchedProvider.Name, chosenModel),
			Provider:         runtimeProv,
			BaseURL:          matchedProvider.BaseURL,
			APIKey:           strings.TrimSpace(matchedProvider.APIKey),
			Model:            chosenModel,
			AllowThinking:    IsAllowThinkingModel(runtimeProv, chosenModel),
			SupportsTools:    supportsTools,
			SupportsThinking: supportsThinking,
			LastLatencyMs:    lastLatency,
		}
		out := []ModelTarget{pinned}
		if tier == store.ModelTierPaid {
			for _, pt := range paidTargets {
				if pt.ID != pinned.ID {
					out = append(out, pt)
				}
			}
			return e.health.prioritizeHealthy(out), nil
		}
		if legacyAllowFallback || allowPaid {
			for _, ft := range freeTargets {
				if ft.ID != pinned.ID {
					out = append(out, ft)
				}
			}
			if (allowPaid || (!ctxExplicit && legacyAllowFallback)) && e.checkTaskPaidQuota(ctx) == nil {
				out = append(out, paidTargets...)
			}
			out = e.health.prioritizeHealthy(out)
		}
		return out, nil
	}

	// Legacy / explicit paid target: "primary", "deepseek-flash", or specific model override
	normModel := NormalizeModelID(reqM)
	if normModel != "" && normModel != "primary" && normModel != "deepseek-flash" {
		if ctxExplicit && !ctxAllowPaid {
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费模型 %s", normModel)
		}
		if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
			return nil, qErr
		}
		primary.Model = normModel
		primary.Label = "付费 · " + normModel
		primary.AllowThinking = IsAllowThinkingModel(primary.Provider, normModel)
		if len(paidTargets) > 0 && primary.APIKey == "" {
			firstPaid := paidTargets[0]
			firstPaid.Model = normModel
			firstPaid.Label = "付费 · " + normModel
			firstPaid.AllowThinking = IsAllowThinkingModel(firstPaid.Provider, normModel)
			return append([]ModelTarget{firstPaid}, paidTargets...), nil
		}
	}

	if primary.APIKey != "" {
		if ctxExplicit && !ctxAllowPaid {
			if len(freeTargets) > 0 {
				return freeTargets, nil
			}
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截主力付费模型")
		}
		if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
			if len(freeTargets) > 0 {
				return freeTargets, nil
			}
			return nil, qErr
		}
		out := []ModelTarget{primary}
		if legacyAllowFallback && len(freeTargets) > 0 {
			out = append(out, freeTargets...)
			out = e.health.prioritizeHealthy(out)
		}
		return out, nil
	}

	if len(paidTargets) > 0 {
		if ctxExplicit && !ctxAllowPaid {
			if len(freeTargets) > 0 {
				return freeTargets, nil
			}
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费模型池")
		}
		if qErr := e.checkTaskPaidQuota(ctx); qErr != nil {
			if len(freeTargets) > 0 {
				return freeTargets, nil
			}
			return nil, qErr
		}
		out := append([]ModelTarget{}, paidTargets...)
		if allowPaid && len(freeTargets) > 0 {
			out = append(out, freeTargets...)
		}
		return e.health.prioritizeHealthy(out), nil
	}

	if len(freeTargets) > 0 {
		return freeTargets, nil
	}
	return nil, fmt.Errorf("尚未配置任何可用模型节点，请先在「设置 -> AI 模型配置」中添加免费模型池或付费模型池")
}

type ProbeResult struct {
	TargetID         string `json:"target_id"`
	LatencyMs        int64  `json:"latency_ms"`
	ReplyPreview     string `json:"reply_preview"`
	SupportsTools    *bool  `json:"supports_tools,omitempty"`
	SupportsThinking *bool  `json:"supports_thinking,omitempty"`
}

// ProbeModelTarget executes a lightweight non-streaming connectivity test against a target provider.
func (e *Engine) ProbeModelTarget(ctx context.Context, baseURL, apiKey, model, provider string) (int64, string, error) {
	return e.ProbeModelTargetWithID(ctx, provider, baseURL, apiKey, model, provider)
}

// ProbeModelTargetWithID probes a target and updates health telemetry under the given targetID.
func (e *Engine) ProbeModelTargetWithID(ctx context.Context, targetID, baseURL, apiKey, model, provider string) (int64, string, error) {
	res, err := e.probeModelTargetInternal(ctx, targetID, "", baseURL, apiKey, model, provider, false)
	if err != nil {
		if res != nil {
			return res.LatencyMs, "", err
		}
		return 0, "", err
	}
	return res.LatencyMs, res.ReplyPreview, nil
}

// ProbeModelTargetWithCapabilities executes a live connectivity test plus optional lightweight Tool Calling
// capability probe, updates runtime health under the canonical target ID, and persists capability flags into SQLite.
func (e *Engine) ProbeModelTargetWithCapabilities(ctx context.Context, providerID, baseURL, apiKey, model, protocol string, probeTools bool) (*ProbeResult, error) {
	pid := strings.TrimSpace(providerID)
	mid := strings.TrimSpace(model)
	canonicalID := "primary"
	if pid != "" && pid != "primary" {
		canonicalID = "pool:" + pid + "::" + mid
	}
	runtimeProv := inferRuntimeProvider(protocol, baseURL)
	return e.probeModelTargetInternal(ctx, canonicalID, pid, baseURL, apiKey, mid, runtimeProv, probeTools)
}

func (e *Engine) probeModelTargetInternal(ctx context.Context, targetID, providerID, baseURL, apiKey, model, provider string, probeTools bool) (*ProbeResult, error) {
	baseURL = store.CleanProviderBaseURL(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if baseURL == "" || apiKey == "" || model == "" {
		return nil, fmt.Errorf("Base URL、API Key 与模型 ID 均不能为空")
	}

	start := time.Now()
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	bodyMap := map[string]any{
		"model":      model,
		"stream":     false,
		"max_tokens": 16,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with OK only."},
		},
	}
	if IsAllowThinkingModel(provider, model) {
		bodyMap["thinking"] = map[string]string{"type": "disabled"}
	}
	payload, _ := json.Marshal(bodyMap)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, BuildChatCompletionsURL(baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if provider == "openrouter" || strings.Contains(strings.ToLower(baseURL), "openrouter.ai") {
		req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
		req.Header.Set("X-Title", "BotHub")
	}

	resp, err := e.httpClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	healthID := strings.TrimSpace(targetID)
	if healthID == "" {
		healthID = provider
	} else if healthID != "primary" && !strings.HasPrefix(healthID, "pool:") {
		healthID = "pool:" + healthID
	}
	if err != nil {
		e.health.recordFailure(healthID, err.Error(), false)
		return &ProbeResult{TargetID: healthID, LatencyMs: elapsed}, fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errText := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		e.health.recordFailure(healthID, errText, resp.StatusCode == http.StatusTooManyRequests)
		return &ProbeResult{TargetID: healthID, LatencyMs: elapsed}, fmt.Errorf("%s", errText)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
				Reasoning        string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		e.health.recordFailure(healthID, "接口返回非标准 OpenAI 格式", false)
		return &ProbeResult{TargetID: healthID, LatencyMs: elapsed}, fmt.Errorf("接口返回非标准 OpenAI 格式")
	}
	msg := parsed.Choices[0].Message
	reply := strings.TrimSpace(msg.Content)
	hasReasoning := strings.TrimSpace(msg.ReasoningContent) != "" || strings.TrimSpace(msg.Reasoning) != ""
	if reply == "" {
		reply = strings.TrimSpace(msg.ReasoningContent)
	}
	if reply == "" {
		reply = strings.TrimSpace(msg.Reasoning)
	}
	e.health.recordSuccess(healthID, elapsed)

	res := &ProbeResult{
		TargetID:     healthID,
		LatencyMs:    elapsed,
		ReplyPreview: reply,
	}

	if hasReasoning || isLikelyThinkingModel(model) {
		v := true
		res.SupportsThinking = &v
	}

	if probeTools {
		if okTools, decided := e.probeToolCallingSupport(ctx, baseURL, apiKey, model, provider); decided {
			res.SupportsTools = &okTools
		}
	}

	if providerID != "" && e.store != nil {
		_ = e.store.UpdatePooledModelProbe(providerID, model, res.SupportsTools, res.SupportsThinking, elapsed)
	}
	return res, nil
}

func (e *Engine) probeToolCallingSupport(ctx context.Context, baseURL, apiKey, model, provider string) (supported bool, decided bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 14*time.Second)
	defer cancel()

	bodyMap := map[string]any{
		"model":       model,
		"stream":      false,
		"max_tokens":  32,
		"tool_choice": "auto",
		"tools": []map[string]any{
			{
				"type": "function",
				"function": map[string]any{
					"name":        "ping",
					"description": "Health check ping tool",
					"parameters": map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
		},
		"messages": []map[string]string{
			{"role": "user", "content": "Say OK without calling any tool."},
		},
	}
	if IsAllowThinkingModel(provider, model) {
		bodyMap["thinking"] = map[string]string{"type": "disabled"}
	}
	payload, _ := json.Marshal(bodyMap)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, BuildChatCompletionsURL(baseURL), bytes.NewReader(payload))
	if err != nil {
		return false, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if provider == "openrouter" || strings.Contains(strings.ToLower(baseURL), "openrouter.ai") {
		req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
		req.Header.Set("X-Title", "BotHub")
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var parsed struct {
			Choices []json.RawMessage `json:"choices"`
			Error   any               `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(raw), &parsed) == nil && len(parsed.Choices) > 0 && parsed.Error == nil {
			return true, true
		}
		return false, false
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity || resp.StatusCode == http.StatusNotImplemented {
		lower := strings.ToLower(string(raw))
		if strings.Contains(lower, "tool") || strings.Contains(lower, "function") || strings.Contains(lower, "unsupported") || strings.Contains(lower, "not support") {
			return false, true
		}
		// Any 400/422 specifically when tools schema was added after base connectivity passed indicates tool incompatibility
		return false, true
	}
	return false, false
}

type UpstreamModelCheck struct {
	ID        string `json:"id"`
	Usable    bool   `json:"usable"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type FetchModelsResult struct {
	Models           []string             `json:"models"`
	CheckedModels    []UpstreamModelCheck `json:"checked_models"`
	FilteredOutCount int                  `json:"filtered_out_count"`
	UnverifiedCount  int                  `json:"unverified_count"`
}

func isLikelyChatModel(id string) bool {
	lower := strings.ToLower(strings.TrimSpace(id))
	if lower == "" {
		return false
	}
	nonChatKeywords := []string{
		"embed", "bge-", "rerank",
		"imagen", "veo", "lyria", "flux", "stable-diffusion", "sdxl", "sd-", "kolors", "wanx", "cogview", "dall-e", "midjourney",
		"image-generation", "-image-preview", "preview-image", "-image",
		"whisper", "sensevoice", "cosyvoice", "fish-speech", "native-audio", "-tts", "tts-",
		"aqa", "bidi", "-live", "live-", "moderation", "shieldgemma",
		"robotics", "computer-use", "deep-research", "gemini-1.0", "gemini-pro-vision",
	}
	for _, kw := range nonChatKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}

func modelCandidatePriority(id string) int {
	lower := strings.ToLower(id)
	score := 0
	if strings.Contains(lower, "flash") {
		score += 40
	}
	if strings.Contains(lower, ":free") || strings.Contains(lower, "free") {
		score += 35
	}
	if strings.Contains(lower, "gemini-2.5") || strings.Contains(lower, "gemini-2.0") {
		score += 25
	}
	if lower == "deepseek-chat" || strings.Contains(lower, "deepseek-v3") || strings.Contains(lower, "glm-4-flash") {
		score += 25
	}
	if strings.Contains(lower, "lite") || strings.Contains(lower, "mini") || strings.Contains(lower, "7b") || strings.Contains(lower, "8b") {
		score += 12
	}
	if strings.Contains(lower, "exp") || strings.Contains(lower, "-001") || strings.Contains(lower, "-002") {
		score -= 8
	}
	if strings.Contains(lower, "pro") || strings.Contains(lower, "ultra") {
		score -= 4
	}
	return score
}

func sortCandidateModels(models []string) {
	sort.SliceStable(models, func(i, j int) bool {
		pi := modelCandidatePriority(models[i])
		pj := modelCandidatePriority(models[j])
		if pi != pj {
			return pi > pj
		}
		return models[i] < models[j]
	})
}

func (e *Engine) fetchGoogleNativeModels(ctx context.Context, apiKey string) ([]string, int, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://generativelanguage.googleapis.com/v1beta/models?pageSize=200", nil)
	if err != nil {
		return nil, 0, false
	}
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, false
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	var parsed struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Models) == 0 {
		return nil, 0, false
	}
	seen := make(map[string]bool)
	models := make([]string, 0, len(parsed.Models))
	filteredOut := 0
	for _, item := range parsed.Models {
		supportsChat := false
		for _, m := range item.SupportedGenerationMethods {
			if m == "generateContent" {
				supportsChat = true
				break
			}
		}
		id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		if !supportsChat || !isLikelyChatModel(id) {
			filteredOut++
			continue
		}
		if id != "" && !seen[id] {
			seen[id] = true
			models = append(models, id)
		}
	}
	sortCandidateModels(models)
	return models, filteredOut, true
}

func summarizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "limit: 0") || strings.Contains(lower, "limit:0"):
		return "免费额度为 0 (不可用)"
	case strings.Contains(msg, "HTTP 429") || strings.Contains(lower, "rate"):
		return "429 配额耗尽或限流"
	case strings.Contains(msg, "HTTP 404"):
		return "404 模型不存在或已下线"
	case strings.Contains(msg, "HTTP 403") || strings.Contains(msg, "HTTP 402"):
		return "无权限或需付费余额"
	case strings.Contains(msg, "HTTP 401"):
		return "API Key 无效或未授权"
	case strings.Contains(msg, "HTTP 400"):
		return "400 不支持对话接口"
	default:
		runes := []rune(strings.TrimSpace(msg))
		if len(runes) > 64 {
			return string(runes[:64]) + "..."
		}
		return string(runes)
	}
}

// FetchUpstreamModels queries the upstream provider's /models endpoint dynamically using the given
// Base URL and API Key, returning ALL filtered chat-capable model IDs at zero inference cost.
func (e *Engine) FetchUpstreamModels(ctx context.Context, baseURL, apiKey, provider string) ([]string, error) {
	res, err := e.FetchAndVerifyUpstreamModels(ctx, baseURL, apiKey, provider, false)
	if err != nil {
		return nil, err
	}
	return res.Models, nil
}

// FetchAndVerifyUpstreamModels fetches ALL candidate chat models (filtering non-chat modalities such as embeddings,
// image/video generators, audio/TTS) with zero truncation when verify=false.
// When verify=true (legacy/optional mode), it also verifies live usability via concurrent chat probes.
func (e *Engine) FetchAndVerifyUpstreamModels(ctx context.Context, baseURL, apiKey, provider string, verify bool) (*FetchModelsResult, error) {
	baseURL = store.CleanProviderBaseURL(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("请先填写 Base URL 与 API Key")
	}

	var candidates []string
	filteredOut := 0

	if strings.Contains(strings.ToLower(baseURL), "generativelanguage.googleapis.com") || strings.EqualFold(strings.TrimSpace(provider), store.ProviderProtocolGemini) {
		if strings.Contains(strings.ToLower(baseURL), "generativelanguage.googleapis.com") {
			if gModels, gFiltered, ok := e.fetchGoogleNativeModels(ctx, apiKey); ok {
				candidates = gModels
				filteredOut = gFiltered
			}
		}
	}

	if len(candidates) == 0 {
		reqCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
		defer cancel()

		endpoint := BuildModelsListURL(baseURL)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if provider == "openrouter" || strings.Contains(strings.ToLower(baseURL), "openrouter.ai") {
			req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
			req.Header.Set("X-Title", "BotHub")
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("拉取模型列表失败: %w", err)
		}
		defer resp.Body.Close()

		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("拉取模型列表 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}

		var parsed struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			Models []struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			} `json:"models"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("解析模型列表 JSON 失败: %w", err)
		}

		seen := make(map[string]bool)
		candidates = make([]string, 0, len(parsed.Data)+len(parsed.Models))
		addModel := func(id string) {
			id = strings.TrimSpace(id)
			id = strings.TrimPrefix(id, "models/")
			if id == "" || seen[id] {
				return
			}
			seen[id] = true
			if !isLikelyChatModel(id) {
				filteredOut++
				return
			}
			candidates = append(candidates, id)
		}
		for _, item := range parsed.Data {
			addModel(item.ID)
		}
		for _, item := range parsed.Models {
			if item.ID != "" {
				addModel(item.ID)
			} else {
				addModel(item.Name)
			}
		}
		sortCandidateModels(candidates)
	}

	result := &FetchModelsResult{
		FilteredOutCount: filteredOut,
	}

	// Zero-cost full list mode (new default): return EVERY chat-capable model with zero 24-model truncation
	if !verify || len(candidates) == 0 {
		result.Models = candidates
		result.CheckedModels = make([]UpstreamModelCheck, 0, len(candidates))
		for _, id := range candidates {
			result.CheckedModels = append(result.CheckedModels, UpstreamModelCheck{
				ID:     id,
				Usable: true,
			})
		}
		return result, nil
	}

	// Optional explicit live verify mode (retained for backward compatibility)
	const maxVerify = 24
	toVerify := candidates
	if len(candidates) > maxVerify {
		toVerify = candidates[:maxVerify]
		result.UnverifiedCount = len(candidates) - maxVerify
	}

	checks := make([]UpstreamModelCheck, len(toVerify))
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	for i, modelID := range toVerify {
		wg.Add(1)
		go func(idx int, mid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			probeCtx, cancel := context.WithTimeout(ctx, 9*time.Second)
			defer cancel()

			start := time.Now()
			bodyMap := map[string]any{
				"model":      mid,
				"stream":     false,
				"max_tokens": 1,
				"messages": []map[string]string{
					{"role": "user", "content": "hi"},
				},
			}
			payload, _ := json.Marshal(bodyMap)
			req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, BuildChatCompletionsURL(baseURL), bytes.NewReader(payload))
			if err != nil {
				checks[idx] = UpstreamModelCheck{ID: mid, Usable: false, Reason: summarizeProbeError(err)}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)
			if provider == "openrouter" || strings.Contains(strings.ToLower(baseURL), "openrouter.ai") {
				req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
				req.Header.Set("X-Title", "BotHub")
			}

			resp, err := e.httpClient.Do(req)
			elapsed := time.Since(start).Milliseconds()
			if err != nil {
				checks[idx] = UpstreamModelCheck{ID: mid, Usable: false, LatencyMs: elapsed, Reason: summarizeProbeError(err)}
				return
			}
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			_ = resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				checks[idx] = UpstreamModelCheck{
					ID:        mid,
					Usable:    false,
					LatencyMs: elapsed,
					Reason:    summarizeProbeError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))),
				}
				return
			}
			trimmed := bytes.TrimSpace(raw)
			if len(trimmed) == 0 || trimmed[0] != '{' {
				checks[idx] = UpstreamModelCheck{
					ID:        mid,
					Usable:    false,
					LatencyMs: elapsed,
					Reason:    "接口返回非 JSON 响应",
				}
				return
			}
			var probeBody struct {
				Choices []json.RawMessage `json:"choices"`
				Error   any               `json:"error"`
			}
			if err := json.Unmarshal(trimmed, &probeBody); err != nil || (len(probeBody.Choices) == 0 && probeBody.Error != nil) {
				checks[idx] = UpstreamModelCheck{
					ID:        mid,
					Usable:    false,
					LatencyMs: elapsed,
					Reason:    "接口返回非标准 OpenAI 格式",
				}
				return
			}
			checks[idx] = UpstreamModelCheck{
				ID:        mid,
				Usable:    true,
				LatencyMs: elapsed,
			}
		}(i, modelID)
	}
	wg.Wait()

	usable := make([]string, 0, len(checks))
	var usableChecks, failedChecks []UpstreamModelCheck
	for _, c := range checks {
		if c.Usable {
			usable = append(usable, c.ID)
			usableChecks = append(usableChecks, c)
		} else {
			failedChecks = append(failedChecks, c)
		}
	}
	result.Models = usable
	result.CheckedModels = append(usableChecks, failedChecks...)
	return result, nil
}
