package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var (
	xaiResponsesEndpoint  = "https://api.x.ai/v1/responses"
	defaultXAISearchModel = "grok-4.7"
	reValidXHandle        = regexp.MustCompile(`^[A-Za-z0-9_]{1,30}$`)
)

func parseXHandles(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '，' || r == ';' || r == '；' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, min(len(parts), 20))
	for _, p := range parts {
		h := strings.TrimSpace(p)
		for _, prefix := range []string{
			"https://x.com/", "http://x.com/", "https://www.x.com/", "x.com/", "www.x.com/",
			"https://twitter.com/", "http://twitter.com/", "https://www.twitter.com/", "twitter.com/", "www.twitter.com/",
		} {
			if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
				h = h[len(prefix):]
				break
			}
		}
		if idx := strings.IndexAny(h, "?#/"); idx >= 0 {
			h = h[:idx]
		}
		h = strings.TrimPrefix(strings.TrimSpace(h), "@")
		if !reValidXHandle.MatchString(h) {
			continue
		}
		low := strings.ToLower(h)
		if seen[low] {
			continue
		}
		seen[low] = true
		out = append(out, h)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

func (e *Engine) resolveXAICredentials() (endpoint, apiKey, model string) {
	endpoint = xaiResponsesEndpoint
	model = defaultXAISearchModel
	if e == nil || e.store == nil {
		return endpoint, "", model
	}

	if k, _ := e.store.GetConfig(store.KeyXAIAPIKey); strings.TrimSpace(k) != "" {
		apiKey = strings.TrimSpace(k)
	}
	if m, _ := e.store.GetConfig(store.KeyXAISearchModel); strings.TrimSpace(m) != "" {
		model = strings.TrimSpace(m)
	}

	providers, _, _ := e.store.GetModelConfig()
	for _, p := range providers {
		if !p.Enabled {
			continue
		}
		baseLower := strings.ToLower(strings.TrimSpace(p.BaseURL))
		if !strings.Contains(baseLower, "api.x.ai") {
			continue
		}
		if apiKey == "" {
			if keys := p.EffectiveAPIKeys(); len(keys) > 0 {
				apiKey = keys[0]
			}
		}
		if endpoint == "https://api.x.ai/v1/responses" {
			trimmedBase := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
			if strings.HasSuffix(trimmedBase, "/v1") {
				endpoint = trimmedBase + "/responses"
			} else if trimmedBase != "" {
				endpoint = trimmedBase + "/v1/responses"
			}
		}
		if apiKey != "" {
			break
		}
	}
	return endpoint, apiKey, model
}

func (e *Engine) hasXAICredentials() bool {
	_, apiKey, _ := e.resolveXAICredentials()
	return strings.TrimSpace(apiKey) != ""
}

type xSearchResult struct {
	OK        bool     `json:"ok"`
	Model     string   `json:"model"`
	FromDate  string   `json:"from_date"`
	ToDate    string   `json:"to_date"`
	Handles   []string `json:"handles,omitempty"`
	Summary   string   `json:"summary"`
	Citations []string `json:"citations,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func (e *Engine) callXAIXSearch(ctx context.Context, query string, handles []string, days int, focus string) (*xSearchResult, error) {
	endpoint, apiKey, model := e.resolveXAICredentials()
	if apiKey == "" {
		return nil, fmt.Errorf("未配置 xAI API Key")
	}
	if days <= 0 {
		days = 2
	} else if days > 14 {
		days = 14
	}
	nowUTC := time.Now().UTC()
	toDate := nowUTC.Format("2006-01-02")
	fromDate := nowUTC.AddDate(0, 0, -days).Format("2006-01-02")

	cleanHandles := make([]string, 0, min(len(handles), 20))
	for _, h := range handles {
		parsed := parseXHandles(h)
		for _, ph := range parsed {
			if len(cleanHandles) < 20 {
				cleanHandles = append(cleanHandles, ph)
			}
		}
	}

	xTool := map[string]any{
		"type":      "x_search",
		"from_date": fromDate,
		"to_date":   toDate,
	}
	if len(cleanHandles) > 0 {
		xTool["allowed_x_handles"] = cleanHandles
	}

	var sysPrompt string
	switch strings.ToLower(strings.TrimSpace(focus)) {
	case "deals":
		sysPrompt = "你是 X (Twitter) 实时羊毛情报挖掘员。仅提取时间窗内 X 上刚发布的新限时羊毛、黑五活动、新免费口子或云/AI 平台突发限时免费玩法。每条列出作者 @handle、发布日期、核心亮点、领取条件/截止时间与原帖链接。严格剔除常驻广为人知的基础福利（如 Google One/Pro 常规 10 刀赠金、GCP 新户 300 刀、AWS 常规 12 月免费、Oracle 永久免费、GitHub 学生包、Azure 新户 200 刀、常规注册体验金）。若无符合条件的新帖请直接回复“暂无符合条件的新帖”。严禁使用 Emoji。"
	case "tech":
		sysPrompt = "你是 X (Twitter) 一线技术与开源情报挖掘员。仅提取时间窗内 X 上高价值的 AI、Go、开源新项目、工程实战或开发者生态真实新动态。每条列出作者 @handle、发布日期、具体亮点与原帖/项目链接。禁止空泛鸡汤与旧闻，严禁使用 Emoji。"
	default:
		sysPrompt = "你是 X (Twitter) 实时情报助手。提取时间窗内 X 上最相关的一手真实推文，列出作者 @handle、发布日期、核心事实与原帖链接，严禁使用 Emoji。"
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model": model,
		"input": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": strings.TrimSpace(query)},
		},
		"tools": []map[string]any{xTool},
	})

	reqCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := e.apiHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errText := strings.TrimSpace(string(rawBytes))
		if len([]rune(errText)) > 240 {
			errText = string([]rune(errText)[:240]) + "..."
		}
		return nil, fmt.Errorf("xAI x_search HTTP %d: %s", resp.StatusCode, errText)
	}

	var parsed struct {
		OutputText string   `json:"output_text"`
		Citations  []string `json:"citations"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Annotations []struct {
					Type  string `json:"type"`
					URL   string `json:"url"`
					Title string `json:"title"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(rawBytes, &parsed); err != nil {
		return nil, fmt.Errorf("解析 xAI 响应失败: %w", err)
	}

	var textParts []string
	seenCite := make(map[string]bool)
	var citations []string
	addCite := func(raw string) {
		u := strings.TrimSpace(raw)
		if u == "" || seenCite[u] {
			return
		}
		seenCite[u] = true
		citations = append(citations, u)
	}

	for _, c := range parsed.Citations {
		addCite(c)
	}
	for _, outItem := range parsed.Output {
		for _, cnt := range outItem.Content {
			if txt := strings.TrimSpace(cnt.Text); txt != "" {
				textParts = append(textParts, txt)
			}
			for _, ann := range cnt.Annotations {
				if ann.URL != "" {
					addCite(ann.URL)
				}
			}
		}
	}
	summary := strings.TrimSpace(strings.Join(textParts, "\n\n"))
	if summary == "" {
		summary = strings.TrimSpace(parsed.OutputText)
	}
	summary = notifier.StripEmoji(summary)

	return &xSearchResult{
		OK:        true,
		Model:     model,
		FromDate:  fromDate,
		ToDate:    toDate,
		Handles:   cleanHandles,
		Summary:   summary,
		Citations: citations,
	}, nil
}

func xSearchToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "x_search",
				Description: "通过 xAI Grok 原生实时检索 X (Twitter) 最新原帖与指定博主动态，支持按关注账号列表(handles)和近几天(days)筛选。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "检索主题或关键词（建议中英文结合）",
						},
						"handles": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "限定只看指定 X/Twitter 博主账号（不带 @，最多 20 个）；留空则全网检索 X",
						},
						"days": map[string]any{
							"type":        "integer",
							"description": "检索最近几天内的推文（默认 2，范围 1~14）",
						},
						"focus": map[string]any{
							"type":        "string",
							"enum":        []string{"general", "deals", "tech"},
							"description": "聚焦方向：deals(限时羊毛/黑五/突发免费)、tech(AI/Go/开源/技术)、general",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		Timeout:        55 * time.Second,
		ReadOnly:       true,
		StatusLabel:    "正在检索 X (Twitter) 实时动态...",
		DomainKeywords: []string{"x", "twitter", "推特", "博主", "羊毛", "黑五", "白嫖", "优惠", "ai", "go", "开源", "热帖", "动态"},
		ErrorHint:      "X (Twitter) 检索失败，可先检查 xAI API Key 或改用 web_search 补充检索",
		RetryErrorHint: "X (Twitter) 检索超时，请缩小查询词或改用 web_search",
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolXSearch(ctx, cleanArgs)
		},
	}
}

func (e *Engine) toolXSearch(ctx context.Context, argsJSON string) (string, error) {
	var rawMap map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &rawMap); err != nil {
		return "", fmt.Errorf("解析 x_search 参数失败: %w", err)
	}
	query, _ := rawMap["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("query 不能为空")
	}

	var handles []string
	switch hv := rawMap["handles"].(type) {
	case string:
		handles = parseXHandles(hv)
	case []any:
		for _, item := range hv {
			if s, ok := item.(string); ok {
				handles = append(handles, parseXHandles(s)...)
			}
		}
	}
	if len(handles) == 0 {
		handles = store.TaskXHandlesFromContext(ctx)
	}
	if len(handles) == 0 && e.store != nil {
		if defRaw, _ := e.store.GetConfig(store.KeyDefaultXHandles); strings.TrimSpace(defRaw) != "" {
			handles = parseXHandles(defRaw)
		}
	}

	days := 2
	switch dv := rawMap["days"].(type) {
	case float64:
		if int(dv) > 0 {
			days = int(dv)
		}
	case int:
		if dv > 0 {
			days = dv
		}
	}

	focus, _ := rawMap["focus"].(string)
	if strings.TrimSpace(focus) == "" {
		topic := detectSearchTopic(query)
		if topic == "deals" || topic == "tech" {
			focus = topic
		} else {
			focus = "general"
		}
	}

	if !e.hasXAICredentials() {
		b, _ := json.Marshal(xSearchResult{
			OK:    false,
			Error: "未配置 xAI API Key，无法直连 X 实时检索，请改用 web_search 或在系统设置中填写 xAI API Key",
		})
		return string(b), nil
	}

	res, err := e.callXAIXSearch(ctx, query, handles, days, focus)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}
