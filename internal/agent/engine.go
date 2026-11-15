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
	"github.com/YN1753/vesper/bothub/internal/store"
)

// Engine orchestrates DeepSeek streaming chat, persona injection, long-term memory, and tool calling.
type Engine struct {
	store      *store.Store
	scheduler  TaskController
	httpClient *http.Client
}

func NewEngine(st *store.Store, sched TaskController) *Engine {
	return &Engine{
		store:     st,
		scheduler: sched,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Persona   string `json:"persona"`
	Model     string `json:"model"`
}

type ToolTrace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

type ChatResponse struct {
	SessionID        string      `json:"session_id"`
	Persona          string      `json:"persona"`
	Model            string      `json:"model"`
	ReasoningContent string      `json:"reasoning_content,omitempty"`
	Content          string      `json:"content"`
	ToolTraces       []ToolTrace `json:"tool_traces,omitempty"`
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

type chatCompletionRequest struct {
	Model    string           `json:"model"`
	Messages []apiMessage     `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
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
	case "", "deepseek-chat":
		return store.DefaultAIModel // "deepseek-flash"
	case "deepseek-reasoner":
		return "deepseek-v4-pro"
	default:
		return m
	}
}

// CallLightModel executes a non-streaming completion using the secondary lightweight model config
// (light_base_url / light_api_key / light_model), falling back to the primary DeepSeek config if unset.
func (e *Engine) CallLightModel(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	baseURL, _ := e.store.GetConfig(store.KeyLightBaseURL)
	apiKey, _ := e.store.GetConfig(store.KeyLightAPIKey)
	model, _ := e.store.GetConfig(store.KeyLightModel)

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	model = strings.TrimSpace(model)

	if apiKey == "" {
		apiKey, _ = e.store.GetConfig(store.KeyDeepSeekAPIKey)
		apiKey = strings.TrimSpace(apiKey)
	}
	if apiKey == "" {
		return "", fmt.Errorf("未配置轻量模型或主模型 API Key")
	}
	if baseURL == "" {
		baseURL, _ = e.store.GetConfig(store.KeyDeepSeekBaseURL)
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			baseURL = store.DefaultDeepSeekBaseURL
		}
	}
	if model == "" {
		model = store.DefaultAIModel
	} else {
		model = NormalizeModelID(model)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	bodyMap := map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	payload, err := json.Marshal(bodyMap)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("light model HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBytes)))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("解析轻量模型响应失败")
	}
	return notifier.StripEmoji(strings.TrimSpace(parsed.Choices[0].Message.Content)), nil
}

// ChatStream executes a multi-round streaming conversation with DeepSeek and invokes emit for SSE events.
// Event types: "session", "reasoning", "tool_start", "tool_result", "content", "done", "error".
func (e *Engine) ChatStream(ctx context.Context, req ChatRequest, emit func(event string, data any)) (*ChatResponse, error) {
	userMsg := notifier.StripEmoji(strings.TrimSpace(req.Message))
	if userMsg == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}

	apiKey, _ := e.store.GetConfig(store.KeyDeepSeekAPIKey)
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("尚未配置 DeepSeek API Key，请先在控制台「设置 -> AI 大脑设置」中填入密钥")
	}

	baseURL, _ := e.store.GetConfig(store.KeyDeepSeekBaseURL)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = store.DefaultDeepSeekBaseURL
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model, _ = e.store.GetConfig(store.KeyAIModel)
	}
	model = NormalizeModelID(model)

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
			messages = append(messages, apiMessage{
				Role:       "tool",
				Content:    m.Content,
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

	const maxRounds = 5
	for round := 0; round < maxRounds; round++ {
		roundReasoning, roundContent, toolCalls, err := e.streamSingleRound(ctx, baseURL, apiKey, model, messages, tools, func(deltaReasoning, deltaContent string) {
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
	if emit != nil {
		emit("done", resp)
	}
	return resp, nil
}

func (e *Engine) streamSingleRound(
	ctx context.Context,
	baseURL, apiKey, model string,
	messages []apiMessage,
	tools []ToolDefinition,
	onDelta func(deltaReasoning, deltaContent string),
) (string, string, []apiToolCall, error) {
	reqBody := chatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
		Stream:   true,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", nil, err
	}

	endpoint := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", "", nil, fmt.Errorf("调用 DeepSeek 接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return "", "", nil, fmt.Errorf("DeepSeek API 返回错误 (%d): %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var reasoningBuf strings.Builder
	var contentBuf strings.Builder
	toolMap := make(map[int]*apiToolCall)

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
		return reasoningBuf.String(), contentBuf.String(), nil, fmt.Errorf("读取流式响应中断: %w", err)
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

	return reasoningBuf.String(), contentBuf.String(), toolCalls, nil
}
