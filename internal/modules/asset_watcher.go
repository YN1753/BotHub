package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/notifier"
	"github.com/YN1753/vesper/bothub/internal/store"
)

// AssetSyncAdapter defines an extensible synchronization plugin for external asset providers.
type AssetSyncAdapter interface {
	ID() string
	Matches(asset store.Asset, extra map[string]any) bool
	Sync(ctx context.Context, st *store.Store, client *http.Client, asset *store.Asset, extra map[string]any) bool
}

type AssetWatcherModule struct {
	store    *store.Store
	push     *notifier.Client
	http     *http.Client
	adapters []AssetSyncAdapter
}

func NewAssetWatcherModule(st *store.Store, push *notifier.Client) *AssetWatcherModule {
	return &AssetWatcherModule{
		store: st,
		push:  push,
		http:  NewHTTPClient(15 * time.Second),
		adapters: []AssetSyncAdapter{
			&cloudflareDomainAdapter{},
			&bilibiliVIPAdapter{},
			&anchorDayAdapter{},
		},
	}
}

// RegisterAdapter allows registering additional custom asset sync adapters cleanly without modifying core watcher logic.
func (w *AssetWatcherModule) RegisterAdapter(adapter AssetSyncAdapter) {
	if adapter != nil {
		w.adapters = append(w.adapters, adapter)
	}
}

func (w *AssetWatcherModule) ID() string          { return "asset_watcher" }
func (w *AssetWatcherModule) Name() string        { return "数字资产守望" }
func (w *AssetWatcherModule) Description() string { return "巡检资产台账周期与扩展适配器状态，触发临期与续费提醒" }

func (w *AssetWatcherModule) Run(ctx context.Context) (RunResult, error) {
	return w.RunWithTask(ctx, store.Task{
		ID:   "asset_watcher",
		Name: "数字资产守望",
		Type: "asset_watcher",
	})
}

func (w *AssetWatcherModule) RunWithTask(ctx context.Context, task store.Task) (RunResult, error) {
	assets, err := w.store.ListAssets()
	if err != nil {
		return RunResult{Summary: "读取资产失败: " + err.Error()}, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var noticeAssets []store.Asset
	nowDateStr := time.Now().In(ShanghaiLocation()).Format("2006-01-02")
	syncedCount := 0
	sem := make(chan struct{}, 6)

	for _, a := range assets {
		wg.Add(1)
		go func(asset store.Asset) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			updated := w.syncSingleAsset(ctx, &asset)
			mu.Lock()
			if updated {
				syncedCount++
			}
			if w.shouldWarn(asset) {
				itemKey := fmt.Sprintf("%s:%s:%d", asset.ID, nowDateStr, asset.DaysLeft)
				seen, _ := w.store.IsSeen("asset_warn", itemKey)
				if !seen {
					noticeAssets = append(noticeAssets, asset)
				}
			}
			mu.Unlock()
		}(a)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return RunResult{Summary: "巡检已取消"}, ctx.Err()
	}

	sort.Slice(noticeAssets, func(i, j int) bool {
		if noticeAssets[i].DaysLeft != noticeAssets[j].DaysLeft {
			return noticeAssets[i].DaysLeft < noticeAssets[j].DaysLeft
		}
		return noticeAssets[i].ID < noticeAssets[j].ID
	})

	itemsPushed := 0
	if len(noticeAssets) > 0 && w.push != nil {
		if err := w.dispatchNotice(ctx, task.ID, noticeAssets, nowDateStr); err == nil {
			itemsPushed = len(noticeAssets)
			warnSeen := make([]store.SeenItem, 0, len(noticeAssets))
			for _, a := range noticeAssets {
				itemKey := fmt.Sprintf("%s:%s:%d", a.ID, nowDateStr, a.DaysLeft)
				warnSeen = append(warnSeen, store.SeenItem{
					Source:  "asset_warn",
					ItemKey: itemKey,
					Title:   a.Name,
					URL:     "bothub://asset/" + a.ID,
				})
			}
			_ = w.store.BatchMarkSeen(warnSeen)
		}
	}

	// Automatic maintenance: prune logs older than retention period
	_ = w.store.PruneOldRecords()

	return RunResult{
		Summary:     fmt.Sprintf("巡检 %d 项资产（同步 %d 项，发送 %d 项临期强提醒）", len(assets), syncedCount, itemsPushed),
		ItemsPushed: itemsPushed,
	}, nil
}

func (w *AssetWatcherModule) syncSingleAsset(ctx context.Context, asset *store.Asset) bool {
	nowStr := store.NowShanghai()
	var extra map[string]any
	_ = json.Unmarshal([]byte(asset.ExtraConfig), &extra)
	if extra == nil {
		extra = make(map[string]any)
	}

	updated := false
	for _, adapter := range w.adapters {
		if adapter.Matches(*asset, extra) {
			if adapter.Sync(ctx, w.store, w.http, asset, extra) {
				updated = true
			}
			break
		}
	}

	asset.Status = store.NormalizeAssetStatus(asset.Status)
	asset.DaysLeft, _ = store.CalculateDaysLeft(asset.DueDate)
	if asset.Status == "生效中" && asset.DaysLeft < 0 && !strings.HasPrefix(asset.DueDate, "2099") {
		asset.Status = "已到期"
	}
	asset.WarnFlag = (asset.Status == "生效中" && asset.DaysLeft >= 0 && asset.DaysLeft <= 7) || asset.Status == "已到期"
	if updated {
		_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
	}
	return updated
}

type cloudflareDomainAdapter struct{}

func (a *cloudflareDomainAdapter) ID() string { return "cloudflare_domain" }
func (a *cloudflareDomainAdapter) Matches(asset store.Asset, extra map[string]any) bool {
	adapter, _ := extra["sync_adapter"].(string)
	accountID, _ := extra["account_id"].(string)
	if strings.TrimSpace(accountID) == "" {
		accountID, _ = extra["cf_account_id"].(string)
	}
	return adapter == "cloudflare_domain" || (asset.Category == "domain" && strings.TrimSpace(accountID) != "")
}
func (a *cloudflareDomainAdapter) Sync(ctx context.Context, st *store.Store, client *http.Client, asset *store.Asset, extra map[string]any) bool {
	domain, _ := extra["domain_name"].(string)
	if domain == "" {
		domain, _ = extra["domain"].(string)
	}
	if domain == "" && strings.Contains(asset.Name, ".") {
		domain = strings.TrimSpace(asset.Name)
	}
	accountID, _ := extra["account_id"].(string)
	if strings.TrimSpace(accountID) == "" {
		accountID, _ = extra["cf_account_id"].(string)
	}
	token, _ := extra["cf_api_token"].(string)
	if strings.TrimSpace(token) == "" {
		token, _ = st.GetConfig(store.KeyCFAPIToken)
	}
	if strings.TrimSpace(token) == "" || strings.TrimSpace(domain) == "" || strings.TrimSpace(accountID) == "" {
		return false
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/registrar/domains/%s", strings.TrimSpace(accountID), strings.TrimSpace(domain))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false
	}
	var res struct {
		Success bool `json:"success"`
		Result  struct {
			ExpiresAt string `json:"expires_at"`
			AutoRenew bool   `json:"auto_renew"`
		} `json:"result"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&res) == nil && res.Success && res.Result.ExpiresAt != "" {
		asset.DueDate = res.Result.ExpiresAt
		asset.AutoRenew = res.Result.AutoRenew
		asset.Status = "生效中"
		return true
	}
	return false
}

type bilibiliVIPAdapter struct{}

func (b *bilibiliVIPAdapter) ID() string { return "bilibili_vip" }
func (b *bilibiliVIPAdapter) Matches(_ store.Asset, extra map[string]any) bool {
	adapter, _ := extra["sync_adapter"].(string)
	platform, _ := extra["platform"].(string)
	return adapter == "bilibili_vip" || strings.EqualFold(platform, "bilibili")
}
func (b *bilibiliVIPAdapter) Sync(ctx context.Context, st *store.Store, client *http.Client, asset *store.Asset, extra map[string]any) bool {
	sessdata, _ := extra["sessdata"].(string)
	if sessdata == "" {
		sessdata, _ = extra["credentials"].(string)
	}
	if sessdata == "" {
		sessdata, _ = st.GetConfig(store.KeyBiliSessdata)
	}
	if strings.TrimSpace(sessdata) == "" {
		return false
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/web-interface/nav", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Cookie", "SESSDATA="+strings.TrimSpace(sessdata))
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return false
	}
	var res struct {
		Code int `json:"code"`
		Data struct {
			Vip struct {
				Status  int   `json:"status"`
				DueDate int64 `json:"due_date"`
			} `json:"vip"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&res) == nil && res.Code == 0 && res.Data.Vip.DueDate > 0 {
		asset.DueDate = time.UnixMilli(res.Data.Vip.DueDate).In(ShanghaiLocation()).Format(time.RFC3339)
		if res.Data.Vip.Status == 1 {
			asset.Status = "生效中"
		} else {
			asset.Status = "已到期"
		}
		return true
	}
	return false
}

type anchorDayAdapter struct{}

func (a *anchorDayAdapter) ID() string { return "monthly_anchor" }
func (a *anchorDayAdapter) Matches(_ store.Asset, extra map[string]any) bool {
	_, ok := extra["anchor_day"]
	return ok
}
func (a *anchorDayAdapter) Sync(_ context.Context, _ *store.Store, _ *http.Client, asset *store.Asset, extra map[string]any) bool {
	anchorVal, ok := extra["anchor_day"]
	if !ok {
		return false
	}
	var anchorDay int
	switch v := anchorVal.(type) {
	case float64:
		anchorDay = int(v)
	case int:
		anchorDay = v
	case string:
		anchorDay, _ = strconv.Atoi(v)
	}
	if anchorDay >= 1 && anchorDay <= 31 {
		next := calcNextAnchorDate(anchorDay, ShanghaiLocation())
		asset.DueDate = next.Format(time.RFC3339)
		asset.Status = "生效中"
		asset.AutoRenew = true
		return true
	}
	return false
}

func (w *AssetWatcherModule) dispatchNotice(ctx context.Context, taskID string, items []store.Asset, dateStr string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 数字资产临期预警 · %s\n\n", dateStr))
	sb.WriteString("守望哨兵检测到以下资产临近到期或自动扣款日：\n\n")

	for i, a := range items {
		autoStr := "否"
		if a.AutoRenew {
			autoStr = "是"
		}

		sb.WriteString(fmt.Sprintf("%d. **%s（%s）**\n", i+1, a.Name, a.Spec))
		sb.WriteString(fmt.Sprintf("   - **到期时间**：%s\n", a.DueDate))
		sb.WriteString(fmt.Sprintf("   - **剩余时间**：**剩余 %d 天** · 自动续费：%s\n", a.DaysLeft, autoStr))

		if a.AutoRenew {
			sb.WriteString("   - **[自动续费提醒]** 该项目已启用自动续费，如不再使用请及时关闭免密/自动扣款授权。\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n*来自 Vesper BotHub 自动化汇报*")
	title := fmt.Sprintf("[资产临期预警] %d 项资产即将到期（最快剩余 %d 天）", len(items), items[0].DaysLeft)
	return w.push.SendMarkdown(ctx, taskID, title, sb.String())
}

// shouldWarn determines if an asset should trigger an alert based on its active status,
// personalized WarnDays gradient, or critical countdown window.
func (w *AssetWatcherModule) shouldWarn(asset store.Asset) bool {
	if store.NormalizeAssetStatus(asset.Status) != "生效中" || asset.DaysLeft < 0 || strings.HasPrefix(asset.DueDate, "2099") {
		return false
	}
	if asset.DaysLeft <= 3 {
		return true
	}
	warnDays := store.ParseWarnDays(asset.WarnDays)
	for _, wd := range warnDays {
		if asset.DaysLeft == wd {
			return true
		}
	}
	return false
}

// calcNextAnchorDate calculates the next billing date for a monthly subscription anchor day,
// safely clamping to the actual days in the target month (e.g., 31st becomes 28th/29th in Feb, 30th in Apr).
func calcNextAnchorDate(anchorDay int, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	y, m, d := now.Date()

	targetYear := y
	targetMonth := m
	if d >= anchorDay {
		if targetMonth == time.December {
			targetYear++
			targetMonth = time.January
		} else {
			targetMonth++
		}
	}

	// Calculate maximum days in target month
	// Date(targetYear, targetMonth+1, 0) gives the last day of targetMonth
	daysInMonth := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, loc).Day()
	actualDay := anchorDay
	if actualDay > daysInMonth {
		actualDay = daysInMonth
	}

	return time.Date(targetYear, targetMonth, actualDay, 0, 0, 0, 0, loc)
}
