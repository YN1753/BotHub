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
	"sort"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/pushstream"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// Engine orchestrates DeepSeek streaming chat, persona injection, long-term memory, and tool calling.
type Engine struct {
	store                 *store.Store
	scheduler             TaskController
	hub                   *pushstream.Hub
	httpClient            *http.Client
	health                *ModelHealthTracker
	allowLocalhostForTest bool
	stashSem              chan struct{}
	biliAPIBase           string
	githubAPIBase         string
	githubRawBase         string
}

func (e *Engine) SetHub(hub *pushstream.Hub) {
	e.hub = hub
}

func (e *Engine) publishAssistantUpdate(kind string) {
	if e != nil && e.hub != nil {
		e.hub.PublishEvent("assistant_update", map[string]any{
			"kind": kind,
			"time": store.NowShanghai(),
		})
	}
}

func NewEngine(st *store.Store, sched TaskController) *Engine {
	return &Engine{
		store:     st,
		scheduler: sched,
		health:    newModelHealthTracker(),
		stashSem:  make(chan struct{}, 3),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ChatRequest struct {
	SessionID      string `json:"session_id"`
	BotID          string `json:"bot_id,omitempty"`
	ThreadID       string `json:"thread_id,omitempty"`
	MsgKind        string `json:"-"`
	Message        string `json:"message"`
	Persona        string `json:"persona"`
	Model          string `json:"model"`
	EnableThinking *bool  `json:"enable_thinking,omitempty"`
	ReplyToID      int64  `json:"reply_to_id,omitempty"`
	ReplyPreview   string `json:"reply_preview,omitempty"`
}

type ToolTrace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	SessionID        string      `json:"session_id"`
	BotID            string      `json:"bot_id,omitempty"`
	ThreadID         string      `json:"thread_id,omitempty"`
	SenderBotID      string      `json:"sender_bot_id,omitempty"`
	Persona          string      `json:"persona"`
	Model            string      `json:"model"`
	ProviderLabel    string      `json:"provider_label,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"`
	Content          string      `json:"content"`
	ToolTraces       []ToolTrace `json:"tool_traces,omitempty"`
	Usage            *TokenUsage `json:"usage,omitempty"`
}

type apiMessage struct {
	Role             string        `json:"role"`
	Content          string        `json:"content"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	ToolCalls        []apiToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

type apiToolCall struct {
	Index    *int            `json:"index,omitempty"`
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function apiToolFunction `json:"function"`
}

type apiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type thinkingConfig struct {
	Type string `json:"type"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatCompletionRequest struct {
	Model         string           `json:"model"`
	Messages      []apiMessage     `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Stream        bool             `json:"stream"`
	StreamOptions *streamOptions   `json:"stream_options,omitempty"`
	Thinking      *thinkingConfig  `json:"thinking,omitempty"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role             string        `json:"role"`
			Content          string        `json:"content"`
			ReasoningContent string        `json:"reasoning_content"`
			ToolCalls        []apiToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *TokenUsage `json:"usage"`
}

func newSessionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sess_%d_%s", time.Now().Unix(), hex.EncodeToString(b))
}

func truncateTitle(msg string) string {
	clean := strings.TrimSpace(notifier.StripEmoji(msg))
	clean = strings.ReplaceAll(clean, "\n", " ")
	runes := []rune(clean)
	if len(runes) > 28 {
		return string(runes[:28]) + "..."
	}
	if clean == "" {
		return "新对话"
	}
	return clean
}

func NormalizeModelID(model string) string {
	m := strings.TrimSpace(model)
	switch m {
	case "", "deepseek-v4-pro":
		return ""
	default:
		return m
	}
}

// CallLightModel executes a non-streaming completion preferring explicit light model config,
// then active Free Model Pool providers (when pool_for_light is enabled), then primary DeepSeek / relay.
func (e *Engine) CallLightModel(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var candidates []ModelTarget

	lightKey, _ := e.store.GetConfig(store.KeyLightAPIKey)
	lightBase, _ := e.store.GetConfig(store.KeyLightBaseURL)
	lightModel, _ := e.store.GetConfig(store.KeyLightModel)
	lightKey = strings.TrimSpace(lightKey)
	lightBase = strings.TrimRight(strings.TrimSpace(lightBase), "/")
	lightModel = strings.TrimSpace(lightModel)

	if lightKey != "" {
		primary := primaryDeepSeekTarget(e.store)
		b := lightBase
		if b == "" {
			b = primary.BaseURL
		}
		m := NormalizeModelID(lightModel)
		if m == "" {
			m = primary.Model
		}
		candidates = append(candidates, ModelTarget{
			ID:       "light",
			Label:    "轻量模型 · " + m,
			Provider: "custom",
			BaseURL:  b,
			APIKey:   lightKey,
			Model:    m,
		})
	}

	if isConfigTrue(e.store, store.KeyPoolForLight, true) {
		candidates = append(candidates, activeFreePoolTargets(e.store)...)
	}

	primary := primaryDeepSeekTarget(e.store)
	if primary.APIKey != "" {
		candidates = append(candidates, primary)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("未配置轻量模型、免费模型池或主力 API Key")
	}
	candidates = e.health.prioritizeHealthy(candidates)

	var lastErr error
	for _, target := range candidates {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		turnStart := time.Now()
		reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		bodyMap := map[string]any{
			"model":  target.Model,
			"stream": false,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
		}
		if target.AllowThinking {
			bodyMap["thinking"] = map[string]string{"type": "disabled"}
		}
		payload, err := json.Marshal(bodyMap)
		if err != nil {
			cancel()
			return "", err
		}

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, BuildChatCompletionsURL(target.BaseURL), bytes.NewReader(payload))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+target.APIKey)
		if target.Provider == "openrouter" {
			req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
			req.Header.Set("X-Title", "BotHub")
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			cancel()
			if ctx.Err() == nil {
				e.health.recordFailure(target.ID, err.Error(), false)
			}
			lastErr = err
			continue
		}
		rawBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errMsg := fmt.Sprintf("%s HTTP %d: %s", target.Label, resp.StatusCode, strings.TrimSpace(string(rawBytes)))
			e.health.recordFailure(target.ID, errMsg, resp.StatusCode == http.StatusTooManyRequests)
			lastErr = fmt.Errorf("%s", errMsg)
			continue
		}

		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(rawBytes, &parsed); err != nil || len(parsed.Choices) == 0 {
			e.health.recordFailure(target.ID, "解析响应失败", false)
			lastErr = fmt.Errorf("%s 解析响应失败", target.Label)
			continue
		}
		e.health.recordSuccess(target.ID, time.Since(turnStart).Milliseconds())
		return notifier.StripEmoji(strings.TrimSpace(parsed.Choices[0].Message.Content)), nil
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("所有轻量及免费模型节点均调用失败")
}

// ChatStream executes a multi-round streaming conversation and invokes emit for SSE events.
// Event types: "session", "reasoning", "tool_start", "tool_result", "content", "done", "error".
func (e *Engine) ChatStream(ctx context.Context, req ChatRequest, emit func(event string, data any)) (*ChatResponse, error) {
	userMsg := notifier.StripEmoji(strings.TrimSpace(req.Message))
	if userMsg == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}

	if strings.TrimSpace(req.BotID) != "" {
		return e.chatStreamForBot(ctx, req, emit)
	}

	targets, err := e.ResolveChatTargets(req.Model)
	if err != nil {
		return nil, err
	}

	activeTarget := targets[0]
	model := activeTarget.Model

	personaID := strings.TrimSpace(req.Persona)
	if personaID == "" {
		personaID, _ = e.store.GetConfig(store.KeyActivePersona)
		personaID = strings.TrimSpace(personaID)
	}
	persona := GetPersona(personaID)

	sessionID := strings.TrimSpace(req.SessionID)
	var session *store.AgentSession
	if sessionID != "" {
		session, _ = e.store.GetAgentSession(sessionID)
	}
	if session == nil {
		if sessionID == "" {
			sessionID = newSessionID()
		}
		session = &store.AgentSession{
			ID:      sessionID,
			Title:   truncateTitle(userMsg),
			Persona: persona.ID,
			Model:   model,
		}
	} else {
		session.Persona = persona.ID
		session.Model = model
		if session.Title == "" || session.Title == "新对话" {
			session.Title = truncateTitle(userMsg)
		}
	}
	if err := e.store.SaveAgentSession(*session); err != nil {
		return nil, fmt.Errorf("保存会话失败: %w", err)
	}

	if emit != nil {
		emit("session", map[string]any{
			"session_id": session.ID,
			"title":      session.Title,
			"persona":    session.Persona,
			"model":      session.Model,
		})
	}

	// Load prior conversation history before appending current user message
	history, _ := e.store.ListAgentMessages(session.ID, 30)

	// Persist current user message
	if _, err := e.store.AppendAgentMessage(store.AgentMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   userMsg,
	}); err != nil {
		return nil, fmt.Errorf("记录用户消息失败: %w", err)
	}

	// Build system prompt with long-term memories
	memories, _ := e.store.ListAgentMemories("", 30)
	sysPrompt := BuildSystemPrompt(persona.ID, memories)

	messages := make([]apiMessage, 0, len(history)+8)
	messages = append(messages, apiMessage{
		Role:    "system",
		Content: sysPrompt,
	})

	for _, m := range history {
		switch m.Role {
		case "user":
			messages = append(messages, apiMessage{
				Role:    "user",
				Content: m.Content,
			})
		case "assistant":
			msg := apiMessage{
				Role:    "assistant",
				Content: m.Content,
			}
			if m.ToolCalls != "" && m.ToolCalls != "[]" {
				var tcs []apiToolCall
				if json.Unmarshal([]byte(m.ToolCalls), &tcs) == nil && len(tcs) > 0 {
					msg.ToolCalls = tcs
				}
			}
			messages = append(messages, msg)
		case "tool":
			content := m.Content
			runes := []rune(content)
			if len(runes) > 600 {
				content = string(runes[:600]) + "\n...[历史工具输出已折叠，核心结论见下方助手回复]"
			}
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    content,
				ToolCallID: m.ToolCallID,
			})
		}
	}

	messages = append(messages, apiMessage{
		Role:    "user",
		Content: userMsg,
	})

	tools := BuiltinTools()
	var totalReasoning strings.Builder
	var finalContent strings.Builder
	traces := make([]ToolTrace, 0)
	var totalUsage TokenUsage
	hasUsage := false

	const maxRounds = 5
	for round := 0; round < maxRounds; round++ {
		roundReasoning, roundContent, toolCalls, roundUsage, usedTarget, err := e.streamRoundWithFailover(ctx, targets, messages, tools, req.EnableThinking, emit, func(deltaReasoning, deltaContent string) {
			if deltaReasoning != "" {
				clean := notifier.StripEmoji(deltaReasoning)
				totalReasoning.WriteString(clean)
				if emit != nil {
					emit("reasoning", map[string]any{"delta": clean})
				}
			}
			if deltaContent != "" {
				clean := notifier.StripEmoji(deltaContent)
				finalContent.WriteString(clean)
				if emit != nil {
					emit("content", map[string]any{"delta": clean})
				}
			}
		})
		if err != nil {
			return nil, err
		}
		model = usedTarget.Model
		if roundUsage != nil {
			hasUsage = true
			totalUsage.PromptTokens += roundUsage.PromptTokens
			totalUsage.CompletionTokens += roundUsage.CompletionTokens
			totalUsage.TotalTokens += roundUsage.TotalTokens
		}

		// If no tool calls were requested, we have our final answer
		if len(toolCalls) == 0 {
			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID:        session.ID,
				Role:             "assistant",
				Content:          notifier.StripEmoji(roundContent),
				ReasoningContent: notifier.StripEmoji(totalReasoning.String()),
				ToolCalls:        "[]",
			})
			break
		}

		// Persist assistant tool-calling turn
		tcBytes, _ := json.Marshal(toolCalls)
		_, _ = e.store.AppendAgentMessage(store.AgentMessage{
			SessionID:        session.ID,
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        string(tcBytes),
		})

		messages = append(messages, apiMessage{
			Role:             "assistant",
			Content:          notifier.StripEmoji(roundContent),
			ReasoningContent: notifier.StripEmoji(roundReasoning),
			ToolCalls:        toolCalls,
		})

		// Execute each requested tool call
		for _, tc := range toolCalls {
			if emit != nil {
				emit("tool_start", map[string]any{
					"id":        tc.ID,
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				})
			}

			toolOutput, toolErr := e.ExecuteTool(ctx, tc.Function.Name, tc.Function.Arguments)
			if toolErr != nil {
				toolOutput = toolErrorPayload(tc.Function.Name, toolErr)
			}
			toolOutput = notifier.StripEmoji(toolOutput)

			traces = append(traces, ToolTrace{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Result:    toolOutput,
			})

			if emit != nil {
				emit("tool_result", map[string]any{
					"id":     tc.ID,
					"name":   tc.Function.Name,
					"result": toolOutput,
				})
			}

			_, _ = e.store.AppendAgentMessage(store.AgentMessage{
				SessionID:  session.ID,
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})

			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
			})
		}
	}

	_ = e.store.SaveAgentSession(*session)

	resp := &ChatResponse{
		SessionID:        session.ID,
		Persona:          persona.ID,
		Model:            model,
		ReasoningContent: totalReasoning.String(),
		Content:          finalContent.String(),
		ToolTraces:       traces,
	}
	if hasUsage {
		resp.Usage = &totalUsage
	}
	if emit != nil {
		emit("done", resp)
	}

	// Background memory distillation using the lightweight model when configured and no explicit remember_fact was invoked
	rememberedInTurn := false
	for _, tr := range traces {
		if tr.Name == "remember_fact" {
			rememberedInTurn = true
			break
		}
	}
	if !rememberedInTurn && strings.TrimSpace(finalContent.String()) != "" && e.hasLightTarget() {
		go e.distillMemoryAsync(userMsg, finalContent.String())
	}

	return resp, nil
}

func (e *Engine) hasLightTarget() bool {
	if e == nil || e.store == nil {
		return false
	}
	if lightKey, _ := e.store.GetConfig(store.KeyLightAPIKey); strings.TrimSpace(lightKey) != "" {
		return true
	}
	if isConfigTrue(e.store, store.KeyPoolForLight, true) && len(activeFreePoolTargets(e.store)) > 0 {
		return true
	}
	return primaryDeepSeekTarget(e.store).APIKey != ""
}

func (e *Engine) distillMemoryAsync(userMsg, assistantReply string) {
	e.distillMemoryWithScope("global", store.BotModeHybrid, userMsg, assistantReply)
}

func normalizeDistilledFactKey(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			prevUnderscore = true
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "_")
}

func normalizeDistilledCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "preference", "relation", "emotion", "insight", "infra", "schedule", "general":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "general"
	}
}

func (e *Engine) distillMemoryWithScope(scope, botMode, userMsg, assistantReply string) {
	if len([]rune(strings.TrimSpace(userMsg))) < 6 || !e.hasLightTarget() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	existingMems, _ := e.store.ListScopedMemories([]string{scope}, "", 25)
	var existingKeysHint string
	if len(existingMems) > 0 {
		pairs := make([]string, 0, len(existingMems))
		for _, m := range existingMems {
			pairs = append(pairs, fmt.Sprintf("%s=%s", m.FactKey, truncateTitle(m.Content)))
		}
		existingKeysHint = "\n当前作用域已有记忆键（若讨论同一主题请沿用对应 fact_key 更新，不要重复造同义新键）：\n- " + strings.Join(pairs, "\n- ")
	}

	var sys string
	if botMode == store.BotModeCompanion {
		sys = `你是专属陪伴助手的情境记忆提炼器。从这一轮自然对话中，判断是否包含值得长期记住的个人点滴（用户最近的心情、喜好禁忌、重要事件、人际关系、聊天约定或生活习惯）或明确否定了旧习惯。
如果有且值得写入或更新，输出纯 JSON 对象（不要 Markdown 代码块）：{"save":true,"action":"upsert","category":"preference|relation|emotion|general","fact_key":"简短英文蛇形键名","content":"自然具体的中文记忆描述","importance":4,"pinned":false}。若用户明确推翻/要求忘掉已有记忆，设置 "action":"forget" 并填对应 fact_key。importance 为 1-5，非常重要的人生节点或核心喜好 pinned 设 true。
如果只是毫无保留价值的日常寒暄（如“在吗”“你好”），输出：{"save":false}。严禁包含任何 Emoji。` + existingKeysHint
	} else {
		sys = `你是长期记忆提炼器。从用户和助手的最新一轮对话中判断是否包含值得长期跨会话记住的稳定事实（如架构约定、偏好风格、重要项目背景、常用主机的非敏感约束）或明确废止了旧约定。
如果有，输出纯 JSON 对象（不要 Markdown 代码块）：{"save":true,"action":"upsert","category":"infra|preference|schedule|insight|general","fact_key":"英文蛇形键名","content":"简洁中文事实","importance":3,"pinned":false}。若用户明确表示废止/取消某条已有记忆，可设 "action":"forget" 并填对应 fact_key。importance 为 1-5。
如果只是一次性闲聊或常规流水查询，输出：{"save":false}。严禁包含任何 Emoji。` + existingKeysHint
	}

	prompt := fmt.Sprintf("用户：%s\n回应：%s", userMsg, assistantReply)
	raw, err := e.CallLightModel(ctx, sys, prompt)
	if err != nil {
		return
	}
	clean := stripJSONCodeFence(raw)

	var parsed struct {
		Save       bool   `json:"save"`
		Action     string `json:"action"`
		Category   string `json:"category"`
		FactKey    string `json:"fact_key"`
		Content    string `json:"content"`
		Importance int    `json:"importance"`
		Pinned     bool   `json:"pinned"`
	}
	if json.Unmarshal([]byte(clean), &parsed) == nil && parsed.Save && strings.TrimSpace(parsed.FactKey) != "" {
		cleanKey := normalizeDistilledFactKey(parsed.FactKey)
		if cleanKey == "" {
			cleanKey = strings.TrimSpace(parsed.FactKey)
		}
		if strings.EqualFold(strings.TrimSpace(parsed.Action), "forget") {
			if deleted, _ := e.store.DeleteAgentMemoryByScopeKey(scope, cleanKey); deleted {
				e.publishAssistantUpdate("memories")
			}
			return
		}
		if strings.TrimSpace(parsed.Content) != "" {
			if parsed.Importance <= 0 {
				parsed.Importance = 3
			} else if parsed.Importance > 5 {
				parsed.Importance = 5
			}
			if err := e.store.SaveAgentMemoryDetailed(scope, normalizeDistilledCategory(parsed.Category), cleanKey, notifier.StripEmoji(parsed.Content), parsed.Importance, parsed.Pinned); err == nil {
				e.publishAssistantUpdate("memories")
			}
		}
	}
}

// SummarizeThreadToMemory distills a multi-turn conversation thread into a concise summary and optional persistent fact.
func (e *Engine) SummarizeThreadToMemory(ctx context.Context, bot *store.Bot, thread *store.BotThread, msgs []store.AgentMessage, saveToMemory bool) (summary string, savedFactKey string, err error) {
	if bot == nil || len(msgs) == 0 {
		return "", "", fmt.Errorf("当前话题暂无可提炼的对话内容")
	}
	var transcript strings.Builder
	for _, m := range msgs {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		who := "用户"
		if m.Role == "assistant" {
			who = bot.Name
		}
		line := strings.TrimSpace(notifier.StripEmoji(m.Content))
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 320 {
			line = string(runes[:320]) + "..."
		}
		transcript.WriteString(fmt.Sprintf("%s: %s\n", who, line))
	}
	if transcript.Len() == 0 {
		return "", "", fmt.Errorf("话题内暂无有效对话文本")
	}

	threadTitle := "主时间线"
	threadKey := "main"
	if thread != nil {
		if strings.TrimSpace(thread.Title) != "" {
			threadTitle = thread.Title
		}
		if strings.TrimSpace(thread.ID) != "" {
			threadKey = thread.ID
		}
	}

	// Deterministic fallback summary if no LLM key is configured
	if !e.hasLightTarget() {
		rawLines := strings.Split(strings.TrimSpace(transcript.String()), "\n")
		if len(rawLines) > 3 {
			rawLines = rawLines[len(rawLines)-3:]
		}
		summary = truncateTitle(strings.Join(rawLines, " | "))
	} else {
		sys := `请对以下话题对话提炼结构化复盘，输出纯 JSON（不要 Markdown 代码块）：
{"summary":"不超过80字的话题核心结论摘要","fact_key":"英文蛇形记忆键名","memory_content":"值得长期记住的关键结论或约定（不超过120字）","importance":4}。严禁包含 Emoji。`
		userText := fmt.Sprintf("话题：%s\n对话：\n%s", threadTitle, transcript.String())
		raw, lErr := e.CallLightModel(ctx, sys, userText)
		if lErr == nil {
			clean := stripJSONCodeFence(raw)
			var parsed struct {
				Summary       string `json:"summary"`
				FactKey       string `json:"fact_key"`
				MemoryContent string `json:"memory_content"`
				Importance    int    `json:"importance"`
			}
			if json.Unmarshal([]byte(clean), &parsed) == nil && strings.TrimSpace(parsed.Summary) != "" {
				summary = notifier.StripEmoji(strings.TrimSpace(parsed.Summary))
				if saveToMemory && strings.TrimSpace(parsed.MemoryContent) != "" {
					fk := strings.TrimSpace(parsed.FactKey)
					if fk == "" {
						fk = "thread_" + threadKey
					}
					imp := parsed.Importance
					if imp <= 0 {
						imp = 4
					}
					scope := "bot:" + bot.ID
					if e.store.SaveAgentMemoryDetailed(scope, "insight", fk, notifier.StripEmoji(parsed.MemoryContent), imp, false) == nil {
						savedFactKey = fk
					}
				}
			}
		}
		if summary == "" {
			summary = truncateTitle(transcript.String())
		}
	}

	if thread != nil && thread.ID != "" {
		thread.Summary = summary
		_ = e.store.SaveBotThread(*thread)
		if e.hub != nil {
			e.hub.PublishEvent("thread_updated", map[string]any{
				"bot_id":    bot.ID,
				"thread_id": thread.ID,
				"summary":   summary,
			})
		}
	}
	if savedFactKey != "" {
		e.publishAssistantUpdate("memories")
	}
	return summary, savedFactKey, nil
}

func (e *Engine) streamRoundWithFailover(
	ctx context.Context,
	targets []ModelTarget,
	messages []apiMessage,
	tools []ToolDefinition,
	enableThinking *bool,
	emit func(event string, data any),
	onDelta func(deltaReasoning, deltaContent string),
) (string, string, []apiToolCall, *TokenUsage, ModelTarget, error) {
	var lastErr error
	tryList := targets
	// Single-node transient resilience: allow one short-backoff retry on 429/5xx network hiccup
	if len(targets) == 1 {
		tryList = []ModelTarget{targets[0], targets[0]}
	}
	for idx, target := range tryList {
		if idx > 0 {
			if len(targets) == 1 {
				timer := time.NewTimer(450 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return "", "", nil, nil, target, ctx.Err()
				case <-timer.C:
				}
			} else if emit != nil {
				reason := ""
				if lastErr != nil {
					reason = truncateTitle(lastErr.Error())
				}
				emit("model_switch", map[string]any{
					"from_label": targets[idx-1].Label,
					"to_label":   target.Label,
					"to_model":   target.Model,
					"reason":     reason,
				})
			}
		}
		var thinkingCfg *thinkingConfig
		if target.AllowThinking {
			enabled := true
			if enableThinking != nil {
				enabled = *enableThinking
			}
			if enabled {
				thinkingCfg = &thinkingConfig{Type: "enabled"}
			} else {
				thinkingCfg = &thinkingConfig{Type: "disabled"}
			}
		}

		turnStart := time.Now()
		emittedAny := false
		rReasoning, rContent, tcs, usage, err := e.streamSingleRoundWithProvider(
			ctx, target.Provider, target.BaseURL, target.APIKey, target.Model,
			messages, tools, thinkingCfg,
			func(dR, dC string) {
				if dR != "" || dC != "" {
					emittedAny = true
				}
				if onDelta != nil {
					onDelta(dR, dC)
				}
			},
		)
		if err == nil {
			e.health.recordSuccess(target.ID, time.Since(turnStart).Milliseconds())
			return rReasoning, rContent, tcs, usage, target, nil
		}
		if ctx.Err() != nil {
			return "", "", nil, nil, target, err
		}
		lastErr = err
		lowerMsg := strings.ToLower(err.Error())
		is429 := strings.Contains(err.Error(), "(429)") || strings.Contains(lowerMsg, "rate")
		isTransient := is429 || strings.Contains(err.Error(), "(502)") || strings.Contains(err.Error(), "(503)") || strings.Contains(err.Error(), "(504)") || strings.Contains(lowerMsg, "eof") || strings.Contains(lowerMsg, "reset")
		if len(targets) == 1 && idx == 0 && !isTransient {
			e.health.recordFailure(target.ID, err.Error(), is429)
			return "", "", nil, nil, target, err
		}
		if len(targets) > 1 || idx > 0 || isTransient {
			e.health.recordFailure(target.ID, err.Error(), is429)
		}
		if emittedAny {
			return "", "", nil, nil, target, err
		}
	}
	if lastErr != nil {
		return "", "", nil, nil, ModelTarget{}, lastErr
	}
	return "", "", nil, nil, ModelTarget{}, fmt.Errorf("无可用模型节点")
}

// trimContextMessages safely drops the oldest middle conversation turns when upstream rejects context length,
// keeping leading system prompts intact and ensuring no orphaned tool responses remain.
func trimContextMessages(messages []apiMessage) ([]apiMessage, bool) {
	sysEnd := 0
	for sysEnd < len(messages) && messages[sysEnd].Role == "system" {
		sysEnd++
	}
	tail := messages[sysEnd:]
	if len(tail) <= 4 {
		return messages, false
	}
	cut := len(tail) / 2
	// Advance cut until we land on a clean user boundary (avoid orphaned assistant tool responses)
	for cut < len(tail)-2 && tail[cut].Role != "user" {
		cut++
	}
	if cut == 0 || cut >= len(tail)-1 {
		return messages, false
	}
	trimmed := make([]apiMessage, 0, sysEnd+len(tail)-cut)
	trimmed = append(trimmed, messages[:sysEnd]...)
	trimmed = append(trimmed, tail[cut:]...)
	return trimmed, true
}

func foldSystemIntoUserMessages(messages []apiMessage) []apiMessage {
	var sysParts []string
	rest := make([]apiMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			if s := strings.TrimSpace(m.Content); s != "" {
				sysParts = append(sysParts, s)
			}
			continue
		}
		rest = append(rest, m)
	}
	if len(sysParts) == 0 {
		return messages
	}
	prefix := "[系统行为指令]\n" + strings.Join(sysParts, "\n") + "\n\n"
	for i := range rest {
		if rest[i].Role == "user" {
			rest[i].Content = prefix + rest[i].Content
			return rest
		}
	}
	return append([]apiMessage{{Role: "user", Content: prefix}}, rest...)
}

func formatUpstreamHTTPError(model string, statusCode int, raw string) error {
	trimmed := strings.TrimSpace(raw)
	// Extract OpenAI-style JSON error message if available
	if strings.HasPrefix(trimmed, "{") {
		var errPayload struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal([]byte(trimmed), &errPayload) == nil {
			msg := strings.TrimSpace(errPayload.Error.Message)
			if msg == "" {
				msg = strings.TrimSpace(errPayload.Message)
			}
			if msg != "" {
				return fmt.Errorf("[%s] 返回错误 (%d): %s", model, statusCode, msg)
			}
		}
	}
	// Strip HTML gateway error pages (e.g., 502/504 Cloudflare HTML) down to <title>
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<html") || strings.Contains(lower, "<!doctype") {
		if title := extractHTMLTitle(trimmed); title != "" {
			return fmt.Errorf("[%s] 网关响应 (%d): %s", model, statusCode, title)
		}
		return fmt.Errorf("[%s] 上游网关异常 (%d)", model, statusCode)
	}
	runes := []rune(trimmed)
	if len(runes) > 280 {
		trimmed = string(runes[:280]) + "..."
	}
	return fmt.Errorf("[%s] 返回错误 (%d): %s", model, statusCode, trimmed)
}

func (e *Engine) streamSingleRoundWithProvider(
	ctx context.Context,
	provider, baseURL, apiKey, model string,
	messages []apiMessage,
	tools []ToolDefinition,
	thinking *thinkingConfig,
	onDelta func(deltaReasoning, deltaContent string),
) (string, string, []apiToolCall, *TokenUsage, error) {
	incUsage := true
	curMessages := messages
	curTools := tools
	curThinking := thinking

	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		var streamOpts *streamOptions
		if incUsage {
			streamOpts = &streamOptions{IncludeUsage: true}
		}
		reqBody := chatCompletionRequest{
			Model:         model,
			Messages:      curMessages,
			Tools:         curTools,
			Stream:        true,
			StreamOptions: streamOpts,
			Thinking:      curThinking,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return "", "", nil, nil, err
		}

		endpoint := BuildChatCompletionsURL(baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return "", "", nil, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if provider == "openrouter" {
			req.Header.Set("HTTP-Referer", "https://github.com/YN1753/BotHub")
			req.Header.Set("X-Title", "BotHub")
		}

		var doErr error
		resp, doErr = e.httpClient.Do(req)
		if doErr != nil {
			return "", "", nil, nil, fmt.Errorf("调用模型 [%s] 接口失败: %w", model, doErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break
		}

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()
		errText := strings.TrimSpace(string(errBody))
		lowerErr := strings.ToLower(errText)

		// Auto-downgrade optional OpenAI params on 400/413/422 or trim oversized history before giving up
		if (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge || resp.StatusCode == http.StatusUnprocessableEntity) && attempt < 2 {
			adjusted := false
			if strings.Contains(lowerErr, "context_length") || strings.Contains(lowerErr, "maximum context") || strings.Contains(lowerErr, "too many tokens") || strings.Contains(lowerErr, "context window") {
				if trimmed, ok := trimContextMessages(curMessages); ok {
					curMessages = trimmed
					adjusted = true
				}
			}
			if curThinking != nil && (strings.Contains(lowerErr, "thinking") || strings.Contains(lowerErr, "unknown")) {
				curThinking = nil
				adjusted = true
			}
			if incUsage && (strings.Contains(lowerErr, "stream_options") || strings.Contains(lowerErr, "include_usage") || strings.Contains(lowerErr, "unknown")) {
				incUsage = false
				adjusted = true
			}
			if len(curTools) > 0 && (strings.Contains(lowerErr, "tool") || strings.Contains(lowerErr, "function")) {
				curTools = nil
				adjusted = true
			}
			if strings.Contains(lowerErr, "developer instruction") || strings.Contains(lowerErr, "system instruction") {
				curMessages = foldSystemIntoUserMessages(curMessages)
				adjusted = true
			}
			if adjusted {
				continue
			}
		}
		return "", "", nil, nil, formatUpstreamHTTPError(model, resp.StatusCode, errText)
	}
	defer resp.Body.Close()

	var reasoningBuf strings.Builder
	var contentBuf strings.Builder
	toolMap := make(map[int]*apiToolCall)
	var finalUsage *TokenUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil && (chunk.Usage.TotalTokens > 0 || chunk.Usage.PromptTokens > 0) {
			finalUsage = chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if delta.ReasoningContent != "" || delta.Content != "" {
			reasoningBuf.WriteString(delta.ReasoningContent)
			contentBuf.WriteString(delta.Content)
			if onDelta != nil {
				onDelta(delta.ReasoningContent, delta.Content)
			}
		}

		for i, tc := range delta.ToolCalls {
			idx := i
			if tc.Index != nil {
				idx = *tc.Index
			}
			entry, exists := toolMap[idx]
			if !exists {
				entry = &apiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: apiToolFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
				toolMap[idx] = entry
			} else {
				if tc.ID != "" && entry.ID == "" {
					entry.ID = tc.ID
				}
				if tc.Function.Name != "" && entry.Function.Name == "" {
					entry.Function.Name = tc.Function.Name
				}
				entry.Function.Arguments += tc.Function.Arguments
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return reasoningBuf.String(), contentBuf.String(), nil, finalUsage, fmt.Errorf("读取流式响应中断: %w", err)
	}

	var toolCalls []apiToolCall
	if len(toolMap) > 0 {
		indices := make([]int, 0, len(toolMap))
		for idx := range toolMap {
			indices = append(indices, idx)
		}
		sort.Ints(indices)
		toolCalls = make([]apiToolCall, 0, len(indices))
		for _, idx := range indices {
			tc := toolMap[idx]
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("call_%d", idx)
			}
			if tc.Function.Arguments == "" {
				tc.Function.Arguments = "{}"
			}
			toolCalls = append(toolCalls, *tc)
		}
	}

	return reasoningBuf.String(), contentBuf.String(), toolCalls, finalUsage, nil
}
