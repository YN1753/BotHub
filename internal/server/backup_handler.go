package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type BackupData struct {
	Version       string              `json:"version"`
	ExportedAt    string              `json:"exported_at"`
	Configs       map[string]string   `json:"configs"`
	Assets        []store.Asset       `json:"assets"`
	Tasks         []store.Task        `json:"tasks"`
	Bots          []store.Bot         `json:"bots,omitempty"`
	Skills        []store.Skill       `json:"skills,omitempty"`
	Todos         []store.TodoItem    `json:"todos,omitempty"`
	Notes         []store.WorkNote    `json:"notes,omitempty"`
	StashItems    []store.StashItem   `json:"stash_items,omitempty"`
	AgentMemories []store.AgentMemory `json:"agent_memories,omitempty"`
}

func (s *Server) handleExportBackup(w http.ResponseWriter, r *http.Request) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Minute))
	assets, err := s.store.ListAssets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "导出资产失败: "+err.Error())
		return
	}
	tasks, err := s.store.ListTasks()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "导出任务失败: "+err.Error())
		return
	}
	bots, _ := s.store.ListBots()
	skills, _ := s.store.ListSkills()
	todos, _ := s.store.ListTodos("all", "", 300)
	notes, _ := s.store.ListNotes("", "", 200)
	stashItems, _ := s.store.SearchStashItems("", "everything", "", 2000)
	memories, _ := s.store.ListAgentMemories("", 500)
	cfgs, _ := s.store.GetAllConfigs()
	delete(cfgs, store.KeyAdminPassHash)
	delete(cfgs, store.KeySessionSecret)
	maskSecrets := r.URL.Query().Get("mask_secrets") == "1" || r.URL.Query().Get("mask_secrets") == "true"
	if maskSecrets {
		store.MaskSensitiveConfigs(cfgs)
		if _, ok := cfgs[store.KeyModelProviders]; ok {
			providers, _, _ := s.store.GetModelConfig()
			if maskedProvBytes, mErr := json.Marshal(store.MaskModelProviders(providers)); mErr == nil {
				cfgs[store.KeyModelProviders] = string(maskedProvBytes)
			}
		}
	}

	if assets == nil {
		assets = []store.Asset{}
	}
	if tasks == nil {
		tasks = []store.Task{}
	}
	if bots == nil {
		bots = []store.Bot{}
	}
	if skills == nil {
		skills = []store.Skill{}
	}
	if todos == nil {
		todos = []store.TodoItem{}
	}
	if notes == nil {
		notes = []store.WorkNote{}
	}
	if stashItems == nil {
		stashItems = []store.StashItem{}
	}
	if memories == nil {
		memories = []store.AgentMemory{}
	}
	if cfgs == nil {
		cfgs = map[string]string{}
	}

	backup := BackupData{
		Version:       s.version,
		ExportedAt:    store.NowShanghai(),
		Configs:       cfgs,
		Assets:        assets,
		Tasks:         tasks,
		Bots:          bots,
		Skills:        skills,
		Todos:         todos,
		Notes:         notes,
		StashItems:    stashItems,
		AgentMemories: memories,
	}

	filename := fmt.Sprintf("bothub_backup_%s.json", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_ = json.NewEncoder(w).Encode(backup)
}

func (s *Server) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(2 * time.Minute))
	_ = rc.SetWriteDeadline(time.Now().Add(2 * time.Minute))

	var backup BackupData
	if !decodeJSON(w, r, 20*1024*1024, &backup) {
		return
	}

	// 1. Restore Configs (never overwrite stored secrets with masked "****" placeholders)
	_ = s.store.SetConfig(store.KeyPushChannel, "native")
	for k, v := range backup.Configs {
		val := strings.TrimSpace(v)
		if k == store.KeyModelProviders {
			var provs []store.ModelProvider
			if json.Unmarshal([]byte(val), &provs) == nil {
				_, existingModels, existingStrat := s.store.GetModelConfig()
				_ = s.store.SaveModelConfig(provs, existingModels, existingStrat)
			}
			continue
		}
		if k == store.KeyPooledModels {
			var mods []store.PooledModel
			if json.Unmarshal([]byte(val), &mods) == nil {
				existingProvs, _, existingStrat := s.store.GetModelConfig()
				_ = s.store.SaveModelConfig(existingProvs, mods, existingStrat)
			}
			continue
		}
		spec, ok := store.LookupConfigSpec(k)
		if !ok || !spec.Editable {
			continue
		}
		if spec.IsURL && !validateHTTPURL(val) {
			continue
		}
		if spec.Sensitive && strings.Contains(val, "****") {
			continue
		}
		_ = s.store.SetConfig(k, val)
	}

	// 2. Restore Assets
	assetsCount := 0
	for _, a := range backup.Assets {
		a.ID = strings.TrimSpace(a.ID)
		a.Name = notifier.StripEmoji(strings.TrimSpace(a.Name))
		if a.ID == "" || a.Name == "" {
			continue
		}
		a.Spec = notifier.StripEmoji(strings.TrimSpace(a.Spec))
		a.Price = notifier.StripEmoji(strings.TrimSpace(a.Price))
		a.Category = notifier.StripEmoji(strings.TrimSpace(a.Category))
		if err := s.store.SaveAsset(a); err == nil {
			assetsCount++
		}
	}

	// 3. Restore Tasks
	tasksCount := 0
	for _, t := range backup.Tasks {
		t.ID = strings.TrimSpace(t.ID)
		t.Name = notifier.StripEmoji(strings.TrimSpace(t.Name))
		if t.ID == "" || t.Name == "" {
			continue
		}
		t.Description = notifier.StripEmoji(strings.TrimSpace(t.Description))
		if err := s.store.SaveTask(t); err == nil {
			tasksCount++
		}
	}

	// 4. Restore Agent Memories (preserve scope, clamped importance, pinned)
	for _, m := range backup.AgentMemories {
		key := notifier.StripEmoji(strings.TrimSpace(m.FactKey))
		content := notifier.StripEmoji(strings.TrimSpace(m.Content))
		if key == "" || content == "" {
			continue
		}
		scope := notifier.StripEmoji(strings.TrimSpace(m.Scope))
		if scope == "" {
			scope = "global"
		}
		cat := notifier.StripEmoji(strings.TrimSpace(m.Category))
		if cat == "" {
			cat = "general"
		}
		_ = s.store.SaveAgentMemoryDetailed(scope, cat, key, content, clampMemoryImportance(m.Importance), m.Pinned)
	}

	// 5. Restore Custom Skills
	skillsCount := 0
	for _, sk := range backup.Skills {
		if strings.TrimSpace(sk.ID) == "" || strings.TrimSpace(sk.Name) == "" {
			continue
		}
		if err := s.store.SaveSkill(sk); err == nil {
			skillsCount++
		}
	}

	// 6. Restore Custom Bots & Group Configurations
	botsCount := 0
	for _, b := range backup.Bots {
		if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.Name) == "" {
			continue
		}
		if err := s.store.SaveBot(b); err == nil {
			botsCount++
		}
	}

	// 7. Restore Todos & Notes
	todosCount := 0
	for _, td := range backup.Todos {
		if strings.TrimSpace(td.Title) == "" {
			continue
		}
		if _, err := s.store.SaveTodo(td); err == nil {
			todosCount++
		}
	}
	notesCount := 0
	for _, nt := range backup.Notes {
		if strings.TrimSpace(nt.Title) == "" {
			continue
		}
		if _, err := s.store.SaveNote(nt); err == nil {
			notesCount++
		}
	}
	stashCount := 0
	for _, st := range backup.StashItems {
		st.Title = notifier.StripEmoji(strings.TrimSpace(st.Title))
		st.RawInput = notifier.StripEmoji(strings.TrimSpace(st.RawInput))
		st.UserNote = notifier.StripEmoji(strings.TrimSpace(st.UserNote))
		st.AITldr = notifier.StripEmoji(strings.TrimSpace(st.AITldr))
		st.ContentExcerpt = notifier.StripEmoji(strings.TrimSpace(st.ContentExcerpt))
		st.Tags = notifier.StripEmoji(strings.TrimSpace(st.Tags))
		st.ErrorMsg = notifier.StripEmoji(strings.TrimSpace(st.ErrorMsg))
		for i := range st.Points {
			st.Points[i] = notifier.StripEmoji(strings.TrimSpace(st.Points[i]))
		}
		if st.Title == "" && st.URL == "" && st.RawInput == "" {
			continue
		}
		if _, err := s.store.UpsertStashItemFull(st); err == nil {
			stashCount++
		}
	}
	if stashCount > 0 {
		_ = s.store.RebuildStashFTS()
	}

	// 8. Reload scheduler and notify connected clients
	if s.scheduler != nil {
		_ = s.scheduler.Reload()
	}
	if s.push != nil && s.push.Hub() != nil {
		s.push.Hub().PublishEvent("assistant_update", map[string]any{"kind": "backup_restored"})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"message":         fmt.Sprintf("备份恢复成功：已恢复 %d 个资产，%d 个任务，%d 个技能，%d 个 Bot，%d 条待办，%d 篇文档与 %d 条收藏", assetsCount, tasksCount, skillsCount, botsCount, todosCount, notesCount, stashCount),
		"assets_restored": assetsCount,
		"tasks_restored":  tasksCount,
		"skills_restored": skillsCount,
		"bots_restored":   botsCount,
		"todos_restored":  todosCount,
		"notes_restored":  notesCount,
		"stash_restored":  stashCount,
	})
}
