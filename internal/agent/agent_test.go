package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	_ "modernc.org/sqlite"
)

type fakeTaskController struct {
	st *store.Store
}

func (f *fakeTaskController) Status() ([]scheduler.TaskStatus, error) {
	tasks, err := f.st.ListTasks()
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.TaskStatus, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, scheduler.TaskStatus{
			ID:          t.ID,
			Name:        t.Name,
			Type:        t.Type,
			Description: t.Description,
			CronExpr:    t.CronExpr,
			Enabled:     t.Enabled,
			Params:      t.Params,
		})
	}
	return out, nil
}

func (f *fakeTaskController) RunNow(_ context.Context, taskID string) (modules.RunResult, error) {
	return modules.RunResult{
		Summary:     "模拟执行成功: " + taskID,
		ItemsPushed: 1,
	}, nil
}

func (f *fakeTaskController) Reload() error {
	return nil
}

func TestChatStreamWithToolCallsAndReasoner(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "bothub_test.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	var round int32
	mockDeepSeek := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		cur := atomic.AddInt32(&round, 1)
		flusher, _ := w.(http.Flusher)

		if cur == 1 {
			// Round 1: stream reasoning_content + tool_call for remember_fact
			chunks := []string{
				`data: {"choices":[{"delta":{"role":"assistant","reasoning_content":"用户希望记住主节点位置，先调用 remember_fact 工具。"}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_mem_1","type":"function","function":{"name":"remember_fact","arguments":"{\"category\":\"infra\","}}]}}]}`,
				`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"fact_key\":\"primary_vps\",\"content\":\"CloudCone LA DC2\"}"}}]}}]}`,
				`data: [DONE]`,
			}
			for _, c := range chunks {
				fmt.Fprintln(w, c)
				fmt.Fprintln(w)
				if flusher != nil {
					flusher.Flush()
				}
			}
			return
		}

		// Round 2: stream final answer after receiving tool output
		chunks := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":"已将主节点 CloudCone LA DC2 写入长期记忆库。"}}]}`,
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
	defer mockDeepSeek.Close()

	_ = st.SetConfig(store.KeyDeepSeekAPIKey, "sk-test-key")
	_ = st.SetConfig(store.KeyDeepSeekBaseURL, mockDeepSeek.URL)
	_ = st.SetConfig(store.KeyAIModel, "deepseek-v4-pro")
	_ = st.SetConfig(store.KeyActivePersona, PersonaGrok)

	eng := NewEngine(st, &fakeTaskController{st: st})

	events := make([]string, 0)
	resp, err := eng.ChatStream(context.Background(), ChatRequest{
		Message: "请记住我的主节点是 CloudCone LA DC2",
		Persona: PersonaGrok,
	}, func(event string, _ any) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	if !strings.Contains(resp.ReasoningContent, "remember_fact") {
		t.Fatalf("expected reasoning_content to contain tool thought, got %q", resp.ReasoningContent)
	}
	if !strings.Contains(resp.Content, "CloudCone LA DC2") {
		t.Fatalf("expected final content to mention CloudCone LA DC2, got %q", resp.Content)
	}
	if len(resp.ToolTraces) != 1 || resp.ToolTraces[0].Name != "remember_fact" {
		t.Fatalf("expected 1 remember_fact tool trace, got %+v", resp.ToolTraces)
	}

	// Verify memory was persisted in SQLite
	mems, err := st.ListAgentMemories("CloudCone", 10)
	if err != nil || len(mems) != 1 {
		t.Fatalf("expected 1 saved memory, got %v (err=%v)", mems, err)
	}
	if mems[0].FactKey != "primary_vps" {
		t.Fatalf("unexpected fact_key: %s", mems[0].FactKey)
	}

	// Verify session messages persisted
	msgs, err := st.ListAgentMessages(resp.SessionID, 20)
	if err != nil || len(msgs) != 4 {
		t.Fatalf("expected 4 persisted messages (user, assistant-tool, tool, assistant-final), got %d (err=%v)", len(msgs), err)
	}
}

func TestBuiltinToolsExecution(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.Open(filepath.Join(tmpDir, "bothub_tools.db"))
	if err != nil {
		t.Fatalf("open store failed: %v", err)
	}
	defer st.Close()

	// Create a mock Nezha SQLite DB
	nezhaPath := filepath.Join(tmpDir, "nezha.db")
	nezhaDB, err := sql.Open("sqlite", nezhaPath)
	if err != nil {
		t.Fatalf("open mock nezha db failed: %v", err)
	}
	_, err = nezhaDB.Exec(`
		CREATE TABLE servers (
			id INTEGER PRIMARY KEY,
			name TEXT,
			host TEXT,
			display_index INTEGER DEFAULT 0,
			updated_at TEXT
		);
		INSERT INTO servers (id, name, host, display_index, updated_at)
		VALUES (1, 'CloudCone-LA', '{"Platform":"debian","CPU":["AMD EPYC"]}', 10, '2026-11-15 16:00:00');
	`)
	_ = nezhaDB.Close()
	if err != nil {
		t.Fatalf("seed mock nezha db failed: %v", err)
	}
	_ = st.SetConfig(store.KeyNezhaDBPath, nezhaPath)

	eng := NewEngine(st, &fakeTaskController{st: st})

	// 1. Test query_nezha_nodes
	nezhaOut, err := eng.ExecuteTool(context.Background(), "query_nezha_nodes", `{"keyword":"CloudCone"}`)
	if err != nil {
		t.Fatalf("query_nezha_nodes failed: %v", err)
	}
	if !strings.Contains(nezhaOut, "nezha_sqlite_ro") || !strings.Contains(nezhaOut, "CloudCone-LA") {
		t.Fatalf("unexpected query_nezha_nodes output: %s", nezhaOut)
	}

	// 2. Test manage_assets upsert & list
	_, err = eng.ExecuteTool(context.Background(), "manage_assets", `{
		"action":"upsert",
		"id":"test-vps",
		"name":"RackNerd Seattle",
		"category":"compute",
		"price":"$11.29/Yr",
		"due_date":"2026-12-01"
	}`)
	if err != nil {
		t.Fatalf("manage_assets upsert failed: %v", err)
	}
	assetsOut, err := eng.ExecuteTool(context.Background(), "manage_assets", `{"action":"list","category":"compute"}`)
	if err != nil || !strings.Contains(assetsOut, "RackNerd Seattle") {
		t.Fatalf("manage_assets list failed: %v, out=%s", err, assetsOut)
	}

	// 3. Test manage_tasks create, list & run
	_ = st.SaveTask(store.Task{
		ID:       "asset_watcher",
		Name:     "资产守望",
		Type:     "asset_watcher",
		CronExpr: "0 3 * * *",
		Enabled:  true,
	})
	tasksOut, err := eng.ExecuteTool(context.Background(), "manage_tasks", `{"action":"list"}`)
	if err != nil || !strings.Contains(tasksOut, "asset_watcher") {
		t.Fatalf("manage_tasks list failed: %v, out=%s", err, tasksOut)
	}
	runOut, err := eng.ExecuteTool(context.Background(), "manage_tasks", `{"action":"run","id":"asset_watcher"}`)
	if err != nil || !strings.Contains(runOut, "模拟执行成功") {
		t.Fatalf("manage_tasks run failed: %v, out=%s", err, runOut)
	}

	// 4. Test read_url SSRF protection blocks loopback by default
	longText := strings.Repeat("A", 12000)
	pageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><script>console.log(1)</script><style>.a{}</style></head><body><nav>noisy nav</nav><h1>Vesper Architecture</h1><p>Zero Docker Native SQLite WAL. ` + longText + `</p><footer>noisy footer</footer></body></html>`))
	}))
	defer pageSrv.Close()

	urlArgs, _ := json.Marshal(map[string]string{"url": pageSrv.URL})
	if _, ssrfErr := eng.ExecuteTool(context.Background(), "read_url", string(urlArgs)); ssrfErr == nil {
		t.Fatalf("expected SSRF protection to block loopback URL %s", pageSrv.URL)
	}

	// Enable loopback for local mock test and verify noise removal + 10000 char truncation
	eng.allowLocalhostForTest = true
	readOut, err := eng.ExecuteTool(context.Background(), "read_url", string(urlArgs))
	if err != nil || !strings.Contains(readOut, "Zero Docker Native SQLite WAL") {
		t.Fatalf("read_url failed: %v, out=%s", err, readOut)
	}
	if strings.Contains(readOut, "noisy nav") || strings.Contains(readOut, "noisy footer") {
		t.Fatalf("expected nav/footer noise to be stripped, got: %s", readOut)
	}
	var parsedRead struct {
		CharCount int  `json:"char_count"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(readOut), &parsedRead); err != nil {
		t.Fatalf("unmarshal read_url output failed: %v", err)
	}
	if !parsedRead.Truncated || parsedRead.CharCount > MaxReadURLChars {
		t.Fatalf("expected truncated=true and char_count <= %d, got %+v", MaxReadURLChars, parsedRead)
	}

	// 5. Verify formatUpstreamHTTPError extracts JSON message & HTML <title> cleanly
	jsonErr := formatUpstreamHTTPError("gpt-4o", 429, `{"error":{"message":"Rate limit exceeded"}}`)
	if !strings.Contains(jsonErr.Error(), "Rate limit exceeded") {
		t.Fatalf("expected JSON error extraction, got: %v", jsonErr)
	}
	htmlErr := formatUpstreamHTTPError("deepseek-chat", 502, `<!DOCTYPE html><html><head><title>502 Bad Gateway</title></head><body><h1>cloudflare</h1></body></html>`)
	if !strings.Contains(htmlErr.Error(), "502 Bad Gateway") || strings.Contains(htmlErr.Error(), "<html") {
		t.Fatalf("expected clean HTML title extraction, got: %v", htmlErr)
	}

	// 6. Verify budgetChatMessages preserves system messages and recent turns within budget
	msgs := []apiMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: strings.Repeat("u", 200)},
		{Role: "assistant", Content: strings.Repeat("a", 200)},
		{Role: "user", Content: strings.Repeat("m", 200)},
		{Role: "assistant", Content: strings.Repeat("b", 200)},
		{Role: "user", Content: "latest question"},
	}
	budgeted := budgetChatMessages(msgs, 500)
	if len(budgeted) >= len(msgs) || budgeted[0].Role != "system" || budgeted[len(budgeted)-1].Content != "latest question" {
		t.Fatalf("unexpected budgeted history: %+v", budgeted)
	}

	// 7. Verify distilled memory key/category sanitization
	if k := normalizeDistilledFactKey("User-Preferred Theme!!"); k != "user_preferred_theme" {
		t.Fatalf("unexpected normalized fact key: %q", k)
	}
	if cat := normalizeDistilledCategory("unknown_bucket"); cat != "general" {
		t.Fatalf("unexpected fallback category: %q", cat)
	}

	// 8. Verify skill arg formatting and empty memory filtering in BuildSystemPrompt
	if got := formatSkillArg(nil, true); got != "" {
		t.Fatalf("expected empty string for nil skill arg, got %q", got)
	}
	if got := formatSkillArg("  hello ", true); got != "hello" {
		t.Fatalf("expected trimmed skill arg, got %q", got)
	}
	sysPrompt := BuildSystemPrompt("grok", []store.AgentMemory{
		{Category: "", FactKey: "  ", Content: "   "},
		{Category: "infra", FactKey: "region", Content: "tokyo"},
	})
	if strings.Contains(sysPrompt, "[] :") || !strings.Contains(sysPrompt, "[infra] region: tokyo") {
		t.Fatalf("unexpected BuildSystemPrompt memory output: %s", sysPrompt)
	}

	// 9. Verify SSRF IP guard blocks Tailscale/CGNAT, benchmark, and loopback while allowing public IPs
	for _, blocked := range []string{"127.0.0.1", "10.0.0.5", "100.64.0.1", "100.127.255.254", "198.18.0.1", "0.0.0.0", "::1"} {
		if !isPrivateOrLoopbackIP(net.ParseIP(blocked)) {
			t.Fatalf("expected IP %s to be blocked by SSRF guard", blocked)
		}
	}
	for _, allowed := range []string{"8.8.8.8", "1.1.1.1", "100.128.0.1"} {
		if isPrivateOrLoopbackIP(net.ParseIP(allowed)) {
			t.Fatalf("expected public IP %s to be allowed", allowed)
		}
	}

	// 10. Verify toolManageMemory pin matches normalized fact keys & clamps importance
	if _, err := eng.ExecuteTool(context.Background(), "remember_fact", `{"fact_key":"primary_vps","content":"RackNerd"}`); err != nil {
		t.Fatalf("remember_fact failed: %v", err)
	}
	pinOut, err := eng.ExecuteTool(context.Background(), "manage_memory", `{"action":"pin","fact_key":" Primary VPS ","importance":9,"pinned":true}`)
	if err != nil || !strings.Contains(pinOut, `"fact_key":"primary_vps"`) {
		t.Fatalf("expected manage_memory pin to resolve normalized key, out=%s err=%v", pinOut, err)
	}
}
