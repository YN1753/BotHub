package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func (s *Server) resolveModelPoolTargetKey(targetID, rawKey string) string {
	apiKey := strings.TrimSpace(rawKey)
	if !strings.Contains(apiKey, "****") && apiKey != "" {
		return apiKey
	}
	if targetID == "primary" {
		stored, _ := s.store.GetConfig(store.KeyDeepSeekAPIKey)
		return strings.TrimSpace(stored)
	}
	for _, item := range s.store.GetModelPool() {
		if item.ID == targetID {
			return strings.TrimSpace(item.APIKey)
		}
	}
	return apiKey
}

func (s *Server) handleSaveModelPool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entries []store.ModelPoolEntry `json:"entries"`
	}
	if !decodeJSON(w, r, 64*1024, &req) {
		return
	}
	for _, entry := range req.Entries {
		if strings.TrimSpace(entry.BaseURL) != "" && !validateHTTPURL(entry.BaseURL) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("模型池条目 %s 的 Base URL 必须为合法的 http/https 地址", entry.Name))
			return
		}
	}
	if err := s.store.SaveModelPool(req.Entries); err != nil {
		writeError(w, http.StatusInternalServerError, "保存免费模型池失败: "+err.Error())
		return
	}
	s.publishAssistantEvent("model_health")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"model_pool": store.MaskModelPool(s.store.GetModelPool()),
	})
}

func (s *Server) handleTestModelTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
	}
	if !decodeJSON(w, r, 16*1024, &req) {
		return
	}

	apiKey := s.resolveModelPoolTargetKey(req.TargetID, req.APIKey)
	ctx, cancel := context.WithTimeout(r.Context(), 22*time.Second)
	defer cancel()

	latencyMs, preview, err := s.agent.ProbeModelTargetWithID(ctx, req.TargetID, req.BaseURL, apiKey, req.Model, req.Provider)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"latency_ms":   latencyMs,
		"reply":        preview,
		"model_health": s.agent.GetModelHealth(),
		"message":      fmt.Sprintf("连接成功 (%dms)", latencyMs),
	})
}

func (s *Server) handleFetchUpstreamModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Verify   *bool  `json:"verify,omitempty"`
	}
	if !decodeJSON(w, r, 16*1024, &req) {
		return
	}

	apiKey := s.resolveModelPoolTargetKey(req.TargetID, req.APIKey)
	verify := true
	if req.Verify != nil {
		verify = *req.Verify
	}

	ctx, cancel := context.WithTimeout(r.Context(), 38*time.Second)
	defer cancel()

	res, err := s.agent.FetchAndVerifyUpstreamModels(ctx, req.BaseURL, apiKey, req.Provider, verify)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                 true,
		"models":             res.Models,
		"count":              len(res.Models),
		"checked_models":     res.CheckedModels,
		"filtered_out_count": res.FilteredOutCount,
		"unverified_count":   res.UnverifiedCount,
	})
}
