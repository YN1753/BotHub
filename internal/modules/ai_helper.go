package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// SummarizeWithDeepSeek invokes explicit lightweight model config, or enabled Free Model Pool entries
// (when pool_for_light is active), or falls back to Primary DeepSeek Flash / Relay station.
func SummarizeWithDeepSeek(ctx context.Context, st *store.Store, systemPrompt, userPrompt string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("store is nil")
	}

	type llmCandidate struct {
		baseURL       string
		apiKey        string
		model         string
		provider      string
		allowThinking bool
	}

	var candidates []llmCandidate

	lightKey, _ := st.GetConfig(store.KeyLightAPIKey)
	lightBase, _ := st.GetConfig(store.KeyLightBaseURL)
	lightModel, _ := st.GetConfig(store.KeyLightModel)
	lightKey = strings.TrimSpace(lightKey)
	lightBase = strings.TrimRight(strings.TrimSpace(lightBase), "/")
	lightModel = strings.TrimSpace(lightModel)

	primaryBase, _ := st.GetConfig(store.KeyDeepSeekBaseURL)
	primaryBase = strings.TrimRight(strings.TrimSpace(primaryBase), "/")
	if primaryBase == "" {
		primaryBase = store.DefaultDeepSeekBaseURL
	}
	primaryKey, _ := st.GetConfig(store.KeyDeepSeekAPIKey)
	primaryKey = strings.TrimSpace(primaryKey)
	primaryModel, _ := st.GetConfig(store.KeyAIModel)
	primaryModel = strings.TrimSpace(primaryModel)
	if primaryModel == "" || primaryModel == "deepseek-v4-pro" {
		primaryModel = store.DefaultAIModel
	}

	if lightKey != "" {
		b := lightBase
		if b == "" {
			b = primaryBase
		}
		m := lightModel
		if m == "" || m == "deepseek-v4-pro" {
			m = primaryModel
		}
		candidates = append(candidates, llmCandidate{
			baseURL:       b,
			apiKey:        lightKey,
			model:         m,
			provider:      "custom",
			allowThinking: (strings.Contains(strings.ToLower(m), "flash") || m == "deepseek-chat") && strings.Contains(strings.ToLower(b), "deepseek.com"),
		})
	}

	poolLightFlag, _ := st.GetConfig(store.KeyPoolForLight)
	usePoolLight := strings.TrimSpace(poolLightFlag) == "" || strings.EqualFold(strings.TrimSpace(poolLightFlag), "true") || strings.TrimSpace(poolLightFlag) == "1"
	if usePoolLight {
		for _, entry := range st.GetModelPool() {
			if !entry.Enabled || strings.TrimSpace(entry.APIKey) == "" || strings.TrimSpace(entry.Model) == "" {
				continue
			}
			candidates = append(candidates, llmCandidate{
				baseURL:  strings.TrimRight(strings.TrimSpace(entry.BaseURL), "/"),
				apiKey:   strings.TrimSpace(entry.APIKey),
				model:    strings.TrimSpace(entry.Model),
				provider: entry.Provider,
			})
		}
	}

	if primaryKey != "" {
		candidates = append(candidates, llmCandidate{
			baseURL:       primaryBase,
			apiKey:        primaryKey,
			model:         primaryModel,
			provider:      "deepseek",
			allowThinking: (strings.Contains(strings.ToLower(primaryModel), "flash") || primaryModel == "deepseek-chat") && strings.Contains(strings.ToLower(primaryBase), "deepseek.com"),
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("api_key not configured")
	}

	// Inject ONLY global shared memories so private bot:<id> memories never leak into global tasks
	memories, _ := st.ListScopedMemories([]string{"global"}, "", 20)
	if len(memories) > 0 {
		var memBuf strings.Builder
		memBuf.WriteString("\n\n[用户长期记忆与技术栈背景]\n")
		for _, m := range memories {
			memBuf.WriteString(fmt.Sprintf("- [%s] %s: %s\n", m.Category, m.FactKey, m.Content))
		}
		systemPrompt += memBuf.String()
	}

	client := NewHTTPClient(40 * time.Second)
	var lastErr error

	for _, c := range candidates {
		reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)

		reqBody := map[string]any{
			"model":      c.model,
			"stream":     false,
			"max_tokens": 1200,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
		}
		if c.allowThinking {
			reqBody["thinking"] = map[string]string{"type": "disabled"}
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			cancel()
			return "", err
		}

		endpoint := c.baseURL
		if !strings.HasSuffix(endpoint, "/chat/completions") {
			endpoint += "/chat/completions"
		}
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		if c.provider == "openrouter" {
			req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
			req.Header.Set("X-Title", "Vesper BotHub")
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("AI HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
			continue
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
			lastErr = fmt.Errorf("parse AI response failed")
			continue
		}
		return notifier.StripEmoji(strings.TrimSpace(parsed.Choices[0].Message.Content)), nil
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("all configured AI endpoints failed")
}
