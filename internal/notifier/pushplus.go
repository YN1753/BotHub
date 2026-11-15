package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const PushPlusEndpoint = "http://www.pushplus.plus/send"

type pushplusSendRequest struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
}

type pushplusSendResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func (c *Client) SendPushPlus(ctx context.Context, moduleID, title, content string) error {
	cleanTitle := StripEmoji(strings.TrimSpace(title))
	cleanContent := StripEmoji(strings.TrimSpace(content))

	const maxContentLen = 18000
	if len(cleanContent) > maxContentLen {
		cleanContent = cleanContent[:maxContentLen] + "\n\n... (内容超过 18KB 已自动截断)"
	}

	token, err := c.store.GetConfig("pushplus_token")
	if err != nil {
		return fmt.Errorf("read pushplus token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", "[PushPlus] missing token")
		return fmt.Errorf("pushplus token is empty")
	}

	payload := pushplusSendRequest{
		Token:    token,
		Title:    cleanTitle,
		Content:  cleanContent,
		Template: "markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pushplus payload: %w", err)
	}

	var lastErr error
	var respMsg string
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1500 * time.Millisecond):
			}
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, PushPlusEndpoint, bytes.NewReader(body))
		if reqErr != nil {
			lastErr = fmt.Errorf("build pushplus request: %w", reqErr)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("pushplus request failed: %w", doErr)
			continue
		}

		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()

		var parsed pushplusSendResponse
		_ = json.Unmarshal(respBytes, &parsed)

		respMsg = strings.TrimSpace(parsed.Msg)
		if respMsg == "" {
			respMsg = fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
		}

		if resp.StatusCode >= 400 || (parsed.Code != 0 && parsed.Code != 200) {
			lastErr = fmt.Errorf("pushplus error (code=%d): %s", parsed.Code, respMsg)
			continue
		}

		// Success
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "sent", "[PushPlus] "+respMsg)
		return nil
	}

	errMsg := "pushplus delivery failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", "[PushPlus] "+errMsg)
	return fmt.Errorf("%s", errMsg)
}
