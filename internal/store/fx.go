package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	reFXPriceAmount = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

	fxRefreshMu        sync.Mutex
	fxRefreshing       bool
	fxLastAttemptAt    time.Time
	fxDisableAutoFetch bool // Set true in unit tests if needed to avoid external network calls

	fxPrimaryEndpoint   = "https://open.er-api.com/v6/latest/CNY"
	fxSecondaryEndpoint = "https://api.frankfurter.app/latest?from=CNY"
)

// FXRateSnapshot stores cached exchange rates where 1 unit of foreign currency = Rates[code] CNY.
type FXRateSnapshot struct {
	Base      string             `json:"base"`
	Rates     map[string]float64 `json:"rates"`
	UpdatedAt string             `json:"updated_at"`
	Source    string             `json:"source"`
}

// DefaultFXRatesToCNY provides built-in offline fallback rates (1 unit of currency -> CNY)
// so asset totals always render accurately even before initial network sync or during outages.
func DefaultFXRatesToCNY() map[string]float64 {
	return map[string]float64{
		"CNY": 1.0,
		"RMB": 1.0,
		"USD": 7.24,
		"EUR": 7.82,
		"HKD": 0.93,
		"GBP": 9.18,
		"JPY": 0.0475,
		"TWD": 0.224,
		"SGD": 5.38,
		"CAD": 5.16,
		"AUD": 4.70,
		"KRW": 0.0052,
	}
}

// GetFXRatesCNY returns the persisted FXRateSnapshot from SQLite (merged with built-in fallback rates),
// and triggers a non-blocking background refresh if the cache is missing or older than 6 hours.
func (s *Store) GetFXRatesCNY() FXRateSnapshot {
	snap := FXRateSnapshot{
		Base:   "CNY",
		Rates:  DefaultFXRatesToCNY(),
		Source: "builtin_fallback",
	}
	if s == nil {
		return snap
	}
	raw, _ := s.GetConfig(KeyFXRatesCNY)
	if strings.TrimSpace(raw) != "" {
		var saved FXRateSnapshot
		if json.Unmarshal([]byte(raw), &saved) == nil && len(saved.Rates) > 0 {
			for k, v := range saved.Rates {
				if v > 0 {
					snap.Rates[strings.ToUpper(strings.TrimSpace(k))] = v
				}
			}
			snap.Rates["CNY"] = 1.0
			snap.Rates["RMB"] = 1.0
			if saved.UpdatedAt != "" {
				snap.UpdatedAt = saved.UpdatedAt
			}
			if saved.Source != "" {
				snap.Source = saved.Source
			}
		}
	}

	if !fxDisableAutoFetch && s.isFXCacheStale(snap.UpdatedAt, 6*time.Hour) {
		s.triggerAsyncFXRefresh()
	}
	return snap
}

func (s *Store) isFXCacheStale(updatedAt string, maxAge time.Duration) bool {
	if strings.TrimSpace(updatedAt) == "" {
		return true
	}
	loc := ShanghaiLocation()
	ts, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(updatedAt), loc)
	if err != nil {
		return true
	}
	return time.Now().In(loc).Sub(ts) >= maxAge
}

func (s *Store) triggerAsyncFXRefresh() {
	fxRefreshMu.Lock()
	if fxRefreshing || (!fxLastAttemptAt.IsZero() && time.Since(fxLastAttemptAt) < 15*time.Minute) {
		fxRefreshMu.Unlock()
		return
	}
	fxRefreshing = true
	fxLastAttemptAt = time.Now()
	fxRefreshMu.Unlock()

	go func() {
		defer func() {
			fxRefreshMu.Lock()
			fxRefreshing = false
			fxRefreshMu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_, _ = s.RefreshFXRatesCNY(ctx)
	}()
}

// RefreshFXRatesCNY fetches live exchange rates against CNY from public keyless APIs
// and persists the result into SQLite configs (fx_rates_cny).
func (s *Store) RefreshFXRatesCNY(ctx context.Context) (FXRateSnapshot, error) {
	client := &http.Client{Timeout: 6 * time.Second}

	fetchInverseRates := func(endpoint, sourceName string) (map[string]float64, string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("User-Agent", "VesperBotHub/2.73")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
		if err != nil {
			return nil, "", err
		}
		var payload struct {
			Rates map[string]float64 `json:"rates"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || len(payload.Rates) == 0 {
			return nil, "", fmt.Errorf("invalid fx response")
		}
		toCNY := DefaultFXRatesToCNY()
		for code, perCNY := range payload.Rates {
			upper := strings.ToUpper(strings.TrimSpace(code))
			if perCNY <= 0 {
				continue
			}
			cnyPerUnit := 1.0 / perCNY
			if cnyPerUnit >= 0.001 {
				toCNY[upper] = math.Round(cnyPerUnit*10000) / 10000
			} else {
				toCNY[upper] = math.Round(cnyPerUnit*1000000) / 1000000
			}
		}
		toCNY["CNY"] = 1.0
		toCNY["RMB"] = 1.0
		return toCNY, sourceName, nil
	}

	rates, src, err := fetchInverseRates(fxPrimaryEndpoint, "open.er-api.com")
	if err != nil {
		rates, src, err = fetchInverseRates(fxSecondaryEndpoint, "frankfurter.app")
	}
	if err != nil {
		return s.GetFXRatesCNY(), err
	}

	snap := FXRateSnapshot{
		Base:      "CNY",
		Rates:     rates,
		UpdatedAt: NowShanghai(),
		Source:    src,
	}
	if s != nil {
		if b, mErr := json.Marshal(snap); mErr == nil {
			_ = s.SetConfig(KeyFXRatesCNY, string(b))
		}
	}
	return snap, nil
}

// DetectPriceCurrencyCode extracts the canonical ISO currency code from a free-form price string (e.g., "$21.66 / 年", "¥25.3/月", "HK$58/月").
func DetectPriceCurrencyCode(priceRaw string) string {
	s := strings.TrimSpace(priceRaw)
	upper := strings.ToUpper(s)
	switch {
	case strings.Contains(upper, "HK$") || strings.Contains(upper, "HKD") || strings.Contains(s, "港币") || strings.Contains(s, "港元"):
		return "HKD"
	case strings.Contains(upper, "NT$") || strings.Contains(upper, "TWD") || strings.Contains(s, "台币"):
		return "TWD"
	case strings.Contains(upper, "S$") || strings.Contains(upper, "SGD") || strings.Contains(s, "新币"):
		return "SGD"
	case strings.Contains(upper, "C$") || strings.Contains(upper, "CAD") || strings.Contains(s, "加币"):
		return "CAD"
	case strings.Contains(upper, "A$") || strings.Contains(upper, "AUD") || strings.Contains(s, "澳币"):
		return "AUD"
	case strings.Contains(upper, "JPY") || strings.Contains(s, "日元") || strings.Contains(s, "円"):
		return "JPY"
	case strings.Contains(s, "€") || strings.Contains(upper, "EUR") || strings.Contains(s, "欧元"):
		return "EUR"
	case strings.Contains(s, "£") || strings.Contains(upper, "GBP") || strings.Contains(s, "英镑"):
		return "GBP"
	case strings.Contains(s, "$") || strings.Contains(upper, "USD") || strings.Contains(s, "美元") || strings.Contains(s, "刀"):
		return "USD"
	case strings.Contains(s, "¥") || strings.Contains(s, "￥") || strings.Contains(upper, "CNY") || strings.Contains(upper, "RMB") || strings.Contains(s, "元"):
		return "CNY"
	default:
		return "CNY"
	}
}

// SummarizeAssetsCNY converts all active assets into unified monthly and annualized CNY totals using cached FX rates.
func SummarizeAssetsCNY(assets []Asset, rates map[string]float64) (monthlyCNY float64, yearlyCNY float64, pricedCount int) {
	if len(rates) == 0 {
		rates = DefaultFXRatesToCNY()
	}
	for _, a := range assets {
		status := NormalizeAssetStatus(a.Status)
		if status == "已停用" || status == "已归档" {
			continue
		}
		pStr := strings.TrimSpace(a.Price)
		if pStr == "" || strings.Contains(pStr, "免费") {
			continue
		}
		m := reFXPriceAmount.FindStringSubmatch(pStr)
		if len(m) < 2 {
			continue
		}
		amt, err := strconv.ParseFloat(m[1], 64)
		if err != nil || amt <= 0 {
			continue
		}
		code := DetectPriceCurrencyCode(pStr)
		rate := rates[code]
		if rate <= 0 {
			rate = DefaultFXRatesToCNY()[code]
			if rate <= 0 {
				rate = 1.0
			}
		}
		cnyAmt := amt * rate
		lowerP := strings.ToLower(pStr)
		var mo, yr float64
		switch {
		case strings.Contains(pStr, "年") || strings.Contains(lowerP, "/yr") || strings.Contains(lowerP, "year") || strings.Contains(lowerP, "/y"):
			mo = cnyAmt / 12.0
			yr = cnyAmt
		case strings.Contains(pStr, "季") || strings.Contains(lowerP, "quarter"):
			mo = cnyAmt / 3.0
			yr = cnyAmt * 4.0
		case strings.Contains(pStr, "次") || strings.Contains(lowerP, "one"):
			continue
		default:
			mo = cnyAmt
			yr = cnyAmt * 12.0
		}
		monthlyCNY += mo
		yearlyCNY += yr
		pricedCount++
	}
	monthlyCNY = math.Round(monthlyCNY*10) / 10
	yearlyCNY = math.Round(yearlyCNY)
	return monthlyCNY, yearlyCNY, pricedCount
}
