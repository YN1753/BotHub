package notifier

import (
	"context"
	"strings"

	"github.com/YN1753/vesper/bothub/internal/pushstream"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type Client struct {
	store *store.Store
	hub   *pushstream.Hub
}

func NewClient(st *store.Store) *Client {
	return &Client{
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

// SendMarkdown dispatches an alert exclusively to the native App notification channel
// (persisting to SQLite inbox and broadcasting in real time over the SSE Hub).
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
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "sent", "native_app_stream")
	}
	return nil
}
