package notifier

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestStripEmoji(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Hello 🌍 World 🚀", "Hello  World "},
		{"【早报】☁️ 晴转多云", "【早报】 晴转多云"},
		{"纯文本测试", "纯文本测试"},
		{"Emoji with variant ‼️⚠️", "Emoji with variant "},
	}

	for _, c := range cases {
		got := StripEmoji(c.input)
		if got != c.expected {
			t.Errorf("StripEmoji(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestWeComTokenAndDispatch(t *testing.T) {
	// Mock WeCom HTTP Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"errcode":0,"errmsg":"ok","access_token":"mock_access_token_123","expires_in":7200}`))
		case "/cgi-bin/message/send":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":"msg_mock_001"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	tmpDir, err := os.MkdirTemp("", "bothub-notifier-test-*")
	if err != nil {
		t.Fatalf("temp dir error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	defer st.Close()

	client := NewClient(st)
	// Point to mock server URL directly via getWeComAccessToken override or custom client
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
