package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/agent"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var reSkillSlugClean = regexp.MustCompile(`[^a-z0-9_]+`)

func normalizeSkillSlug(raw, fallbackName string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "skill_")
	s = strings.TrimPrefix(s, "mcp_")
	s = reSkillSlugClean.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		prefix := reSkillSlugClean.ReplaceAllString(strings.ToLower(fallbackName), "_")
		prefix = strings.Trim(prefix, "_")
		if prefix == "" {
			prefix = "custom"
		}
		s = prefix + "_" + hex.EncodeToString(b)
	}
	return s
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.store.ListSkills()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "获取 MCP 与技能列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skills": skills})
}

func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var req store.Skill
	if !decodeJSON(w, r, 64*1024, &req) {
		return
	}
	req.Kind = store.NormalizeSkillKind(req.Kind)
	if req.Kind == store.SkillKindBuiltinMCP {
		req.Kind = store.SkillKindMCP
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		if req.Kind == store.SkillKindMCP {
			req.ID = "mcp_" + hex.EncodeToString(b)
		} else {
			req.ID = "sk_" + hex.EncodeToString(b)
		}
	}
	req.Name = notifier.StripEmoji(strings.TrimSpace(req.Name))
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "MCP / 技能名称不能为空")
		return
	}
	req.Slug = normalizeSkillSlug(req.Slug, req.Name)
	req.Description = notifier.StripEmoji(strings.TrimSpace(req.Description))
	req.Instruction = notifier.StripEmoji(strings.TrimSpace(req.Instruction))
	req.HTTPURL = strings.TrimSpace(req.HTTPURL)
	if msg := validateWebhookSkillFields(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	req.Enabled = true
	if req.Kind == store.SkillKindMCP && s.agent != nil && (strings.TrimSpace(req.ParamsSchema) == "" || strings.TrimSpace(req.ParamsSchema) == "[]") {
		dCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		if discovered, dErr := s.agent.DiscoverMCPServerTools(dCtx, req); dErr == nil && len(discovered) > 0 {
			if raw, mErr := json.Marshal(discovered); mErr == nil {
				req.ParamsSchema = string(raw)
			}
		}
		cancel()
	}
	if err := s.store.SaveSkill(req); err != nil {
		writeError(w, http.StatusInternalServerError, "保存 MCP / 技能失败: "+err.Error())
		return
	}
	saved, _ := s.store.GetSkill(req.ID)
	s.publishAssistantEvent("skills")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skill": saved})
}

func (s *Server) handleImportMCPServers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JSON  string `json:"json"`
		BotID string `json:"bot_id"`
	}
	if !decodeJSON(w, r, 128*1024, &req) {
		return
	}
	parsedList, err := agent.ParseMCPServersJSON(req.JSON)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	imported := make([]store.Skill, 0, len(parsedList))
	for _, item := range parsedList {
		item.Slug = normalizeSkillSlug(item.Slug, item.Name)
		if existing, _ := s.store.GetSkillBySlug(item.Slug); existing != nil && !existing.IsBuiltinMCP() {
			item.ID = existing.ID
			item.CreatedAt = existing.CreatedAt
		}
		if s.agent != nil {
			dCtx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			if discovered, dErr := s.agent.DiscoverMCPServerTools(dCtx, item); dErr == nil && len(discovered) > 0 {
				if raw, mErr := json.Marshal(discovered); mErr == nil {
					item.ParamsSchema = string(raw)
				}
			}
			cancel()
		}
		if err := s.store.SaveSkill(item); err != nil {
			writeError(w, http.StatusInternalServerError, "保存导入的 MCP 服务失败: "+err.Error())
			return
		}
		if saved, _ := s.store.GetSkill(item.ID); saved != nil {
			imported = append(imported, *saved)
		}
	}

	if botID := strings.TrimSpace(req.BotID); botID != "" && len(imported) > 0 {
		if bot, bErr := s.store.GetBot(botID); bErr == nil && bot != nil {
			cur := bot.SkillIDList()
			have := make(map[string]bool, len(cur))
			for _, id := range cur {
				have[id] = true
			}
			for _, sk := range imported {
				if !have[sk.ID] {
					cur = append(cur, sk.ID)
					have[sk.ID] = true
				}
			}
			if raw, mErr := json.Marshal(cur); mErr == nil {
				bot.SkillIDs = string(raw)
				_ = s.store.SaveBot(*bot)
				s.publishAssistantEvent("bots")
			}
		}
	}

	s.publishAssistantEvent("skills")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"count":    len(imported),
		"imported": imported,
	})
}

func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetSkill(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "MCP / 技能不存在")
		return
	}

	var req struct {
		Name         *string `json:"name"`
		Slug         *string `json:"slug"`
		Kind         *string `json:"kind"`
		Description  *string `json:"description"`
		Instruction  *string `json:"instruction"`
		HTTPMethod   *string `json:"http_method"`
		HTTPURL      *string `json:"http_url"`
		HTTPHeaders  *string `json:"http_headers"`
		ParamsSchema *string `json:"params_schema"`
		Enabled      *bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, 64*1024, &req) {
		return
	}

	if existing.IsBuiltinMCP() {
		// Built-in MCP servers allow toggling enabled or updating trigger instruction/description
		if req.Enabled != nil {
			existing.Enabled = *req.Enabled
		}
		if req.Description != nil {
			if v := notifier.StripEmoji(strings.TrimSpace(*req.Description)); v != "" {
				existing.Description = v
			}
		}
		if req.Instruction != nil {
			existing.Instruction = notifier.StripEmoji(strings.TrimSpace(*req.Instruction))
		}
		if err := s.store.SaveSkill(*existing); err != nil {
			writeError(w, http.StatusInternalServerError, "更新内置 MCP 状态失败: "+err.Error())
			return
		}
		updated, _ := s.store.GetSkill(id)
		s.publishAssistantEvent("skills")
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skill": updated})
		return
	}

	if req.Name != nil {
		if v := notifier.StripEmoji(strings.TrimSpace(*req.Name)); v != "" {
			existing.Name = v
		}
	}
	if req.Slug != nil {
		existing.Slug = normalizeSkillSlug(*req.Slug, existing.Name)
	}
	if req.Kind != nil {
		k := store.NormalizeSkillKind(*req.Kind)
		if k != store.SkillKindBuiltinMCP {
			existing.Kind = k
		}
	}
	if req.Description != nil {
		existing.Description = notifier.StripEmoji(strings.TrimSpace(*req.Description))
	}
	if req.Instruction != nil {
		existing.Instruction = notifier.StripEmoji(strings.TrimSpace(*req.Instruction))
	}
	if req.HTTPMethod != nil {
		existing.HTTPMethod = strings.TrimSpace(*req.HTTPMethod)
	}
	if req.HTTPURL != nil {
		existing.HTTPURL = strings.TrimSpace(*req.HTTPURL)
	}
	if req.HTTPHeaders != nil {
		existing.HTTPHeaders = strings.TrimSpace(*req.HTTPHeaders)
	}
	if req.ParamsSchema != nil {
		existing.ParamsSchema = strings.TrimSpace(*req.ParamsSchema)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	existing.Kind = store.NormalizeSkillKind(existing.Kind)
	if msg := validateWebhookSkillFields(existing); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if err := s.store.SaveSkill(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "更新 MCP / 技能失败: "+err.Error())
		return
	}
	updated, _ := s.store.GetSkill(id)
	s.publishAssistantEvent("skills")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skill": updated})
}

func validateWebhookSkillFields(sk *store.Skill) string {
	kind := store.NormalizeSkillKind(sk.Kind)
	if kind == store.SkillKindBuiltinMCP {
		return ""
	}
	if kind == store.SkillKindMCP {
		if sk.HTTPURL == "" {
			return "远程 MCP 服务必须填写 Endpoint URL"
		}
		parsed, err := url.Parse(sk.HTTPURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "MCP 服务 URL 必须以 http:// 或 https:// 开头且包含有效主机"
		}
		sk.HTTPMethod = http.MethodPost
		if h := strings.TrimSpace(sk.HTTPHeaders); h != "" {
			var hdrs map[string]any
			if json.Unmarshal([]byte(h), &hdrs) != nil {
				return "http_headers 必须为合法 JSON 对象"
			}
		}
		return ""
	}
	if kind != store.SkillKindWebhook {
		return ""
	}
	if sk.HTTPURL == "" {
		return "Webhook 外部技能必须填写请求 URL"
	}
	sanitizedURL := strings.ReplaceAll(strings.ReplaceAll(sk.HTTPURL, "{{", "var_"), "}}", "")
	parsed, err := url.Parse(sanitizedURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "Webhook URL 必须以 http:// 或 https:// 开头且包含有效主机"
	}
	method := strings.ToUpper(strings.TrimSpace(sk.HTTPMethod))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		sk.HTTPMethod = method
	default:
		return "Webhook 请求方法仅支持 GET / POST / PUT / PATCH / DELETE"
	}
	if h := strings.TrimSpace(sk.HTTPHeaders); h != "" {
		var hdrs map[string]any
		if json.Unmarshal([]byte(h), &hdrs) != nil {
			return "http_headers 必须为合法 JSON 对象"
		}
	}
	if ps := strings.TrimSpace(sk.ParamsSchema); ps != "" {
		var params []map[string]any
		if json.Unmarshal([]byte(ps), &params) != nil {
			return "params_schema 必须为合法 JSON 数组"
		}
	}
	return ""
}

func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少 MCP / 技能 ID")
		return
	}
	if err := s.store.DeleteSkill(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.publishAssistantEvent("skills")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	sk, err := s.store.GetSkill(id)
	if err != nil || sk == nil {
		writeError(w, http.StatusNotFound, "MCP / 技能不存在")
		return
	}
	kind := store.NormalizeSkillKind(sk.Kind)
	if kind == store.SkillKindBuiltinMCP {
		tools := sk.MCPTools()
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, t.Name)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"tools":  tools,
			"result": fmt.Sprintf("内置 MCP 服务「%s」就绪，包含 %d 项工具：%s", sk.Name, len(tools), strings.Join(names, ", ")),
		})
		return
	}
	if kind == store.SkillKindMCP {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		start := time.Now()
		tools, dErr := s.agent.DiscoverMCPServerTools(ctx, *sk)
		latencyMs := time.Since(start).Milliseconds()
		if dErr != nil {
			writeError(w, http.StatusBadGateway, dErr.Error())
			return
		}
		if raw, mErr := json.Marshal(tools); mErr == nil {
			sk.ParamsSchema = string(raw)
			_ = s.store.SaveSkill(*sk)
			s.publishAssistantEvent("skills")
		}
		names := make([]string, 0, len(tools))
		for _, t := range tools {
			names = append(names, t.Name)
		}
		summary := fmt.Sprintf("MCP JSON-RPC 握手与 tools/list 同步成功（耗时 %dms），共发现 %d 个工具", latencyMs, len(tools))
		if len(names) > 0 {
			summary += "：" + strings.Join(names, ", ")
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"latency_ms": latencyMs,
			"tools":      tools,
			"skill":      sk,
			"result":     summary,
		})
		return
	}
	if kind != store.SkillKindWebhook {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"result": "指令型扩展已就绪，将在挂载 Bot 对话时自动注入上下文规范。",
		})
		return
	}

	var req struct {
		Args map[string]any `json:"args"`
	}
	_ = decodeOptionalJSON(w, r, 16*1024, &req)

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	start := time.Now()
	statusCode, output, execErr := s.agent.ExecuteWebhookSkill(ctx, *sk, req.Args)
	latencyMs := time.Since(start).Milliseconds()
	if execErr != nil {
		writeError(w, http.StatusBadGateway, execErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"status_code": statusCode,
		"latency_ms":  latencyMs,
		"result":      output,
	})
}
