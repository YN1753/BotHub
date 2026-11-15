package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type wecomTokenResp struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type wecomMarkdownPayload struct {
	ToUser  string `json:"touser"`
	MsgType string `json:"msgtype"`
	AgentID int    `json:"agentid"`
	Markdown struct {
		Content string `json:"content"`
	} `json:"markdown"`
	EnableDuplicateCheck   int `json:"enable_duplicate_check"`
	DuplicateCheckInterval int `json:"duplicate_check_interval"`
}

type wecomSendResp struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   string `json:"msgid"`
}

func (c *Client) getWeComAccessToken(ctx context.Context, corpID, secret string) (string, error) {
	c.wecomMu.Lock()
	defer c.wecomMu.Unlock()

	// Reuse valid cached token (reserve 5 minutes leeway)
	if c.wecomToken != "" && time.Now().Before(c.wecomExpiresAt.Add(-5*time.Minute)) {
		return c.wecomToken, nil
	}

	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		url.QueryEscape(corpID), url.QueryEscape(secret))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	var res wecomTokenResp
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("parse token response (%s): %w", string(body), err)
	}

	if res.ErrCode != 0 {
		return "", fmt.Errorf("wecom token error (code=%d): %s", res.ErrCode, res.ErrMsg)
	}
	if res.AccessToken == "" {
		return "", fmt.Errorf("empty access_token returned by wecom")
	}

	c.wecomToken = res.AccessToken
	expiresIn := res.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	c.wecomExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return c.wecomToken, nil
}

func (c *Client) clearWeComToken() {
	c.wecomMu.Lock()
	defer c.wecomMu.Unlock()
	c.wecomToken = ""
	c.wecomExpiresAt = time.Time{}
}

func (c *Client) SendWeCom(ctx context.Context, moduleID, title, content string) error {
	cleanTitle := StripEmoji(strings.TrimSpace(title))
	cleanContent := StripEmoji(strings.TrimSpace(content))

	// Guard against WeCom 4096-byte markdown payload limit
	const maxContentLen = 3800
	if len(cleanContent) > maxContentLen {
		cleanContent = cleanContent[:maxContentLen] + "\n\n... (内容过长已自动截断)"
	}

	// Format title as header if not already formatted
	if cleanTitle != "" && !strings.HasPrefix(cleanContent, "#") {
		cleanContent = "### " + cleanTitle + "\n\n" + cleanContent
	}

	corpID, _ := c.store.GetConfig("wecom_corpid")
	agentIDStr, _ := c.store.GetConfig("wecom_agentid")
	secret, _ := c.store.GetConfig("wecom_secret")
	toUser, _ := c.store.GetConfig("wecom_touser")

	corpID = strings.TrimSpace(corpID)
	agentIDStr = strings.TrimSpace(agentIDStr)
	secret = strings.TrimSpace(secret)
	toUser = strings.TrimSpace(toUser)

	if corpID == "" || agentIDStr == "" || secret == "" {
		errMsg := "wecom credentials missing (corp_id, agent_id or secret empty)"
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	agentID, err := strconv.Atoi(agentIDStr)
	if err != nil {
		errMsg := fmt.Sprintf("invalid wecom_agentid %q: must be integer", agentIDStr)
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	if toUser == "" {
		toUser = "@all"
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		token, tokenErr := c.getWeComAccessToken(ctx, corpID, secret)
		if tokenErr != nil {
			lastErr = fmt.Errorf("get access token: %w", tokenErr)
			break
		}

		payload := wecomMarkdownPayload{
			ToUser:  toUser,
			MsgType: "markdown",
			AgentID: agentID,
		}
		payload.Markdown.Content = cleanContent
		payload.EnableDuplicateCheck = 0
		payload.DuplicateCheckInterval = 1800

		reqBody, err := json.Marshal(payload)
		if err != nil {
			lastErr = fmt.Errorf("marshal payload: %w", err)
			break
		}

		sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", url.QueryEscape(token))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(reqBody))
		if err != nil {
			lastErr = fmt.Errorf("build request: %w", err)
			break
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("post message: %w", err)
			time.Sleep(1 * time.Second)
			continue
		}

		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()

		var sendRes wecomSendResp
		_ = json.Unmarshal(bodyBytes, &sendRes)

		// Token expired or invalid -> clear cache and retry once
		if sendRes.ErrCode == 40014 || sendRes.ErrCode == 42001 {
			c.clearWeComToken()
			lastErr = fmt.Errorf("wecom token expired: %s", sendRes.ErrMsg)
			continue
		}

		if sendRes.ErrCode != 0 {
			lastErr = fmt.Errorf("wecom api error (code=%d): %s", sendRes.ErrCode, sendRes.ErrMsg)
			break
		}

		// Success
		_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "sent", "[企业微信] msgid="+sendRes.MsgID)
		return nil
	}

	errMsg := "wecom delivery failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	_ = c.store.AddPushLog(moduleID, cleanTitle, cleanContent, "failed", "[企业微信] "+errMsg)
	return fmt.Errorf("%s", errMsg)
}
