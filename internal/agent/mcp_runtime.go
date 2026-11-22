package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// nativeCoreToolSet defines the cognitive core tools that every Bot inherently possesses
// regardless of bot_mode or mounted MCPs. These tools are never stripped by filterToolsForBot
// and web_search / read_url / mount_mcp are never pruned out by pruneToolsForTurn.
var nativeCoreToolOrder = []string{
	"web_search",
	"read_url",
	"remember_fact",
	"recall_memory",
	"manage_memory",
	"datetime_calc",
	"calculate_eval",
	"consult_bot",
	"mount_mcp",
}

var nativeCoreToolMap = func() map[string]bool {
	m := make(map[string]bool, len(nativeCoreToolOrder))
	for _, name := range nativeCoreToolOrder {
		m[name] = true
	}
	return m
}()

// IsNativeCoreTool reports whether a tool belongs to the permanent Agent Native Core.
func IsNativeCoreTool(name string) bool {
	return nativeCoreToolMap[strings.TrimSpace(name)]
}

// IsPinnedCoreTool reports whether a tool must never be dropped by per-turn tool pruning.
func IsPinnedCoreTool(name string) bool {
	switch strings.TrimSpace(name) {
	case "web_search", "read_url", "mount_mcp", "datetime_calc", "recall_memory", "remember_fact":
		return true
	default:
		return false
	}
}

// NativeCoreTools returns the OpenAI-compatible ToolDefinitions for the Agent Native Core.
func NativeCoreTools() []ToolDefinition {
	out := make([]ToolDefinition, 0, len(nativeCoreToolOrder))
	for _, name := range nativeCoreToolOrder {
		if sp, ok := LookupToolSpec(name); ok {
			out = append(out, sp.Definition)
		}
	}
	return out
}

func mountMCPToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "mount_mcp",
				Description: "动态发现并将一个或多个已创建的 MCP 服务（如个人工作台 mcp_workbench、数字资产 mcp_assets、哪吒探针 mcp_nezha、自动化系统管控 mcp_ops 或用户自建 MCP）永久挂载到当前 Bot。当用户在聊天或定时任务中突然提到某个尚未挂载的 MCP 相关功能时，立即调用此工具挂载，挂载后同一回合即可直接调用其下属工具。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"mcp_id": map[string]any{
							"type":        "string",
							"description": "要挂载的 MCP 服务 ID、标识 slug、名称或目标工具名（例如 mcp_workbench、mcp_assets、mcp_nezha、mcp_ops，或填 all 挂载全部相关 MCP；支持逗号分隔多个）",
						},
					},
					"required": []string{"mcp_id"},
				},
			},
		},
		Timeout:        10 * time.Second,
		ReadOnly:       false,
		StatusLabel:    "正在动态加载并永久挂载 MCP 服务...",
		DomainKeywords: []string{"mcp", "挂载", "加载工具", "扩展服务"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolMountMCP(ctx, cleanArgs)
		},
	}
}

func (e *Engine) toolMountMCP(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		MCPID  string   `json:"mcp_id"`
		MCPIDs []string `json:"mcp_ids"`
		Name   string   `json:"name"`
		Slug   string   `json:"slug"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 mount_mcp 参数失败: %w", err)
	}

	var queries []string
	for _, raw := range append(args.MCPIDs, args.MCPID, args.Name, args.Slug) {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == '，' || r == ';' || r == ' '
		}) {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				queries = append(queries, trimmed)
			}
		}
	}
	if len(queries) == 0 {
		return "", fmt.Errorf("请提供要挂载的 mcp_id 或名称")
	}

	sc := toolScopeFromContext(ctx)
	botID := strings.TrimSpace(sc.BotID)
	if botID == "" {
		botID = strings.TrimSpace(sc.TimelineBotID)
	}
	if botID == "" {
		return "", fmt.Errorf("当前上下文未绑定 Bot，无法挂载 MCP")
	}
	bot, err := e.store.GetBot(botID)
	if err != nil || bot == nil {
		return "", fmt.Errorf("未找到当前 Bot: %s", botID)
	}

	allSkills, err := e.store.ListSkills()
	if err != nil {
		return "", err
	}

	var matched []store.Skill
	matchedSet := make(map[string]bool)
	for _, q := range queries {
		qLow := strings.ToLower(q)
		for _, sk := range allSkills {
			if !sk.Enabled || matchedSet[sk.ID] {
				continue
			}
			if qLow == "all" || qLow == "*" ||
				strings.EqualFold(sk.ID, q) ||
				strings.EqualFold(sk.Slug, q) ||
				strings.EqualFold(sk.Name, q) ||
				strings.Contains(strings.ToLower(sk.Name), qLow) ||
				strings.Contains(strings.ToLower(sk.Slug), qLow) ||
				strings.Contains(strings.ToLower(sk.ID), qLow) {
				matched = append(matched, sk)
				matchedSet[sk.ID] = true
				continue
			}
			for _, tName := range mcpExposedToolNames(sk) {
				if strings.EqualFold(tName, q) || strings.Contains(strings.ToLower(tName), qLow) {
					matched = append(matched, sk)
					matchedSet[sk.ID] = true
					break
				}
			}
		}
	}

	if len(matched) == 0 {
		avail := make([]string, 0, len(allSkills))
		for _, sk := range allSkills {
			if sk.Enabled {
				avail = append(avail, fmt.Sprintf("%s(%s)", sk.ID, sk.Name))
			}
		}
		return "", fmt.Errorf("未找到匹配的已启用 MCP 服务: %v；当前可用 MCP: %s", queries, strings.Join(avail, ", "))
	}

	mountedSkills, unlockedTools := e.persistMountedMCPsOnBot(bot, matched)
	mountedNames := make([]string, 0, len(mountedSkills))
	for _, sk := range mountedSkills {
		mountedNames = append(mountedNames, fmt.Sprintf("%s(%s)", sk.Name, sk.ID))
	}

	b, _ := json.Marshal(map[string]any{
		"ok":             true,
		"bot_id":         bot.ID,
		"bot_name":       bot.Name,
		"mounted_mcps":   mountedNames,
		"unlocked_tools": unlockedTools,
		"hint":           "上述 MCP 服务已永久挂载到你的配置中，且其下属工具已在本回合实时解锁，请立即在下一步直接调用对应工具完成用户需求。",
	})
	return string(b), nil
}

func (e *Engine) persistMountedMCPsOnBot(bot *store.Bot, toMount []store.Skill) ([]store.Skill, []string) {
	if bot == nil || len(toMount) == 0 {
		return nil, nil
	}
	cur := bot.SkillIDList()
	have := make(map[string]bool, len(cur))
	for _, id := range cur {
		have[strings.TrimSpace(id)] = true
	}
	changed := false
	for _, sk := range toMount {
		if !have[sk.ID] {
			cur = append(cur, sk.ID)
			have[sk.ID] = true
			changed = true
		}
	}
	if changed {
		raw, _ := json.Marshal(cur)
		bot.SkillIDs = string(raw)
		if e != nil && e.store != nil {
			_ = e.store.SaveBot(*bot)
			e.publishAssistantUpdate("bots")
		}
	}
	var unlocked []string
	for _, sk := range toMount {
		unlocked = append(unlocked, mcpExposedToolNames(sk)...)
	}
	return toMount, unlocked
}

func mcpExposedToolNames(sk store.Skill) []string {
	switch store.NormalizeSkillKind(sk.Kind) {
	case store.SkillKindBuiltinMCP:
		return sk.BuiltinMCPToolNames()
	case store.SkillKindMCP:
		tools := sk.MCPTools()
		if len(tools) == 0 {
			return []string{MCPToolFunctionName(sk.Slug, "call")}
		}
		out := make([]string, 0, len(tools))
		for _, t := range tools {
			out = append(out, MCPToolFunctionName(sk.Slug, t.Name))
		}
		return out
	case store.SkillKindWebhook:
		return []string{SkillToolName(sk.Slug)}
	default:
		return nil
	}
}

// extractCoreGoalFromRoutinePrompt strips system-injected routine wrappers ([今日实时上下文快照],
// [历史已推送去重账本], 执行铁律) so tool pruning and MCP auto-mounting only inspect
// the user's actual task or chat message.
func extractCoreGoalFromRoutinePrompt(msg string) string {
	trimmed := strings.TrimSpace(msg)
	if idx := strings.Index(trimmed, "[本次例程任务目标]"); idx >= 0 {
		rest := strings.TrimSpace(trimmed[idx+len("[本次例程任务目标]"):])
		if cut := strings.Index(rest, "[用户对上一条推送不满意"); cut > 0 {
			rest = strings.TrimSpace(rest[:cut])
		}
		if cut := strings.Index(rest, "[系统质量拦截强制重写"); cut > 0 {
			rest = strings.TrimSpace(rest[:cut])
		}
		if rest != "" {
			return rest
		}
	}
	return trimmed
}

// autoMountRelevantMCPs inspects the current user message (or core routine goal), discovers any
// enabled but unmounted MCP services in the store that match the intent, permanently mounts them
// onto actingBot, and returns the newly mounted skills.
func (e *Engine) autoMountRelevantMCPs(actingBot *store.Bot, rawUserMsg string) []store.Skill {
	if e == nil || e.store == nil || actingBot == nil {
		return nil
	}
	goal := strings.ToLower(strings.TrimSpace(extractCoreGoalFromRoutinePrompt(rawUserMsg)))
	if goal == "" {
		return nil
	}
	allSkills, err := e.store.ListSkills()
	if err != nil || len(allSkills) == 0 {
		return nil
	}
	mountedSet := make(map[string]bool)
	for _, id := range actingBot.SkillIDList() {
		mountedSet[strings.TrimSpace(id)] = true
	}

	var newlyMatched []store.Skill
	for _, sk := range allSkills {
		if !sk.Enabled || mountedSet[sk.ID] {
			continue
		}
		if shouldAutoMountMCP(sk, goal) {
			newlyMatched = append(newlyMatched, sk)
			mountedSet[sk.ID] = true
		}
	}
	if len(newlyMatched) == 0 {
		return nil
	}
	e.persistMountedMCPsOnBot(actingBot, newlyMatched)
	return newlyMatched
}

func shouldAutoMountMCP(sk store.Skill, goalLower string) bool {
	nameLow := strings.ToLower(strings.TrimSpace(sk.Name))
	slugLow := strings.ToLower(strings.TrimSpace(sk.Slug))
	idLow := strings.ToLower(strings.TrimSpace(sk.ID))

	// 1. Direct mention of MCP name, ID, or non-trivial slug
	if nameLow != "" {
		coreName := strings.TrimSpace(strings.TrimSuffix(nameLow, "mcp"))
		if strings.Contains(goalLower, nameLow) || (len([]rune(coreName)) >= 2 && strings.Contains(goalLower, coreName)) {
			return true
		}
	}
	if idLow != "" && strings.Contains(goalLower, idLow) {
		return true
	}
	if len(slugLow) >= 3 && strings.Contains(goalLower, slugLow) {
		return true
	}

	// 2. Mention of any tool name inside this MCP
	for _, t := range sk.MCPTools() {
		tLow := strings.ToLower(strings.TrimSpace(t.Name))
		if len(tLow) >= 4 && strings.Contains(goalLower, tLow) {
			return true
		}
	}

	// 3. Explicit trigger keywords (for built-in domain MCPs or user-configured keywords)
	if sk.IsBuiltinMCP() {
		for _, kw := range sk.TriggerKeywords() {
			if kw != "" && strings.Contains(goalLower, kw) {
				return true
			}
		}
		return false
	}

	// 4. Custom external MCP / Webhook / Prompt matching by domain token overlap
	tokens := extractMatchTokens(sk.Name + " " + sk.Description)
	hits := 0
	for _, tok := range tokens {
		if tok == "mcp" || tok == "服务" || tok == "工具" || tok == "技能" || tok == "查询" || tok == "管理" {
			continue
		}
		if len([]rune(tok)) >= 2 && strings.Contains(goalLower, tok) {
			hits++
			if hits >= 2 {
				return true
			}
		}
	}
	return false
}

var reMCPToolTokenClean = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// MCPToolFunctionName formats a remote MCP tool into an OpenAI-compatible function name: mcp_<slug>__<tool>.
func MCPToolFunctionName(slug, toolName string) string {
	cleanSlug := reMCPToolTokenClean.ReplaceAllString(strings.ToLower(strings.TrimSpace(slug)), "_")
	cleanSlug = strings.Trim(cleanSlug, "_")
	if cleanSlug == "" {
		cleanSlug = "server"
	}
	cleanTool := reMCPToolTokenClean.ReplaceAllString(strings.TrimSpace(toolName), "_")
	cleanTool = strings.Trim(cleanTool, "_")
	if cleanTool == "" {
		cleanTool = "call"
	}
	full := "mcp_" + cleanSlug + "__" + cleanTool
	if len(full) > 64 {
		full = full[:64]
	}
	return full
}

func splitMCPToolFunctionName(fnName string) (slug string, remoteTool string, ok bool) {
	if !strings.HasPrefix(fnName, "mcp_") {
		return "", "", false
	}
	rest := strings.TrimPrefix(fnName, "mcp_")
	idx := strings.Index(rest, "__")
	if idx <= 0 || idx+2 >= len(rest) {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
}

// CompileMCPServerTools converts a remote MCP server (Kind == "mcp") into OpenAI Function Calling ToolDefinitions.
func CompileMCPServerTools(sk store.Skill) []ToolDefinition {
	discovered := sk.MCPTools()
	if len(discovered) == 0 {
		return []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        MCPToolFunctionName(sk.Slug, "call"),
					Description: fmt.Sprintf("调用远程 MCP 服务「%s」(%s)", sk.Name, sk.Description),
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"tool_name": map[string]any{
								"type":        "string",
								"description": "远程 MCP 工具名称",
							},
							"arguments": map[string]any{
								"type":        "object",
								"description": "传递给该 MCP 工具的参数对象",
							},
						},
						"required": []string{"tool_name"},
					},
				},
			},
		}
	}

	out := make([]ToolDefinition, 0, len(discovered))
	for _, dt := range discovered {
		schema := dt.InputSchema
		if len(schema) == 0 {
			schema = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		} else if _, hasType := schema["type"]; !hasType {
			copied := make(map[string]any, len(schema)+1)
			for k, v := range schema {
				copied[k] = v
			}
			copied["type"] = "object"
			schema = copied
		}
		desc := strings.TrimSpace(dt.Description)
		if desc == "" {
			desc = fmt.Sprintf("调用 MCP 服务「%s」工具 %s", sk.Name, dt.Name)
		} else {
			desc = fmt.Sprintf("[%s] %s", sk.Name, desc)
		}
		out = append(out, ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        MCPToolFunctionName(sk.Slug, dt.Name),
				Description: desc,
				Parameters:  schema,
			},
		})
	}
	return out
}

type mcpJSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// sendMCPJSONRPC sends a JSON-RPC 2.0 request to a remote MCP server (supporting both Streamable HTTP JSON
// and text/event-stream SSE responses) and returns the decoded response + optional Mcp-Session-Id.
func (e *Engine) sendMCPJSONRPC(ctx context.Context, sk store.Skill, endpointURL, sessionID string, payload mcpJSONRPCRequest) (*mcpJSONRPCResponse, string, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, sessionID, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, sessionID, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "Vesper-BotHub-MCP/2.82")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	for hk, hv := range sk.HeadersMap() {
		k := strings.TrimSpace(hk)
		if k != "" {
			req.Header.Set(k, strings.TrimSpace(hv))
		}
	}

	client := e.safeFetchClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, sessionID, fmt.Errorf("连接远程 MCP 服务失败: %w", err)
	}
	defer resp.Body.Close()

	if sid := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sid != "" {
		sessionID = sid
	}

	const maxMCPBytes = 256 * 1024
	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxMCPBytes))
	if err != nil {
		return nil, sessionID, fmt.Errorf("读取 MCP 响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusAccepted && payload.ID == nil {
		return &mcpJSONRPCResponse{JSONRPC: "2.0"}, sessionID, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := strings.TrimSpace(string(rawBytes))
		if len([]rune(preview)) > 240 {
			preview = string([]rune(preview)[:240]) + "..."
		}
		return nil, sessionID, fmt.Errorf("MCP 服务返回 HTTP %d: %s", resp.StatusCode, preview)
	}

	if payload.ID == nil {
		return &mcpJSONRPCResponse{JSONRPC: "2.0"}, sessionID, nil
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	trimmed := strings.TrimSpace(string(rawBytes))
	if strings.Contains(ct, "text/event-stream") || strings.HasPrefix(trimmed, "event:") || strings.HasPrefix(trimmed, "data:") {
		scanner := bufio.NewScanner(strings.NewReader(trimmed))
		scanner.Buffer(make([]byte, 0, 64*1024), maxMCPBytes)
		var lastRPC *mcpJSONRPCResponse
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataStr == "" || !strings.HasPrefix(dataStr, "{") {
				continue
			}
			var rpc mcpJSONRPCResponse
			if json.Unmarshal([]byte(dataStr), &rpc) == nil && (len(rpc.Result) > 0 || rpc.Error != nil) {
				lastRPC = &rpc
			}
		}
		if lastRPC != nil {
			if lastRPC.Error != nil {
				return nil, sessionID, fmt.Errorf("MCP 错误 (%d): %s", lastRPC.Error.Code, lastRPC.Error.Message)
			}
			return lastRPC, sessionID, nil
		}
	}

	var rpc mcpJSONRPCResponse
	if err := json.Unmarshal(rawBytes, &rpc); err != nil {
		return nil, sessionID, fmt.Errorf("解析 MCP JSON-RPC 响应失败: %w", err)
	}
	if rpc.Error != nil {
		return nil, sessionID, fmt.Errorf("MCP 错误 (%d): %s", rpc.Error.Code, rpc.Error.Message)
	}
	return &rpc, sessionID, nil
}

// DiscoverMCPServerTools connects to a remote MCP server via JSON-RPC 2.0 (initialize -> tools/list)
// and returns its exposed tool schemas.
func (e *Engine) DiscoverMCPServerTools(ctx context.Context, sk store.Skill) ([]store.MCPToolMeta, error) {
	endpoint := strings.TrimSpace(sk.HTTPURL)
	if endpoint == "" {
		return nil, fmt.Errorf("MCP 服务未配置 URL")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("MCP 服务 URL 必须以 http:// 或 https:// 开头")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()

	// Step 1: initialize handshake
	initReq := mcpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "Vesper-BotHub-MCP",
				"version": "2.82.0",
			},
		},
	}
	_, sessionID, initErr := e.sendMCPJSONRPC(reqCtx, sk, endpoint, "", initReq)
	if initErr == nil {
		// Best-effort notifications/initialized
		_, sessionID, _ = e.sendMCPJSONRPC(reqCtx, sk, endpoint, sessionID, mcpJSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "notifications/initialized",
		})
	}

	// Step 2: tools/list
	listResp, _, listErr := e.sendMCPJSONRPC(reqCtx, sk, endpoint, sessionID, mcpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if listErr != nil {
		if initErr != nil {
			return nil, initErr
		}
		return nil, listErr
	}

	var result struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
			AltSchema   map[string]any `json:"input_schema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(listResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析 MCP tools/list 结果失败: %w", err)
	}

	tools := make([]store.MCPToolMeta, 0, len(result.Tools))
	for _, t := range result.Tools {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		schema := t.InputSchema
		if len(schema) == 0 {
			schema = t.AltSchema
		}
		tools = append(tools, store.MCPToolMeta{
			Name:        name,
			Description: notifier.StripEmoji(strings.TrimSpace(t.Description)),
			InputSchema: schema,
		})
	}
	return tools, nil
}

// ExecuteRemoteMCPTool invokes tools/call on a remote MCP server (mcp_<slug>__<tool_name>).
func (e *Engine) ExecuteRemoteMCPTool(ctx context.Context, fnName, cleanArgs string) (string, error) {
	slug, remoteTool, ok := splitMCPToolFunctionName(fnName)
	if !ok {
		return "", fmt.Errorf("无效的 MCP 工具名: %s", fnName)
	}
	sk, err := e.store.GetSkillBySlug(slug)
	if err != nil || sk == nil || !sk.Enabled || store.NormalizeSkillKind(sk.Kind) != store.SkillKindMCP {
		return "", fmt.Errorf("未找到或未启用远程 MCP 服务: %s", slug)
	}

	var argsObj map[string]any
	if cleanArgs != "" && cleanArgs != "{}" {
		if err := json.Unmarshal([]byte(cleanArgs), &argsObj); err != nil {
			return "", fmt.Errorf("解析 MCP 工具参数 JSON 失败: %w", err)
		}
	}
	if argsObj == nil {
		argsObj = map[string]any{}
	}

	// Support generic mcp_<slug>__call fallback
	if remoteTool == "call" {
		if tn, ok := argsObj["tool_name"].(string); ok && strings.TrimSpace(tn) != "" {
			remoteTool = strings.TrimSpace(tn)
			if nested, ok := argsObj["arguments"].(map[string]any); ok {
				argsObj = nested
			} else {
				delete(argsObj, "tool_name")
			}
		}
	}

	endpoint := strings.TrimSpace(sk.HTTPURL)
	if endpoint == "" {
		return "", fmt.Errorf("MCP 服务「%s」未配置 URL", sk.Name)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 22*time.Second)
	defer cancel()

	initReq := mcpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "Vesper-BotHub-MCP",
				"version": "2.82.0",
			},
		},
	}
	_, sessionID, _ := e.sendMCPJSONRPC(reqCtx, *sk, endpoint, "", initReq)
	if sessionID != "" {
		_, sessionID, _ = e.sendMCPJSONRPC(reqCtx, *sk, endpoint, sessionID, mcpJSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "notifications/initialized",
		})
	}

	callResp, _, callErr := e.sendMCPJSONRPC(reqCtx, *sk, endpoint, sessionID, mcpJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      remoteTool,
			"arguments": argsObj,
		},
	})
	if callErr != nil {
		return "", callErr
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if json.Unmarshal(callResp.Result, &callResult) == nil && len(callResult.Content) > 0 {
		var parts []string
		for _, c := range callResult.Content {
			if strings.TrimSpace(c.Text) != "" {
				parts = append(parts, notifier.StripEmoji(strings.TrimSpace(c.Text)))
			}
		}
		joined := strings.Join(parts, "\n\n")
		if callResult.IsError {
			return "", fmt.Errorf("MCP 工具 %s 返回错误: %s", remoteTool, joined)
		}
		out, _ := json.Marshal(map[string]any{
			"ok":     true,
			"mcp":    sk.Name,
			"tool":   remoteTool,
			"result": joined,
		})
		return string(out), nil
	}

	rawResult := notifier.StripEmoji(strings.TrimSpace(string(callResp.Result)))
	out, _ := json.Marshal(map[string]any{
		"ok":     true,
		"mcp":    sk.Name,
		"tool":   remoteTool,
		"result": rawResult,
	})
	return string(out), nil
}

// ParseMCPServersJSON parses standard Claude/Cursor/Muse style {"mcpServers": {...}} or single MCP server JSON
// into ready-to-save store.Skill structs.
func ParseMCPServersJSON(rawJSON string) ([]store.Skill, error) {
	cleaned := stripJSONCodeFence(rawJSON)
	if cleaned == "" {
		return nil, fmt.Errorf("JSON 配置内容不能为空")
	}

	type serverEntry struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		URL         string            `json:"url"`
		ServerURL   string            `json:"serverUrl"`
		Endpoint    string            `json:"endpoint"`
		HTTPURL     string            `json:"http_url"`
		Headers     map[string]string `json:"headers"`
	}

	var wrapper struct {
		MCPServers map[string]serverEntry `json:"mcpServers"`
	}
	entries := make(map[string]serverEntry)
	if err := json.Unmarshal([]byte(cleaned), &wrapper); err == nil && len(wrapper.MCPServers) > 0 {
		entries = wrapper.MCPServers
	} else {
		var single serverEntry
		if err := json.Unmarshal([]byte(cleaned), &single); err == nil && (single.URL != "" || single.ServerURL != "" || single.Endpoint != "" || single.HTTPURL != "") {
			key := strings.TrimSpace(single.Name)
			if key == "" {
				key = "custom_mcp"
			}
			entries[key] = single
		} else {
			var directMap map[string]serverEntry
			if err := json.Unmarshal([]byte(cleaned), &directMap); err == nil && len(directMap) > 0 {
				for k, v := range directMap {
					if v.URL != "" || v.ServerURL != "" || v.Endpoint != "" || v.HTTPURL != "" {
						entries[k] = v
					}
				}
			}
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("未识别到有效的 MCP 服务配置（需包含 mcpServers 或 url 字段）")
	}

	out := make([]store.Skill, 0, len(entries))
	for key, entry := range entries {
		targetURL := strings.TrimSpace(entry.URL)
		if targetURL == "" {
			targetURL = strings.TrimSpace(entry.ServerURL)
		}
		if targetURL == "" {
			targetURL = strings.TrimSpace(entry.Endpoint)
		}
		if targetURL == "" {
			targetURL = strings.TrimSpace(entry.HTTPURL)
		}
		if targetURL == "" {
			continue
		}
		name := notifier.StripEmoji(strings.TrimSpace(entry.Name))
		if name == "" {
			name = notifier.StripEmoji(strings.TrimSpace(key))
		}
		slug := reMCPToolTokenClean.ReplaceAllString(strings.ToLower(strings.TrimSpace(key)), "_")
		slug = strings.Trim(slug, "_")
		if slug == "" {
			slug = "mcp_ext"
		}
		hdrBytes, _ := json.Marshal(entry.Headers)
		if len(entry.Headers) == 0 {
			hdrBytes = []byte("{}")
		}
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		desc := notifier.StripEmoji(strings.TrimSpace(entry.Description))
		if desc == "" {
			desc = "远程标准 MCP 服务: " + name
		}
		out = append(out, store.Skill{
			ID:           "mcp_" + hex.EncodeToString(b),
			Name:         name,
			Slug:         slug,
			Kind:         store.SkillKindMCP,
			Description:  desc,
			HTTPMethod:   "POST",
			HTTPURL:      targetURL,
			HTTPHeaders:  string(hdrBytes),
			ParamsSchema: "[]",
			Enabled:      true,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("MCP 配置中缺少有效的 http/https url")
	}
	return out, nil
}

var noInternetExcusePhrases = []string{
	"没有联网方式",
	"没有联网工具",
	"没有联网能力",
	"无法联网",
	"不能联网",
	"不具备联网",
	"没有实时联网",
	"无法访问互联网",
	"无法访问外部网络",
	"无法获取实时",
	"没有搜索工具",
	"无法进行实时搜索",
	"无法查询实时",
	"no internet access",
	"cannot browse the internet",
	"cannot access real-time",
}

// containsNoInternetExcuse detects when a model falsely claims it has no way to access the internet.
func containsNoInternetExcuse(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, phrase := range noInternetExcusePhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

var liveWebRoutineHints = []string{
	"联网", "搜索", "实时", "今日", "今天", "最新", "新闻", "动态", "热点", "趋势",
	"资讯", "早报", "日报", "雷达", "开源", "发布", "行情", "赛事", "比分", "更新",
	"汇总", "速递", "盘点", "搜集", "抓取",
}

// routineNeedsLiveWebSearch reports whether a scheduled routine task's core goal
// expects fresh external/live web information.
func routineNeedsLiveWebSearch(rawMsg string) bool {
	goal := strings.ToLower(strings.TrimSpace(extractCoreGoalFromRoutinePrompt(rawMsg)))
	if goal == "" {
		return false
	}
	for _, kw := range liveWebRoutineHints {
		if strings.Contains(goal, kw) {
			return true
		}
	}
	return false
}

// buildFallbackWebSearchQuery distills a concise search query from a routine or chat prompt
// when automatic fallback web_search is triggered.
func buildFallbackWebSearchQuery(rawMsg string) string {
	goal := strings.TrimSpace(extractCoreGoalFromRoutinePrompt(rawMsg))
	goal, _ = extractPromptExamples(goal)
	lines := strings.Split(goal, "\n")
	var picked []string
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "[") {
			continue
		}
		l = strings.TrimLeft(l, "-*0123456789.、 ")
		if l != "" {
			picked = append(picked, l)
			if len(picked) >= 2 {
				break
			}
		}
	}
	joined := strings.Join(picked, " ")
	if joined == "" {
		joined = goal
	}
	q, _ := distillSearchKeywords(joined)
	if strings.TrimSpace(q) == "" {
		runes := []rune(joined)
		if len(runes) > 48 {
			runes = runes[:48]
		}
		q = strings.TrimSpace(string(runes))
	}
	return q
}

func hasExecutedWebSearch(traces []ToolTrace) bool {
	for _, tr := range traces {
		if tr.Name == "web_search" || tr.Name == "read_url" {
			return true
		}
	}
	return false
}

