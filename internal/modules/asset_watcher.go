package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/YN1753/vesper/bothub/internal/pushplus"
	"github.com/YN1753/vesper/bothub/internal/store"
)

type AssetWatcherModule struct {
	store *store.Store
	push  *pushplus.Client
	http  *http.Client
}

func NewAssetWatcherModule(st *store.Store, push *pushplus.Client) *AssetWatcherModule {
	return &AssetWatcherModule{store: st, push: push, http: NewHTTPClient(15 * time.Second)}
}

func (w *AssetWatcherModule) ID() string          { return "asset_watcher" }
func (w *AssetWatcherModule) Name() string        { return "数字资产与会员守望" }
func (w *AssetWatcherModule) Description() string { return "嗅探域名、B站/QQ音乐会员与VPS周期，触发临期与防扣款微信预警" }

func (w *AssetWatcherModule) Run(ctx context.Context) (RunResult, error) {
	assets, err := w.store.ListAssets()
	if err != nil {
		return RunResult{Summary: "读取资产失败: " + err.Error()}, err
	}

	globalCFToken, _ := w.store.GetConfig(store.KeyCFAPIToken)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var noticeAssets []store.Asset
	nowDateStr := time.Now().In(ShanghaiLocation()).Format("2006-01-02")
	syncedCount := 0

	for _, a := range assets {
		wg.Add(1)
		go func(asset store.Asset) {
			defer wg.Done()
			updated := w.syncSingleAsset(ctx, &asset, globalCFToken)
			mu.Lock()
			if updated {
				syncedCount++
			}
			// Check expiration threshold
			if asset.DaysLeft <= 7 && asset.DaysLeft >= 0 {
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

	itemsPushed := 0
	if len(noticeAssets) > 0 {
		if err := w.dispatchNotice(ctx, noticeAssets, nowDateStr); err == nil {
			itemsPushed = len(noticeAssets)
			for _, a := range noticeAssets {
				itemKey := fmt.Sprintf("%s:%s:%d", a.ID, nowDateStr, a.DaysLeft)
				_ = w.store.MarkSeen("asset_warn", itemKey, a.Name, "bothub://asset/"+a.ID)
			}
		}
	}

	return RunResult{
		Summary:     fmt.Sprintf("巡检 %d 项资产（同步 %d 项，发送 %d 项临期强提醒）", len(assets), syncedCount, itemsPushed),
		ItemsPushed: itemsPushed,
	}, nil
}

func (w *AssetWatcherModule) syncSingleAsset(ctx context.Context, asset *store.Asset, globalCFToken string) bool {
	nowStr := store.NowShanghai()
	var extra map[string]any
	_ = json.Unmarshal([]byte(asset.ExtraConfig), &extra)
	if extra == nil {
		extra = make(map[string]any)
	}

	updated := false
	if asset.Category == "domain" || strings.HasPrefix(asset.ID, "cf_") {
		domain, _ := extra["domain"].(string)
		if domain == "" { domain = asset.Name }
		accountID, _ := extra["account_id"].(string)
		if accountID == "" { accountID = "0c248525a82b6da799e178614786b970" }
		token, _ := extra["cf_api_token"].(string)
		if token == "" { token = globalCFToken }

		if token != "" && domain != "" && accountID != "" {
			url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/registrar/domains/%s", accountID, domain)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			if resp, err := w.http.Do(req); err == nil {
				defer resp.Body.Close()
				var res struct {
					Success bool `json:"success"`
					Result  struct {
						ExpiresAt string `json:"expires_at"`
						AutoRenew bool   `json:"auto_renew"`
					} `json:"result"`
				}
				if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Success && res.Result.ExpiresAt != "" {
					asset.DueDate = res.Result.ExpiresAt
					asset.AutoRenew = res.Result.AutoRenew
					asset.Status = "运行中"
					updated = true
				}
			}
		}
	} else if asset.ID == "bilibili" {
		sessdata, _ := extra["sessdata"].(string)
		if sessdata != "" {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/web-interface/nav", nil)
			req.Header.Set("User-Agent", "Mozilla/5.0")
			req.Header.Set("Cookie", "SESSDATA="+strings.TrimSpace(sessdata))
			if resp, err := w.http.Do(req); err == nil {
				defer resp.Body.Close()
				var res struct {
					Code int `json:"code"`
					Data struct {
						Vip struct {
							Status  int   `json:"status"`
							DueDate int64 `json:"due_date"`
						} `json:"vip"`
					} `json:"data"`
				}
				if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Code == 0 && res.Data.Vip.DueDate > 0 {
					asset.DueDate = time.UnixMilli(res.Data.Vip.DueDate).In(ShanghaiLocation()).Format(time.RFC3339)
					if res.Data.Vip.Status == 1 {
						asset.Status = "会员有效"
					} else {
						asset.Status = "已到期"
					}
					updated = true
				}
			}
		}
	} else if anchorVal, ok := extra["anchor_day"]; ok {
		var anchorDay int
		switch v := anchorVal.(type) {
		case float64: anchorDay = int(v)
		case int:     anchorDay = v
		case string:  anchorDay, _ = strconv.Atoi(v)
		}
		if anchorDay >= 1 && anchorDay <= 31 {
			now := time.Now().In(ShanghaiLocation())
			y, m, d := now.Date()
			if d >= anchorDay {
				m++
			}
			next := time.Date(y, m, anchorDay, 0, 0, 0, 0, ShanghaiLocation())
			asset.DueDate = next.Format(time.RFC3339)
			asset.Status = "运行中"
			asset.AutoRenew = true
			updated = true
		}
	}

	asset.DaysLeft, _ = store.CalculateDaysLeft(asset.DueDate)
	asset.WarnFlag = (asset.DaysLeft >= 0 && asset.DaysLeft <= 7) || strings.Contains(asset.Status, "到期")
	if updated {
		_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
	}
	return updated
}

func (w *AssetWatcherModule) dispatchNotice(ctx context.Context, items []store.Asset, dateStr string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 数字资产与会员临期预警 · %s\n\n", dateStr))
	sb.WriteString("守望哨兵检测到以下关键资产临近到期或扣费日：\n\n")

	for i, a := range items {
		autoStr := "否"
		if a.AutoRenew { autoStr = "是" }

		sb.WriteString(fmt.Sprintf("%d. **%s（%s）**\n", i+1, a.Name, a.Spec))
		sb.WriteString(fmt.Sprintf("   - **到期时间**：%s\n", a.DueDate))
		sb.WriteString(fmt.Sprintf("   - **剩余时间**：**剩余 %d 天** · 自动续费：%s\n", a.DaysLeft, autoStr))

		if strings.Contains(a.Price, "活动") || strings.Contains(a.Status, "活动") || strings.Contains(a.Name, "QQ") {
			sb.WriteString("   - **[防自动扣款提醒]** 您的活动免费体验即将到期，请及时前往微信/支付宝确认是否已关闭免密续费授权！\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n*来自 Vesper BotHub 守望哨兵 · bot.starryheavens.uk*")
	title := fmt.Sprintf("[资产临期预警] %d 项资产即将到期（最快剩余 %d 天）", len(items), items[0].DaysLeft)
	return w.push.SendMarkdown(ctx, w.ID(), title, sb.String())
}
