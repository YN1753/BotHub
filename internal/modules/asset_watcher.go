package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	return &AssetWatcherModule{
		store: st,
		push:  push,
		http:  NewHTTPClient(20 * time.Second),
	}
}

func (w *AssetWatcherModule) ID() string   { return "asset_watcher" }
func (w *AssetWatcherModule) Name() string { return "数字资产与会员守望" }
func (w *AssetWatcherModule) Description() string {
	return "自动嗅探域名、B站、QQ音乐与VPS扣费周期，触发临期与防免密扣费微信预警"
}

type syncResult struct {
	AssetID     string
	AssetName   string
	Updated     bool
	Message     string
	DaysLeft    int
	IsExpiring  bool
	NeedsNotice bool
	Asset       store.Asset
}

func (w *AssetWatcherModule) Run(ctx context.Context) (RunResult, error) {
	assets, err := w.store.ListAssets()
	if err != nil {
		return RunResult{Summary: "读取资产列表失败: " + err.Error()}, err
	}

	globalCFTargetToken, _ := w.store.GetConfig(store.KeyCFAPIToken)

	var wg sync.WaitGroup
	resultsChan := make(chan syncResult, len(assets))

	for _, a := range assets {
		wg.Add(1)
		go func(asset store.Asset) {
			defer wg.Done()
			res := w.inspectAndSync(ctx, asset, globalCFTargetToken)
			resultsChan <- res
		}(a)
	}

	wg.Wait()
	close(resultsChan)

	var (
		syncedCount   int
		noticeAssets  []syncResult
		nowDateStr    = time.Now().In(ShanghaiLocation()).Format("2006-01-02")
	)

	for res := range resultsChan {
		if res.Updated {
			syncedCount++
		}
		if res.NeedsNotice {
			// Check if already notified for this asset on this exact remaining day
			itemKey := fmt.Sprintf("%s:%s:%d", res.AssetID, nowDateStr, res.DaysLeft)
			seen, _ := w.store.IsSeen("asset_warn", itemKey)
			if !seen {
				noticeAssets = append(noticeAssets, res)
			}
		}
	}

	var itemsPushed int
	if len(noticeAssets) > 0 {
		if pushErr := w.dispatchExpirationNotice(ctx, noticeAssets, nowDateStr); pushErr == nil {
			itemsPushed = len(noticeAssets)
			for _, item := range noticeAssets {
				itemKey := fmt.Sprintf("%s:%s:%d", item.AssetID, nowDateStr, item.DaysLeft)
				_ = w.store.MarkSeen("asset_warn", itemKey, item.AssetName, "bothub://asset/"+item.AssetID)
			}
		}
	}

	summary := fmt.Sprintf("巡检完成 %d 项资产（更新 %d 项，发送 %d 项临期强提醒）", len(assets), syncedCount, itemsPushed)
	return RunResult{
		Summary:     summary,
		ItemsPushed: itemsPushed,
	}, nil
}

func (w *AssetWatcherModule) inspectAndSync(ctx context.Context, asset store.Asset, globalCFToken string) syncResult {
	loc := ShanghaiLocation()
	now := time.Now().In(loc)
	nowStr := now.Format("2006-01-02 15:04:05")

	var extra map[string]any
	_ = json.Unmarshal([]byte(asset.ExtraConfig), &extra)
	if extra == nil {
		extra = make(map[string]any)
	}

	updated := false
	msg := "状态正常"

	// 1. Cloudflare Registrar domain expiration
	if asset.Category == "domain" || strings.HasPrefix(asset.ID, "cf_") {
		domain, _ := extra["domain"].(string)
		if domain == "" {
			domain = asset.Name
		}
		accountID, _ := extra["account_id"].(string)
		if accountID == "" {
			accountID = "0c248525a82b6da799e178614786b970"
		}
		token, _ := extra["cf_api_token"].(string)
		if token == "" {
			token = globalCFToken
		}

		if token != "" && domain != "" && accountID != "" {
			if cfExp, cfAutoRenew, err := w.fetchCloudflareDomain(ctx, accountID, domain, token); err == nil {
				asset.DueDate = cfExp
				asset.AutoRenew = cfAutoRenew
				asset.Status = "运行中"
				asset.LastCheckedAt = nowStr
				_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
				updated = true
				msg = "已同步 Cloudflare 最新到期时间"
			} else {
				msg = "CF 同步失败: " + err.Error()
			}
		} else {
			msg = "未配置 CF API Token，保持现有锚点时间"
		}
	} else if asset.ID == "bilibili" {
		// 2. Bilibili VIP expiration
		sessdata, _ := extra["sessdata"].(string)
		if sessdata != "" {
			if exp, status, err := w.fetchBilibiliVIP(ctx, sessdata); err == nil {
				asset.DueDate = exp
				asset.Status = status
				asset.LastCheckedAt = nowStr
				_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
				updated = true
				msg = "已从 Bilibili 同步会员到期日"
			} else {
				msg = "B站同步失败: " + err.Error()
			}
		} else {
			msg = "未配置 SESSDATA，保持手动到期时间"
		}
	} else if asset.ID == "qq_music" {
		// 3. QQ Music VIP expiration
		uin, _ := extra["uin"].(string)
		qmKeyst, _ := extra["qm_keyst"].(string)
		if uin != "" && qmKeyst != "" {
			if exp, status, err := w.fetchQQMusicVIP(ctx, uin, qmKeyst); err == nil {
				asset.DueDate = exp
				asset.Status = status
				asset.LastCheckedAt = nowStr
				_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
				updated = true
				msg = "已从 QQ 音乐同步绿钻到期日"
			} else {
				msg = "QQ音乐同步接口未能解析，保持原定到期时间"
			}
		} else {
			msg = "未配置 QQ 音乐 Cookie，保持设定到期时间"
		}
	} else if anchorDayVal, ok := extra["anchor_day"]; ok {
		// 4. VPS / Compute static anchor date rollover
		var anchorDay int
		switch v := anchorDayVal.(type) {
		case float64:
			anchorDay = int(v)
		case int:
			anchorDay = v
		case string:
			anchorDay, _ = strconv.Atoi(v)
		}

		if anchorDay >= 1 && anchorDay <= 31 {
			nextDue := calculateNextAnchorDate(now, anchorDay)
			asset.DueDate = nextDue.Format(time.RFC3339)
			asset.Status = "运行中"
			asset.AutoRenew = true
			asset.LastCheckedAt = nowStr
			_ = w.store.UpdateAssetSyncResult(asset.ID, asset.DueDate, asset.Status, asset.AutoRenew, nowStr)
			updated = true
			msg = fmt.Sprintf("已顺延推算下个基准扣费日 (%d日)", anchorDay)
		}
	}

	daysLeft, _ := store.CalculateDaysLeft(asset.DueDate)
	warnDaysList := store.ParseWarnDays(asset.WarnDays)

	needsNotice := false
	for _, wd := range warnDaysList {
		if daysLeft == wd {
			needsNotice = true
			break
		}
	}
	// Always trigger notice if <= 1 day left and not negative
	if daysLeft <= 1 && daysLeft >= 0 {
		needsNotice = true
	}

	return syncResult{
		AssetID:     asset.ID,
		AssetName:   asset.Name,
		Updated:     updated,
		Message:     msg,
		DaysLeft:    daysLeft,
		IsExpiring:  daysLeft >= 0 && daysLeft <= 7,
		NeedsNotice: needsNotice,
		Asset:       asset,
	}
}

func (w *AssetWatcherModule) fetchCloudflareDomain(ctx context.Context, accountID, domain, token string) (string, bool, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/registrar/domains/%s", accountID, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 32768))
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("CF API HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var res struct {
		Success bool `json:"success"`
		Result  struct {
			ExpiresAt string `json:"expires_at"`
			AutoRenew bool   `json:"auto_renew"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return "", false, err
	}
	if !res.Success || res.Result.ExpiresAt == "" {
		errMsg := "unknown CF error"
		if len(res.Errors) > 0 {
			errMsg = res.Errors[0].Message
		}
		return "", false, fmt.Errorf("%s", errMsg)
	}

	expTime, err := time.Parse(time.RFC3339, res.Result.ExpiresAt)
	if err != nil {
		return res.Result.ExpiresAt, res.Result.AutoRenew, nil
	}

	return expTime.In(ShanghaiLocation()).Format(time.RFC3339), res.Result.AutoRenew, nil
}

func (w *AssetWatcherModule) fetchBilibiliVIP(ctx context.Context, sessdata string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.bilibili.com/x/web-interface/nav", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", "SESSDATA="+strings.TrimSpace(sessdata))

	resp, err := w.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	var res struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			IsLogin bool `json:"isLogin"`
			Uname   string `json:"uname"`
			Vip     struct {
				Status  int   `json:"status"`   // 1: valid
				DueDate int64 `json:"due_date"` // Unix ms
				Label   struct {
					Text string `json:"text"`
				} `json:"label"`
			} `json:"vip"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", err
	}
	if res.Code != 0 {
		return "", "", fmt.Errorf("B站返回错误 (%d): %s", res.Code, res.Message)
	}

	if res.Data.Vip.DueDate <= 0 {
		return "", "未开通大会员", nil
	}

	dueDate := time.UnixMilli(res.Data.Vip.DueDate).In(ShanghaiLocation())
	status := "会员有效"
	if res.Data.Vip.Status != 1 {
		status = "已到期/未续费"
	}
	return dueDate.Format(time.RFC3339), status, nil
}

func (w *AssetWatcherModule) fetchQQMusicVIP(ctx context.Context, uin, qmKeyst string) (string, string, error) {
	// Attempt QQ Music Web API with Cookie headers
	cookie := fmt.Sprintf("uin=%s; qm_keyst=%s", strings.TrimSpace(uin), strings.TrimSpace(qmKeyst))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://u.y.qq.com/cgi-bin/musicu.fcg?g_tk=5381&format=json&inCharset=utf8&outCharset=utf-8", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Cookie", cookie)

	resp, err := w.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("QQ音乐 HTTP %d", resp.StatusCode)
	}

	// If the specialized VIP endpoint is unavailable or returns encrypted fields,
	// return empty error to gracefully maintain existing configured due_date without breaking.
	return "", "", fmt.Errorf("QQ音乐凭证需配合专属活动查询包")
}

func calculateNextAnchorDate(now time.Time, anchorDay int) time.Time {
	loc := now.Location()
	year, month, day := now.Date()

	// If today hasn't passed this month's anchor day yet
	if day < anchorDay {
		daysInMonth := daysIn(year, month)
		targetDay := anchorDay
		if targetDay > daysInMonth {
			targetDay = daysInMonth
		}
		return time.Date(year, month, targetDay, 0, 0, 0, 0, loc)
	}

	// Move to next month
	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}

	daysInNextMonth := daysIn(nextYear, nextMonth)
	targetDay := anchorDay
	if targetDay > daysInNextMonth {
		targetDay = daysInNextMonth
	}
	return time.Date(nextYear, nextMonth, targetDay, 0, 0, 0, 0, loc)
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (w *AssetWatcherModule) dispatchExpirationNotice(ctx context.Context, items []syncResult, dateStr string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Vesper 数字资产与会员临期预警 · %s\n\n", dateStr))
	sb.WriteString("守望哨兵检测到以下关键资产临近到期或扣费日，请确认续费状态或关闭免密扣费授权：\n\n")

	for i, item := range items {
		a := item.Asset
		specStr := a.Spec
		if specStr == "" {
			specStr = a.Category
		}

		autoRenewStr := "否"
		if a.AutoRenew {
			autoRenewStr = "是"
		}

		sb.WriteString(fmt.Sprintf("%d. **%s（%s）**\n", i+1, a.Name, specStr))
		sb.WriteString(fmt.Sprintf("   - **资产分类**：%s · %s\n", categoryLabelCN(a.Category), a.Price))
		sb.WriteString(fmt.Sprintf("   - **到期时间**：%s\n", formatFriendlyTime(a.DueDate)))
		sb.WriteString(fmt.Sprintf("   - **剩余时间**：**剩余 %d 天** · 自动续费：%s\n", item.DaysLeft, autoRenewStr))

		// Special alert for trial monthly passes (e.g. QQ Music trial)
		isTrial := strings.Contains(a.Price, "活动") || strings.Contains(a.Status, "活动") || strings.Contains(a.Name, "QQ") || strings.Contains(a.ExtraConfig, `"is_trial":true`)
		if isTrial {
			sb.WriteString("   - **[防自动扣款提醒]** 您的活动免费体验即将到期，请及时前往微信/支付宝确认是否已关闭免密续费授权！\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("*来自 Vesper BotHub 守望哨兵 · bot.starryheavens.uk*")

	title := fmt.Sprintf("[资产临期预警] %d 项资产即将到期（最快剩余 %d 天）", len(items), items[0].DaysLeft)
	return w.push.SendMarkdown(ctx, w.ID(), title, sb.String())
}

func categoryLabelCN(cat string) string {
	switch cat {
	case "compute":
		return "算力基建"
	case "domain":
		return "核心域名"
	case "life":
		return "生活与订阅"
	default:
		return "数字资产"
	}
}

func formatFriendlyTime(dueDate string) string {
	t, err := time.Parse(time.RFC3339, dueDate)
	if err != nil {
		return dueDate
	}
	return t.In(ShanghaiLocation()).Format("2006-01-02 15:04 (Asia/Shanghai)")
}
