package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

func (s *Server) handleListTaskTemplates(w http.ResponseWriter, r *http.Request) {
	if s.scheduler != nil {
		writeJSON(w, http.StatusOK, map[string]any{"templates": s.scheduler.TaskTemplates()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": modules.BuiltinTemplates()})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.scheduler.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		ID                 string `json:"id"`
		Name               string `json:"name"`
		Type               string `json:"type"`
		BotID              string `json:"bot_id"`
		Description        string `json:"description"`
		CronExpr           string `json:"cron_expr"`
		RunAt              string `json:"run_at"`
		Model              string `json:"model"`
		AllowPaidFallback  *bool  `json:"allow_paid_fallback"`
		MaxPaidCallsPerDay int    `json:"max_paid_calls_per_day"`
		Enabled            *bool  `json:"enabled"`
		Params             string `json:"params"`
	}
	if !decodeJSON(w, r, 16384, &raw) {
		return
	}
	allowPaid := true
	if raw.AllowPaidFallback != nil {
		allowPaid = *raw.AllowPaidFallback
	}
	enabled := true
	if raw.Enabled != nil {
		enabled = *raw.Enabled
	}
	req := store.Task{
		ID:                 strings.TrimSpace(raw.ID),
		Name:               strings.TrimSpace(raw.Name),
		Type:               strings.TrimSpace(raw.Type),
		BotID:              strings.TrimSpace(raw.BotID),
		Description:        strings.TrimSpace(raw.Description),
		CronExpr:           strings.TrimSpace(raw.CronExpr),
		RunAt:              strings.TrimSpace(raw.RunAt),
		Model:              strings.TrimSpace(raw.Model),
		AllowPaidFallback:  allowPaid,
		MaxPaidCallsPerDay: raw.MaxPaidCallsPerDay,
		Enabled:            enabled,
		Params:             raw.Params,
	}

	if req.ID == "" || req.Name == "" || req.Type == "" || (req.CronExpr == "" && req.RunAt == "") {
		writeError(w, http.StatusBadRequest, "任务 ID、名称、模板类型与 Cron 表达式或单次触发时间(run_at)为必填项")
		return
	}

	if req.RunAt != "" {
		if _, err := scheduler.ParseRunAt(req.RunAt, nil); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if req.CronExpr != "" {
		if _, err := cron.ParseStandard(req.CronExpr); err != nil {
			writeError(w, http.StatusBadRequest, "Cron 表达式语法无效: "+err.Error())
			return
		}
	}

	existing, _ := s.store.GetTask(req.ID)
	if existing != nil {
		writeError(w, http.StatusConflict, "任务 ID 已存在")
		return
	}

	if req.Params != "" {
		var js map[string]any
		if json.Unmarshal([]byte(req.Params), &js) != nil {
			writeError(w, http.StatusBadRequest, "任务参数需为合法 JSON")
			return
		}
	} else {
		req.Params = "{}"
	}

	if err := s.store.SaveTask(req); err != nil {
		writeError(w, http.StatusInternalServerError, "创建任务失败: "+err.Error())
		return
	}

	if err := s.scheduler.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, "重载调度失败: "+err.Error())
		return
	}

	s.publishAssistantEvent("tasks")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetTask(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}

	var req struct {
		ID                string  `json:"id"`
		Name              string  `json:"name"`
		Type              string  `json:"type"`
		BotID             *string `json:"bot_id"`
		Description       *string `json:"description"`
		CronExpr          string  `json:"cron_expr"`
		RunAt             string  `json:"run_at"`
		Model              *string `json:"model"`
		AllowPaidFallback  *bool   `json:"allow_paid_fallback"`
		MaxPaidCallsPerDay *int    `json:"max_paid_calls_per_day"`
		Enabled            *bool   `json:"enabled"`
		Params            string  `json:"params"`
	}
	if !decodeJSON(w, r, 16384, &req) {
		return
	}

	if req.Name != "" { existing.Name = strings.TrimSpace(req.Name) }
	if req.BotID != nil { existing.BotID = strings.TrimSpace(*req.BotID) }
	if req.Description != nil { existing.Description = strings.TrimSpace(*req.Description) }
	if req.Model != nil { existing.Model = strings.TrimSpace(*req.Model) }
	if req.AllowPaidFallback != nil { existing.AllowPaidFallback = *req.AllowPaidFallback }
	if req.MaxPaidCallsPerDay != nil && *req.MaxPaidCallsPerDay >= 0 { existing.MaxPaidCallsPerDay = *req.MaxPaidCallsPerDay }
	if req.RunAt != "" {
		if _, err := scheduler.ParseRunAt(req.RunAt, nil); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		existing.RunAt = strings.TrimSpace(req.RunAt)
		existing.CronExpr = ""
	}
	if req.CronExpr != "" {
		trimmed := strings.TrimSpace(req.CronExpr)
		if _, err := cron.ParseStandard(trimmed); err != nil {
			writeError(w, http.StatusBadRequest, "Cron 表达式语法无效: "+err.Error())
			return
		}
		existing.CronExpr = trimmed
		if req.RunAt == "" {
			existing.RunAt = ""
		}
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Params != "" {
		var js map[string]any
		if json.Unmarshal([]byte(req.Params), &js) != nil {
			writeError(w, http.StatusBadRequest, "任务参数需为合法 JSON")
			return
		}
		existing.Params = req.Params
	}

	if err := s.store.SaveTask(*existing); err != nil {
		writeError(w, http.StatusInternalServerError, "更新任务失败: "+err.Error())
		return
	}

	if err := s.scheduler.Reload(); err != nil {
		writeError(w, http.StatusInternalServerError, "重载调度失败: "+err.Error())
		return
	}

	s.publishAssistantEvent("tasks")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少任务 ID")
		return
	}
	if err := s.store.DeleteTask(id); err != nil {
		writeError(w, http.StatusInternalServerError, "删除任务失败: "+err.Error())
		return
	}
	_ = s.scheduler.Reload()
	s.publishAssistantEvent("tasks")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	id := strings.TrimSpace(r.PathValue("id"))
	if existing, _ := s.store.GetTask(id); existing == nil {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}
	res, err := s.scheduler.RunNow(ctx, id)
	s.publishAssistantEvent("tasks")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "summary": res.Summary, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": res.Summary, "items_pushed": res.ItemsPushed})
}

func (s *Server) handleOptimizeTaskPrompt(w http.ResponseWriter, r *http.Request) {
	if s.agent == nil {
		writeError(w, http.StatusServiceUnavailable, "Agent 引擎未就绪")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Prompt      string `json:"prompt"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, 16384, &req) {
		return
	}
	rawPrompt := strings.TrimSpace(req.Prompt)
	if rawPrompt == "" {
		rawPrompt = strings.TrimSpace(req.Description)
	}
	if rawPrompt == "" {
		writeError(w, http.StatusBadRequest, "请先填写任务指令内容再进行精修优化")
		return
	}

	sysPrompt := `你是定时推送 Prompt 工程专家。请将用户草拟的定时任务指令改写为“高信息密度、防复读、可执行”的专业版定时例程指令（中文，120-260 字）。
改写原则：
1. 剥离并抽象用户原话里随口举的“比如/例如”具体示例，改为泛化主题范围与质量标准，明确禁止每次照搬示例。
2. 明确要求直接输出干货正文，禁止寒暄问候、套话与空泛鸡汤。
3. 若涉及实时信息、待办、资产、收藏或外部新闻，明确要求先调用内置工具获取真实数据后再凝练总结。
4. 明确要求每次触发结合当日视角与历史去重账本，产出全新切入点与具体事实/案例。
5. 只输出改写后的最终指令正文，不要加任何解释、标题引号或 Emoji。`

	userInput := "任务名称：" + strings.TrimSpace(req.Name) + "\n原始指令：\n" + rawPrompt
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	optimized, err := s.agent.CallLightModel(ctx, sysPrompt, userInput)
	if err != nil {
		writeError(w, http.StatusBadGateway, "AI 精修失败: "+err.Error())
		return
	}
	optimized = notifier.StripEmoji(strings.TrimSpace(optimized))
	if optimized == "" {
		writeError(w, http.StatusBadGateway, "AI 未返回有效精修结果")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"optimized_prompt": optimized,
	})
}
