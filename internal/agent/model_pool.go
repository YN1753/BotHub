package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

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

func IsAllowThinkingModel(provider, model string) bool {
	if provider != "" && provider != "deepseek" {
		return false
	}
	return strings.Contains(strings.ToLower(model), "flash")
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
	return ModelTarget{
		ID:            "primary",
		Label:         "主力 · " + model,
		Provider:      "deepseek",
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Model:         model,
		AllowThinking: IsAllowThinkingModel("deepseek", model),
	}
}

func activeFreePoolTargets(st *store.Store) []ModelTarget {
	entries := st.GetModelPool()
	targets := make([]ModelTarget, 0, len(entries))
	for _, entry := range entries {
		if !entry.Enabled || strings.TrimSpace(entry.APIKey) == "" {
			continue
		}
		targets = append(targets, ModelTarget{
			ID:            "pool:" + entry.ID,
			Label:         entry.Name,
			Provider:      entry.Provider,
			BaseURL:       entry.BaseURL,
			APIKey:        strings.TrimSpace(entry.APIKey),
			Model:         strings.TrimSpace(entry.Model),
			AllowThinking: false,
		})
	}
	return targets
}

// ResolveChatTargets builds an ordered failover slice of upstream ModelTargets
// supporting:
// - "free-pool": round-robin / priority failover across all enabled free AI pool providers
// - "pool:<id>": pinned free provider with optional fallback to remaining pool entries
// - "deepseek-flash" / "" / relay model ID: primary DeepSeek Flash / relay station with optional free pool failover
func (e *Engine) ResolveChatTargets(requestedModel string) ([]ModelTarget, error) {
	reqM := strings.TrimSpace(requestedModel)
	poolTargets := activeFreePoolTargets(e.store)
	primary := primaryDeepSeekTarget(e.store)
	allowFallback := isConfigTrue(e.store, store.KeyPoolFallback, true)

	if reqM == "free-pool" {
		if len(poolTargets) > 0 {
			if primary.APIKey != "" && allowFallback {
				return append(poolTargets, primary), nil
			}
			return poolTargets, nil
		}
		if primary.APIKey != "" {
			return []ModelTarget{primary}, nil
		}
		return nil, fmt.Errorf("免费模型池暂无可用 Key，请先在「设置 -> AI 大脑设置」中至少填入一个免费节点（如 Google AI Studio 或硅基流动）")
	}

	if strings.HasPrefix(reqM, "pool:") {
		targetID := strings.TrimPrefix(reqM, "pool:")
		allEntries := e.store.GetModelPool()
		var matched *store.ModelPoolEntry
		for i := range allEntries {
			if allEntries[i].ID == targetID {
				matched = &allEntries[i]
				break
			}
		}
		if matched == nil {
			return nil, fmt.Errorf("未找到指定的免费模型节点: %s", targetID)
		}
		if strings.TrimSpace(matched.APIKey) == "" {
			return nil, fmt.Errorf("模型「%s」尚未填写 API Key，请先在系统设置中填入密钥", matched.Name)
		}
		pinned := ModelTarget{
			ID:       "pool:" + matched.ID,
			Label:    matched.Name,
			Provider: matched.Provider,
			BaseURL:  matched.BaseURL,
			APIKey:   strings.TrimSpace(matched.APIKey),
			Model:    strings.TrimSpace(matched.Model),
		}
		out := []ModelTarget{pinned}
		if allowFallback {
			for _, pt := range poolTargets {
				if pt.ID != pinned.ID {
					out = append(out, pt)
				}
			}
			if primary.APIKey != "" {
				out = append(out, primary)
			}
		}
		return out, nil
	}

	// Primary DeepSeek Flash (or Relay Station custom model)
	normModel := NormalizeModelID(reqM)
	if reqM != "" && normModel != store.DefaultAIModel {
		primary.Model = normModel
		primary.Label = "主力 · " + normModel
		primary.AllowThinking = IsAllowThinkingModel("deepseek", normModel)
	}

	if primary.APIKey != "" {
		out := []ModelTarget{primary}
		if allowFallback && len(poolTargets) > 0 {
			out = append(out, poolTargets...)
		}
		return out, nil
	}

	if len(poolTargets) > 0 {
		return poolTargets, nil
	}
	return nil, fmt.Errorf("尚未配置主力 DeepSeek / 中转站 API Key，且免费模型池暂无可用 Key，请先在「设置 -> AI 大脑设置」中配置")
}

// ProbeModelTarget executes a lightweight non-streaming connectivity test against a target provider.
func (e *Engine) ProbeModelTarget(ctx context.Context, baseURL, apiKey, model, provider string) (int64, string, error) {
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
		req.Header.Set("HTTP-Referer", "https://bot.starryheavens.uk")
		req.Header.Set("X-Title", "Vesper BotHub")
	}

	resp, err := e.httpClient.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return elapsed, "", fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return elapsed, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return elapsed, "", fmt.Errorf("接口返回非标准 OpenAI 格式")
	}
	return elapsed, strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
