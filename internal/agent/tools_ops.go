package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/YN1753/vesper/bothub/internal/modules"
	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/scheduler"
	"github.com/YN1753/vesper/bothub/internal/store"
	"github.com/robfig/cron/v3"
	_ "modernc.org/sqlite"
)

func queryNezhaNodesToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "query_nezha_nodes",
				Description: "查询哪吒探针（Nezha）服务器节点的实时在线状态、系统信息、最近心跳与本地算力资产台账。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"keyword": map[string]any{
							"type":        "string",
							"description": "可选：按节点名称或备注过滤关键词",
						},
					},
				},
			},
		},
		Timeout:        25 * time.Second,
		ReadOnly:       true,
		StatusLabel:    "正在巡检探针节点...",
		DomainKeywords: []string{"探针", "哪吒", "vps", "节点", "服务器", "cpu", "内存", "负载", "掉线", "宕机", "告警", "延迟", "ssh", "nezha"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolQueryNezhaNodes(ctx, cleanArgs)
		},
	}
}

func manageAssetsToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_assets",
				Description: "查询或管理 BotHub 资产与周期账单台账（支持任意自定义分类与可扩展属性）。支持 list（列表）、summary（月度与年度费用分币种汇总）、expiring（查询即将到期）、upsert（新增或修改资产）、delete（删除资产）、sync（立即执行资产外部同步任务）。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "summary", "expiring", "upsert", "delete", "sync"},
							"description": "操作类型：list | summary | expiring | upsert | delete | sync",
						},
						"category": map[string]any{
							"type":        "string",
							"description": "可选：按分类过滤或设置分类（支持任意用户自定义分类标识或名称）",
						},
						"within_days": map[string]any{
							"type":        "integer",
							"description": "当 action=expiring 时，筛选多少天内到期的资产（默认 30）",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "资产唯一标识 ID（upsert 或 delete 时必填）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "资产名称（upsert 时使用）",
						},
						"spec": map[string]any{
							"type":        "string",
							"description": "规格或区域描述（upsert 时使用）",
						},
						"price": map[string]any{
							"type":        "string",
							"description": "费用与周期描述，如 $15.00 / 年、¥99 / 月（upsert 时使用）",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "生命周期状态：生效中 | 已到期 | 已停用（upsert 时使用）",
						},
						"due_date": map[string]any{
							"type":        "string",
							"description": "到期日期 YYYY-MM-DD（upsert 时使用）",
						},
						"auto_renew": map[string]any{
							"type":        "boolean",
							"description": "是否自动续费（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在核查资产台账...",
		DomainKeywords: []string{"资产", "域名", "到期", "续费", "账单", "会员", "cloudflare"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageAssets(ctx, cleanArgs)
		},
	}
}

func manageTasksToolSpec() ToolSpec {
	return ToolSpec{
		Definition: ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        "manage_tasks",
				Description: "查询或管理 BotHub 自动化调度任务与一次性定时提醒。支持 list、run、upsert（支持周期 cron_expr 或一次性 delay_minutes / run_at 提醒，默认绑定当前 Bot）、toggle、delete。",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"enum":        []string{"list", "run", "upsert", "toggle", "delete"},
							"description": "操作类型：list | run | upsert | toggle | delete",
						},
						"id": map[string]any{
							"type":        "string",
							"description": "任务 ID（run / upsert / toggle / delete 时填写，新建也可自动生成）",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "任务或提醒名称（upsert 时使用）",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "任务执行器类型，默认 bot_prompt（也支持 morning / radar / asset_watcher）",
						},
						"bot_id": map[string]any{
							"type":        "string",
							"description": "可选：绑定的 Bot ID，缺省自动绑定当前对话 Bot",
						},
						"cron_expr": map[string]any{
							"type":        "string",
							"description": "标准 5 段周期 Cron 表达式（与一次性 run_at / delay_minutes 二选一）",
						},
						"run_at": map[string]any{
							"type":        "string",
							"description": "可选：一次性触发的上海时间 YYYY-MM-DD HH:MM（到点自动执行一次后停用）",
						},
						"delay_minutes": map[string]any{
							"type":        "integer",
							"description": "可选：多少分钟后触发一次性提醒（自动换算 run_at，如 15 表示 15 分钟后）",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "可选：bot_prompt 类型任务到期时要执行的具体指令内容",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "是否启用任务（upsert 或 toggle 时使用）",
						},
						"model": map[string]any{
							"type":        "string",
							"description": "可选：任务指定模型（如 auto:free、auto:paid 或 pool:<id>[::<model>]，留空沿用默认）",
						},
						"allow_paid_fallback": map[string]any{
							"type":        "boolean",
							"description": "可选：是否允许该任务在免费池不可用或指定付费模型时使用付费模型池（默认 false，防误消耗）",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "任务说明（upsert 时使用）",
						},
						"params": map[string]any{
							"type":        "string",
							"description": "JSON 字符串格式的任务参数（upsert 时使用）",
						},
					},
					"required": []string{"action"},
				},
			},
		},
		StatusLabel:    "正在安排调度提醒...",
		DomainKeywords: []string{"定时", "例程", "cron", "日程", "晨报", "调度"},
		Execute: func(ctx context.Context, e *Engine, cleanArgs string) (string, error) {
			return e.toolManageTasks(ctx, cleanArgs)
		},
	}
}

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

var reAssetNumber = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

func (e *Engine) summarizeAssetsPayload(categoryFilter string) (string, error) {
	assets, err := e.store.ListAssets()
	if err != nil {
		return "", err
	}
	type currTotal struct {
		Currency   string  `json:"currency"`
		Monthly    float64 `json:"monthly_estimated"`
		Annualized float64 `json:"annualized_estimated"`
		Count      int     `json:"priced_items"`
	}
	currMap := make(map[string]*currTotal)
	byCategory := make(map[string]int)
	var activeCount, expiring30Count, inactiveCount int
	cat := strings.TrimSpace(categoryFilter)

	for _, a := range assets {
		if cat != "" && a.Category != cat {
			continue
		}
		if store.NormalizeAssetStatus(a.Status) == "已停用" {
			inactiveCount++
			continue
		}
		activeCount++
		catName := strings.TrimSpace(a.Category)
		if catName == "" {
			catName = "other"
		}
		byCategory[catName]++
		if a.DueDate != "" && !strings.HasPrefix(a.DueDate, "2099") && a.DaysLeft <= 30 {
			expiring30Count++
		}
		pStr := strings.TrimSpace(a.Price)
		if pStr == "" {
			continue
		}
		m := reAssetNumber.FindStringSubmatch(pStr)
		if len(m) < 2 {
			continue
		}
		amt, pErr := strconv.ParseFloat(m[1], 64)
		if pErr != nil || amt <= 0 {
			continue
		}
		currency := "USD"
		lowerP := strings.ToLower(pStr)
		switch {
		case strings.Contains(pStr, "¥") || strings.Contains(lowerP, "cny") || strings.Contains(lowerP, "rmb") || strings.Contains(pStr, "元"):
			currency = "CNY"
		case strings.Contains(pStr, "€") || strings.Contains(lowerP, "eur"):
			currency = "EUR"
		case strings.Contains(lowerP, "hkd"):
			currency = "HKD"
		}
		monthly := amt
		annual := amt * 12
		switch {
		case strings.Contains(pStr, "年") || strings.Contains(lowerP, "/yr") || strings.Contains(lowerP, "year") || strings.Contains(lowerP, "/y"):
			monthly = amt / 12
			annual = amt
		case strings.Contains(pStr, "季") || strings.Contains(lowerP, "quarter"):
			monthly = amt / 3
			annual = amt * 4
		case strings.Contains(pStr, "次") || strings.Contains(lowerP, "one"):
			monthly = 0
			annual = 0
		}
		entry, ok := currMap[currency]
		if !ok {
			entry = &currTotal{Currency: currency}
			currMap[currency] = entry
		}
		entry.Monthly = math.Round((entry.Monthly+monthly)*100) / 100
		entry.Annualized = math.Round((entry.Annualized+annual)*100) / 100
		entry.Count++
	}

	breakdown := make([]currTotal, 0, len(currMap))
	for _, v := range currMap {
		breakdown = append(breakdown, *v)
	}
	out, _ := json.Marshal(map[string]any{
		"active_count":       activeCount,
		"inactive_count":     inactiveCount,
		"expiring_30d_count": expiring30Count,
		"by_category":        byCategory,
		"cost_by_currency":   breakdown,
	})
	return string(out), nil
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
		Action            string `json:"action"`
		ID                string `json:"id"`
		Name              string `json:"name"`
		Type              string `json:"type"`
		BotID             string `json:"bot_id"`
		CronExpr          string `json:"cron_expr"`
		RunAt             string `json:"run_at"`
		DelayMinutes      int    `json:"delay_minutes"`
		Prompt            string `json:"prompt"`
		Enabled           *bool  `json:"enabled"`
		Model             string `json:"model"`
		AllowPaidFallback *bool  `json:"allow_paid_fallback"`
		Description       string `json:"description"`
		Params            string `json:"params"`
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
		if m := strings.TrimSpace(args.Model); m != "" {
			task.Model = m
		}
		if args.AllowPaidFallback != nil {
			task.AllowPaidFallback = *args.AllowPaidFallback
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
