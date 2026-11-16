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

func TestSelectSmartGroupMembersAndQuoteTempSummon(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "agent_group_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Verify smart group selection picks right domain bots without calling all 4
	allBots, _ := st.ListBots()
	var members []store.Bot
	for _, b := range allBots {
		if !b.IsGroup {
			members = append(members, b)
		}
	}
	pickedSentinel := selectSmartGroupMembers(members, "查一下探针节点的 CPU 负载")
	if len(pickedSentinel) != 1 || pickedSentinel[0].ID != "sentinel" {
		t.Fatalf("expected sentinel for CPU/nezha prompt, got %+v", pickedSentinel)
	}
	pickedAll := selectSmartGroupMembers(members, "请各位专家一起评审方案")
	if len(pickedAll) != len(members) {
		t.Fatalf("expected all %d members for collective cue, got %d", len(members), len(pickedAll))
	}

	// Verify 1v1 @external Bot temporary summon + quote-reply prompt injection
	var capturedBody string
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capturedBody = string(raw)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"探针哨兵已临时介入完成巡检。"}}]}`,
			`data: [DONE]`,
		}
		for _, c := range chunks {
			fmt.Fprintln(w, c)
			fmt.Fprintln(w)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer mockLLM.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-test-key")
	_ = st.SetConfig(store.KeyDeepSeekBaseURL, mockLLM.URL)
	_ = st.SetConfig(store.KeyAIModel, "deepseek-flash")

	eng := NewEngine(st, &fakeTaskController{st: st})

	var events []string
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:        "butler",
		Message:      "@探针哨兵 帮我看下这条历史告警",
		ReplyToID:    42,
		ReplyPreview: "管理员: CloudCone-LA 延迟抖动",
	}, func(event string, data any) {
		b, _ := json.Marshal(data)
		events = append(events, event+":"+string(b))
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}
	if resp.SenderBotID != "sentinel" || !strings.Contains(resp.Content, "探针哨兵") {
		t.Fatalf("expected sentinel response via temp summon, got %+v", resp)
	}
	if !strings.Contains(capturedBody, "CloudCone-LA 延迟抖动") {
		t.Fatalf("expected quoted reply_preview injected into LLM messages, got %s", capturedBody)
	}
	joinedEvents := strings.Join(events, "\n")
	if !strings.Contains(joinedEvents, "system_notice:") || !strings.Contains(joinedEvents, "is_temp_guest\":true") {
		t.Fatalf("expected system_notice and is_temp_guest bot_turn event, got:\n%s", joinedEvents)
	}

	stored, _ := st.ListBotMessages("butler", 10)
	if len(stored) < 3 || stored[0].ReplyToID != 42 || stored[1].Role != "system" {
		t.Fatalf("expected user quote + system notice + sentinel reply stored on butler timeline, got: %+v", stored)
	}
}

func TestSmartGroupAutoBatonRelay(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "agent_relay_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	callCount := 0
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		var chunks []string
		if callCount == 1 {
			// Lead bot delegates follow-up check to @探针哨兵
			chunks = []string{
				`data: {"choices":[{"delta":{"role":"assistant","content":"域名与续费周期核验无误，请 @探针哨兵 顺便确认节点实时在线率。"}}]}`,
				`data: [DONE]`,
			}
		} else {
			chunks = []string{
				`data: {"choices":[{"delta":{"role":"assistant","content":"探针哨兵已接力复核：所有核心节点在线率 100%。"}}]}`,
				`data: [DONE]`,
			}
		}
		for _, c := range chunks {
			fmt.Fprintln(w, c)
			fmt.Fprintln(w)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer mockLLM.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-test-key")
	_ = st.SetConfig(store.KeyDeepSeekBaseURL, mockLLM.URL)
	_ = st.SetConfig(store.KeyAIModel, "deepseek-flash")

	eng := NewEngine(st, &fakeTaskController{st: st})
	var turns []string
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		BotID:   "team_room",
		Message: "检查一下域名到期情况",
	}, func(event string, data any) {
		if event == "bot_turn" {
			b, _ := json.Marshal(data)
			turns = append(turns, string(b))
		}
	})
	if err != nil {
		t.Fatalf("ChatStream relay failed: %v", err)
	}
	if len(turns) != 2 || !strings.Contains(turns[0], `"sender_bot_id":"butler"`) || !strings.Contains(turns[1], `"sender_bot_id":"sentinel"`) {
		t.Fatalf("expected 1-hop auto relay from butler to sentinel, got turns: %v", turns)
	}
	if !strings.Contains(resp.Content, "在线率 100%") {
		t.Fatalf("expected relayed sentinel response in combined content, got: %s", resp.Content)
	}
}

