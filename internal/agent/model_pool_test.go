package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestFreeModelPoolRoutingFailoverAndFlashOnlyNormalization(t *testing.T) {
	if got := NormalizeModelID("deepseek-v4-pro"); got != "deepseek-flash" {
		t.Fatalf("expected deepseek-v4-pro normalized to deepseek-flash, got %q", got)
	}
	if got := NormalizeModelID("deepseek-reasoner"); got != "deepseek-flash" {
		t.Fatalf("expected deepseek-reasoner normalized to deepseek-flash, got %q", got)
	}

	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "model_pool_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Verify default pool ships with Google AI Studio & SiliconFlow
	defaults := st.GetModelPool()
	var hasGoogle, hasSilicon bool
	for _, d := range defaults {
		if d.ID == "google_ai_studio" {
			hasGoogle = true
		}
		if d.ID == "siliconflow_free" {
			hasSilicon = true
		}
	}
	if !hasGoogle || !hasSilicon {
		t.Fatalf("expected default pool to contain google_ai_studio and siliconflow_free, got %+v", defaults)
	}

	// Mock Provider 1 (Google AI Studio) returning HTTP 429 rate limit
	googleRateLimited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
	}))
	defer googleRateLimited.Close()

	// Mock Provider 2 (SiliconFlow) checking no DeepSeek `thinking` field is sent and streaming reply
	var sfPayload string
	sfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		sfPayload = string(raw)
		if strings.Contains(r.URL.Path, "chat/completions") && !strings.Contains(r.URL.Path, "chat/completions/chat") {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			for _, c := range []string{
				`data: {"choices":[{"delta":{"role":"assistant","content":"硅基流动免费模型已自动接管并回复。"}}]}`,
				`data: [DONE]`,
			} {
				fmt.Fprintln(w, c)
				fmt.Fprintln(w)
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer sfServer.Close()

	// Configure pool with NO primary DeepSeek key at all (pure free pool mode!)
	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "")
	for i := range defaults {
		if defaults[i].ID == "google_ai_studio" {
			defaults[i].BaseURL = googleRateLimited.URL
			defaults[i].APIKey = "AIzaTestGoogleKey123456"
			defaults[i].Priority = 10
		} else if defaults[i].ID == "siliconflow_free" {
			defaults[i].BaseURL = sfServer.URL
			defaults[i].APIKey = "sk-sf-testkey123456"
			defaults[i].Priority = 20
		}
	}
	if err := st.SaveModelPool(defaults); err != nil {
		t.Fatalf("SaveModelPool: %v", err)
	}

	// Verify SaveModelPool preserves existing key when masked placeholder is submitted back
	masked := store.MaskModelPool(st.GetModelPool())
	if err := st.SaveModelPool(masked); err != nil {
		t.Fatalf("SaveModelPool masked roundtrip: %v", err)
	}
	reloaded := st.GetModelPool()
	if reloaded[0].APIKey != "AIzaTestGoogleKey123456" {
		t.Fatalf("expected Google API key preserved across **** mask roundtrip, got %q", reloaded[0].APIKey)
	}

	eng := NewEngine(st, &fakeTaskController{st: st})

	// Test ChatStream on Bot with model="free-pool": should hit Google (429) -> failover cleanly to SiliconFlow
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "butler",
		Message: "测试免费模型池自动容灾切换",
		Model:   "free-pool",
	}, nil)
	if err != nil {
		t.Fatalf("ChatStream free-pool failed: %v", err)
	}
	if resp.Model != "Qwen/Qwen2.5-7B-Instruct" || !strings.Contains(resp.Content, "硅基流动免费模型") {
		t.Fatalf("unexpected free-pool failover response: %+v", resp)
	}
	if strings.Contains(sfPayload, `"thinking"`) {
		t.Fatalf("non-DeepSeek pool provider must not receive proprietary DeepSeek thinking field: %s", sfPayload)
	}

	// Test Primary Relay Station URL + custom relay model name
	var relayUsedModel, relayAuth string
	relaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relayAuth = r.Header.Get("Authorization")
		var reqBody map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &reqBody)
		relayUsedModel, _ = reqBody["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer relaySrv.Close()

	_, preview, probeErr := eng.ProbeModelTarget(context.Background(), relaySrv.URL+"/v1/chat/completions", "sk-relay-token", "deepseek-v3-flash-relay", "deepseek")
	if probeErr != nil || preview != "OK" {
		t.Fatalf("ProbeModelTarget relay failed: err=%v preview=%q", probeErr, preview)
	}
	if relayUsedModel != "deepseek-v3-flash-relay" || relayAuth != "Bearer sk-relay-token" {
		t.Fatalf("unexpected relay probe request: model=%q auth=%q", relayUsedModel, relayAuth)
	}
}
