package agent

import (
	"bytes"
	"context"
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

var reSkillPlaceholder = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// SkillToolName returns the OpenAI-compatible function name exposed to LLMs for a webhook skill.
func SkillToolName(slug string) string {
	clean := strings.TrimSpace(strings.ToLower(slug))
	clean = strings.TrimPrefix(clean, "skill_")
	return "skill_" + clean
}

// CompileSkillTool converts a user-created webhook Skill into a live OpenAI Function Calling ToolDefinition.
func CompileSkillTool(sk store.Skill) ToolDefinition {
	props := make(map[string]any)
	required := make([]string, 0)
	for _, f := range sk.Params() {
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		ftype := strings.TrimSpace(f.Type)
		if ftype != "integer" && ftype != "boolean" {
			ftype = "string"
		}
		props[key] = map[string]any{
			"type":        ftype,
			"description": strings.TrimSpace(f.Description),
		}
		if f.Required {
			required = append(required, key)
		}
	}
	paramsObj := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		paramsObj["required"] = required
	}

	desc := strings.TrimSpace(sk.Description)
	if desc == "" {
		desc = "调用自定义技能: " + sk.Name
	}
	if instr := strings.TrimSpace(sk.Instruction); instr != "" && strings.ToUpper(sk.HTTPMethod) == "GET" {
		desc = desc + " (" + instr + ")"
	}

	return ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        SkillToolName(sk.Slug),
			Description: desc,
			Parameters:  paramsObj,
		},
	}
}

func interpolateSkillTemplate(tpl string, args map[string]any, urlEscape, jsonStringEscape bool) string {
	if tpl == "" {
		return ""
	}
	return reSkillPlaceholder.ReplaceAllStringFunc(tpl, func(match string) string {
		sub := reSkillPlaceholder.FindStringSubmatch(match)
		if len(sub) < 2 {
			return ""
		}
		val, ok := args[sub[1]]
		if !ok || val == nil {
			return ""
		}
		str := fmt.Sprintf("%v", val)
		if urlEscape {
			return url.QueryEscape(str)
		}
		if jsonStringEscape {
			b, err := json.Marshal(str)
			if err == nil && len(b) >= 2 {
				return string(b[1 : len(b)-1])
			}
		}
		return str
	})
}

// ExecuteWebhookSkill executes a webhook skill with parameter defaults, required-field checks,
// SSRF guards, argument interpolation, and clean HTML-to-text sanitization.
func (e *Engine) ExecuteWebhookSkill(ctx context.Context, sk store.Skill, args map[string]any) (int, string, error) {
	workingArgs := make(map[string]any, len(args)+4)
	for k, v := range args {
		workingArgs[k] = v
	}

	for _, f := range sk.Params() {
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		val, exists := workingArgs[key]
		valStr := strings.TrimSpace(fmt.Sprintf("%v", val))
		if (!exists || val == nil || valStr == "") && strings.TrimSpace(f.Default) != "" {
			workingArgs[key] = strings.TrimSpace(f.Default)
			val = workingArgs[key]
			valStr = strings.TrimSpace(fmt.Sprintf("%v", val))
			exists = true
		}
		if f.Required && (!exists || val == nil || valStr == "") {
			return 0, "", fmt.Errorf("技能「%s」缺少必填参数: %s", sk.Name, key)
		}
	}

	rawURL := strings.TrimSpace(interpolateSkillTemplate(sk.HTTPURL, workingArgs, true, false))
	if rawURL == "" {
		return 0, "", fmt.Errorf("技能 %s 未配置 HTTP URL", sk.Name)
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return 0, "", fmt.Errorf("技能 URL 非法或仅支持 http/https")
	}

	method := strings.ToUpper(strings.TrimSpace(sk.HTTPMethod))
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	hasBody := method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
	if hasBody {
		tpl := strings.TrimSpace(sk.Instruction)
		if tpl != "" && strings.Contains(tpl, "{{") {
			looksJSON := strings.HasPrefix(tpl, "{") || strings.HasPrefix(tpl, "[")
			bodyText := interpolateSkillTemplate(tpl, workingArgs, false, looksJSON)
			bodyReader = strings.NewReader(bodyText)
		} else {
			payloadBytes, _ := json.Marshal(workingArgs)
			bodyReader = bytes.NewReader(payloadBytes)
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, bodyReader)
	if err != nil {
		return 0, "", err
	}
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "Vesper-BotHub-Skill/1.0")
	for hk, hv := range sk.HeadersMap() {
		k := strings.TrimSpace(hk)
		if k == "" {
			continue
		}
		req.Header.Set(k, interpolateSkillTemplate(hv, workingArgs, false, false))
	}

	client := e.safeFetchClient()
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("执行自定义技能失败: %w", err)
	}
	defer resp.Body.Close()

	rawBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	rawStr := strings.TrimSpace(string(rawBytes))

	var text string
	if strings.Contains(contentType, "html") || strings.HasPrefix(strings.ToLower(rawStr), "<!doctype") || strings.HasPrefix(strings.ToLower(rawStr), "<html") {
		text = stripHTMLToText(rawStr)
	} else {
		text = notifier.StripEmoji(rawStr)
	}

	runes := []rune(text)
	if len(runes) > 6000 {
		text = string(runes[:6000]) + "\n...[已截断过长响应]"
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, text, fmt.Errorf("技能接口返回 HTTP %d: %s", resp.StatusCode, text)
	}
	return resp.StatusCode, text, nil
}
