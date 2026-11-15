package notifier

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/pushstream"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type Client struct {
	http  *http.Client
	store *store.Store
	hub   *pushstream.Hub

	wecomMu        sync.Mutex
	wecomToken     string
	wecomExpiresAt time.Time
}

func NewClient(st *store.Store) *Client {
	return &Client{
		http:  &http.Client{Timeout: 15 * time.Second},
		store: st,
		hub:   pushstream.NewHub(),
	}
}

func (c *Client) Hub() *pushstream.Hub {
	return c.hub
}

func inferPriority(moduleID string) string {
	switch strings.ToLower(strings.TrimSpace(moduleID)) {
	case "nezha_webhook", "asset_watcher", "asset_warn", "ssl_check", "http_ping":
		return "high"
	default:
		return "normal"
	}
}

// SendMarkdown dispatches a markdown alert:
// 1. Always persists to the native SQLite `notifications` inbox and broadcasts over the SSE Hub.
// 2. Routes to WeCom, PushPlus, or native-only depending on `push_channel`.
func (c *Client) SendMarkdown(ctx context.Context, moduleID, title, content string) error {
	return c.SendWithPriority(ctx, moduleID, title, content, inferPriority(moduleID))
}

func (c *Client) SendWithPriority(ctx context.Context, moduleID, title, content, priority string) error {
	cleanTitle := StripEmoji(strings.TrimSpace(title))
	cleanContent := StripEmoji(strings.TrimSpace(content))

	if c.store != nil {
		if notif, err := c.store.CreateNotification(moduleID, cleanTitle, "", cleanContent, priority); err == nil && c.hub != nil {
			c.hub.Publish(notif)
		}
	}

	channel := "wecom"
	if c.store != nil {
		if ch, err := c.store.GetConfig(store.KeyPushChannel); err == nil && strings.TrimSpace(ch) != "" {
			channel = strings.ToLower(strings.TrimSpace(ch))
		}
	}

	if channel == "native" {
		if c.store != nil {
			_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "sent", "native_inbox_stream")
		}
		return nil
	}

	if channel == "pushplus" {
		return c.SendPushPlus(ctx, moduleID, cleanTitle, cleanContent)
	}

	// Default to WeCom (with native inbox already recorded above)
	err := c.SendWeCom(ctx, moduleID, cleanTitle, cleanContent)
	if err != nil && c.store != nil {
		// If WeCom fails and a PushPlus token is available, attempt fallback
		if ppToken, _ := c.store.GetConfig(store.KeyPushPlusToken); strings.TrimSpace(ppToken) != "" {
			_ = c.SendPushPlus(ctx, moduleID, cleanTitle+" (WeCom降级通道)", cleanContent)
		}
	}
	return err
}
