package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

var reSkillSlugClean = regexp.MustCompile(`[^a-z0-9_]+`)

func normalizeSkillSlug(raw, fallbackName string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "skill_")
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
		writeError(w, http.StatusInternalServerError, "获取技能列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skills": skills})
}

func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var req store.Skill
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效技能请求体")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		req.ID = "sk_" + hex.EncodeToString(b)
	}
	req.Name = notifier.StripEmoji(strings.TrimSpace(req.Name))
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "技能名称不能为空")
		return
	}
	req.Kind = strings.TrimSpace(req.Kind)
	if req.Kind != "webhook" {
		req.Kind = "prompt"
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
	if err := s.store.SaveSkill(req); err != nil {
		writeError(w, http.StatusInternalServerError, "保存技能失败: "+err.Error())
		return
	}
	saved, _ := s.store.GetSkill(req.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skill": saved})
}

func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetSkill(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "技能不存在")
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
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效更新请求体")
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
		existing.Kind = strings.TrimSpace(*req.Kind)
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
	if existing.Kind != "webhook" {
		existing.Kind = "prompt"
	}
	if msg := validateWebhookSkillFields(existing); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	if err := s.store.SaveSkill(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "更新技能失败: "+err.Error())
		return
	}
	updated, _ := s.store.GetSkill(id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skill": updated})
}

func validateWebhookSkillFields(sk *store.Skill) string {
	if sk.Kind != "webhook" {
		return ""
	}
	if sk.HTTPURL == "" {
		return "Webhook 外部技能必须填写请求 URL"
	}
	// Replace {{placeholder}} tokens temporarily so query interpolation doesn't break URL validation
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
		writeError(w, http.StatusBadRequest, "缺少技能 ID")
		return
	}
	if err := s.store.DeleteSkill(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除技能失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestSkill(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	sk, err := s.store.GetSkill(id)
	if err != nil || sk == nil {
		writeError(w, http.StatusNotFound, "技能不存在")
		return
	}
	if sk.Kind != "webhook" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"result": "指令型技能已就绪，将在绑定 Bot 对话时自动注入上下文规范。",
		})
		return
	}

	var req struct {
		Args map[string]any `json:"args"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req)

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
