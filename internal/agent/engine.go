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
}

func (e *Engine) SetHub(hub *pushstream.Hub) {
	e.hub = hub
}

func NewEngine(st *store.Store, sched TaskController) *Engine {
	return &Engine{
		store:     st,
		scheduler: sched,
		health:    newModelHealthTracker(),
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ChatRequest struct {
	SessionID      string `json:"session_id"`
	BotID          string `json:"bot_id,omitempty"`
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
	Name             string        `json:"name,omitempty"`
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
	case "", "deepseek-chat", "deepseek-reasoner", "deepseek-v4-pro":
		return store.DefaultAIModel // "deepseek-flash" (DeepSeek Flash only)
	default:
		return m
	}
}

// CallLightModel executes a non-streaming completion preferring explicit light model config,
// then active Free Model Pool providers (when pool_for_light is enabled), then primary DeepSeek Flash / relay.
func (e *Engine) CallLightModel(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var candidates []ModelTarget

	lightKey, _ := e.store.GetConfig(store.KeyLightAPIKey)
	lightBase, _ := e.store.GetConfig(store.KeyLightBaseURL)
	lightModel, _ := e.store.GetConfig(store.KeyLightModel)
	lightKey = strings.TrimSpace(lightKey)
	lightBase = strings.TrimRight(strings.TrimSpace(lightBase), "/")
	lightModel = strings.TrimSpace(lightModel)

	if lightKey != "" {
		b := lightBase
		if b == "" {
			b = primaryDeepSeekTarget(e.store).BaseURL
		}
		m := NormalizeModelID(lightModel)
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

	var lastErr error
	for _, target := range candidates {
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
			req.Header.Set("HTTP-Referer", "https://bot.starryheavens.uk")
			req.Header.Set("X-Title", "Vesper BotHub")
		}

		resp, err := e.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		rawBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s HTTP %d: %s", target.Label, resp.StatusCode, strings.TrimSpace(string(rawBytes)))
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
			lastErr = fmt.Errorf("%s 解析响应失败", target.Label)
			continue
		}
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

	targets, err := e.ResolveChatTargets(req.Model)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.BotID) != "" {
		return e.chatStreamForBot(ctx, req, targets[0].BaseURL, targets[0].APIKey, emit)
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
				errPayload, _ := json.Marshal(map[string]any{
					"ok":    false,
					"error": toolErr.Error(),
				})
				toolOutput = string(errPayload)
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
	if !rememberedInTurn && strings.TrimSpace(finalContent.String()) != "" {
		if lightKey, _ := e.store.GetConfig(store.KeyLightAPIKey); strings.TrimSpace(lightKey) != "" {
			go e.distillMemoryAsync(userMsg, finalContent.String())
		}
	}

	return resp, nil
}

func (e *Engine) distillMemoryAsync(userMsg, assistantReply string) {
	if len([]rune(userMsg)) < 8 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sys := `你是长期记忆提炼器。从用户和助手的最新一轮对话中判断是否包含值得长期跨会话记住的稳定事实（如服务器地址、个人偏好、常用域名、架构约定）。
如果有，输出纯 JSON 对象（不要 Markdown 代码块）：{"save":true,"category":"infra|preference|schedule|general","fact_key":"英文蛇形键名","content":"简洁中文事实"}。
如果只是一次性闲聊、常规状态查询或无新长期事实，输出：{"save":false}。严禁包含任何 Emoji。`
	prompt := fmt.Sprintf("用户：%s\n助手：%s", userMsg, assistantReply)
	raw, err := e.CallLightModel(ctx, sys, prompt)
	if err != nil {
		return
	}
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var parsed struct {
		Save     bool   `json:"save"`
		Category string `json:"category"`
		FactKey  string `json:"fact_key"`
		Content  string `json:"content"`
	}
	if json.Unmarshal([]byte(clean), &parsed) == nil && parsed.Save && strings.TrimSpace(parsed.FactKey) != "" && strings.TrimSpace(parsed.Content) != "" {
		_ = e.store.SaveAgentMemory(parsed.Category, parsed.FactKey, notifier.StripEmoji(parsed.Content))
	}
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
	for idx, target := range targets {
		if idx > 0 && emit != nil {
			emit("model_switch", map[string]any{
				"from_label": targets[idx-1].Label,
				"to_label":   target.Label,
				"to_model":   target.Model,
			})
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
		lastErr = err
		is429 := strings.Contains(err.Error(), "(429)") || strings.Contains(strings.ToLower(err.Error()), "rate")
		e.health.recordFailure(target.ID, err.Error(), is429)
		if emittedAny || ctx.Err() != nil {
			return "", "", nil, nil, target, err
		}
	}
	if lastErr != nil {
		return "", "", nil, nil, ModelTarget{}, lastErr
	}
	return "", "", nil, nil, ModelTarget{}, fmt.Errorf("无可用模型节点")
}

func (e *Engine) streamSingleRound(
	ctx context.Context,
	baseURL, apiKey, model string,
	messages []apiMessage,
	tools []ToolDefinition,
	thinking *thinkingConfig,
	onDelta func(deltaReasoning, deltaContent string),
) (string, string, []apiToolCall, *TokenUsage, error) {
	return e.streamSingleRoundWithProvider(ctx, "deepseek", baseURL, apiKey, model, messages, tools, thinking, onDelta)
}

func (e *Engine) streamSingleRoundWithProvider(
	ctx context.Context,
	provider, baseURL, apiKey, model string,
	messages []apiMessage,
	tools []ToolDefinition,
	thinking *thinkingConfig,
	onDelta func(deltaReasoning, deltaContent string),
) (string, string, []apiToolCall, *TokenUsage, error) {
	reqBody := chatCompletionRequest{
		Model:         model,
		Messages:      messages,
		Tools:         tools,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
		Thinking:      thinking,
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
		req.Header.Set("HTTP-Referer", "https://bot.starryheavens.uk")
		req.Header.Set("X-Title", "Vesper BotHub")
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("调用 DeepSeek 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", "", nil, nil, fmt.Errorf("DeepSeek API 返回错误 (%d): %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

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
