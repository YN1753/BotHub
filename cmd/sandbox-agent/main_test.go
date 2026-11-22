package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSandboxAgentAuthAndPathConfinement(t *testing.T) {
	ws := t.TempDir()
	cfg := config{
		token:      "test-secret-token",
		port:       "9099",
		workspace:  ws,
		allowedIPs: map[string]struct{}{"127.0.0.1": {}},
	}
	h := newHandler(cfg)

	// 1. Reject disallowed IP
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("Authorization", "Bearer test-secret-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed IP, got %d", rr.Code)
	}

	// 2. Reject invalid bearer token
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Authorization", "Bearer wrong")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", rr.Code)
	}

	// 3. Reject path traversal
	if _, err := cfg.safePath("../../etc/passwd"); err == nil {
		t.Fatalf("expected path traversal error")
	}

	// 4. Write, read, list, and shell execution inside workspace
	callTool := func(name string, args map[string]any) (string, bool) {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params":  map[string]any{"name": name, "arguments": args},
		})
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("Authorization", "Bearer test-secret-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("tool %s expected 200, got %d", name, w.Code)
		}
		var resp struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode tool %s response: %v", name, err)
		}
		return resp.Result.Content[0].Text, resp.Result.IsError
	}

	out, isErr := callTool("write_file", map[string]any{"path": "sub/hello.txt", "content": "vesper-go"})
	if isErr || !strings.Contains(out, "9 bytes") {
		t.Fatalf("write_file failed: %q err=%v", out, isErr)
	}
	out, isErr = callTool("read_file", map[string]any{"path": "sub/hello.txt"})
	if isErr || out != "vesper-go" {
		t.Fatalf("read_file mismatch: %q err=%v", out, isErr)
	}
	out, isErr = callTool("run_shell", map[string]any{"command": "cat sub/hello.txt"})
	if isErr || !strings.Contains(out, "vesper-go") {
		t.Fatalf("run_shell failed: %q err=%v", out, isErr)
	}
}
