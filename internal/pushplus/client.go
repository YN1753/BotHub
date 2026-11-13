package pushplus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/YN1753/vesper/bothub/internal/store"
)

const Endpoint = "http://www.pushplus.plus/send"

type Client struct {
	http  *http.Client
	store *store.Store
}

type sendRequest struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
}

type sendResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func NewClient(st *store.Store) *Client {
	return &Client{
		http:  &http.Client{Timeout: 15 * time.Second},
		store: st,
	}
}

func (c *Client) SendMarkdown(ctx context.Context, moduleID, title, content string) error {
	cleanTitle := StripEmoji(strings.TrimSpace(title))
	cleanContent := StripEmoji(strings.TrimSpace(content))

	token, err := c.store.GetConfig(store.KeyPushPlusToken)
	if err != nil {
		return fmt.Errorf("read pushplus token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", "missing pushplus token")
		return fmt.Errorf("pushplus token is empty")
	}

	payload := sendRequest{
		Token:    token,
		Title:    cleanTitle,
		Content:  cleanContent,
		Template: "markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pushplus payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build pushplus request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", err.Error())
		return fmt.Errorf("pushplus request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	var parsed sendResponse
	_ = json.Unmarshal(respBytes, &parsed)

	respMsg := strings.TrimSpace(parsed.Msg)
	if respMsg == "" {
		respMsg = fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	if resp.StatusCode >= 400 || (parsed.Code != 0 && parsed.Code != 200) {
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", respMsg)
		return fmt.Errorf("pushplus error (code=%d): %s", parsed.Code, respMsg)
	}

	_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "sent", respMsg)
	return nil
}

// StripEmoji removes Unicode emoji/pictograph runes and variation selectors to
// enforce the STRICT NO EMOJI policy across all WeChat Markdown dispatches.
func StripEmoji(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isEmojiRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isEmojiRune(r rune) bool {
	if r == 0xFE0F || r == 0xFE0E || r == 0x200D || r == 0x20E3 {
		return true
	}
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF: // Misc symbols, pictographs, emoticons, transport, supplemental
		return true
	case r >= 0x1F000 && r <= 0x1F0FF: // Mahjong, Domino, Playing Cards
		return true
	case r >= 0x2600 && r <= 0x27BF: // Misc symbols & Dingbats
		return true
	case r >= 0x2300 && r <= 0x23FF: // Misc technical (watches, hourglasses, etc.)
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // Misc symbols and arrows
		return true
	case unicode.In(r, unicode.So) && r > 0x2000:
		return true
	}
	return false
}
