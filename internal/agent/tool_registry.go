package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
)

type toolScopeCtxKey struct{}

// ToolScope defines isolated execution/memory boundaries for the current Bot or Group turn.
type ToolScope struct {
	BotID         string
	TimelineBotID string
	ThreadID      string
	MemoryPolicy  string
	Depth         int
	VisitedBots   []string
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

// ToolSpec declaratively co-locates schema, timeout, read-only concurrency metadata, UI status text,
// group domain routing keywords, error recovery hints, and execution handler for a built-in tool.
type ToolSpec struct {
	Definition     ToolDefinition
	Timeout        time.Duration
	ReadOnly       bool
	StatusLabel    string
	StatusFunc     func(argsJSON string) string
	DomainKeywords []string
	ErrorHint      string
	RetryErrorHint string
	Execute        func(ctx context.Context, e *Engine, cleanArgs string) (string, error)
}

const (
	defaultToolTimeout = 12 * time.Second
	maxToolOutputRunes = 12000
)

var (
	builtinToolSpecs []ToolSpec
	builtinToolIndex map[string]ToolSpec
)

func init() {
	builtinToolSpecs = []ToolSpec{
		readURLToolSpec(),
		webSearchToolSpec(),
		queryNezhaNodesToolSpec(),
		manageAssetsToolSpec(),
		manageTasksToolSpec(),
		rememberFactToolSpec(),
		recallMemoryToolSpec(),
		manageMemoryToolSpec(),
		manageTodosToolSpec(),
		manageNotesToolSpec(),
		manageStashToolSpec(),
		sendNotificationToolSpec(),
		manageBotsToolSpec(),
		manageModelsToolSpec(),
		manageSkillsToolSpec(),
		consultBotToolSpec(),
		workspaceBriefingToolSpec(),
		calculateEvalToolSpec(),
		datetimeCalcToolSpec(),
	}
	builtinToolIndex = make(map[string]ToolSpec, len(builtinToolSpecs))
	for _, sp := range builtinToolSpecs {
		builtinToolIndex[sp.Definition.Function.Name] = sp
	}
}

func LookupToolSpec(name string) (ToolSpec, bool) {
	sp, ok := builtinToolIndex[strings.TrimSpace(name)]
	return sp, ok
}

// BuiltinTools returns all OpenAI-compatible tool schemas exposed to LLM providers in deterministic order.
func BuiltinTools() []ToolDefinition {
	out := make([]ToolDefinition, 0, len(builtinToolSpecs))
	for _, sp := range builtinToolSpecs {
		out = append(out, sp.Definition)
	}
	return out
}

func AvailableToolsPayload() []AvailableToolMeta {
	out := make([]AvailableToolMeta, 0, len(builtinToolSpecs))
	for _, sp := range builtinToolSpecs {
		out = append(out, AvailableToolMeta{
			ID:          sp.Definition.Function.Name,
			Description: sp.Definition.Function.Description,
		})
	}
	return out
}

var reTrailingJSONComma = regexp.MustCompile(`,\s*([}\]])`)

// stripJSONCodeFence removes optional Markdown ```json / ``` code fences from LLM outputs.
func stripJSONCodeFence(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			firstLine := strings.TrimSpace(s[3:nl])
			if firstLine == "" || strings.EqualFold(firstLine, "json") {
				s = s[nl+1:]
			} else {
				s = strings.TrimPrefix(s, "```json")
				s = strings.TrimPrefix(s, "```JSON")
				s = strings.TrimPrefix(s, "```")
			}
		} else {
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimPrefix(s, "```JSON")
			s = strings.TrimPrefix(s, "```")
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func normalizeToolArgsJSON(raw string) string {
	s := stripJSONCodeFence(raw)
	if s == "" {
		return "{}"
	}
	s = reTrailingJSONComma.ReplaceAllString(s, "$1")
	if s == "" {
		return "{}"
	}
	return s
}

// ExecuteTool dispatches built-in tools from the declarative registry and user-defined custom webhook skills.
func (e *Engine) ExecuteTool(ctx context.Context, name string, argsJSON string) (out string, err error) {
	timeout := defaultToolTimeout
	if strings.HasPrefix(name, "skill_") {
		timeout = 25 * time.Second
	} else if spec, ok := LookupToolSpec(name); ok && spec.Timeout > 0 {
		timeout = spec.Timeout
	}

	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			out = ""
			err = fmt.Errorf("工具 %s 执行异常中断: %v", name, r)
		}
	}()

	cleanArgs := normalizeToolArgsJSON(argsJSON)
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
		spec, ok := LookupToolSpec(name)
		if !ok || spec.Execute == nil {
			return "", fmt.Errorf("未知工具: %s", name)
		}
		out, err = spec.Execute(toolCtx, e, cleanArgs)
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

func (e *Engine) isReadOnlyTool(name string) bool {
	if spec, ok := LookupToolSpec(name); ok {
		return spec.ReadOnly
	}
	if strings.HasPrefix(name, "skill_") {
		slug := strings.TrimPrefix(name, "skill_")
		if sk, err := e.store.GetSkillBySlug(slug); err == nil && sk != nil {
			m := strings.ToUpper(strings.TrimSpace(sk.HTTPMethod))
			return m == "" || m == "GET"
		}
	}
	return false
}

func humanToolStatus(name string, argsJSON ...string) string {
	if spec, ok := LookupToolSpec(name); ok {
		if spec.StatusFunc != nil {
			raw := ""
			if len(argsJSON) > 0 {
				raw = argsJSON[0]
			}
			if label := spec.StatusFunc(raw); label != "" {
				return label
			}
		}
		if spec.StatusLabel != "" {
			return spec.StatusLabel
		}
	}
	if strings.HasPrefix(name, "skill_") {
		return "正在执行自定义技能..."
	}
	return "正在调用工具..."
}

func toolErrorPayload(name string, err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	retryable := strings.Contains(lower, "timeout") || strings.Contains(lower, "超时") ||
		strings.Contains(lower, "connection") || strings.Contains(lower, "eof") || strings.Contains(lower, "temporary")
	var hint string
	switch {
	case strings.Contains(lower, "json") || strings.Contains(msg, "参数") || strings.Contains(msg, "必填"):
		hint = "请检查并修正工具参数 JSON 字段后重试"
	case strings.Contains(msg, "未找到") || strings.Contains(msg, "不存在"):
		hint = "建议先调用 list/search 动作核对准确 id 或 fact_key"
	default:
		if spec, ok := LookupToolSpec(name); ok {
			if retryable && spec.RetryErrorHint != "" {
				hint = spec.RetryErrorHint
			} else if !retryable && spec.ErrorHint != "" {
				hint = spec.ErrorHint
			}
		}
		if hint == "" && retryable {
			hint = "遇到瞬时连接或超时问题，可重试一次或降级为精简查询"
		}
	}
	payload := map[string]any{
		"ok":        false,
		"tool":      name,
		"error":     msg,
		"retryable": retryable,
	}
	if hint != "" {
		payload["hint"] = hint
	}
	b, _ := json.Marshal(payload)
	return string(b)
}
