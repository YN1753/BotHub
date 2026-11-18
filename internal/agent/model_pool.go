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

type ModelTarget struct {
	ID            string
	Label         string
	Provider      string
	BaseURL       string
	APIKey        string
	Model         string
	AllowThinking bool
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
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
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
		Label:         "主力 · " + model,
		Provider:      provider,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         model,
		AllowThinking: IsAllowThinkingModel(provider, model),
	}
}

type ModelOptionDTO struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Tier      string `json:"tier"` // auto | free | paid
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Available bool   `json:"available"`
}

func activePoolTargetsByTier(st *store.Store, wantTier string) []ModelTarget {
	if st == nil {
		return nil
	}
	entries := st.GetModelPool()
	targets := make([]ModelTarget, 0, len(entries)*2)
	for _, entry := range entries {
		if !entry.Enabled || strings.TrimSpace(entry.APIKey) == "" {
			continue
		}
		if store.NormalizeModelTier(entry.Tier) != wantTier {
			continue
		}
		models := entry.EffectiveModels()
		for idx, modelID := range models {
			tid := "pool:" + entry.ID
			if idx > 0 {
				tid = tid + "::" + modelID
			}
			label := entry.Name
			if len(models) > 1 {
				label = fmt.Sprintf("%s · %s", entry.Name, modelID)
			}
			targets = append(targets, ModelTarget{
				ID:            tid,
				Label:         label,
				Provider:      entry.Provider,
				BaseURL:       entry.BaseURL,
				APIKey:        strings.TrimSpace(entry.APIKey),
				Model:         modelID,
				AllowThinking: IsAllowThinkingModel(entry.Provider, modelID),
			})
		}
	}
	if wantTier == store.ModelTierPaid && len(targets) == 0 {
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

// ModelOptionsPayload returns unified, tier-tagged model & pool routing options for Web & Android clients.
func (e *Engine) ModelOptionsPayload() []ModelOptionDTO {
	if e == nil || e.store == nil {
		return []ModelOptionDTO{}
	}
	entries := e.store.GetModelPool()
	freeReady := 0
	paidReady := 0
	var freeOpts []ModelOptionDTO
	var paidOpts []ModelOptionDTO

	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		hasKey := strings.TrimSpace(entry.APIKey) != ""
		tier := store.NormalizeModelTier(entry.Tier)
		models := entry.EffectiveModels()
		if len(models) == 0 {
			models = []string{""}
		}
		for idx, m := range models {
			ready := hasKey && m != ""
			if ready {
				if tier == store.ModelTierPaid {
					paidReady++
				} else {
					freeReady++
				}
			}
			optID := "pool:" + entry.ID
			if idx > 0 && m != "" {
				optID = optID + "::" + m
			}
			tag := "[免费]"
			if tier == store.ModelTierPaid {
				tag = "[付费]"
			}
			modelPart := m
			if modelPart == "" {
				modelPart = "待选择模型"
			}
			statusPart := "可用"
			if !ready {
				statusPart = "待配置"
			}
			dto := ModelOptionDTO{
				ID:        optID,
				Label:     fmt.Sprintf("%s %s · %s (%s)", tag, entry.Name, modelPart, statusPart),
				Tier:      tier,
				Provider:  entry.Provider,
				Model:     m,
				Available: ready,
			}
			if tier == store.ModelTierPaid {
				paidOpts = append(paidOpts, dto)
			} else {
				freeOpts = append(freeOpts, dto)
			}
		}
	}

	// Backward-compatible legacy Paid node if user has deepseek_api_key without explicit paid entries
	if paidReady == 0 {
		primary := primaryDeepSeekTarget(e.store)
		if primary.APIKey != "" {
			paidReady = 1
			paidOpts = append(paidOpts, ModelOptionDTO{
				ID:        "primary",
				Label:     fmt.Sprintf("[付费] 主力配置 · %s (可用)", primary.Model),
				Tier:      store.ModelTierPaid,
				Provider:  primary.Provider,
				Model:     primary.Model,
				Available: true,
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

// ResolveChatTargetsWithContext builds an ordered failover slice of upstream ModelTargets
// enforcing per-task / per-bot paid-fallback guardrails:
//   - "auto:free" / "free-pool": routes across Free Pool, falling back to Paid Pool ONLY when allowPaidFallback (or context PaidFallback) is true.
//   - "auto:paid" / "paid-pool": routes across Paid Pool explicitly.
//   - "pool:<id>" / "pool:<id>::<model>": pins a specific entry/model, keeping same-tier failover and honoring allowPaidFallback for free nodes.
//   - "" / "primary" / custom model: uses Free Pool first when only Free Pool or auto:free is desired; preserves legacy primary fallback when no Free Pool exists.
func (e *Engine) ResolveChatTargetsWithContext(ctx context.Context, requestedModel string, allowPaidParam bool) ([]ModelTarget, error) {
	reqM := strings.TrimSpace(requestedModel)
	freeTargets := activePoolTargetsByTier(e.store, store.ModelTierFree)
	paidTargets := activePoolTargetsByTier(e.store, store.ModelTierPaid)
	primary := primaryDeepSeekTarget(e.store)

	allowPaid := allowPaidParam
	ctxAllowPaid, ctxExplicit := store.PaidFallbackFromContext(ctx)
	if ctxExplicit {
		allowPaid = ctxAllowPaid
	}
	legacyAllowFallback := isConfigTrue(e.store, store.KeyPoolFallback, true)

	if reqM == "auto:free" || reqM == "free-pool" {
		if len(freeTargets) > 0 {
			out := append([]ModelTarget{}, freeTargets...)
			// Strictly honor per-task/bot allowPaid when caller uses auto:free or explicit context guard;
			// preserve legacy fallback ONLY for legacy "free-pool" callers with no explicit guard.
			shouldFailoverToPaid := allowPaid || (reqM == "free-pool" && !ctxExplicit && legacyAllowFallback)
			if shouldFailoverToPaid {
				out = append(out, paidTargets...)
			}
			return e.health.prioritizeHealthy(out), nil
		}
		shouldUsePaid := allowPaid || (reqM == "free-pool" && !ctxExplicit && len(paidTargets) > 0)
		if shouldUsePaid && len(paidTargets) > 0 {
			return e.health.prioritizeHealthy(paidTargets), nil
		}
		return nil, fmt.Errorf("免费模型池暂无可用节点；若当前任务或对话需要调用付费模型，请开启「允许付费兜底」或改选付费模型")
	}

	if reqM == "auto:paid" || reqM == "paid-pool" {
		if ctxExplicit && !ctxAllowPaid {
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费模型池请求")
		}
		if len(paidTargets) > 0 {
			return e.health.prioritizeHealthy(paidTargets), nil
		}
		return nil, fmt.Errorf("付费模型池暂无可用节点，请先在「设置 -> AI 模型池」的付费模型池中添加并配置 API Key")
	}

	if reqM == "" {
		if len(freeTargets) > 0 {
			out := append([]ModelTarget{}, freeTargets...)
			if allowPaid {
				out = append(out, paidTargets...)
			}
			return e.health.prioritizeHealthy(out), nil
		}
		poolEntries := e.store.GetModelPool()
		if allowPaid && len(paidTargets) > 0 {
			return e.health.prioritizeHealthy(paidTargets), nil
		}
		if len(poolEntries) == 0 && primary.APIKey != "" {
			return []ModelTarget{primary}, nil
		}
		if len(paidTargets) > 0 {
			return nil, fmt.Errorf("免费模型池暂无可用节点；若当前任务或对话需要调用付费模型，请开启「允许付费兜底」或改选付费模型")
		}
		return nil, fmt.Errorf("尚未配置任何可用模型节点，请先在「设置 -> AI 模型池」中添加免费模型池或付费模型池")
	}

	if strings.HasPrefix(reqM, "pool:") {
		rawSpec := strings.TrimPrefix(reqM, "pool:")
		entryID := rawSpec
		pinnedModel := ""
		if idx := strings.Index(rawSpec, "::"); idx >= 0 {
			entryID = rawSpec[:idx]
			pinnedModel = strings.TrimSpace(rawSpec[idx+2:])
		}
		allEntries := e.store.GetModelPool()
		var matched *store.ModelPoolEntry
		for i := range allEntries {
			if allEntries[i].ID == entryID {
				matched = &allEntries[i]
				break
			}
		}
		if matched == nil {
			return nil, fmt.Errorf("未找到指定的模型节点: %s", entryID)
		}
		if strings.TrimSpace(matched.APIKey) == "" {
			return nil, fmt.Errorf("模型「%s」尚未填写 API Key，请先在系统设置中填入密钥", matched.Name)
		}
		chosenModel := pinnedModel
		if chosenModel == "" {
			eff := matched.EffectiveModels()
			if len(eff) > 0 {
				chosenModel = eff[0]
			}
		}
		if chosenModel == "" {
			return nil, fmt.Errorf("模型节点「%s」尚未选择具体模型 ID，请先在设置中拉取并选择模型", matched.Name)
		}
		tier := store.NormalizeModelTier(matched.Tier)
		if tier == store.ModelTierPaid && ctxExplicit && !ctxAllowPaid {
			return nil, fmt.Errorf("当前任务未开启「允许付费兜底/付费调用」，已拦截付费节点「%s」", matched.Name)
		}
		pinnedID := "pool:" + matched.ID
		if pinnedModel != "" && len(matched.EffectiveModels()) > 1 && chosenModel != matched.EffectiveModels()[0] {
			pinnedID = pinnedID + "::" + chosenModel
		}
		pinned := ModelTarget{
			ID:            pinnedID,
			Label:         matched.Name,
			Provider:      matched.Provider,
			BaseURL:       matched.BaseURL,
			APIKey:        strings.TrimSpace(matched.APIKey),
			Model:         chosenModel,
			AllowThinking: IsAllowThinkingModel(matched.Provider, chosenModel),
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
			if allowPaid || (!ctxExplicit && legacyAllowFallback) {
				out = append(out, paidTargets...)
			}
			out = e.health.prioritizeHealthy(out)
		}
		return out, nil
	}

	// Legacy / explicit paid target: "", "primary", "deepseek-flash", or specific model override
	normModel := NormalizeModelID(reqM)
	if normModel != "" && normModel != "primary" && normModel != "deepseek-flash" {
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
		out := []ModelTarget{primary}
		if legacyAllowFallback && len(freeTargets) > 0 {
			out = append(out, freeTargets...)
			out = e.health.prioritizeHealthy(out)
		}
		return out, nil
	}

	if len(paidTargets) > 0 {
		out := append([]ModelTarget{}, paidTargets...)
		if allowPaid && len(freeTargets) > 0 {
			out = append(out, freeTargets...)
		}
		return e.health.prioritizeHealthy(out), nil
	}

	if len(freeTargets) > 0 {
		return e.health.prioritizeHealthy(freeTargets), nil
	}
	return nil, fmt.Errorf("尚未配置任何可用模型节点，请先在「设置 -> AI 模型池」中添加免费模型池或付费模型池")
}

// ProbeModelTarget executes a lightweight non-streaming connectivity test against a target provider.
func (e *Engine) ProbeModelTarget(ctx context.Context, baseURL, apiKey, model, provider string) (int64, string, error) {
	return e.ProbeModelTargetWithID(ctx, provider, baseURL, apiKey, model, provider)
}

// ProbeModelTargetWithID probes a target and updates health telemetry under the given targetID.
func (e *Engine) ProbeModelTargetWithID(ctx context.Context, targetID, baseURL, apiKey, model, provider string) (int64, string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)
	if baseURL == "" || apiKey == "" || model == "" {
		return 0, "", fmt.Errorf("Base URL、API Key 与模型 ID 均不能为空")
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
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if provider == "openrouter" {
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
		return elapsed, "", fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errText := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		e.health.recordFailure(healthID, errText, resp.StatusCode == http.StatusTooManyRequests)
		return elapsed, "", fmt.Errorf("%s", errText)
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
		return elapsed, "", fmt.Errorf("接口返回非标准 OpenAI 格式")
	}
	reply := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if reply == "" {
		reply = strings.TrimSpace(parsed.Choices[0].Message.ReasoningContent)
	}
	if reply == "" {
		reply = strings.TrimSpace(parsed.Choices[0].Message.Reasoning)
	}
	e.health.recordSuccess(healthID, elapsed)
	return elapsed, reply, nil
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
// OpenAI-compatible Base URL and API Key, returning filtered chat-capable model IDs.
func (e *Engine) FetchUpstreamModels(ctx context.Context, baseURL, apiKey, provider string) ([]string, error) {
	res, err := e.FetchAndVerifyUpstreamModels(ctx, baseURL, apiKey, provider, false)
	if err != nil {
		return nil, err
	}
	return res.Models, nil
}

// FetchAndVerifyUpstreamModels fetches candidate chat models (filtering non-chat modalities such as embeddings,
// image/video generators, audio/TTS) and optionally verifies live usability via concurrent chat probes.
func (e *Engine) FetchAndVerifyUpstreamModels(ctx context.Context, baseURL, apiKey, provider string, verify bool) (*FetchModelsResult, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("请先填写 Base URL 与 API Key")
	}

	var candidates []string
	filteredOut := 0

	if strings.Contains(strings.ToLower(baseURL), "generativelanguage.googleapis.com") {
		if gModels, gFiltered, ok := e.fetchGoogleNativeModels(ctx, apiKey); ok {
			candidates = gModels
			filteredOut = gFiltered
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
		if provider == "openrouter" {
			req.Header.Set("X-Title", "BotHub")
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("拉取模型列表失败: %w", err)
		}
		defer resp.Body.Close()

		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
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

	if !verify || len(candidates) == 0 {
		result.Models = candidates
		for _, id := range candidates {
			result.CheckedModels = append(result.CheckedModels, UpstreamModelCheck{
				ID:     id,
				Usable: true,
			})
		}
		return result, nil
	}

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
			if provider == "openrouter" {
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
