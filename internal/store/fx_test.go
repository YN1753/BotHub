package store

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestFXRatesCNYRefreshCacheAndAssetSummary(t *testing.T) {
	fxDisableAutoFetch = true
	defer func() { fxDisableAutoFetch = false }()

	tmpDir := t.TempDir()
	st, err := Open(filepath.Join(tmpDir, "fx_test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// 1. Default offline fallback works immediately before any network sync
	initial := st.GetFXRatesCNY()
	if initial.Base != "CNY" || initial.Rates["USD"] < 6.0 || initial.Rates["CNY"] != 1.0 {
		t.Fatalf("unexpected initial fallback fx snapshot: %+v", initial)
	}

	// 2. RefreshFXRatesCNY fetches inverse CNY rates, persists to SQLite, and survives offline fallback
	mockFX := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 1 CNY = 0.137931 USD -> 1 USD = 7.25 CNY; 1 CNY = 1.075269 HKD -> 1 HKD = 0.93 CNY
		_, _ = w.Write([]byte(`{"result":"success","rates":{"CNY":1,"USD":0.137931,"HKD":1.075269,"EUR":0.128205}}`))
	}))
	defer mockFX.Close()

	oldPrimary := fxPrimaryEndpoint
	fxPrimaryEndpoint = mockFX.URL
	defer func() { fxPrimaryEndpoint = oldPrimary }()

	refreshed, err := st.RefreshFXRatesCNY(context.Background())
	if err != nil {
		t.Fatalf("RefreshFXRatesCNY failed: %v", err)
	}
	if math.Abs(refreshed.Rates["USD"]-7.25) > 0.01 || math.Abs(refreshed.Rates["EUR"]-7.80) > 0.01 {
		t.Fatalf("unexpected refreshed rates: %+v", refreshed.Rates)
	}

	cached := st.GetFXRatesCNY()
	if cached.Source != "open.er-api.com" || math.Abs(cached.Rates["USD"]-7.25) > 0.01 {
		t.Fatalf("expected persisted fx snapshot in SQLite, got %+v", cached)
	}

	// 3. SummarizeAssetsCNY unifies USD + CNY + HKD into single RMB monthly and yearly totals
	assets := []Asset{
		{ID: "a1", Name: "CloudCone", Price: "$24 / 年", Status: "生效中"}, // $2/mo = ¥14.5/mo, ¥174/yr
		{ID: "a2", Name: "Bilibili", Price: "¥25 / 月", Status: "生效中"},  // ¥25/mo, ¥300/yr
		{ID: "a3", Name: "OldVPS", Price: "$10 / 月", Status: "已停用"},    // ignored
	}
	moCNY, yrCNY, cnt := SummarizeAssetsCNY(assets, cached.Rates)
	if cnt != 2 || math.Abs(moCNY-39.5) > 0.2 || math.Abs(yrCNY-474) > 2 {
		t.Fatalf("unexpected CNY asset summary: mo=%.1f yr=%.0f cnt=%d", moCNY, yrCNY, cnt)
	}
}
