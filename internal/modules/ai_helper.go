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

// SummarizeWithDeepSeek invokes the configured lightweight model (light_base_url / light_api_key / light_model)
// or falls back to the primary DeepSeek model (deepseek-flash / deepseek-v4-pro) to synthesize raw intelligence.
// If neither API Key is configured or an error occurs, it returns ("", err) so callers can gracefully fall back.
func SummarizeWithDeepSeek(ctx context.Context, st *store.Store, systemPrompt, userPrompt string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("store is nil")
	}

	// Prefer lightweight model config for batch morning/radar tasks to save costs
	apiKey, _ := st.GetConfig(store.KeyLightAPIKey)
	baseURL, _ := st.GetConfig(store.KeyLightBaseURL)
	model, _ := st.GetConfig(store.KeyLightModel)

	apiKey = strings.TrimSpace(apiKey)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	model = strings.TrimSpace(model)

	if apiKey == "" {
		apiKey, _ = st.GetConfig(store.KeyDeepSeekAPIKey)
		apiKey = strings.TrimSpace(apiKey)
	}
	if apiKey == "" {
		return "", fmt.Errorf("api_key not configured")
	}

	if baseURL == "" {
		baseURL, _ = st.GetConfig(store.KeyDeepSeekBaseURL)
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			baseURL = store.DefaultDeepSeekBaseURL
		}
	}

	if model == "" {
		model, _ = st.GetConfig(store.KeyAIModel)
		model = strings.TrimSpace(model)
	}
	switch model {
	case "", "deepseek-chat":
		model = store.DefaultAIModel // "deepseek-flash"
	case "deepseek-reasoner":
		model = "deepseek-v4-pro"
	}

	// Inject long-term user memories so AI curation matches user tech stack & infra context
	memories, _ := st.ListAgentMemories("", 20)
	if len(memories) > 0 {
		var memBuf strings.Builder
		memBuf.WriteString("\n\n[用户长期记忆与技术栈背景]\n")
		for _, m := range memories {
			memBuf.WriteString(fmt.Sprintf("- [%s] %s: %s\n", m.Category, m.FactKey, m.Content))
		}
		systemPrompt += memBuf.String()
	}

	reqCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	reqBody := map[string]any{
		"model":      model,
		"stream":     false,
		"max_tokens": 1200,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	if strings.Contains(strings.ToLower(model), "flash") {
		reqBody["thinking"] = map[string]string{"type": "disabled"}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := NewHTTPClient(40 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("decode response failed")
	}

	out := notifier.StripEmoji(strings.TrimSpace(parsed.Choices[0].Message.Content))
	if out == "" {
		return "", fmt.Errorf("empty ai response")
	}
	return out, nil
}
