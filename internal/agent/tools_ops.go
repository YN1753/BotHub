package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
	_ "modernc.org/sqlite"
)

func (e *Engine) toolQueryNezhaNodes(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Keyword string `json:"keyword"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)
	kw := strings.ToLower(strings.TrimSpace(args.Keyword))

	type nodeRecord struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		HostInfo   string `json:"host_info,omitempty"`
		LastActive string `json:"last_active,omitempty"`
		Status     string `json:"status"`
		Source     string `json:"source"`
	}
	nodes := make([]nodeRecord, 0)
	sourceUsed := "local_assets"
	nezhaStatus := ""

	// 1. Try read-only direct SQLite connection to Nezha Dashboard DB only when configured
	dbPath, _ := e.store.GetConfig(store.KeyNezhaDBPath)
	dbPath = strings.TrimSpace(dbPath)

	if dbPath != "" {
		info, err := os.Stat(dbPath)
		if err != nil {
			nezhaStatus = fmt.Sprintf("Nezha DB 文件不可访问: %v", err)
		} else if info.IsDir() {
			nezhaStatus = "nezha_db_path 指向了目录而非 SQLite 文件"
		} else {
			dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(3000)", dbPath)
			if db, err := sql.Open("sqlite", dsn); err != nil {
				nezhaStatus = fmt.Sprintf("打开 Nezha SQLite 失败: %v", err)
			} else {
				db.SetMaxOpenConns(1)
				defer db.Close()
				qCtx, qCancel := context.WithTimeout(ctx, 5*time.Second)
				defer qCancel()
				rows, qErr := db.QueryContext(qCtx, `SELECT id, name, COALESCE(host, ''), COALESCE(updated_at, '') FROM servers ORDER BY display_index DESC, id ASC`)
				if qErr != nil {
					nezhaStatus = fmt.Sprintf("查询 Nezha servers 表失败: %v", qErr)
				} else {
					defer rows.Close()
					sourceUsed = "nezha_sqlite_ro"
					for rows.Next() {
						var id int64
						var name, hostJSON, updatedAt string
						if err := rows.Scan(&id, &name, &hostJSON, &updatedAt); err != nil {
							continue
						}
						if kw != "" && !strings.Contains(strings.ToLower(name), kw) && !strings.Contains(strings.ToLower(hostJSON), kw) {
							continue
						}
						cleanHost := notifier.StripEmoji(hostJSON)
						if hr := []rune(cleanHost); len(hr) > 400 {
							cleanHost = string(hr[:400]) + "..."
						}
						nodes = append(nodes, nodeRecord{
							ID:         fmt.Sprintf("nezha-%d", id),
							Name:       notifier.StripEmoji(name),
							HostInfo:   cleanHost,
							LastActive: updatedAt,
							Status:     "online",
							Source:     "nezha_sqlite",
						})
					}
				}
			}
		}
	}

	// 2. Also include local compute assets from BotHub store
	assets, _ := e.store.ListAssets()
	computeAssets := make([]store.Asset, 0)
	for _, a := range assets {
		if a.Category != "compute" {
			continue
		}
		if kw != "" && !strings.Contains(strings.ToLower(a.Name), kw) && !strings.Contains(strings.ToLower(a.Spec), kw) && !strings.Contains(strings.ToLower(a.ID), kw) {
			continue
		}
		computeAssets = append(computeAssets, a)
	}

	payloadMap := map[string]any{
		"source":         sourceUsed,
		"nezha_db_path":  dbPath,
		"nezha_nodes":    nodes,
		"compute_assets": computeAssets,
		"checked_at":     store.NowShanghai(),
	}
	if nezhaStatus != "" {
		payloadMap["nezha_error"] = nezhaStatus
	}
	payload, _ := json.Marshal(payloadMap)
	return string(payload), nil
}

func (e *Engine) toolManageAssets(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action     string `json:"action"`
		Category   string `json:"category"`
		WithinDays int    `json:"within_days"`
		ID         string `json:"id"`
		Name       string `json:"name"`
		Spec       string `json:"spec"`
		Price      string `json:"price"`
		Status     string `json:"status"`
		DueDate    string `json:"due_date"`
		AutoRenew  *bool  `json:"auto_renew"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_assets 参数失败: %w", err)
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		assets, err := e.store.ListAssets()
		if err != nil {
			return "", err
		}
		filtered := make([]store.Asset, 0, len(assets))
		cat := strings.TrimSpace(args.Category)
		for _, a := range assets {
			if cat != "" && a.Category != cat {
				continue
			}
			filtered = append(filtered, a)
		}
		out, _ := json.Marshal(map[string]any{
			"count":  len(filtered),
			"assets": filtered,
		})
		return string(out), nil

	case "summary":
		return e.summarizeAssetsPayload(args.Category)

	case "expiring":
		within := args.WithinDays
		if within <= 0 {
			within = 30
		}
		assets, err := e.store.ListAssets()
		if err != nil {
			return "", err
		}
		expiring := make([]store.Asset, 0)
		for _, a := range assets {
			if store.NormalizeAssetStatus(a.Status) == "已停用" {
				continue
			}
			if a.DueDate == "" || strings.HasPrefix(a.DueDate, "2099") {
				continue
			}
			if a.DaysLeft <= within {
				expiring = append(expiring, a)
			}
		}
		out, _ := json.Marshal(map[string]any{
			"within_days": within,
			"count":       len(expiring),
			"expiring":    expiring,
		})
		return string(out), nil

	case "upsert":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("upsert 资产时 id 必填")
		}
		existing, _ := e.store.GetAsset(id)
		var asset store.Asset
		if existing != nil {
			asset = *existing
		} else {
			asset = store.Asset{
				ID:          id,
				Category:    "compute",
				Status:      "生效中",
				WarnDays:    "[7, 3, 1]",
				ExtraConfig: "{}",
			}
		}
		if strings.TrimSpace(args.Name) != "" {
			asset.Name = notifier.StripEmoji(strings.TrimSpace(args.Name))
		}
		if asset.Name == "" {
			asset.Name = id
		}
		if strings.TrimSpace(args.Category) != "" {
			asset.Category = strings.TrimSpace(args.Category)
		}
		if strings.TrimSpace(args.Spec) != "" {
			asset.Spec = notifier.StripEmoji(strings.TrimSpace(args.Spec))
		}
		if strings.TrimSpace(args.Price) != "" {
			asset.Price = notifier.StripEmoji(strings.TrimSpace(args.Price))
		}
		if strings.TrimSpace(args.Status) != "" {
			asset.Status = store.NormalizeAssetStatus(args.Status)
		}
		if strings.TrimSpace(args.DueDate) != "" {
			asset.DueDate = strings.TrimSpace(args.DueDate)
		}
		if args.AutoRenew != nil {
			asset.AutoRenew = *args.AutoRenew
		}
		asset.LastCheckedAt = store.NowShanghai()
		if err := e.store.SaveAsset(asset); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("assets")
		out, _ := json.Marshal(map[string]any{
			"ok":    true,
			"asset": asset,
		})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("delete 资产时 id 必填")
		}
		if err := e.store.DeleteAsset(id); err != nil {
			return "", err
		}
		e.publishAssistantUpdate("assets")
		out, _ := json.Marshal(map[string]any{"ok": true, "deleted_id": id})
		return string(out), nil

	case "sync":
		tasks, _ := e.store.ListTasks()
		var watcherTaskID string
		for _, t := range tasks {
			if t.Type == "asset_watcher" {
				watcherTaskID = t.ID
				break
			}
		}
		if watcherTaskID != "" && e.scheduler != nil {
			res, err := e.scheduler.RunNow(ctx, watcherTaskID)
			if err != nil {
				return "", err
			}
			e.publishAssistantUpdate("assets")
			out, _ := json.Marshal(map[string]any{
				"ok":      true,
				"task_id": watcherTaskID,
				"summary": res.Summary,
			})
			return string(out), nil
		}
		res, err := modules.NewAssetWatcherModule(e.store, notifier.NewClientWithHub(e.store, e.hub)).Run(ctx)
		if err != nil {
			return "", err
		}
		e.publishAssistantUpdate("assets")
		out, _ := json.Marshal(map[string]any{
			"ok":      true,
			"summary": res.Summary,
		})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_assets action: %s", action)
	}
}

func (e *Engine) toolManageTasks(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action       string `json:"action"`
		ID           string `json:"id"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		BotID        string `json:"bot_id"`
		CronExpr     string `json:"cron_expr"`
		RunAt        string `json:"run_at"`
		DelayMinutes int    `json:"delay_minutes"`
		Prompt       string `json:"prompt"`
		Enabled      *bool  `json:"enabled"`
		Description  string `json:"description"`
		Params       string `json:"params"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("解析 manage_tasks 参数失败: %w", err)
	}

	findTaskByIDOrName := func(wantID, wantName string) (*store.Task, error) {
		if wantID != "" {
			t, err := e.store.GetTask(wantID)
			if err != nil {
				return nil, err
			}
			if t != nil {
				return t, nil
			}
		}
		if wantName != "" {
			all, _ := e.store.ListTasks()
			needle := strings.ToLower(strings.TrimSpace(wantName))
			for i := range all {
				if strings.EqualFold(strings.TrimSpace(all[i].Name), wantName) {
					return &all[i], nil
				}
			}
			for i := range all {
				if strings.Contains(strings.ToLower(all[i].Name), needle) {
					return &all[i], nil
				}
			}
		}
		return nil, nil
	}

	action := strings.ToLower(strings.TrimSpace(args.Action))
	switch action {
	case "list":
		if e.scheduler != nil {
			statuses, err := e.scheduler.Status()
			if err != nil {
				return "", err
			}
			out, _ := json.Marshal(map[string]any{
				"count": len(statuses),
				"tasks": statuses,
			})
			return string(out), nil
		}
		tasks, err := e.store.ListTasks()
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(map[string]any{
			"count": len(tasks),
			"tasks": tasks,
		})
		return string(out), nil

	case "run":
		id := strings.TrimSpace(args.ID)
		if id == "" {
			return "", fmt.Errorf("run 任务时 id 必填")
		}
		if e.scheduler == nil {
			return "", fmt.Errorf("调度器未就绪")
		}
		res, err := e.scheduler.RunNow(ctx, id)
		if err != nil {
			return "", fmt.Errorf("执行任务 %s 失败: %w (摘要: %s)", id, err, res.Summary)
		}
		out, _ := json.Marshal(map[string]any{
			"ok":           true,
			"task_id":      id,
			"summary":      res.Summary,
			"items_pushed": res.ItemsPushed,
		})
		return string(out), nil

	case "upsert":
		sc := toolScopeFromContext(ctx)
		id := strings.TrimSpace(args.ID)
		nameArg := notifier.StripEmoji(strings.TrimSpace(args.Name))
		existing, _ := findTaskByIDOrName(id, nameArg)
		if existing != nil {
			id = existing.ID
		} else if id == "" {
			id = fmt.Sprintf("task_%d", time.Now().UnixNano()%100000000)
		}
		var task store.Task
		if existing != nil {
			task = *existing
		} else {
			defaultBotID := strings.TrimSpace(args.BotID)
			if defaultBotID == "" {
				defaultBotID = strings.TrimSpace(sc.TimelineBotID)
			}
			if defaultBotID == "" {
				defaultBotID = strings.TrimSpace(sc.BotID)
			}
			task = store.Task{
				ID:       id,
				Type:     "bot_prompt",
				BotID:    defaultBotID,
				CronExpr: "0 9 * * *",
				Enabled:  true,
				Params:   "{}",
			}
		}
		if strings.TrimSpace(args.BotID) != "" {
			task.BotID = strings.TrimSpace(args.BotID)
		}
		if nameArg != "" {
			task.Name = nameArg
		}
		if task.Name == "" {
			task.Name = id
		}
		if strings.TrimSpace(args.Type) != "" {
			task.Type = strings.TrimSpace(args.Type)
		}
		if strings.TrimSpace(args.Description) != "" {
			task.Description = notifier.StripEmoji(strings.TrimSpace(args.Description))
		}
		if args.DelayMinutes > 0 {
			loc := store.ShanghaiLocation()
			targetAt := time.Now().In(loc).Add(time.Duration(args.DelayMinutes) * time.Minute)
			task.RunAt = targetAt.Format("2006-01-02 15:04")
			task.CronExpr = ""
		} else if strings.TrimSpace(args.RunAt) != "" {
			runAt := strings.TrimSpace(args.RunAt)
			if _, err := scheduler.ParseRunAt(runAt, nil); err != nil {
				return "", err
			}
			task.RunAt = runAt
			task.CronExpr = ""
		} else if strings.TrimSpace(args.CronExpr) != "" {
			cronExpr := strings.TrimSpace(args.CronExpr)
			if _, err := cron.ParseStandard(cronExpr); err != nil {
				return "", fmt.Errorf("无效 Cron 表达式 %q: %w", cronExpr, err)
			}
			task.CronExpr = cronExpr
			task.RunAt = ""
		}
		if args.Enabled != nil {
			task.Enabled = *args.Enabled
		}
		if strings.TrimSpace(args.Params) != "" {
			var js map[string]any
			if json.Unmarshal([]byte(args.Params), &js) != nil {
				return "", fmt.Errorf("params 必须是合法的 JSON 对象字符串")
			}
			task.Params = strings.TrimSpace(args.Params)
		}
		if p := notifier.StripEmoji(strings.TrimSpace(args.Prompt)); p != "" {
			paramMap := map[string]any{}
			if task.Params != "" && task.Params != "{}" {
				_ = json.Unmarshal([]byte(task.Params), &paramMap)
			}
			paramMap["prompt"] = p
			if b, mErr := json.Marshal(paramMap); mErr == nil {
				task.Params = string(b)
			}
			if task.Description == "" {
				task.Description = truncateTitle(p)
			}
		}
		if err := e.store.SaveTask(task); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		e.publishAssistantUpdate("tasks")
		var nextRuns []string
		if task.RunAt != "" {
			nextRuns = []string{task.RunAt}
		} else if sched, pErr := cron.ParseStandard(task.CronExpr); pErr == nil {
			cur := time.Now().In(store.ShanghaiLocation())
			for i := 0; i < 3; i++ {
				cur = sched.Next(cur)
				nextRuns = append(nextRuns, cur.Format("2006-01-02 15:04"))
			}
		}
		out, _ := json.Marshal(map[string]any{
			"ok":        true,
			"updated":   existing != nil,
			"task":      task,
			"next_runs": nextRuns,
		})
		return string(out), nil

	case "toggle":
		id := strings.TrimSpace(args.ID)
		nameArg := strings.TrimSpace(args.Name)
		if id == "" && nameArg == "" {
			return "", fmt.Errorf("toggle 任务时 id 或 name 必填")
		}
		existing, err := findTaskByIDOrName(id, nameArg)
		if err != nil || existing == nil {
			return "", fmt.Errorf("未找到任务: %s", id+nameArg)
		}
		if args.Enabled != nil {
			existing.Enabled = *args.Enabled
		} else {
			existing.Enabled = !existing.Enabled
		}
		if err := e.store.SaveTask(*existing); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		e.publishAssistantUpdate("tasks")
		out, _ := json.Marshal(map[string]any{
			"ok":      true,
			"id":      existing.ID,
			"name":    existing.Name,
			"enabled": existing.Enabled,
		})
		return string(out), nil

	case "delete":
		id := strings.TrimSpace(args.ID)
		nameArg := strings.TrimSpace(args.Name)
		if id == "" && nameArg == "" {
			return "", fmt.Errorf("delete 任务时 id 或 name 必填")
		}
		if id == "" {
			if existing, _ := findTaskByIDOrName("", nameArg); existing != nil {
				id = existing.ID
			}
		}
		if id == "" {
			return "", fmt.Errorf("未找到待删除任务: %s", nameArg)
		}
		if err := e.store.DeleteTask(id); err != nil {
			return "", err
		}
		if e.scheduler != nil {
			_ = e.scheduler.Reload()
		}
		e.publishAssistantUpdate("tasks")
		out, _ := json.Marshal(map[string]any{"ok": true, "deleted_id": id})
		return string(out), nil

	default:
		return "", fmt.Errorf("不支持的 manage_tasks action: %s", action)
	}
}
