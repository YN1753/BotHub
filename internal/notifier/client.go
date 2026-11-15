package notifier

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

type Client struct {
	http  *http.Client
	store *store.Store

	wecomMu        sync.Mutex
	wecomToken     string
	wecomExpiresAt time.Time
}

func NewClient(st *store.Store) *Client {
	return &Client{
		http:  &http.Client{Timeout: 15 * time.Second},
		store: st,
	}
}

// SendMarkdown dispatches a markdown alert using the currently configured push channel.
// By default, it routes to WeCom (企业微信应用直达). If channel is "pushplus", it routes to PushPlus.
func (c *Client) SendMarkdown(ctx context.Context, moduleID, title, content string) error {
	channel, _ := c.store.GetConfig(store.KeyPushChannel)
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = "wecom"
	}

	if channel == "pushplus" {
		return c.SendPushPlus(ctx, moduleID, title, content)
	}

	// Default to WeCom
	err := c.SendWeCom(ctx, moduleID, title, content)
	if err != nil {
		// If WeCom fails and a PushPlus token is available, attempt fallback
		if ppToken, _ := c.store.GetConfig(store.KeyPushPlusToken); strings.TrimSpace(ppToken) != "" {
			_ = c.SendPushPlus(ctx, moduleID, title+" (WeCom降级通道)", content)
		}
	}
	return err
}
