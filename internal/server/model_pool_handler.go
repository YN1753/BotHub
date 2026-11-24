package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) modelConfigPayload() map[string]any {
	providers, models, strategy := s.store.GetModelConfig()
	usage7d, _ := s.store.SummarizeModelUsage(7)
	usage30d, _ := s.store.SummarizeModelUsage(30)
	resp := map[string]any{
		"ok":                  true,
		"providers":           store.MaskModelProviders(providers),
		"models":              models,
		"strategy":            strategy,
		"model_providers":     store.MaskModelProviders(providers),
		"pooled_models":       models,
		"pool_strategy":       strategy,
		"model_usage_summary": usage7d,
		"model_usage_30d":     usage30d,
	}
	if s.agent != nil {
		resp["model_options"] = s.agent.ModelOptionsPayload()
		resp["model_health"] = s.agent.GetModelHealth()
	}
	return resp
}

func (s *Server) handleGetModelConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.modelConfigPayload())
}

func (s *Server) handleSaveModelConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Providers []store.ModelProvider `json:"providers"`
		Models    []store.PooledModel   `json:"models"`
		Strategy  string                `json:"strategy"`
	}
	if !decodeJSON(w, r, 512*1024, &req) {
		return
	}
	if err := s.store.SaveModelConfig(req.Providers, req.Models, req.Strategy); err != nil {
		writeError(w, http.StatusInternalServerError, "保存模型配置失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("models")
	s.publishAssistantEvent("model_health")
	writeJSON(w, http.StatusOK, s.modelConfigPayload())
}

func (s *Server) handleClearModelCooldown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
	}
	_ = decodeJSON(w, r, 4096, &req)
	if s.agent != nil {
		s.agent.ClearModelCooldown(strings.TrimSpace(req.TargetID))
	}
	s.publishAssistantEvent("models")
	s.publishAssistantEvent("model_health")
	resp := map[string]any{
		"ok": true,
	}
	if s.agent != nil {
		resp["model_health"] = s.agent.GetModelHealth()
		resp["model_options"] = s.agent.ModelOptionsPayload()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) resolveProviderSecrets(providerID, entryID, rawKey string, rawKeys []string, rawBaseURL, rawProtocol string) (apiKeys []string, baseURL, protocol string) {
	baseURL = strings.TrimSpace(rawBaseURL)
	protocol = strings.TrimSpace(rawProtocol)
	lookupID := strings.TrimSpace(providerID)
	if lookupID == "" {
		lookupID = strings.TrimSpace(entryID)
	}

	var storedKeys []string
	if lookupID != "" {
		providers, _, _ := s.store.GetModelConfig()
		for _, p := range providers {
			matched := p.ID == lookupID
			if !matched {
				for _, alias := range p.AliasIDs {
					if alias == lookupID {
						matched = true
						break
					}
				}
			}
			if matched {
				storedKeys = p.EffectiveAPIKeys()
				if baseURL == "" {
					baseURL = p.BaseURL
				}
				if protocol == "" {
					protocol = p.Protocol
				}
				break
			}
		}
	}

	if len(storedKeys) == 0 && (lookupID == "primary" || lookupID == "legacy_deepseek") {
		if k, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey); strings.TrimSpace(k) != "" {
			storedKeys = store.SplitAPIKeys(k)
		}
		if baseURL == "" {
			baseURL, _ = s.store.GetConfig(store.KeyDeepSeekBaseURL)
		}
	}

	submitted := store.SplitAPIKeys(rawKeys...)
	if len(submitted) == 0 {
		submitted = store.SplitAPIKeys(rawKey)
	}
	if len(submitted) == 0 {
		apiKeys = storedKeys
	} else {
		seen := make(map[string]bool, len(submitted))
		for idx, k := range submitted {
			rk := k
			if strings.Contains(k, "****") {
				rk = ""
				for _, sk := range storedKeys {
					if store.MaskSecretValue(sk) == k {
						rk = sk
						break
					}
				}
				if rk == "" && idx < len(storedKeys) {
					rk = storedKeys[idx]
				}
			}
			if rk != "" && !seen[rk] {
				seen[rk] = true
				apiKeys = append(apiKeys, rk)
			}
		}
		if len(apiKeys) == 0 {
			apiKeys = storedKeys
		}
	}

	return apiKeys, store.CleanProviderBaseURL(baseURL), store.NormalizeProviderProtocol(protocol, baseURL)
}

func (s *Server) handleFetchUpstreamModels(w http.ResponseWriter, r *http.Request) {
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "Agent 引擎未就绪")
		return
	}
	var req struct {
		ProviderID string   `json:"provider_id"`
		EntryID    string   `json:"entry_id"`
		Protocol   string   `json:"protocol"`
		Provider   string   `json:"provider"`
		BaseURL    string   `json:"base_url"`
		APIKey     string   `json:"api_key"`
		APIKeys    []string `json:"api_keys"`
		Verify     *bool    `json:"verify"`
	}
	if !decodeJSON(w, r, 32768, &req) {
		return
	}

	protoHint := strings.TrimSpace(req.Protocol)
	if protoHint == "" {
		protoHint = strings.TrimSpace(req.Provider)
	}
	apiKeys, baseURL, protocol := s.resolveProviderSecrets(req.ProviderID, req.EntryID, req.APIKey, req.APIKeys, req.BaseURL, protoHint)

	verify := false
	if req.Verify != nil {
		verify = *req.Verify
	}

	timeout := 25 * time.Second
	if verify {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	res, err := s.agent.FetchAndVerifyUpstreamModelsWithKeys(ctx, baseURL, apiKeys, protocol, verify)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"models":              res.Models,
		"checked_models":      res.CheckedModels,
		"filtered_out_count":  res.FilteredOutCount,
		"failed_verify_count": res.FailedVerifyCount,
		"unverified_count":    res.UnverifiedCount,
		"count":               len(res.Models),
	})
}

func (s *Server) handleTestModelTarget(w http.ResponseWriter, r *http.Request) {
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "Agent 引擎未就绪")
		return
	}
	var req struct {
		ProviderID string   `json:"provider_id"`
		EntryID    string   `json:"entry_id"`
		Protocol   string   `json:"protocol"`
		Provider   string   `json:"provider"`
		BaseURL    string   `json:"base_url"`
		APIKey     string   `json:"api_key"`
		APIKeys    []string `json:"api_keys"`
		Model      string   `json:"model"`
		ProbeTools *bool    `json:"probe_tools"`
	}
	if !decodeJSON(w, r, 32768, &req) {
		return
	}

	protoHint := strings.TrimSpace(req.Protocol)
	if protoHint == "" {
		protoHint = strings.TrimSpace(req.Provider)
	}
	pid := strings.TrimSpace(req.ProviderID)
	if pid == "" {
		pid = strings.TrimSpace(req.EntryID)
	}
	apiKeys, baseURL, protocol := s.resolveProviderSecrets(req.ProviderID, req.EntryID, req.APIKey, req.APIKeys, req.BaseURL, protoHint)

	probeTools := true
	if req.ProbeTools != nil {
		probeTools = *req.ProbeTools
	}

	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	res, err := s.agent.ProbeModelTargetWithCapabilitiesAndKeys(ctx, pid, baseURL, apiKeys, req.Model, protocol, probeTools)
	if err != nil {
		latency := int64(0)
		targetID := ""
		if res != nil {
			latency = res.LatencyMs
			targetID = res.TargetID
		}
		s.publishAssistantEvent("model_health")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           false,
			"target_id":    targetID,
			"latency_ms":   latency,
			"error":        err.Error(),
			"model_health": s.agent.GetModelHealth(),
		})
		return
	}

	s.publishAssistantEvent("models")
	s.publishAssistantEvent("model_health")
	_, models, _ := s.store.GetModelConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                true,
		"target_id":         res.TargetID,
		"latency_ms":        res.LatencyMs,
		"preview":           res.ReplyPreview,
		"supports_tools":    res.SupportsTools,
		"supports_thinking": res.SupportsThinking,
		"pooled_models":     models,
		"model_health":      s.agent.GetModelHealth(),
		"model_options":     s.agent.ModelOptionsPayload(),
	})
}

type batchModelProbeResult struct {
	TargetID         string `json:"target_id"`
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	OK               bool   `json:"ok"`
	LatencyMs        int64  `json:"latency_ms"`
	Error            string `json:"error,omitempty"`
	SupportsTools    *bool  `json:"supports_tools,omitempty"`
	SupportsThinking *bool  `json:"supports_thinking,omitempty"`
}

func (s *Server) handleTestModelBatch(w http.ResponseWriter, r *http.Request) {
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "Agent 引擎未就绪")
		return
	}
	var req struct {
		DisableFailed bool  `json:"disable_failed"`
		ProbeTools    *bool `json:"probe_tools"`
	}
	_ = decodeOptionalJSON(w, r, 8192, &req)

	probeTools := true
	if req.ProbeTools != nil {
		probeTools = *req.ProbeTools
	}

	providers, models, strategy := s.store.GetModelConfig()
	provMap := make(map[string]store.ModelProvider, len(providers))
	for _, p := range providers {
		provMap[p.ID] = p
	}

	var candidates []store.PooledModel
	for _, m := range models {
		if !m.Enabled {
			continue
		}
		p, ok := provMap[m.ProviderID]
		if !ok || !p.Enabled || strings.TrimSpace(p.APIKey) == "" {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"results":       []batchModelProbeResult{},
			"pooled_models": models,
			"model_health":  s.agent.GetModelHealth(),
			"model_options": s.agent.ModelOptionsPayload(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 55*time.Second)
	defer cancel()

	results := make([]batchModelProbeResult, len(candidates))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for idx, item := range candidates {
		wg.Add(1)
		go func(i int, pm store.PooledModel) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			p := provMap[pm.ProviderID]
			tCtx, tCancel := context.WithTimeout(ctx, 20*time.Second)
			res, err := s.agent.ProbeModelTargetWithCapabilities(tCtx, p.ID, p.BaseURL, p.APIKey, pm.ModelID, p.Protocol, probeTools)
			tCancel()

			entry := batchModelProbeResult{
				TargetID:   pm.CanonicalTargetID(),
				ProviderID: pm.ProviderID,
				ModelID:    pm.ModelID,
			}
			if err != nil {
				entry.OK = false
				entry.Error = err.Error()
				if res != nil {
					entry.LatencyMs = res.LatencyMs
				}
			} else if res != nil {
				entry.OK = true
				entry.LatencyMs = res.LatencyMs
				entry.SupportsTools = res.SupportsTools
				entry.SupportsThinking = res.SupportsThinking
			}
			results[i] = entry
		}(idx, item)
	}
	wg.Wait()

	if req.DisableFailed {
		failedSet := make(map[string]bool)
		for _, res := range results {
			if !res.OK {
				failedSet[res.TargetID] = true
			}
		}
		if len(failedSet) > 0 {
			_, latestModels, latestStrat := s.store.GetModelConfig()
			changed := false
			for i := range latestModels {
				if failedSet[latestModels[i].CanonicalTargetID()] && latestModels[i].Enabled {
					latestModels[i].Enabled = false
					changed = true
				}
			}
			if changed {
				_ = s.store.SaveModelConfig(providers, latestModels, latestStrat)
			} else {
				_ = strategy
			}
		}
	}

	s.publishAssistantEvent("models")
	s.publishAssistantEvent("model_health")
	_, updatedModels, _ := s.store.GetModelConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"results":       results,
		"pooled_models": updatedModels,
		"model_health":  s.agent.GetModelHealth(),
		"model_options": s.agent.ModelOptionsPayload(),
	})
}
