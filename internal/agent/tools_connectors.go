package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// ============================================================================
// 1. GitHub App Connector (github_connector)
// ============================================================================

func githubConnectorToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "github_connector",
				Description: "GitHub 应用连接器（读+写全能力）：支持查看个人或指定用户的代码仓库列表(list_repos)、获取仓库详情(get_repo)、查询或创建 Issue(list_issues / create_issue / comment_issue)、查询 Pull Requests(list_pulls)、查看或重跑 GitHub Actions 工作流(list_runs / rerun_workflow)、查看未读通知(list_notifications)及搜索仓库或 Issue(search)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "操作类型：list_repos | get_repo | list_issues | create_issue | comment_issue | list_pulls | list_runs | rerun_workflow | list_notifications | search",
						},
						"owner": map[string]any{
							"type":        "string",
							"description": "仓库所有者用户名或组织名（若留空则使用连接器配置的 default_owner 或当前认证用户）",
						},
						"repo": map[string]any{
							"type":        "string",
							"description": "仓库名称（也支持直接填 owner/repo 格式）",
						},
						"state": map[string]any{
							"type":        "string",
							"description": "过滤状态：open (默认) | closed | all",
						},
						"number": map[string]any{
							"type":        "integer",
							"description": "Issue 或 PR 编号（用于 comment_issue）",
						},
						"run_id": map[string]any{
							"type":        "integer",
							"description": "GitHub Actions Workflow Run ID（用于 rerun_workflow）",
						},
						"title": map[string]any{
							"type":        "string",
							"description": "创建 Issue 的标题（用于 create_issue）",
						},
						"body": map[string]any{
							"type":        "string",
							"description": "Issue 正文或评论内容（用于 create_issue / comment_issue）",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词（用于 search）",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回条数上限（默认 10，最大 25）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		Timeout:        20 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在操作 GitHub 连接器...",
		DomainKeywords: []string{"github", "repo", "仓库", "issue", "pr", "pull request", "actions", "workflow", "流水线"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolGitHubConnector(ctx, cleanArgs)
		},
	}
}

func (e *Engine) loadAppConnectorConfig(id string) (store.Skill, map[string]string, error) {
	if e == nil || e.store == nil {
		return store.Skill{}, nil, fmt.Errorf("存储未初始化")
	}
	sk, err := e.store.GetSkill(id)
	if err != nil || sk == nil {
		return store.Skill{}, nil, fmt.Errorf("未找到应用连接器: %s", id)
	}
	if !sk.Enabled {
		return *sk, nil, fmt.Errorf("应用连接器「%s」尚未启用或连接，请先在 MCP 中心「应用连接器」中填入凭证并启用", sk.Name)
	}
	return *sk, sk.HeadersMap(), nil
}

func (e *Engine) githubAPIRequest(ctx context.Context, method, apiPath, token string, payload any) (int, []byte, error) {
	baseURL := "https://api.github.com"
	if e.allowLocalhostForTest {
		if sk, err := e.store.GetSkill(store.BuiltinMCPAppGitHubID); err == nil && sk != nil {
			if customBase := strings.TrimSpace(sk.HeadersMap()["api_base"]); customBase != "" {
				baseURL = strings.TrimRight(customBase, "/")
			}
		}
	}
	fullURL := baseURL + apiPath
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Vesper-BotHub-Connector/2.83")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.safeFetchClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, raw, fmt.Errorf("GitHub API 返回 HTTP %d: %s", resp.StatusCode, truncateRunes(strings.TrimSpace(string(raw)), 240))
	}
	return resp.StatusCode, raw, nil
}

func splitOwnerRepo(owner, repo, defaultOwner string) (string, string) {
	o := strings.TrimSpace(owner)
	r := strings.TrimSpace(repo)
	if strings.Contains(r, "/") {
		parts := strings.SplitN(r, "/", 2)
		if o == "" {
			o = strings.TrimSpace(parts[0])
		}
		r = strings.TrimSpace(parts[1])
	}
	if o == "" {
		o = strings.TrimSpace(defaultOwner)
	}
	return o, r
}

func (e *Engine) toolGitHubConnector(ctx context.Context, argsJSON string) (string, error) {
	_, cfg, err := e.loadAppConnectorConfig(store.BuiltinMCPAppGitHubID)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(cfg["token"])
	defaultOwner := strings.TrimSpace(cfg["default_owner"])

	var args struct {
		Action string `json:"action"`
		Owner  string `json:"owner"`
		Repo   string `json:"repo"`
		State  string `json:"state"`
		Number int    `json:"number"`
		RunID  int64  `json:"run_id"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Query  string `json:"query"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 github_connector 参数失败: %w", err)
	}
	limit := args.Limit
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	state := strings.ToLower(strings.TrimSpace(args.State))
	if state == "" {
		state = "open"
	}
	owner, repo := splitOwnerRepo(args.Owner, args.Repo, defaultOwner)
	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "list_repos":
		apiPath := fmt.Sprintf("/user/repos?sort=updated&per_page=%d", limit)
		if owner != "" && (token == "" || !strings.EqualFold(owner, defaultOwner)) {
			apiPath = fmt.Sprintf("/users/%s/repos?sort=updated&per_page=%d", url.PathEscape(owner), limit)
		} else if token == "" && owner != "" {
			apiPath = fmt.Sprintf("/users/%s/repos?sort=updated&per_page=%d", url.PathEscape(owner), limit)
		}
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, apiPath, token, nil)
		if err != nil {
			return "", err
		}
		var repos []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Private     bool   `json:"private"`
			Stars       int    `json:"stargazers_count"`
			OpenIssues  int    `json:"open_issues_count"`
			Language    string `json:"language"`
			UpdatedAt   string `json:"updated_at"`
			HTMLURL     string `json:"html_url"`
		}
		_ = json.Unmarshal(raw, &repos)
		b, _ := json.Marshal(map[string]any{"ok": true, "count": len(repos), "repos": repos})
		return string(b), nil

	case "get_repo":
		if owner == "" || repo == "" {
			return "", fmt.Errorf("get_repo 需要提供 owner 和 repo（或 owner/repo）")
		}
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), token, nil)
		if err != nil {
			return "", err
		}
		var info map[string]any
		_ = json.Unmarshal(raw, &info)
		b, _ := json.Marshal(map[string]any{
			"ok":             true,
			"full_name":      info["full_name"],
			"description":    info["description"],
			"default_branch": info["default_branch"],
			"stars":          info["stargazers_count"],
			"forks":          info["forks_count"],
			"open_issues":    info["open_issues_count"],
			"language":       info["language"],
			"updated_at":     info["updated_at"],
			"html_url":       info["html_url"],
		})
		return string(b), nil

	case "list_issues", "list_pulls":
		if owner == "" || repo == "" {
			return "", fmt.Errorf("%s 需要提供 owner 和 repo", action)
		}
		endpoint := "issues"
		if action == "list_pulls" {
			endpoint = "pulls"
		}
		apiPath := fmt.Sprintf("/repos/%s/%s/%s?state=%s&per_page=%d", url.PathEscape(owner), url.PathEscape(repo), endpoint, url.QueryEscape(state), limit)
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, apiPath, token, nil)
		if err != nil {
			return "", err
		}
		var items []struct {
			Number    int    `json:"number"`
			Title     string `json:"title"`
			State     string `json:"state"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			HTMLURL   string `json:"html_url"`
			User      struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		_ = json.Unmarshal(raw, &items)
		b, _ := json.Marshal(map[string]any{"ok": true, "repo": owner + "/" + repo, "count": len(items), "items": items})
		return string(b), nil

	case "create_issue":
		if owner == "" || repo == "" || strings.TrimSpace(args.Title) == "" {
			return "", fmt.Errorf("create_issue 需要提供 owner、repo 和 title")
		}
		if token == "" {
			return "", fmt.Errorf("create_issue 需要配置 GitHub Personal Access Token")
		}
		apiPath := fmt.Sprintf("/repos/%s/%s/issues", url.PathEscape(owner), url.PathEscape(repo))
		_, raw, err := e.githubAPIRequest(ctx, http.MethodPost, apiPath, token, map[string]any{
			"title": strings.TrimSpace(args.Title),
			"body":  strings.TrimSpace(args.Body),
		})
		if err != nil {
			return "", err
		}
		var created struct {
			Number  int    `json:"number"`
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
		}
		_ = json.Unmarshal(raw, &created)
		b, _ := json.Marshal(map[string]any{"ok": true, "created": created})
		return string(b), nil

	case "comment_issue":
		if owner == "" || repo == "" || args.Number <= 0 || strings.TrimSpace(args.Body) == "" {
			return "", fmt.Errorf("comment_issue 需要提供 owner、repo、number 和 body")
		}
		apiPath := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", url.PathEscape(owner), url.PathEscape(repo), args.Number)
		_, raw, err := e.githubAPIRequest(ctx, http.MethodPost, apiPath, token, map[string]any{
			"body": strings.TrimSpace(args.Body),
		})
		if err != nil {
			return "", err
		}
		var comment struct {
			ID      int64  `json:"id"`
			HTMLURL string `json:"html_url"`
		}
		_ = json.Unmarshal(raw, &comment)
		b, _ := json.Marshal(map[string]any{"ok": true, "comment": comment})
		return string(b), nil

	case "list_runs":
		if owner == "" || repo == "" {
			return "", fmt.Errorf("list_runs 需要提供 owner 和 repo")
		}
		apiPath := fmt.Sprintf("/repos/%s/%s/actions/runs?per_page=%d", url.PathEscape(owner), url.PathEscape(repo), limit)
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, apiPath, token, nil)
		if err != nil {
			return "", err
		}
		var resp struct {
			WorkflowRuns []struct {
				ID         int64  `json:"id"`
				Name       string `json:"name"`
				HeadBranch string `json:"head_branch"`
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				CreatedAt  string `json:"created_at"`
				HTMLURL    string `json:"html_url"`
			} `json:"workflow_runs"`
		}
		_ = json.Unmarshal(raw, &resp)
		b, _ := json.Marshal(map[string]any{"ok": true, "repo": owner + "/" + repo, "runs": resp.WorkflowRuns})
		return string(b), nil

	case "rerun_workflow":
		if owner == "" || repo == "" || args.RunID <= 0 {
			return "", fmt.Errorf("rerun_workflow 需要提供 owner、repo 和 run_id")
		}
		apiPath := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/rerun-failed-jobs", url.PathEscape(owner), url.PathEscape(repo), args.RunID)
		_, _, err := e.githubAPIRequest(ctx, http.MethodPost, apiPath, token, map[string]any{})
		if err != nil {
			// Fallback to full rerun if rerun-failed-jobs is not applicable
			apiPath = fmt.Sprintf("/repos/%s/%s/actions/runs/%d/rerun", url.PathEscape(owner), url.PathEscape(repo), args.RunID)
			if _, _, err2 := e.githubAPIRequest(ctx, http.MethodPost, apiPath, token, map[string]any{}); err2 != nil {
				return "", err
			}
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "repo": owner + "/" + repo, "run_id": args.RunID, "message": "已触发 GitHub Actions 工作流重跑"})
		return string(b), nil

	case "list_notifications":
		if token == "" {
			return "", fmt.Errorf("list_notifications 需要配置 GitHub Personal Access Token")
		}
		apiPath := fmt.Sprintf("/notifications?per_page=%d", limit)
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, apiPath, token, nil)
		if err != nil {
			return "", err
		}
		var notes []struct {
			ID         string `json:"id"`
			Reason     string `json:"reason"`
			Unread     bool   `json:"unread"`
			UpdatedAt  string `json:"updated_at"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			Subject struct {
				Title string `json:"title"`
				Type  string `json:"type"`
			} `json:"subject"`
		}
		_ = json.Unmarshal(raw, &notes)
		b, _ := json.Marshal(map[string]any{"ok": true, "count": len(notes), "notifications": notes})
		return string(b), nil

	case "search":
		q := strings.TrimSpace(args.Query)
		if q == "" {
			return "", fmt.Errorf("search 需要提供 query 关键词")
		}
		apiPath := fmt.Sprintf("/search/repositories?q=%s&per_page=%d", url.QueryEscape(q), limit)
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, apiPath, token, nil)
		if err != nil {
			return "", err
		}
		var res struct {
			TotalCount int `json:"total_count"`
			Items      []struct {
				FullName    string `json:"full_name"`
				Description string `json:"description"`
				Stars       int    `json:"stargazers_count"`
				Language    string `json:"language"`
				HTMLURL     string `json:"html_url"`
			} `json:"items"`
		}
		_ = json.Unmarshal(raw, &res)
		b, _ := json.Marshal(map[string]any{"ok": true, "total_count": res.TotalCount, "items": res.Items})
		return string(b), nil

	default:
		return "", fmt.Errorf("不支持的 github_connector action: %s", action)
	}
}

// ============================================================================
// 2. Gmail & Generic IMAP/SMTP Email Connector (email_connector)
// ============================================================================

func emailConnectorToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "email_connector",
				Description: "Gmail 与通用 IMAP/SMTP 邮箱应用连接器（读+写全能力）：支持查看未读邮件(list_unread)、查看最近收件箱邮件(list_recent)、按主题或发件人关键词搜索邮件(search_emails)、读取指定序号邮件正文(read_email)以及通过 SMTP 发送或回复邮件(send_email)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "操作类型：list_unread | list_recent | search_emails | read_email | send_email",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "搜索关键词（用于 search_emails，匹配发件人或主题）",
						},
						"seq_num": map[string]any{
							"type":        "integer",
							"description": "邮件序号（用于 read_email，从 list_unread / list_recent 返回的 seq_num 获取）",
						},
						"to": map[string]any{
							"type":        "string",
							"description": "收件人邮箱地址（用于 send_email，多个收件人用逗号分隔）",
						},
						"subject": map[string]any{
							"type":        "string",
							"description": "发送邮件主题（用于 send_email）",
						},
						"body": map[string]any{
							"type":        "string",
							"description": "发送邮件正文内容（用于 send_email）",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "读取邮件条数上限（默认 8，最大 20）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		Timeout:        25 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在操作 Gmail / 邮箱连接器...",
		DomainKeywords: []string{"gmail", "邮件", "邮箱", "email", "收件箱", "未读邮件", "发邮件", "smtp", "imap"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolEmailConnector(ctx, cleanArgs)
		},
	}
}

func resolveEmailHosts(emailAddr, imapHost, smtpHost string) (string, string) {
	ih := strings.TrimSpace(imapHost)
	sh := strings.TrimSpace(smtpHost)
	domain := ""
	if at := strings.LastIndexByte(emailAddr, '@'); at > 0 && at+1 < len(emailAddr) {
		domain = strings.ToLower(strings.TrimSpace(emailAddr[at+1:]))
	}
	if ih == "" {
		switch domain {
		case "gmail.com", "googlemail.com":
			ih = "imap.gmail.com:993"
		case "outlook.com", "hotmail.com", "live.com":
			ih = "outlook.office365.com:993"
		case "qq.com":
			ih = "imap.qq.com:993"
		case "163.com":
			ih = "imap.163.com:993"
		default:
			if domain != "" {
				ih = "imap." + domain + ":993"
			}
		}
	} else if !strings.Contains(ih, ":") {
		ih += ":993"
	}
	if sh == "" {
		switch domain {
		case "gmail.com", "googlemail.com":
			sh = "smtp.gmail.com:465"
		case "outlook.com", "hotmail.com", "live.com":
			sh = "smtp.office365.com:587"
		case "qq.com":
			sh = "smtp.qq.com:465"
		case "163.com":
			sh = "smtp.163.com:465"
		default:
			if domain != "" {
				sh = "smtp." + domain + ":465"
			}
		}
	} else if !strings.Contains(sh, ":") {
		sh += ":465"
	}
	return ih, sh
}

func decodeRFC2047Word(s string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(strings.TrimSpace(s))
	if err != nil {
		return strings.TrimSpace(s)
	}
	return out
}

type imapLiteSession struct {
	conn   net.Conn
	reader *bufio.Reader
	seq    int
}

func dialIMAPLite(ctx context.Context, hostPort, user, pass string) (*imapLiteSession, error) {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
		hostPort = hostPort + ":993"
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	rawConn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return nil, fmt.Errorf("连接 IMAP 服务器 %s 失败: %w", hostPort, err)
	}
	tlsConn := tls.Client(rawConn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	_ = tlsConn.SetDeadline(time.Now().Add(18 * time.Second))
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = rawConn.Close()
		return nil, fmt.Errorf("IMAP TLS 握手失败: %w", err)
	}
	sess := &imapLiteSession{
		conn:   tlsConn,
		reader: bufio.NewReaderSize(tlsConn, 64*1024),
	}
	// Read greeting
	if _, err := sess.reader.ReadString('\n'); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("读取 IMAP 欢迎响应失败: %w", err)
	}
	quoteIMAP := func(v string) string {
		escaped := strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`)
		return `"` + escaped + `"`
	}
	if _, err := sess.cmd(fmt.Sprintf("LOGIN %s %s", quoteIMAP(user), quoteIMAP(pass))); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("IMAP 邮箱登录认证失败（Gmail 请确认已使用 16 位应用专用密码）: %w", err)
	}
	return sess, nil
}

var reIMAPLiteralLen = regexp.MustCompile(`\{(\d+)\}\r?\n$`)

func (s *imapLiteSession) cmd(command string) ([]string, error) {
	s.seq++
	tag := fmt.Sprintf("A%03d", s.seq)
	if _, err := fmt.Fprintf(s.conn, "%s %s\r\n", tag, command); err != nil {
		return nil, err
	}
	var lines []string
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return lines, err
		}
		if m := reIMAPLiteralLen.FindStringSubmatch(line); len(m) == 2 {
			litLen, _ := strconv.Atoi(m[1])
			if litLen > 0 && litLen <= 512*1024 {
				buf := make([]byte, litLen)
				if _, rErr := io.ReadFull(s.reader, buf); rErr == nil {
					line += string(buf)
				}
			}
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, tag+" ") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, tag+" "))
			upper := strings.ToUpper(rest)
			if strings.HasPrefix(upper, "OK") {
				return lines, nil
			}
			return lines, fmt.Errorf("IMAP 响应错误: %s", rest)
		}
		lines = append(lines, trimmed)
	}
}

func (s *imapLiteSession) close() {
	if s == nil || s.conn == nil {
		return
	}
	_, _ = fmt.Fprintf(s.conn, "A999 LOGOUT\r\n")
	_ = s.conn.Close()
}

type parsedEmailSummary struct {
	SeqNum  int    `json:"seq_num"`
	Subject string `json:"subject"`
	From    string `json:"from"`
	Date    string `json:"date"`
	Snippet string `json:"snippet,omitempty"`
}

func parseEmailHeaderAndBody(rawBlock string) (subject, from, date, bodySnippet string) {
	parts := strings.SplitN(rawBlock, "\r\n\r\n", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(rawBlock, "\n\n", 2)
	}
	headerPart := parts[0]
	if len(parts) == 2 {
		bodySnippet = parts[1]
	}

	// Unfold multi-line headers
	unfolded := regexp.MustCompile(`\r?\n[ \t]+`).ReplaceAllString(headerPart, " ")
	contentTransferEnc := ""
	for _, line := range strings.Split(unfolded, "\n") {
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(line[:colon]))
		v := strings.TrimSpace(line[colon+1:])
		switch k {
		case "subject":
			subject = notifier.StripEmoji(decodeRFC2047Word(v))
		case "from":
			from = notifier.StripEmoji(decodeRFC2047Word(v))
		case "date":
			date = v
		case "content-transfer-encoding":
			contentTransferEnc = strings.ToLower(v)
		}
	}
	if bodySnippet != "" {
		cleanBody := strings.TrimSpace(bodySnippet)
		if idx := strings.LastIndex(cleanBody, ")"); idx > 0 && idx >= len(cleanBody)-4 {
			cleanBody = strings.TrimSpace(cleanBody[:idx])
		}
		switch contentTransferEnc {
		case "base64":
			compact := strings.Map(func(r rune) rune {
				if r == '\r' || r == '\n' || r == ' ' {
					return -1
				}
				return r
			}, cleanBody)
			if decoded, err := base64.StdEncoding.DecodeString(compact); err == nil {
				cleanBody = string(decoded)
			}
		case "quoted-printable":
			if decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(cleanBody))); err == nil {
				cleanBody = string(decoded)
			}
		}
		cleanBody = stripHTMLToText(cleanBody)
		bodySnippet = truncateRunes(notifier.StripEmoji(strings.TrimSpace(cleanBody)), 600)
	}
	return subject, from, date, bodySnippet
}

func (e *Engine) fetchIMAPEmails(ctx context.Context, emailAddr, pass, imapHost string, onlyUnread bool, searchQuery string, targetSeq int, limit int) ([]parsedEmailSummary, int, error) {
	ih, _ := resolveEmailHosts(emailAddr, imapHost, "")
	sess, err := dialIMAPLite(ctx, ih, emailAddr, pass)
	if err != nil {
		return nil, 0, err
	}
	defer sess.close()

	if _, err := sess.cmd("SELECT INBOX"); err != nil {
		return nil, 0, err
	}

	if targetSeq > 0 {
		lines, err := sess.cmd(fmt.Sprintf("FETCH %d (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM DATE CONTENT-TRANSFER-ENCODING)] BODY.PEEK[TEXT]<0.4096>)", targetSeq))
		if err != nil {
			return nil, 0, err
		}
		joined := strings.Join(lines, "\n")
		subj, from, date, body := parseEmailHeaderAndBody(joined)
		return []parsedEmailSummary{{
			SeqNum:  targetSeq,
			Subject: subj,
			From:    from,
			Date:    date,
			Snippet: body,
		}}, 1, nil
	}

	searchCriterion := "ALL"
	if onlyUnread {
		searchCriterion = "UNSEEN"
	}
	searchLines, err := sess.cmd("SEARCH " + searchCriterion)
	if err != nil {
		return nil, 0, err
	}
	var seqNums []int
	for _, l := range searchLines {
		if strings.HasPrefix(strings.ToUpper(l), "* SEARCH") {
			fields := strings.Fields(l)
			for _, f := range fields[2:] {
				if n, nErr := strconv.Atoi(f); nErr == nil && n > 0 {
					seqNums = append(seqNums, n)
				}
			}
		}
	}
	totalMatched := len(seqNums)
	if totalMatched == 0 {
		return []parsedEmailSummary{}, 0, nil
	}

	// Iterate from newest to oldest
	qLow := strings.ToLower(strings.TrimSpace(searchQuery))
	scanCap := limit
	if qLow != "" && scanCap < 30 {
		scanCap = 30
	}
	results := make([]parsedEmailSummary, 0, limit)
	for i := len(seqNums) - 1; i >= 0 && len(results) < limit && (len(seqNums)-1-i) < scanCap; i-- {
		seq := seqNums[i]
		lines, fErr := sess.cmd(fmt.Sprintf("FETCH %d (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM DATE)])", seq))
		if fErr != nil {
			continue
		}
		subj, from, date, _ := parseEmailHeaderAndBody(strings.Join(lines, "\n"))
		if qLow != "" && !strings.Contains(strings.ToLower(subj), qLow) && !strings.Contains(strings.ToLower(from), qLow) {
			continue
		}
		results = append(results, parsedEmailSummary{
			SeqNum:  seq,
			Subject: subj,
			From:    from,
			Date:    date,
		})
	}
	return results, totalMatched, nil
}

func sendSMTPEmail(ctx context.Context, fromEmail, senderName, pass, smtpHost, toListRaw, subject, body string) error {
	_, sh := resolveEmailHosts(fromEmail, "", smtpHost)
	host, port, err := net.SplitHostPort(sh)
	if err != nil {
		host = sh
		port = "465"
		sh = host + ":465"
	}

	var recipients []string
	for _, r := range strings.Split(toListRaw, ",") {
		if clean := strings.TrimSpace(r); clean != "" {
			recipients = append(recipients, clean)
		}
	}
	if len(recipients) == 0 {
		return fmt.Errorf("收件人 to 不能为空")
	}

	fromHeader := fromEmail
	if strings.TrimSpace(senderName) != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", strings.TrimSpace(senderName)), fromEmail)
	}
	encodedSubj := mime.QEncoding.Encode("utf-8", strings.TrimSpace(subject))
	msgBuf := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		fromHeader,
		strings.Join(recipients, ", "),
		encodedSubj,
		time.Now().Format(time.RFC1123Z),
		body,
	)

	auth := smtp.PlainAuth("", fromEmail, pass, host)
	dialer := &net.Dialer{Timeout: 12 * time.Second}
	if port == "465" {
		rawConn, err := dialer.DialContext(ctx, "tcp", sh)
		if err != nil {
			return fmt.Errorf("连接 SMTP 服务器 %s 失败: %w", sh, err)
		}
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		_ = tlsConn.SetDeadline(time.Now().Add(20 * time.Second))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return fmt.Errorf("SMTP TLS 握手失败: %w", err)
		}
		client, err := smtp.NewClient(tlsConn, host)
		if err != nil {
			_ = tlsConn.Close()
			return err
		}
		defer client.Close()
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
		if err := client.Mail(fromEmail); err != nil {
			return err
		}
		for _, rcpt := range recipients {
			if err := client.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msgBuf)); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	// STARTTLS (port 587 / 25)
	rawConn, err := dialer.DialContext(ctx, "tcp", sh)
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器 %s 失败: %w", sh, err)
	}
	_ = rawConn.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(rawConn, host)
	if err != nil {
		_ = rawConn.Close()
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("SMTP STARTTLS 失败: %w", err)
		}
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}
	if err := client.Mail(fromEmail); err != nil {
		return err
	}
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msgBuf)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (e *Engine) toolEmailConnector(ctx context.Context, argsJSON string) (string, error) {
	_, cfg, err := e.loadAppConnectorConfig(store.BuiltinMCPAppEmailID)
	if err != nil {
		return "", err
	}
	emailAddr := strings.TrimSpace(cfg["email"])
	pass := strings.TrimSpace(cfg["password"])
	imapHost := strings.TrimSpace(cfg["imap_host"])
	smtpHost := strings.TrimSpace(cfg["smtp_host"])
	senderName := strings.TrimSpace(cfg["sender_name"])
	if emailAddr == "" || pass == "" {
		return "", fmt.Errorf("邮箱连接器尚未填写 email 或 password（应用专用密码）")
	}

	var args struct {
		Action  string `json:"action"`
		Query   string `json:"query"`
		SeqNum  int    `json:"seq_num"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
		Limit   int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 email_connector 参数失败: %w", err)
	}
	limit := args.Limit
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "list_unread":
		items, totalUnread, err := e.fetchIMAPEmails(ctx, emailAddr, pass, imapHost, true, "", 0, limit)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "account": emailAddr, "total_unread": totalUnread, "emails": items})
		return string(b), nil

	case "list_recent":
		items, totalCount, err := e.fetchIMAPEmails(ctx, emailAddr, pass, imapHost, false, "", 0, limit)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "account": emailAddr, "total_inbox": totalCount, "emails": items})
		return string(b), nil

	case "search_emails":
		q := strings.TrimSpace(args.Query)
		if q == "" {
			return "", fmt.Errorf("search_emails 需要提供 query 关键词")
		}
		items, _, err := e.fetchIMAPEmails(ctx, emailAddr, pass, imapHost, false, q, 0, limit)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "query": q, "count": len(items), "emails": items})
		return string(b), nil

	case "read_email":
		if args.SeqNum <= 0 {
			return "", fmt.Errorf("read_email 需要提供有效的 seq_num")
		}
		items, _, err := e.fetchIMAPEmails(ctx, emailAddr, pass, imapHost, false, "", args.SeqNum, 1)
		if err != nil || len(items) == 0 {
			return "", fmt.Errorf("读取邮件 #%d 失败: %v", args.SeqNum, err)
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "email": items[0]})
		return string(b), nil

	case "send_email":
		if strings.TrimSpace(args.To) == "" || strings.TrimSpace(args.Subject) == "" {
			return "", fmt.Errorf("send_email 需要提供收件人 to 和主题 subject")
		}
		if err := sendSMTPEmail(ctx, emailAddr, senderName, pass, smtpHost, args.To, args.Subject, args.Body); err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{
			"ok":      true,
			"from":    emailAddr,
			"to":      args.To,
			"subject": args.Subject,
			"message": "邮件已通过 SMTP 成功发送",
		})
		return string(b), nil

	default:
		return "", fmt.Errorf("不支持的 email_connector action: %s", action)
	}
}

// ============================================================================
// 3. Cloudflare App Connector (cloudflare_connector)
// ============================================================================

func cloudflareConnectorToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "cloudflare_connector",
				Description: "Cloudflare 应用连接器（读+写全能力）：支持列出账号下所有域名 Zone(list_zones)、查询指定域名的 DNS 解析记录(list_dns)、新增或更新 DNS 记录(upsert_dns)、删除 DNS 记录(delete_dns)以及一键清理域名 CDN 缓存(purge_cache)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "操作类型：list_zones | list_dns | upsert_dns | delete_dns | purge_cache",
						},
						"zone": map[string]any{
							"type":        "string",
							"description": "主域名（如 starryheavens.uk）或 Cloudflare Zone ID（若留空则使用配置的 default_zone）",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "DNS 记录类型：A | AAAA | CNAME | TXT | MX（用于 list_dns / upsert_dns）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "DNS 主机记录名称（如 bot 或 bot.starryheavens.uk）",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "DNS 记录值（如 IP 地址或目标域名，用于 upsert_dns）",
						},
						"proxied": map[string]any{
							"type":        "boolean",
							"description": "是否开启 Cloudflare 橙云代理（用于 upsert_dns，默认 false）",
						},
						"record_id": map[string]any{
							"type":        "string",
							"description": "DNS 记录 ID（用于 delete_dns 或指定更新）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		Timeout:        20 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在操作 Cloudflare 连接器...",
		DomainKeywords: []string{"cloudflare", "dns", "域名解析", "a记录", "cname", "清缓存", "zone"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolCloudflareConnector(ctx, cleanArgs)
		},
	}
}

func (e *Engine) cloudflareAPIRequest(ctx context.Context, method, apiPath, apiToken string, payload any) ([]byte, error) {
	if strings.TrimSpace(apiToken) == "" {
		return nil, fmt.Errorf("Cloudflare 连接器未配置 api_token")
	}
	baseURL := "https://api.cloudflare.com/client/v4"
	if e.allowLocalhostForTest {
		if sk, err := e.store.GetSkill(store.BuiltinMCPAppCloudflareID); err == nil && sk != nil {
			if customBase := strings.TrimSpace(sk.HeadersMap()["api_base"]); customBase != "" {
				baseURL = strings.TrimRight(customBase, "/")
			}
		}
	}
	var bodyReader io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+apiPath, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Vesper-BotHub-Connector/2.83")

	resp, err := e.safeFetchClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Cloudflare API 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	var env struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("解析 Cloudflare 响应失败 (HTTP %d)", resp.StatusCode)
	}
	if !env.Success {
		errMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if len(env.Errors) > 0 {
			errMsg = env.Errors[0].Message
		}
		return nil, fmt.Errorf("Cloudflare API 错误: %s", errMsg)
	}
	return env.Result, nil
}

func (e *Engine) resolveCloudflareZoneID(ctx context.Context, zoneOrDomain, apiToken string) (string, string, error) {
	z := strings.TrimSpace(zoneOrDomain)
	if z == "" {
		return "", "", fmt.Errorf("请提供域名 zone（或在连接器中配置 default_zone）")
	}
	// If it lacks dots and looks like a 32-char hex Zone ID, use directly
	if !strings.Contains(z, ".") && len(z) >= 24 {
		return z, z, nil
	}
	res, err := e.cloudflareAPIRequest(ctx, http.MethodGet, "/zones?name="+url.QueryEscape(z), apiToken, nil)
	if err != nil {
		return "", "", err
	}
	var zones []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(res, &zones)
	if len(zones) == 0 {
		return "", "", fmt.Errorf("在当前 Cloudflare 账号下未找到域名 Zone: %s", z)
	}
	return zones[0].ID, zones[0].Name, nil
}

func (e *Engine) toolCloudflareConnector(ctx context.Context, argsJSON string) (string, error) {
	_, cfg, err := e.loadAppConnectorConfig(store.BuiltinMCPAppCloudflareID)
	if err != nil {
		return "", err
	}
	apiToken := strings.TrimSpace(cfg["api_token"])
	defaultZone := strings.TrimSpace(cfg["default_zone"])

	var args struct {
		Action   string `json:"action"`
		Zone     string `json:"zone"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		Content  string `json:"content"`
		Proxied  *bool  `json:"proxied"`
		RecordID string `json:"record_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 cloudflare_connector 参数失败: %w", err)
	}
	zoneInput := strings.TrimSpace(args.Zone)
	if zoneInput == "" {
		zoneInput = defaultZone
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "list_zones":
		res, err := e.cloudflareAPIRequest(ctx, http.MethodGet, "/zones?per_page=50", apiToken, nil)
		if err != nil {
			return "", err
		}
		var zones []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		_ = json.Unmarshal(res, &zones)
		b, _ := json.Marshal(map[string]any{"ok": true, "count": len(zones), "zones": zones})
		return string(b), nil

	case "list_dns":
		zoneID, zoneName, err := e.resolveCloudflareZoneID(ctx, zoneInput, apiToken)
		if err != nil {
			return "", err
		}
		q := "/zones/" + url.PathEscape(zoneID) + "/dns_records?per_page=100"
		if strings.TrimSpace(args.Type) != "" {
			q += "&type=" + url.QueryEscape(strings.ToUpper(strings.TrimSpace(args.Type)))
		}
		if strings.TrimSpace(args.Name) != "" {
			q += "&name=" + url.QueryEscape(strings.TrimSpace(args.Name))
		}
		res, err := e.cloudflareAPIRequest(ctx, http.MethodGet, q, apiToken, nil)
		if err != nil {
			return "", err
		}
		var records []struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Name    string `json:"name"`
			Content string `json:"content"`
			Proxied bool   `json:"proxied"`
			TTL     int    `json:"ttl"`
		}
		_ = json.Unmarshal(res, &records)
		b, _ := json.Marshal(map[string]any{"ok": true, "zone": zoneName, "count": len(records), "records": records})
		return string(b), nil

	case "upsert_dns":
		zoneID, zoneName, err := e.resolveCloudflareZoneID(ctx, zoneInput, apiToken)
		if err != nil {
			return "", err
		}
		recType := strings.ToUpper(strings.TrimSpace(args.Type))
		if recType == "" {
			recType = "A"
		}
		recName := strings.TrimSpace(args.Name)
		recContent := strings.TrimSpace(args.Content)
		if recName == "" || recContent == "" {
			return "", fmt.Errorf("upsert_dns 需要提供 name 和 content")
		}
		if !strings.Contains(recName, ".") && zoneName != "" && !strings.Contains(zoneName, " ") {
			recName = recName + "." + zoneName
		}
		proxied := false
		if args.Proxied != nil {
			proxied = *args.Proxied
		}
		payload := map[string]any{
			"type":    recType,
			"name":    recName,
			"content": recContent,
			"proxied": proxied,
			"ttl":     1,
		}
		recID := strings.TrimSpace(args.RecordID)
		if recID == "" {
			// Check if record already exists
			q := fmt.Sprintf("/zones/%s/dns_records?type=%s&name=%s", url.PathEscape(zoneID), url.QueryEscape(recType), url.QueryEscape(recName))
			if existingRaw, lErr := e.cloudflareAPIRequest(ctx, http.MethodGet, q, apiToken, nil); lErr == nil {
				var existing []struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(existingRaw, &existing)
				if len(existing) > 0 {
					recID = existing[0].ID
				}
			}
		}
		var res []byte
		if recID != "" {
			res, err = e.cloudflareAPIRequest(ctx, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recID)), apiToken, payload)
		} else {
			res, err = e.cloudflareAPIRequest(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID)), apiToken, payload)
		}
		if err != nil {
			return "", err
		}
		var saved map[string]any
		_ = json.Unmarshal(res, &saved)
		b, _ := json.Marshal(map[string]any{"ok": true, "zone": zoneName, "record": saved})
		return string(b), nil

	case "delete_dns":
		zoneID, zoneName, err := e.resolveCloudflareZoneID(ctx, zoneInput, apiToken)
		if err != nil {
			return "", err
		}
		recID := strings.TrimSpace(args.RecordID)
		if recID == "" && strings.TrimSpace(args.Name) != "" {
			recName := strings.TrimSpace(args.Name)
			if !strings.Contains(recName, ".") && strings.Contains(zoneName, ".") {
				recName = recName + "." + zoneName
			}
			q := fmt.Sprintf("/zones/%s/dns_records?name=%s", url.PathEscape(zoneID), url.QueryEscape(recName))
			if strings.TrimSpace(args.Type) != "" {
				q += "&type=" + url.QueryEscape(strings.ToUpper(strings.TrimSpace(args.Type)))
			}
			if existingRaw, lErr := e.cloudflareAPIRequest(ctx, http.MethodGet, q, apiToken, nil); lErr == nil {
				var existing []struct {
					ID string `json:"id"`
				}
				_ = json.Unmarshal(existingRaw, &existing)
				if len(existing) > 0 {
					recID = existing[0].ID
				}
			}
		}
		if recID == "" {
			return "", fmt.Errorf("delete_dns 需要提供 record_id 或可匹配的 name")
		}
		_, err = e.cloudflareAPIRequest(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recID)), apiToken, nil)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "zone": zoneName, "deleted_record_id": recID})
		return string(b), nil

	case "purge_cache":
		zoneID, zoneName, err := e.resolveCloudflareZoneID(ctx, zoneInput, apiToken)
		if err != nil {
			return "", err
		}
		_, err = e.cloudflareAPIRequest(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/purge_cache", url.PathEscape(zoneID)), apiToken, map[string]any{
			"purge_everything": true,
		})
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "zone": zoneName, "message": "已成功清理该域名 Zone 的全部 Cloudflare 缓存"})
		return string(b), nil

	default:
		return "", fmt.Errorf("不支持的 cloudflare_connector action: %s", action)
	}
}

// ============================================================================
// 4. RSS / Atom Feed Subscription Connector (rss_connector)
// ============================================================================

func rssConnectorToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "rss_connector",
				Description: "RSS / Atom 资讯源订阅应用连接器（读+写全能力）：支持并发抓取已绑定订阅源的最新文章(fetch_latest)、查看当前已配置的全部订阅源(list_feeds)、新增持久化订阅源(add_feed)或移除订阅源(remove_feed)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "操作类型：fetch_latest | list_feeds | add_feed | remove_feed",
						},
						"feed_url": map[string]any{
							"type":        "string",
							"description": "RSS/Atom 订阅源 URL（用于 add_feed / remove_feed，或在 fetch_latest 中临时指定单个源）",
						},
						"feed_name": map[string]any{
							"type":        "string",
							"description": "订阅源备注名称（用于 add_feed / remove_feed）",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "按关键词筛选文章或订阅源（可选）",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "返回最新文章总条数上限（默认 12，最大 30）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		Timeout:        20 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在同步 RSS 资讯订阅源...",
		DomainKeywords: []string{"rss", "订阅源", "atom", "feed", "博客更新", "最新文章"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolRSSConnector(ctx, cleanArgs)
		},
	}
}

type rssFeedConfigEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

func parseConfiguredRSSFeeds(raw string) []rssFeedConfigEntry {
	var out []rssFeedConfigEntry
	seen := make(map[string]bool)
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ';'
	}) {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		name := ""
		u := l
		if sep := strings.Index(l, "|"); sep > 0 {
			name = strings.TrimSpace(l[:sep])
			u = strings.TrimSpace(l[sep+1:])
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			continue
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		if name == "" {
			if parsed, err := url.Parse(u); err == nil {
				name = parsed.Host
			}
		}
		out = append(out, rssFeedConfigEntry{Name: name, URL: u})
	}
	return out
}

func formatConfiguredRSSFeeds(entries []rssFeedConfigEntry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.TrimSpace(e.Name) != "" {
			lines = append(lines, strings.TrimSpace(e.Name)+"|"+strings.TrimSpace(e.URL))
		} else {
			lines = append(lines, strings.TrimSpace(e.URL))
		}
	}
	return strings.Join(lines, "\n")
}

type rssArticleItem struct {
	Feed      string `json:"feed"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"published,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

func (e *Engine) fetchSingleRSSOrAtomFeed(ctx context.Context, entry rssFeedConfigEntry, maxItems int) ([]rssArticleItem, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, entry.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", desktopBrowserUA)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")

	resp, err := e.safeFetchClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}

	// Try RSS 2.0 first
	var rssDoc struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				PubDate     string `xml:"pubDate"`
				Description string `xml:"description"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if xml.Unmarshal(raw, &rssDoc) == nil && len(rssDoc.Channel.Items) > 0 {
		feedTitle := notifier.StripEmoji(strings.TrimSpace(rssDoc.Channel.Title))
		if feedTitle == "" {
			feedTitle = entry.Name
		}
		out := make([]rssArticleItem, 0, maxItems)
		for i, item := range rssDoc.Channel.Items {
			if i >= maxItems {
				break
			}
			out = append(out, rssArticleItem{
				Feed:      feedTitle,
				Title:     notifier.StripEmoji(strings.TrimSpace(item.Title)),
				URL:       strings.TrimSpace(item.Link),
				Published: strings.TrimSpace(item.PubDate),
				Summary:   truncateRunes(notifier.StripEmoji(stripHTMLToText(item.Description)), 200),
			})
		}
		return out, nil
	}

	// Try Atom feed
	var atomDoc struct {
		Title   string `xml:"title"`
		Entries []struct {
			Title   string `xml:"title"`
			Updated string `xml:"updated"`
			Summary string `xml:"summary"`
			Content string `xml:"content"`
			Links   []struct {
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if xml.Unmarshal(raw, &atomDoc) == nil && len(atomDoc.Entries) > 0 {
		feedTitle := notifier.StripEmoji(strings.TrimSpace(atomDoc.Title))
		if feedTitle == "" {
			feedTitle = entry.Name
		}
		out := make([]rssArticleItem, 0, maxItems)
		for i, ent := range atomDoc.Entries {
			if i >= maxItems {
				break
			}
			link := ""
			for _, l := range ent.Links {
				if l.Href != "" && (l.Rel == "" || l.Rel == "alternate") {
					link = l.Href
					break
				}
			}
			if link == "" && len(ent.Links) > 0 {
				link = ent.Links[0].Href
			}
			sum := ent.Summary
			if sum == "" {
				sum = ent.Content
			}
			out = append(out, rssArticleItem{
				Feed:      feedTitle,
				Title:     notifier.StripEmoji(strings.TrimSpace(ent.Title)),
				URL:       strings.TrimSpace(link),
				Published: strings.TrimSpace(ent.Updated),
				Summary:   truncateRunes(notifier.StripEmoji(stripHTMLToText(sum)), 200),
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("未识别到有效的 RSS 2.0 或 Atom XML 条目")
}

func (e *Engine) toolRSSConnector(ctx context.Context, argsJSON string) (string, error) {
	sk, cfg, err := e.loadAppConnectorConfig(store.BuiltinMCPAppRSSID)
	if err != nil {
		return "", err
	}
	feeds := parseConfiguredRSSFeeds(cfg["feeds"])

	var args struct {
		Action   string `json:"action"`
		FeedURL  string `json:"feed_url"`
		FeedName string `json:"feed_name"`
		Query    string `json:"query"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 rss_connector 参数失败: %w", err)
	}
	limit := args.Limit
	if limit <= 0 || limit > 30 {
		limit = 12
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "list_feeds":
		b, _ := json.Marshal(map[string]any{"ok": true, "count": len(feeds), "feeds": feeds})
		return string(b), nil

	case "add_feed":
		u := strings.TrimSpace(args.FeedURL)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return "", fmt.Errorf("add_feed 需要提供以 http:// 或 https:// 开头的 feed_url")
		}
		name := notifier.StripEmoji(strings.TrimSpace(args.FeedName))
		if name == "" {
			if parsed, pErr := url.Parse(u); pErr == nil {
				name = parsed.Host
			}
		}
		for _, f := range feeds {
			if f.URL == u {
				b, _ := json.Marshal(map[string]any{"ok": true, "message": "该订阅源已存在", "feeds": feeds})
				return string(b), nil
			}
		}
		feeds = append(feeds, rssFeedConfigEntry{Name: name, URL: u})
		if cfg == nil {
			cfg = make(map[string]string)
		}
		cfg["feeds"] = formatConfiguredRSSFeeds(feeds)
		rawHdr, _ := json.Marshal(cfg)
		sk.HTTPHeaders = string(rawHdr)
		_ = e.store.SaveSkill(sk)
		e.publishAssistantUpdate("skills")
		b, _ := json.Marshal(map[string]any{"ok": true, "added": rssFeedConfigEntry{Name: name, URL: u}, "total_feeds": len(feeds)})
		return string(b), nil

	case "remove_feed":
		targetURL := strings.TrimSpace(args.FeedURL)
		targetName := strings.ToLower(strings.TrimSpace(args.FeedName))
		next := make([]rssFeedConfigEntry, 0, len(feeds))
		for _, f := range feeds {
			if (targetURL != "" && f.URL == targetURL) || (targetName != "" && strings.Contains(strings.ToLower(f.Name), targetName)) {
				continue
			}
			next = append(next, f)
		}
		if cfg == nil {
			cfg = make(map[string]string)
		}
		cfg["feeds"] = formatConfiguredRSSFeeds(next)
		rawHdr, _ := json.Marshal(cfg)
		sk.HTTPHeaders = string(rawHdr)
		_ = e.store.SaveSkill(sk)
		e.publishAssistantUpdate("skills")
		b, _ := json.Marshal(map[string]any{"ok": true, "total_feeds": len(next), "feeds": next})
		return string(b), nil

	case "fetch_latest":
		targetFeeds := feeds
		if singleURL := strings.TrimSpace(args.FeedURL); singleURL != "" {
			targetFeeds = []rssFeedConfigEntry{{Name: args.FeedName, URL: singleURL}}
		}
		if len(targetFeeds) == 0 {
			return "", fmt.Errorf("当前尚未配置任何 RSS 订阅源，请先调用 add_feed 或在连接器设置中添加订阅源 URL")
		}
		perFeed := 5
		if len(targetFeeds) == 1 {
			perFeed = limit
		}
		var mu sync.Mutex
		var wg sync.WaitGroup
		var allArticles []rssArticleItem
		for _, f := range targetFeeds {
			wg.Add(1)
			go func(entry rssFeedConfigEntry) {
				defer wg.Done()
				items, fErr := e.fetchSingleRSSOrAtomFeed(ctx, entry, perFeed)
				if fErr == nil && len(items) > 0 {
					mu.Lock()
					allArticles = append(allArticles, items...)
					mu.Unlock()
				}
			}(f)
		}
		wg.Wait()

		qLow := strings.ToLower(strings.TrimSpace(args.Query))
		filtered := make([]rssArticleItem, 0, len(allArticles))
		for _, art := range allArticles {
			if qLow != "" && !strings.Contains(strings.ToLower(art.Title), qLow) && !strings.Contains(strings.ToLower(art.Summary), qLow) && !strings.Contains(strings.ToLower(art.Feed), qLow) {
				continue
			}
			filtered = append(filtered, art)
			if len(filtered) >= limit {
				break
			}
		}
		b, _ := json.Marshal(map[string]any{
			"ok":       true,
			"count":    len(filtered),
			"articles": filtered,
		})
		return string(b), nil

	default:
		return "", fmt.Errorf("不支持的 rss_connector action: %s", action)
	}
}

// ============================================================================
// 5. WebDAV / Alist Cloud Storage Connector (webdav_connector)
// ============================================================================

func webdavConnectorToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "webdav_connector",
				Description: "WebDAV / Alist / 坚果云 / Nextcloud 云盘应用连接器（读+写全能力）：支持浏览指定目录文件列表(list_files)、读取云端文本或 Markdown 文件内容(read_file)、上传或保存笔记与文件到云盘(write_file)、新建目录(mkdir)以及删除云端文件(delete_file)。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "操作类型：list_files | read_file | write_file | mkdir | delete_file",
						},
						"path": map[string]any{
							"type":        "string",
							"description": "云盘相对路径（如 / 或 /Notes/daily.md）",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "要写入的文件正文内容（用于 write_file）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		Timeout:        20 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在访问 WebDAV 云盘连接器...",
		DomainKeywords: []string{"webdav", "alist", "坚果云", "nextcloud", "云盘", "网盘", "云端文件"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolWebDAVConnector(ctx, cleanArgs)
		},
	}
}

func buildWebDAVTargetURL(baseURL, relPath string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		return "", fmt.Errorf("WebDAV 连接器尚未配置 base_url")
	}
	parsedBase, err := url.Parse(base)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") {
		return "", fmt.Errorf("WebDAV base_url 非法")
	}
	cleanRel := strings.TrimSpace(relPath)
	cleanRel = strings.TrimPrefix(cleanRel, "/")
	if cleanRel == "" {
		if !strings.HasSuffix(parsedBase.Path, "/") {
			parsedBase.Path += "/"
		}
		return parsedBase.String(), nil
	}
	segments := strings.Split(cleanRel, "/")
	escapedSegs := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			continue
		}
		escapedSegs = append(escapedSegs, url.PathEscape(seg))
	}
	basePath := strings.TrimRight(parsedBase.Path, "/")
	parsedBase.Path = basePath + "/" + strings.Join(escapedSegs, "/")
	parsedBase.RawPath = ""
	return parsedBase.String(), nil
}

type webdavFileEntry struct {
	Name         string `json:"name"`
	Href         string `json:"href"`
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified,omitempty"`
}

func parseWebDAVPropfindXML(raw []byte) []webdavFileEntry {
	var multi struct {
		Responses []struct {
			Href     string `xml:"href"`
			Propstat []struct {
				Prop struct {
					DisplayName  string `xml:"displayname"`
					ContentLen   int64  `xml:"getcontentlength"`
					LastModified string `xml:"getlastmodified"`
					ResourceType struct {
						Collection *struct{} `xml:"collection"`
					} `xml:"resourcetype"`
				} `xml:"prop"`
			} `xml:"propstat"`
		} `xml:"response"`
	}
	if xml.Unmarshal(raw, &multi) != nil {
		return nil
	}
	out := make([]webdavFileEntry, 0, len(multi.Responses))
	for _, r := range multi.Responses {
		hrefDecoded, err := url.PathUnescape(strings.TrimSpace(r.Href))
		if err != nil {
			hrefDecoded = strings.TrimSpace(r.Href)
		}
		var name, lastMod string
		var size int64
		var isDir bool
		for _, ps := range r.Propstat {
			if ps.Prop.DisplayName != "" {
				name = ps.Prop.DisplayName
			}
			if ps.Prop.ContentLen > 0 {
				size = ps.Prop.ContentLen
			}
			if ps.Prop.LastModified != "" {
				lastMod = ps.Prop.LastModified
			}
			if ps.Prop.ResourceType.Collection != nil {
				isDir = true
			}
		}
		if name == "" {
			trimmedHref := strings.TrimRight(hrefDecoded, "/")
			if slash := strings.LastIndexByte(trimmedHref, '/'); slash >= 0 {
				name = trimmedHref[slash+1:]
			} else {
				name = trimmedHref
			}
		}
		out = append(out, webdavFileEntry{
			Name:         name,
			Href:         hrefDecoded,
			IsDir:        isDir,
			Size:         size,
			LastModified: lastMod,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (e *Engine) webdavRequest(ctx context.Context, method, targetURL, user, pass string, depth string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return 0, nil, err
	}
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
	if depth != "" {
		req.Header.Set("Depth", depth)
	}
	req.Header.Set("User-Agent", "Vesper-BotHub-WebDAV/2.83")
	resp, err := e.safeFetchClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("请求 WebDAV 失败: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, raw, fmt.Errorf("WebDAV 返回 HTTP %d: %s", resp.StatusCode, truncateRunes(strings.TrimSpace(string(raw)), 180))
	}
	return resp.StatusCode, raw, nil
}

func (e *Engine) toolWebDAVConnector(ctx context.Context, argsJSON string) (string, error) {
	_, cfg, err := e.loadAppConnectorConfig(store.BuiltinMCPAppWebDAVID)
	if err != nil {
		return "", err
	}
	baseURL := strings.TrimSpace(cfg["base_url"])
	username := strings.TrimSpace(cfg["username"])
	password := strings.TrimSpace(cfg["password"])

	var args struct {
		Action  string `json:"action"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 webdav_connector 参数失败: %w", err)
	}
	targetURL, err := buildWebDAVTargetURL(baseURL, args.Path)
	if err != nil {
		return "", err
	}
	action := strings.ToLower(strings.TrimSpace(args.Action))

	switch action {
	case "list_files":
		_, raw, err := e.webdavRequest(ctx, "PROPFIND", targetURL, username, password, "1", nil)
		if err != nil {
			return "", err
		}
		entries := parseWebDAVPropfindXML(raw)
		b, _ := json.Marshal(map[string]any{"ok": true, "path": args.Path, "count": len(entries), "files": entries})
		return string(b), nil

	case "read_file":
		_, raw, err := e.webdavRequest(ctx, http.MethodGet, targetURL, username, password, "", nil)
		if err != nil {
			return "", err
		}
		content := truncateRunes(notifier.StripEmoji(string(raw)), 8000)
		b, _ := json.Marshal(map[string]any{"ok": true, "path": args.Path, "content": content})
		return string(b), nil

	case "write_file":
		if strings.TrimSpace(args.Path) == "" || strings.TrimSpace(args.Path) == "/" {
			return "", fmt.Errorf("write_file 需要指定具体的文件路径 path（如 /notes/report.md）")
		}
		_, _, err := e.webdavRequest(ctx, http.MethodPut, targetURL, username, password, "", strings.NewReader(args.Content))
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "path": args.Path, "bytes_written": len(args.Content), "message": "文件已成功写入 WebDAV 云盘"})
		return string(b), nil

	case "mkdir":
		_, _, err := e.webdavRequest(ctx, "MKCOL", targetURL, username, password, "", nil)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "path": args.Path, "message": "云盘目录创建成功"})
		return string(b), nil

	case "delete_file":
		if strings.TrimSpace(args.Path) == "" || strings.TrimSpace(args.Path) == "/" {
			return "", fmt.Errorf("禁止删除 WebDAV 根目录")
		}
		_, _, err := e.webdavRequest(ctx, http.MethodDelete, targetURL, username, password, "", nil)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "path": args.Path, "message": "已从 WebDAV 云盘删除"})
		return string(b), nil

	default:
		return "", fmt.Errorf("不支持的 webdav_connector action: %s", action)
	}
}

// ============================================================================
// Live Connectivity Probe for All 5 App Connectors (POST /api/v2/skills/{id}/test)
// ============================================================================

func (e *Engine) TestAppConnector(ctx context.Context, sk store.Skill) (string, error) {
	cfg := sk.HeadersMap()
	switch sk.ID {
	case store.BuiltinMCPAppGitHubID:
		token := strings.TrimSpace(cfg["token"])
		owner := strings.TrimSpace(cfg["default_owner"])
		if token == "" && owner == "" {
			return "", fmt.Errorf("请先填写 GitHub Personal Access Token 或默认用户名 default_owner")
		}
		if token != "" {
			_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, "/user", token, nil)
			if err != nil {
				return "", err
			}
			var u struct {
				Login       string `json:"login"`
				Name        string `json:"name"`
				PublicRepos int    `json:"public_repos"`
			}
			_ = json.Unmarshal(raw, &u)
			return fmt.Sprintf("GitHub 连接成功：当前认证账号 @%s (%s)，公开仓库 %d 个", u.Login, u.Name, u.PublicRepos), nil
		}
		_, raw, err := e.githubAPIRequest(ctx, http.MethodGet, "/users/"+url.PathEscape(owner), "", nil)
		if err != nil {
			return "", err
		}
		var u struct {
			Login       string `json:"login"`
			PublicRepos int    `json:"public_repos"`
		}
		_ = json.Unmarshal(raw, &u)
		return fmt.Sprintf("GitHub 公开连接就绪：默认用户 @%s（公开仓库 %d 个；如需新建 Issue 或重跑 Actions 请补充 Token）", u.Login, u.PublicRepos), nil

	case store.BuiltinMCPAppEmailID:
		emailAddr := strings.TrimSpace(cfg["email"])
		pass := strings.TrimSpace(cfg["password"])
		imapHost := strings.TrimSpace(cfg["imap_host"])
		if emailAddr == "" || pass == "" {
			return "", fmt.Errorf("请先填写邮箱地址与应用专用密码")
		}
		ih, sh := resolveEmailHosts(emailAddr, imapHost, strings.TrimSpace(cfg["smtp_host"]))
		_, totalUnread, err := e.fetchIMAPEmails(ctx, emailAddr, pass, imapHost, true, "", 0, 1)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("邮箱连接成功：%s (IMAP: %s / SMTP: %s) 认证通过，当前收件箱未读邮件 %d 封", emailAddr, ih, sh, totalUnread), nil

	case store.BuiltinMCPAppCloudflareID:
		apiToken := strings.TrimSpace(cfg["api_token"])
		if apiToken == "" {
			return "", fmt.Errorf("请先填写 Cloudflare API Token")
		}
		res, err := e.cloudflareAPIRequest(ctx, http.MethodGet, "/zones?per_page=10", apiToken, nil)
		if err != nil {
			return "", err
		}
		var zones []struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(res, &zones)
		names := make([]string, 0, len(zones))
		for _, z := range zones {
			names = append(names, z.Name)
		}
		return fmt.Sprintf("Cloudflare 连接成功：Token 有效，共识别到 %d 个域名 Zone（%s）", len(zones), strings.Join(names, ", ")), nil

	case store.BuiltinMCPAppRSSID:
		feeds := parseConfiguredRSSFeeds(cfg["feeds"])
		if len(feeds) == 0 {
			return "", fmt.Errorf("请先添加至少 1 个 RSS / Atom 订阅源 URL")
		}
		items, err := e.fetchSingleRSSOrAtomFeed(ctx, feeds[0], 3)
		if err != nil {
			return "", fmt.Errorf("测试首个订阅源 (%s) 失败: %w", feeds[0].URL, err)
		}
		return fmt.Sprintf("RSS 订阅连接成功：共配置 %d 个订阅源，首个订阅源已成功抓取 %d 篇最新文章", len(feeds), len(items)), nil

	case store.BuiltinMCPAppWebDAVID:
		baseURL := strings.TrimSpace(cfg["base_url"])
		user := strings.TrimSpace(cfg["username"])
		pass := strings.TrimSpace(cfg["password"])
		targetURL, err := buildWebDAVTargetURL(baseURL, "/")
		if err != nil {
			return "", err
		}
		_, raw, err := e.webdavRequest(ctx, "PROPFIND", targetURL, user, pass, "1", nil)
		if err != nil {
			return "", err
		}
		entries := parseWebDAVPropfindXML(raw)
		return fmt.Sprintf("WebDAV 云盘连接成功：根目录访问正常（共识别到 %d 个文件/目录条目）", len(entries)), nil

	default:
		return "连接器就绪", nil
	}
}
