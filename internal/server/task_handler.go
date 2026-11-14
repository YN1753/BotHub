package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
)

func (s *Server) handleListTaskTemplates(w http.ResponseWriter, r *http.Request) {
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
	var req store.Task
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.CronExpr = strings.TrimSpace(req.CronExpr)

	if req.ID == "" || req.Name == "" || req.Type == "" || req.CronExpr == "" {
		writeError(w, http.StatusBadRequest, "任务 ID、名称、模板类型与 Cron 表达式为必填项")
		return
	}

	if _, err := cron.ParseStandard(req.CronExpr); err != nil {
		writeError(w, http.StatusBadRequest, "Cron 表达式语法无效: "+err.Error())
		return
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

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	existing, err := s.store.GetTask(id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}

	var req store.Task
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效请求体")
		return
	}

	if req.Name != "" { existing.Name = strings.TrimSpace(req.Name) }
	if req.Description != "" { existing.Description = strings.TrimSpace(req.Description) }
	if req.CronExpr != "" {
		trimmed := strings.TrimSpace(req.CronExpr)
		if _, err := cron.ParseStandard(trimmed); err != nil {
			writeError(w, http.StatusBadRequest, "Cron 表达式语法无效: "+err.Error())
			return
		}
		existing.CronExpr = trimmed
	}
	existing.Enabled = req.Enabled
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	id := strings.TrimSpace(r.PathValue("id"))
	res, err := s.scheduler.RunNow(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "summary": res.Summary, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "summary": res.Summary, "items_pushed": res.ItemsPushed})
}
