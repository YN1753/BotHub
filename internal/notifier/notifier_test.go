package notifier

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestNativeAppDispatch(t *testing.T) {
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
	ch, unsub := client.Hub().Subscribe()
	defer unsub()

	if err := client.SendMarkdown(context.Background(), "asset_watcher", "节点告警 🚀", "测试通知内容"); err != nil {
		t.Fatalf("SendMarkdown failed: %v", err)
	}

	select {
	case notif := <-ch:
		if notif.Title != "节点告警" {
			t.Errorf("expected cleaned title, got %q", notif.Title)
		}
		if notif.Priority != "high" {
			t.Errorf("expected high priority for asset_watcher, got %q", notif.Priority)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE hub notification")
	}

	logs, err := st.ListPushLogs(10)
	if err != nil {
		t.Fatalf("ListPushLogs: %v", err)
	}
	if len(logs) == 0 || logs[0].ResponseMsg != "native_app_stream" {
		t.Fatalf("expected native_app_stream push log, got %+v", logs)
	}
}
