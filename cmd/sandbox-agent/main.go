package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxOutputChars = 16000
	defaultTimeout = 30
	maxTimeout     = 120
	maxBodyBytes   = 2 << 20
)

type config struct {
	token      string
	port       string
	workspace  string
	allowedIPs map[string]struct{}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

var toolDefinitions = []map[string]any{
	{
		"name":        "run_shell",
		"description": "Execute a shell command inside the isolated non-root remote VM sandbox workspace.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Bash command to run"},
				"timeout": map[string]any{"type": "integer", "description": "Timeout in seconds (1-120, default 30)"},
			},
			"required": []string{"command"},
		},
	},
	{
		"name":        "read_file",
		"description": "Read a text file inside the sandbox workspace (relative path only).",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative file path inside workspace"},
			},
			"required": []string{"path"},
		},
	},
	{
		"name":        "write_file",
		"description": "Create or overwrite a text file inside the sandbox workspace (relative path only).",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Relative file path inside workspace"},
				"content": map[string]any{"type": "string", "description": "File content"},
			},
			"required": []string{"path", "content"},
		},
	},
	{
		"name":        "list_files",
		"description": "List files and subdirectories inside the sandbox workspace.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Optional relative subdirectory"},
			},
		},
	},
	{
		"name":        "vm_info",
		"description": "Return hostname, OS/arch, CPU cores, memory, and disk availability of this remote execution node.",
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

func loadConfig() config {
	ws := strings.TrimSpace(os.Getenv("SANDBOX_WORKSPACE"))
	if ws == "" {
		ws = "/home/vesper-sandbox/workspace"
	}
	absWS, err := filepath.Abs(ws)
	if err != nil {
		log.Fatalf("resolve workspace: %v", err)
	}
	if err := os.MkdirAll(absWS, 0o755); err != nil {
		log.Fatalf("mkdir workspace: %v", err)
	}

	port := strings.TrimSpace(os.Getenv("SANDBOX_PORT"))
	if port == "" {
		port = "9099"
	}

	allowed := map[string]struct{}{}
	for _, raw := range strings.Split(os.Getenv("SANDBOX_ALLOWED_IPS"), ",") {
		ip := strings.TrimSpace(raw)
		if ip != "" {
			allowed[ip] = struct{}{}
		}
	}

	return config{
		token:      strings.TrimSpace(os.Getenv("SANDBOX_TOKEN")),
		port:       port,
		workspace:  absWS,
		allowedIPs: allowed,
	}
}

func (c config) safePath(rel string) (string, error) {
	cleaned := strings.TrimLeft(strings.TrimSpace(rel), "/")
	target := filepath.Clean(filepath.Join(c.workspace, cleaned))
	if target != c.workspace && !strings.HasPrefix(target, c.workspace+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes sandbox workspace")
	}
	return target, nil
}

func clampTimeout(raw any) time.Duration {
	sec := defaultTimeout
	switch v := raw.(type) {
	case float64:
		sec = int(v)
	case int:
		sec = v
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			sec = parsed
		}
	}
	if sec < 1 {
		sec = 1
	}
	if sec > maxTimeout {
		sec = maxTimeout
	}
	return time.Duration(sec) * time.Second
}

func trimOutput(s string) string {
	r := []rune(s)
	if len(r) <= maxOutputChars {
		return s
	}
	return string(r[:maxOutputChars]) + fmt.Sprintf("\n... [truncated %d chars]", len(r)-maxOutputChars)
}

func (c config) runTool(ctx context.Context, name string, args map[string]any) (string, bool) {
	switch name {
	case "run_shell":
		cmdStr, _ := args["command"].(string)
		cmdStr = strings.TrimSpace(cmdStr)
		if cmdStr == "" {
			return "command is required", true
		}
		timeout := clampTimeout(args["timeout"])
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(runCtx, "/bin/bash", "-lc", cmdStr)
		cmd.Dir = c.workspace
		cmd.Env = []string{
			"PATH=/usr/local/bin:/usr/bin:/bin",
			"HOME=" + c.workspace,
			"LANG=C.UTF-8",
		}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()

		if runCtx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("command timed out after %s", timeout), true
		}
		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				return fmt.Sprintf("exec error: %v", err), true
			}
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("exit_code: %d\n", exitCode))
		if stdout.Len() > 0 {
			sb.WriteString("--- stdout ---\n")
			sb.WriteString(stdout.String())
			sb.WriteString("\n")
		}
		if stderr.Len() > 0 {
			sb.WriteString("--- stderr ---\n")
			sb.WriteString(stderr.String())
			sb.WriteString("\n")
		}
		return trimOutput(strings.TrimSpace(sb.String())), exitCode != 0

	case "read_file":
		p, _ := args["path"].(string)
		target, err := c.safePath(p)
		if err != nil {
			return err.Error(), true
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return err.Error(), true
		}
		return trimOutput(string(data)), false

	case "write_file":
		p, _ := args["path"].(string)
		content, _ := args["content"].(string)
		target, err := c.safePath(p)
		if err != nil {
			return err.Error(), true
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err.Error(), true
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return err.Error(), true
		}
		rel, _ := filepath.Rel(c.workspace, target)
		return fmt.Sprintf("wrote %d bytes to %s", len(content), rel), false

	case "list_files":
		p, _ := args["path"].(string)
		target, err := c.safePath(p)
		if err != nil {
			return err.Error(), true
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			return err.Error(), true
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		if len(entries) == 0 {
			return "(empty workspace)", false
		}
		var sb strings.Builder
		for _, e := range entries {
			kind := "file"
			var size int64
			if e.IsDir() {
				kind = "dir"
			} else if info, err := e.Info(); err == nil {
				size = info.Size()
			}
			sb.WriteString(fmt.Sprintf("%-4s  %10d  %s\n", kind, size, e.Name()))
		}
		return strings.TrimSpace(sb.String()), false

	case "vm_info":
		host, _ := os.Hostname()
		out, _ := exec.CommandContext(ctx, "/bin/bash", "-lc", "uname -srm; nproc; free -h 2>/dev/null | awk 'NR==2{print $2\" total / \"$7\" avail\"}'; df -h . | awk 'NR==2{print $2\" total / \"$4\" avail\"}'").Output()
		return fmt.Sprintf("hostname: %s\nuid: %d\nworkspace: %s\n%s", host, os.Getuid(), c.workspace, strings.TrimSpace(string(out))), false

	default:
		return fmt.Sprintf("unknown tool: %s", name), true
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (c config) authorized(r *http.Request) (bool, int, string) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(c.allowedIPs) > 0 {
		if _, ok := c.allowedIPs[host]; !ok {
			return false, http.StatusForbidden, "forbidden client ip"
		}
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if c.token == "" || !strings.HasPrefix(auth, "Bearer ") {
		return false, http.StatusUnauthorized, "unauthorized"
	}
	provided := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if subtle.ConstantTimeCompare([]byte(provided), []byte(c.token)) != 1 {
		return false, http.StatusUnauthorized, "unauthorized"
	}
	return true, 0, ""
}

func newHandler(cfg config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "vesper-sandbox-agent",
			"runtime": "go",
		})
	})

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if ok, code, msg := cfg.authorized(r); !ok {
			writeJSON(w, code, map[string]string{"error": msg})
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body failed"})
			return
		}
		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"jsonrpc": "2.0",
				"id":      nil,
				"error":   map[string]any{"code": -32700, "message": err.Error()},
			})
			return
		}

		switch req.Method {
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		case "initialize":
			writeJSON(w, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]string{"name": "vesper-sandbox-agent", "version": "1.0.0"},
				},
			})
			return
		case "tools/list":
			writeJSON(w, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]any{"tools": toolDefinitions},
			})
			return
		case "tools/call":
			var p toolCallParams
			if err := json.Unmarshal(req.Params, &p); err != nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"error":   map[string]any{"code": -32602, "message": err.Error()},
				})
				return
			}
			text, isErr := cfg.runTool(r.Context(), p.Name, p.Arguments)
			writeJSON(w, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"content": []map[string]string{{"type": "text", "text": text}},
					"isError": isErr,
				},
			})
			return
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error":   map[string]any{"code": -32601, "message": "method not found: " + req.Method},
			})
		}
	})

	return mux
}

func main() {
	cfg := loadConfig()
	addr := "0.0.0.0:" + cfg.port
	log.Printf("[vesper-sandbox-agent] listening on %s workspace=%s", addr, cfg.workspace)
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
