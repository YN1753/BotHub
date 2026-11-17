package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
	_ "modernc.org/sqlite"
)

type toolScopeCtxKey struct{}

// ToolScope defines isolated execution/memory boundaries for the current Bot or Group turn.
type ToolScope struct {
	BotID         string
	TimelineBotID string
	ThreadID      string
	MemoryPolicy  string
}

func WithToolScope(ctx context.Context, sc ToolScope) context.Context {
	return context.WithValue(ctx, toolScopeCtxKey{}, sc)
}

func toolScopeFromContext(ctx context.Context) ToolScope {
	if ctx == nil {
		return ToolScope{}
	}
	if v, ok := ctx.Value(toolScopeCtxKey{}).(ToolScope); ok {
		return v
	}
	return ToolScope{}
}

// TaskController abstracts Scheduler operations for tool execution.
type TaskController interface {
	Status() ([]scheduler.TaskStatus, error)
	RunNow(ctx context.Context, taskID string) (modules.RunResult, error)
	Reload() error
}

// ToolDefinition matches the OpenAI / DeepSeek function calling schema.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type AvailableToolMeta struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

func AvailableToolsPayload() []AvailableToolMeta {
	defs := BuiltinTools()
	out := make([]AvailableToolMeta, 0, len(defs))
	for _, d := range defs {
		out = append(out, AvailableToolMeta{
			ID:          d.Function.Name,
			Description: d.Function.Description,
		})
	}
	return out
}

// BuiltinTools returns all OpenAI-compatible tool schemas exposed to DeepSeek.
func BuiltinTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "read_url",
				Description: "抓取指定网页 URL 的正文内容并转换为纯文本摘要，用于阅读文章、文档或监控页面。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"url": map[string]any{
							"type":        "string",
							"description": "要抓取的完整 HTTP/HTTPS 网页地址",
						},
					},
					"required": []string{"url"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "web_search",
				Description: "通过互联网搜索关键词获取最新资讯、技术方案或公开信息。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回结果条数（默认 5，最大 10）",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_nezha_nodes",
				Description: "查询哪吒探针（Nezha）服务器节点的实时在线状态、系统信息、最近心跳与本地算力资产台账。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{
							"type":        "string",
							"description": "可选：按节点名称或备注过滤关键词",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_assets",
				Description: "查询或管理 BotHub 资产台账（VPS、域名、订阅、保号卡）。支持 list（列表）、expiring（查询即将到期）、upsert（新增或修改资产）、delete（删除资产）、sync（立即执行资产外部同步任务）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "expiring", "upsert", "delete", "sync"},
							"description": "操作类型：list | expiring | upsert | delete | sync",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "可选：按分类过滤或设置分类（例如 compute / domain / life / other 或自定义分类）",
						},
						"within_days": map[string]any{
							"type":        "integer",
							"description": "当 action=expiring 时，筛选多少天内到期的资产（默认 30）",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "资产唯一标识 ID（upsert 或 delete 时必填）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "资产名称（upsert 时使用）",
						},
						"spec": map[string]any{
							"type":        "string",
							"description": "规格或区域描述（upsert 时使用）",
						},
						"price": map[string]any{
							"type":        "string",
							"description": "费用与周期描述，如 $15.00 / 年、¥99 / 月（upsert 时使用）",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "生命周期状态：生效中 | 已到期 | 已停用（upsert 时使用）",
						},
						"due_date": map[string]any{
							"type":        "string",
							"description": "到期日期 YYYY-MM-DD（upsert 时使用）",
						},
						"auto_renew": map[string]any{
							"type":        "boolean",
							"description": "是否自动续费（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_tasks",
				Description: "查询或管理 BotHub 自动化调度任务。支持 list（查看任务列表与状态）、run（立即手动触发某任务）、upsert（新增或修改任务）、toggle（启停任务）、delete（删除任务）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "run", "upsert", "toggle", "delete"},
							"description": "操作类型：list | run | upsert | toggle | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "任务 ID（run / upsert / toggle / delete 时必填）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "任务名称（upsert 时使用）",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "已注册的任务执行器类型，当前支持：morning / radar / asset_watcher",
						},
						"cron_expr": map[string]any{
							"type":        "string",
							"description": "标准 5 段 Cron 表达式（upsert 时使用）",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "是否启用任务（upsert 或 toggle 时使用）",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "任务说明（upsert 时使用）",
						},
						"params": map[string]any{
							"type":        "string",
							"description": "JSON 字符串格式的任务参数（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "remember_fact",
				Description: "将与当前会话相关的用户偏好、关系故事、重要约定或核心事实写入隔离的长期记忆库，默认仅绑定当前 Bot/群聊作用域。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类：preference（偏好）/ relation（人际与故事）/ emotion（心情与生活）/ infra（节点与工程）/ schedule（约定）/ insight（洞察）/ general（通用）",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "简短唯一的事实键名（同作用域下同名会覆盖更新），如 user_tea_habit / deploy_rule",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "要记住的具体事实内容",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要程度 1~5（默认 3，核心约定可设为 5）",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶为高优先常驻记忆",
						},
						"scope": map[string]any{
							"type":        "string",
							"description": "可选：'self'（默认，仅当前 Bot/群私有）或 'global'（显式跨全团队共享）",
						},
					},
					"required": []string{"fact_key", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "recall_memory",
				Description: "检索当前 Bot 或群聊专属作用域内的长期记忆（自动隔离其他 Bot 隐私记忆）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "可选：检索关键词（留空则返回当前作用域核心记忆）",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_memory",
				Description: "对当前 Bot 或群聊作用域内的长期记忆进行整理：更正旧事实(upsert)、置顶或取消置顶(pin)、遗忘过时或错误条目(forget)、或列出现有记忆键(list)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "upsert", "pin", "forget"},
							"description": "操作类型：list | upsert | pin | forget",
						},
						"fact_key": map[string]any{
							"type":        "string",
							"description": "目标记忆事实键名（upsert/pin/forget 时必填）",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "更新后的记忆内容（upsert 时必填）",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "记忆分类（preference/relation/emotion/infra/schedule/insight/general）",
						},
						"importance": map[string]any{
							"type":        "integer",
							"description": "重要度权重 1~5",
						},
						"pinned": map[string]any{
							"type":        "boolean",
							"description": "是否置顶",
						},
					},
					"required": []string{"action"},
				},
			},
		},
	}
}

var reTrailingJSONComma = regexp.MustCompile(`,\s*([}\]])`)

func normalizeToolArgsJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	s = reTrailingJSONComma.ReplaceAllString(s, "$1")
	if s == "" {
		return "{}"
	}
	return s
}

const maxToolOutputRunes = 12000

// ExecuteTool dispatches built-in tools and user-defined custom webhook skills.
func (e *Engine) ExecuteTool(ctx context.Context, name string, argsJSON string) (string, error) {
	toolCtx, cancel := context.WithTimeout(ctx, 28*time.Second)
	defer cancel()

	cleanArgs := normalizeToolArgsJSON(argsJSON)

	var out string
	var err error
	if strings.HasPrefix(name, "skill_") {
		slug := strings.TrimPrefix(name, "skill_")
		sk, gErr := e.store.GetSkillBySlug(slug)
		if gErr != nil || sk == nil || !sk.Enabled || sk.Kind != "webhook" {
			return "", fmt.Errorf("未找到或未启用自定义技能: %s", name)
		}
		var args map[string]any
		if cleanArgs != "{}" {
			if uErr := json.Unmarshal([]byte(cleanArgs), &args); uErr != nil {
				return "", fmt.Errorf("解析技能参数 JSON 失败: %w", uErr)
			}
		}
		statusCode, bodyText, execErr := e.ExecuteWebhookSkill(toolCtx, *sk, args)
		if execErr != nil {
			return "", execErr
		}
		b, _ := json.Marshal(map[string]any{
			"ok":          true,
			"skill":       sk.Name,
			"status_code": statusCode,
			"result":      bodyText,
		})
		out = string(b)
	} else {
		switch name {
		case "read_url":
			out, err = e.toolReadURL(toolCtx, cleanArgs)
		case "web_search":
			out, err = e.toolWebSearch(toolCtx, cleanArgs)
		case "query_nezha_nodes":
			out, err = e.toolQueryNezhaNodes(toolCtx, cleanArgs)
		case "manage_assets":
			out, err = e.toolManageAssets(toolCtx, cleanArgs)
		case "manage_tasks":
			out, err = e.toolManageTasks(toolCtx, cleanArgs)
		case "remember_fact":
			out, err = e.toolRememberFact(toolCtx, cleanArgs)
		case "recall_memory":
			out, err = e.toolRecallMemory(toolCtx, cleanArgs)
		case "manage_memory":
			out, err = e.toolManageMemory(toolCtx, cleanArgs)
		default:
			return "", fmt.Errorf("未知工具: %s", name)
		}
	}
	if err != nil {
		return "", err
	}
	runes := []rune(out)
	if len(runes) > maxToolOutputRunes {
		out = string(runes[:maxToolOutputRunes]) + "\n...[工具输出过长已自动裁剪]"
	}
	return out, nil
}

const MaxReadURLChars = 10000

var (
	reHTMLComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	reNoiseBlocks  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|iframe|nav|header|footer|aside|form|button)[^>]*>.*?</(script|style|noscript|svg|iframe|nav|header|footer|aside|form|button)>`)
	reHTMLTags     = regexp.MustCompile(`(?s)<[^>]+>`)
	reMultiSpace   = regexp.MustCompile(`[ \t\r\f]+`)
	reMultiNewline = regexp.MustCompile(`\n{3,}`)
)

func stripHTMLToText(rawHTML string) string {
	cleaned := reHTMLComment.ReplaceAllString(rawHTML, " ")
	cleaned = reNoiseBlocks.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, "</p>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</div>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</li>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</h1>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</h2>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</h3>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br/>", "\n")
	cleaned = reHTMLTags.ReplaceAllString(cleaned, " ")
	cleaned = html.UnescapeString(cleaned)
	cleaned = reMultiSpace.ReplaceAllString(cleaned, " ")
	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	res := strings.Join(out, "\n")
	res = reMultiNewline.ReplaceAllString(res, "\n\n")
	return notifier.StripEmoji(strings.TrimSpace(res))
}

func isPrivateOrLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

func (e *Engine) safeFetchClient() *http.Client {
	if e.allowLocalhostForTest {
		return e.httpClient
	}
	dialer := &net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 15 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("无效目标地址: %w", err)
			}
			ip := net.ParseIP(host)
			if isPrivateOrLoopbackIP(ip) {
				return fmt.Errorf("安全拦截：禁止访问内网或本机回环地址 (%s)", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   8 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("安全拦截：不支持的重定向协议 %s", req.URL.Scheme)
			}
			return nil
		},
	}
}

func (e *Engine) fetchTextWithClient(ctx context.Context, client *http.Client, targetURL, accept string) (int, string, string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return 0, "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VesperBotHub/2.0)")
	if accept == "" {
		accept = "text/html,application/xhtml+xml,application/json,text/plain;q=0.9,*/*;q=0.8"
	}
	req.Header.Set("Accept", accept)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return resp.StatusCode, "", "", err
	}
	return resp.StatusCode, strings.ToLower(resp.Header.Get("Content-Type")), string(bodyBytes), nil
}

func parseGitHubRepo(parsed *url.URL) (owner, repo string, ok bool) {
	if !strings.EqualFold(parsed.Hostname(), "github.com") {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), true
}

func (e *Engine) toolReadURL(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 read_url 参数失败: %w", err)
	}
	target := strings.TrimSpace(args.URL)
	if target == "" {
		return "", fmt.Errorf("url 不能为空")
	}
	parsed, err := url.Parse(target)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("仅支持 http 或 https 协议的 URL")
	}

	maxChars := args.MaxChars
	if maxChars <= 0 || maxChars > MaxReadURLChars {
		maxChars = MaxReadURLChars
	}

	client := e.safeFetchClient()
	var statusCode int
	var text string
	sourceMode := "direct"

	// Fast-path: GitHub repository root URL -> fetch raw README.md directly
	if owner, repo, ok := parseGitHubRepo(parsed); ok && !e.allowLocalhostForTest {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/README.md", owner, repo)
		if sc, _, body, gErr := e.fetchTextWithClient(ctx, client, rawURL, "text/plain"); gErr == nil && sc >= 200 && sc < 300 && strings.TrimSpace(body) != "" {
			statusCode = sc
			text = notifier.StripEmoji(strings.TrimSpace(body))
			sourceMode = "github_raw_readme"
		}
	}

	if text == "" {
		sc, contentType, rawBody, fErr := e.fetchTextWithClient(ctx, client, target, "")
		if fErr != nil {
			return "", fmt.Errorf("请求网页失败: %w", fErr)
		}
		statusCode = sc
		if strings.Contains(contentType, "json") || strings.Contains(contentType, "plain") {
			text = notifier.StripEmoji(strings.TrimSpace(rawBody))
		} else {
			text = stripHTMLToText(rawBody)
		}

		// Fallback: if CSR/SPA page yields very little text, try Jina Reader Markdown extraction
		if len([]rune(text)) < 200 && !e.allowLocalhostForTest && !strings.Contains(strings.ToLower(parsed.Hostname()), "r.jina.ai") {
			jinaURL := "https://r.jina.ai/" + target
			if jsc, _, jBody, jErr := e.fetchTextWithClient(ctx, client, jinaURL, "text/plain"); jErr == nil && jsc >= 200 && jsc < 300 {
				jClean := notifier.StripEmoji(strings.TrimSpace(jBody))
				if len([]rune(jClean)) > len([]rune(text)) {
					text = jClean
					statusCode = jsc
					sourceMode = "jina_reader"
				}
			}
		}
	}

	runes := []rune(text)
	truncated := false
	if len(runes) > maxChars {
		text = string(runes[:maxChars])
		truncated = true
	}

	resultMap := map[string]any{
		"url":         target,
		"status_code": statusCode,
		"source_mode": sourceMode,
		"char_count":  len([]rune(text)),
		"truncated":   truncated,
		"content":     text,
	}

	// If any lightweight/fallback model is available and the article is long (> 3000 chars),
	// pre-distill a summary via CallLightModel to save tokens on the primary reasoning model.
	if e.hasLightTarget() && len([]rune(text)) > 3000 {
		if lightSum, lErr := e.CallLightModel(ctx, "你是正文提炼助手。请将网页正文提炼为高信息密度的核心要点摘要，严禁使用 Emoji。", text); lErr == nil && lightSum != "" {
			resultMap["light_summary"] = lightSum
		}
	}

	payload, _ := json.Marshal(resultMap)
	return string(payload), nil
}

func (e *Engine) toolWebSearch(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 web_search 参数失败: %w", err)
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	limit := args.Limit
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	reqCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	type searchItem struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Snippet string `json:"snippet,omitempty"`
		Source  string `json:"source"`
	}
	results := make([]searchItem, 0, limit)
	seenURLs := make(map[string]bool)
	appendUnique := func(item searchItem) bool {
		u := strings.TrimSpace(item.URL)
		if u == "" || seenURLs[u] || strings.TrimSpace(item.Title) == "" {
			return false
		}
		seenURLs[u] = true
		results = append(results, item)
		return len(results) >= limit
	}

	// 1. DuckDuckGo Instant Answer JSON API (covers general topics, daily queries, docs & entities)
	ddgAPI := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))
	if req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ddgAPI, nil); err == nil {
		req.Header.Set("User-Agent", "VesperBotHub/2.14")
		if resp, err := e.httpClient.Do(req); err == nil {
			defer resp.Body.Close()
			var ddg struct {
				Heading       string `json:"Heading"`
				AbstractText  string `json:"AbstractText"`
				AbstractURL   string `json:"AbstractURL"`
				RelatedTopics []struct {
					Text     string `json:"Text"`
					FirstURL string `json:"FirstURL"`
					Topics   []struct {
						Text     string `json:"Text"`
						FirstURL string `json:"FirstURL"`
					} `json:"Topics"`
				} `json:"RelatedTopics"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&ddg) == nil {
				if ddg.AbstractText != "" && ddg.AbstractURL != "" {
					appendUnique(searchItem{
						Title:   notifier.StripEmoji(ddg.Heading),
						URL:     ddg.AbstractURL,
						Snippet: notifier.StripEmoji(ddg.AbstractText),
						Source:  "DuckDuckGo",
					})
				}
				for _, rt := range ddg.RelatedTopics {
					if len(results) >= limit {
						break
					}
					if rt.Text != "" && rt.FirstURL != "" {
						title := rt.Text
						if idx := strings.Index(title, " - "); idx > 0 {
							title = title[:idx]
						}
						if appendUnique(searchItem{
							Title:   notifier.StripEmoji(truncateTitle(title)),
							URL:     rt.FirstURL,
							Snippet: notifier.StripEmoji(rt.Text),
							Source:  "DuckDuckGo",
						}) {
							break
						}
					}
					for _, sub := range rt.Topics {
						if len(results) >= limit {
							break
						}
						if sub.Text != "" && sub.FirstURL != "" {
							if appendUnique(searchItem{
								Title:   notifier.StripEmoji(truncateTitle(sub.Text)),
								URL:     sub.FirstURL,
								Snippet: notifier.StripEmoji(sub.Text),
								Source:  "DuckDuckGo",
							}) {
								break
							}
						}
					}
				}
			}
		}
	}

	// 2. Hacker News Algolia Search (engineering, open-source, security, startups)
	if len(results) < limit {
		hnURL := fmt.Sprintf("https://hn.algolia.com/api/v1/search?query=%s&hitsPerPage=%d", url.QueryEscape(query), limit)
		if req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, hnURL, nil); err == nil {
			req.Header.Set("User-Agent", "VesperBotHub/2.14")
			if resp, err := e.httpClient.Do(req); err == nil {
				defer resp.Body.Close()
				var hnResp struct {
					Hits []struct {
						Title     string `json:"title"`
						URL       string `json:"url"`
						StoryText string `json:"story_text"`
						ObjectID  string `json:"objectID"`
						Points    int    `json:"points"`
						CreatedAt string `json:"created_at"`
					} `json:"hits"`
				}
				if json.NewDecoder(resp.Body).Decode(&hnResp) == nil {
					for _, h := range hnResp.Hits {
						if h.Title == "" {
							continue
						}
						itemURL := h.URL
						if itemURL == "" {
							itemURL = "https://news.ycombinator.com/item?id=" + h.ObjectID
						}
						snippet := stripHTMLToText(h.StoryText)
						if len([]rune(snippet)) > 200 {
							snippet = string([]rune(snippet)[:200]) + "..."
						}
						if appendUnique(searchItem{
							Title:   notifier.StripEmoji(h.Title),
							URL:     itemURL,
							Snippet: snippet,
							Source:  "HackerNews",
						}) {
							break
						}
					}
				}
			}
		}
	}

	// 3. Wikipedia OpenSearch fallback/supplement for general Chinese & encyclopedic knowledge
	if len(results) < limit {
		wikiURL := fmt.Sprintf("https://zh.wikipedia.org/w/api.php?action=query&list=search&srsearch=%s&utf8=1&format=json&srlimit=%d", url.QueryEscape(query), limit-len(results))
		if req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, wikiURL, nil); err == nil {
			req.Header.Set("User-Agent", "VesperBotHub/2.14")
			if resp, err := e.httpClient.Do(req); err == nil {
				defer resp.Body.Close()
				var wikiResp struct {
					Query struct {
						Search []struct {
							Title   string `json:"title"`
							Snippet string `json:"snippet"`
						} `json:"search"`
					} `json:"query"`
				}
				if json.NewDecoder(resp.Body).Decode(&wikiResp) == nil {
					for _, s := range wikiResp.Query.Search {
						if appendUnique(searchItem{
							Title:   notifier.StripEmoji(s.Title),
							URL:     "https://zh.wikipedia.org/wiki/" + url.PathEscape(s.Title),
							Snippet: stripHTMLToText(s.Snippet),
							Source:  "Wikipedia",
						}) {
							break
						}
					}
				}
			}
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
	return string(payload), nil
}

func (e *Engine) toolQueryNezhaNodes(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Keyword string `json:"keyword"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)
	kw := strings.ToLower(strings.TrimSpace(args.Keyword))

	type nodeRecord struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		HostInfo   string `json:"host_info,omitempty"`
		LastActive string `json:"last_active,omitempty"`
		Status     string `json:"status"`
		Source     string `json:"source"`
	}
	nodes := make([]nodeRecord, 0)
	sourceUsed := "local_assets"

	// 1. Try read-only direct SQLite connection to Nezha Dashboard DB only when configured
	dbPath, _ := e.store.GetConfig(store.KeyNezhaDBPath)
	dbPath = strings.TrimSpace(dbPath)

	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(3000)", dbPath)
		if db, err := sql.Open("sqlite", dsn); err == nil {
			defer db.Close()
			rows, qErr := db.QueryContext(ctx, `SELECT id, name, COALESCE(host, ''), COALESCE(updated_at, '') FROM servers ORDER BY display_index DESC, id ASC`)
			if qErr == nil {
				defer rows.Close()
				sourceUsed = "nezha_sqlite_ro"
				for rows.Next() {
					var id int64
					var name, hostJSON, updatedAt string
					if err := rows.Scan(&id, &name, &hostJSON, &updatedAt); err != nil {
						continue
					}
					if kw != "" && !strings.Contains(strings.ToLower(name), kw) && !strings.Contains(strings.ToLower(hostJSON), kw) {
						continue
					}
					status := "online"
					nodes = append(nodes, nodeRecord{
						ID:         fmt.Sprintf("nezha-%d", id),
						Name:       notifier.StripEmoji(name),
						HostInfo:   notifier.StripEmoji(hostJSON),
						LastActive: updatedAt,
						Status:     status,
						Source:     "nezha_sqlite",
					})
				}
			}
		}
	}

	// 2. Also include local compute assets from BotHub store
	assets, _ := e.store.ListAssets()
	computeAssets := make([]store.Asset, 0)
	for _, a := range assets {
		if a.Category != "compute" {
			continue
		}
		if kw != "" && !strings.Contains(strings.ToLower(a.Name), kw) && !strings.Contains(strings.ToLower(a.Spec), kw) && !strings.Contains(strings.ToLower(a.ID), kw) {
			continue
		}
		computeAssets = append(computeAssets, a)
	}

	payload, _ := json.Marshal(map[string]any{
		"source":         sourceUsed,
		"nezha_db_path":  dbPath,
		"nezha_nodes":    nodes,
		"compute_assets": computeAssets,
		"checked_at":     store.NowShanghai(),
	})
	return string(payload), nil
}

func (e *Engine) toolManageAssets(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action     string `json:"action"`
		Category   string `json:"category"`
		WithinDays int    `json:"within_days"`
		ID         string `json:"id"`
		Name       string `json:"name"`
		Spec       string `json:"spec"`
		Price      string `json:"price"`
		Status     string `json:"status"`
		DueDate    string `json:"due_date"`
		AutoRenew  *bool  `json:"auto_renew"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_assets 参数失败: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		assets, err := e.store.ListAssets()
		if err != nil {
			return "", err
		}
		filtered := make([]store.Asset, 0, len(assets))
		cat := strings.TrimSpace(args.Category)
		for _, a := range assets {
			if cat != "" && a.Category != cat {
				continue
			}
			filtered = append(filtered, a)
		}
		out, _ := json.Marshal(map[string]any{
			"count":  len(filtered),
			"assets": filtered,
		})
		return string(out), nil

	case "expiring":
		within := args.WithinDays
		if within <= 0 {
			within = 30
		}
		assets, err := e.store.ListAssets()
		if err != nil {
			return "", err
		}
		expiring := make([]store.Asset, 0)
		for _, a := range assets {
			if store.NormalizeAssetStatus(a.Status) == "已停用" {
				continue
			}
			if a.DueDate == "" || strings.HasPrefix(a.DueDate, "2099") {
				continue
			}
			if a.DaysLeft <= within {
				expiring = append(expiring, a)
			}
		}
		out, _ := json.Marshal(map[string]any{
			"within_days": within,
			"count":       len(expiring),
			"expiring":    expiring,
		})
		return string(out), nil

	case "upsert":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("upsert 资产时 id 必填")
		}
		existing, _ := e.store.GetAsset(id)
		var asset store.Asset
		if existing != nil {
			asset = *existing
		} else {
			asset = store.Asset{
				ID:          id,
				Category:    "compute",
				Status:      "生效中",
				WarnDays:    "[7, 3, 1]",
				ExtraConfig: "{}",
			}
		}
		if strings.TrimSpace(args.Name) != "" {
			asset.Name = notifier.StripEmoji(strings.TrimSpace(args.Name))
		}
		if asset.Name == "" {
			asset.Name = id
		}
		if strings.TrimSpace(args.Category) != "" {
			asset.Category = strings.TrimSpace(args.Category)
		}
		if strings.TrimSpace(args.Spec) != "" {
			asset.Spec = notifier.StripEmoji(strings.TrimSpace(args.Spec))
		}
		if strings.TrimSpace(args.Price) != "" {
			asset.Price = notifier.StripEmoji(strings.TrimSpace(args.Price))
		}
		if strings.TrimSpace(args.Status) != "" {
			asset.Status = store.NormalizeAssetStatus(args.Status)
		}
		if strings.TrimSpace(args.DueDate) != "" {
			asset.DueDate = strings.TrimSpace(args.DueDate)
		}
		if args.AutoRenew != nil {
			asset.AutoRenew = *args.AutoRenew
		}
		asset.LastCheckedAt = store.NowShanghai()
		if err := e.store.SaveAsset(asset); err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"asset": asset,
		})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 资产时 id 必填")
		}
		if err := e.store.DeleteAsset(id); err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{"ok": true, "deleted_id": id})
		return string(out), nil

	case "sync":
		tasks, _ := e.store.ListTasks()
		var watcherTaskID string
		for _, t := range tasks {
			if t.Type == "asset_watcher" {
				watcherTaskID = t.ID
				break
			}
		}
		if watcherTaskID != "" && e.scheduler != nil {
			res, err := e.scheduler.RunNow(ctx, watcherTaskID)
			if err != nil {
				return "", err
			}
			out, _ := json.Marshal(map[string]any{
				"ok":      true,
				"task_id": watcherTaskID,
				"summary": res.Summary,
			})
			return string(out), nil
		}
		res, err := modules.NewAssetWatcherModule(e.store, notifier.NewClient(e.store)).Run(ctx)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":      true,
			"summary": res.Summary,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_assets action: %s", action)
	}
}

func (e *Engine) toolManageTasks(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action      string `json:"action"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		CronExpr    string `json:"cron_expr"`
		Enabled     *bool  `json:"enabled"`
		Description string `json:"description"`
		Params      string `json:"params"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_tasks 参数失败: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		if e.scheduler != nil {
			statuses, err := e.scheduler.Status()
			if err != nil {
				return "", err
			}
			out, _ := json.Marshal(map[string]any{
				"count": len(statuses),
				"tasks": statuses,
			})
			return string(out), nil
		}
		tasks, err := e.store.ListTasks()
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"count": len(tasks),
			"tasks": tasks,
		})
		return string(out), nil

	case "run":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("run 任务时 id 必填")
		}
		if e.scheduler == nil {
			return "", fmt.Errorf("调度器未就绪")
		}
		res, err := e.scheduler.RunNow(ctx, id)
		if err != nil {
			return "", fmt.Errorf("执行任务 %s 失败: %w (摘要: %s)", id, err, res.Summary)
		}
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"task_id":      id,
			"summary":      res.Summary,
			"items_pushed": res.ItemsPushed,
		})
		return string(out), nil

	case "upsert":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("upsert 任务时 id 必填")
		}
		existing, _ := e.store.GetTask(id)
		var task store.Task
		if existing != nil {
			task = *existing
		} else {
			task = store.Task{
				ID:       id,
				Type:     "asset_watcher",
				CronExpr: "0 9 * * *",
				Enabled:  true,
				Params:   "{}",
			}
		}
		if strings.TrimSpace(args.Name) != "" {
			task.Name = notifier.StripEmoji(strings.TrimSpace(args.Name))
		}
		if task.Name == "" {
			task.Name = id
		}
		if strings.TrimSpace(args.Type) != "" {
			task.Type = strings.TrimSpace(args.Type)
		}
		if strings.TrimSpace(args.Description) != "" {
			task.Description = notifier.StripEmoji(strings.TrimSpace(args.Description))
		}
		if strings.TrimSpace(args.CronExpr) != "" {
			cronExpr := strings.TrimSpace(args.CronExpr)
			if _, err := cron.ParseStandard(cronExpr); err != nil {
				return "", fmt.Errorf("无效 Cron 表达式 %q: %w", cronExpr, err)
			}
			task.CronExpr = cronExpr
		}
		if args.Enabled != nil {
			task.Enabled = *args.Enabled
		}
		if strings.TrimSpace(args.Params) != "" {
			var js map[string]any
			if json.Unmarshal([]byte(args.Params), &js) != nil {
				return "", fmt.Errorf("params 必须是合法的 JSON 对象字符串")
			}
			task.Params = strings.TrimSpace(args.Params)
		}
		if err := e.store.SaveTask(task); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		out, _ := json.Marshal(map[string]any{
			"ok":   true,
			"task": task,
		})
		return string(out), nil

	case "toggle":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("toggle 任务时 id 必填")
		}
		existing, err := e.store.GetTask(id)
		if err != nil || existing == nil {
			return "", fmt.Errorf("未找到任务: %s", id)
		}
		if args.Enabled != nil {
			existing.Enabled = *args.Enabled
		} else {
			existing.Enabled = !existing.Enabled
		}
		if err := e.store.SaveTask(*existing); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		out, _ := json.Marshal(map[string]any{
			"ok":      true,
			"id":      id,
			"enabled": existing.Enabled,
		})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 任务时 id 必填")
		}
		if err := e.store.DeleteTask(id); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		out, _ := json.Marshal(map[string]any{"ok": true, "deleted_id": id})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_tasks action: %s", action)
	}
}

func (e *Engine) toolRememberFact(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Category   string `json:"category"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Importance int    `json:"importance"`
		Pinned     bool   `json:"pinned"`
		Scope      string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 remember_fact 参数失败: %w", err)
	}
	key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
	content := notifier.StripEmoji(strings.TrimSpace(args.Content))
	cat := notifier.StripEmoji(strings.TrimSpace(args.Category))
	if cat == "" {
		cat = "general"
	}

	sc := toolScopeFromContext(ctx)
	targetScope := "global"
	if strings.TrimSpace(sc.TimelineBotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.TimelineBotID)
	} else if strings.TrimSpace(sc.BotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.BotID)
	}
	if strings.EqualFold(strings.TrimSpace(args.Scope), "global") && sc.MemoryPolicy != store.MemoryPolicyIsolated {
		targetScope = "global"
	}

	imp := args.Importance
	if imp <= 0 {
		imp = 3
	}
	if err := e.store.SaveAgentMemoryDetailed(targetScope, cat, key, content, imp, args.Pinned); err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]any{
		"ok":         true,
		"scope":      targetScope,
		"category":   cat,
		"fact_key":   key,
		"content":    content,
		"importance": imp,
		"pinned":     args.Pinned,
	})
	return string(out), nil
}

func (e *Engine) toolRecallMemory(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	sc := toolScopeFromContext(ctx)
	var scopes []string
	if sc.TimelineBotID != "" || sc.BotID != "" {
		seen := make(map[string]bool)
		addScope := func(s string) {
			if s != "" && !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
		if sc.TimelineBotID != "" {
			addScope("bot:" + sc.TimelineBotID)
		}
		if sc.BotID != "" {
			addScope("bot:" + sc.BotID)
		}
		if sc.MemoryPolicy != store.MemoryPolicyIsolated {
			addScope("global")
		}
	}

	memories, err := e.store.ListScopedMemories(scopes, strings.TrimSpace(args.Query), 25)
	if err != nil {
		return "", err
	}
	ids := make([]int64, 0, len(memories))
	for _, m := range memories {
		ids = append(ids, m.ID)
	}
	e.store.TouchRecalledMemories(ids)

	out, _ := json.Marshal(map[string]any{
		"count":    len(memories),
		"scopes":   scopes,
		"memories": memories,
	})
	return string(out), nil
}

func (e *Engine) toolManageMemory(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action     string `json:"action"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Category   string `json:"category"`
		Importance int    `json:"importance"`
		Pinned     *bool  `json:"pinned"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_memory 参数失败: %w", err)
	}

	sc := toolScopeFromContext(ctx)
	targetScope := "global"
	if strings.TrimSpace(sc.TimelineBotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.TimelineBotID)
	} else if strings.TrimSpace(sc.BotID) != "" {
		targetScope = "bot:" + strings.TrimSpace(sc.BotID)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		mems, err := e.store.ListScopedMemories([]string{targetScope}, "", 40)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"scope":    targetScope,
			"count":    len(mems),
			"memories": mems,
		})
		return string(out), nil

	case "upsert":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		content := notifier.StripEmoji(strings.TrimSpace(args.Content))
		if key == "" || content == "" {
			return "", fmt.Errorf("upsert 记忆时 fact_key 与 content 不能为空")
		}
		cat := notifier.StripEmoji(strings.TrimSpace(args.Category))
		if cat == "" {
			cat = "general"
		}
		imp := args.Importance
		if imp <= 0 {
			imp = 3
		}
		pin := args.Pinned != nil && *args.Pinned
		if err := e.store.SaveAgentMemoryDetailed(targetScope, cat, key, content, imp, pin); err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":       true,
			"scope":    targetScope,
			"fact_key": key,
			"content":  content,
		})
		return string(out), nil

	case "pin":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		if key == "" {
			return "", fmt.Errorf("pin 记忆时 fact_key 不能为空")
		}
		mems, err := e.store.ListScopedMemories([]string{targetScope}, "", 100)
		if err != nil {
			return "", err
		}
		for _, m := range mems {
			if m.FactKey == key {
				nextPin := !m.Pinned
				if args.Pinned != nil {
					nextPin = *args.Pinned
				}
				nextImp := m.Importance
				if args.Importance > 0 {
					nextImp = args.Importance
				}
				if err := e.store.UpdateAgentMemory(m.ID, m.Scope, m.Category, m.FactKey, m.Content, nextImp, nextPin); err != nil {
					return "", err
				}
				out, _ := json.Marshal(map[string]any{"ok": true, "fact_key": key, "pinned": nextPin})
				return string(out), nil
			}
		}
		return "", fmt.Errorf("未在当前作用域找到记忆键: %s", key)

	case "forget":
		key := notifier.StripEmoji(strings.TrimSpace(args.FactKey))
		if key == "" {
			return "", fmt.Errorf("forget 记忆时 fact_key 不能为空")
		}
		deleted, err := e.store.DeleteAgentMemoryByScopeKey(targetScope, key)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"ok":       deleted,
			"scope":    targetScope,
			"fact_key": key,
			"forgotten": deleted,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_memory action: %s", action)
	}
}
