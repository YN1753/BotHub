package modules

import (
	"testing"
	"time"

	"github.com/YN1753/vesper/bothub/internal/store"
)

func TestCalcNextAnchorDateClamping(t *testing.T) {
	loc := ShanghaiLocation()

	// Anchor on 31st day
	anchorDay := 31

	// In any month, anchor calculation should clamp properly to the month's maximum days
	next := calcNextAnchorDate(anchorDay, loc)
	if next.Day() > 31 || next.Day() < 28 {
		t.Fatalf("unexpected anchor day result: %v", next)
	}

	// Verify month bounds
	y, m, d := next.Date()
	maxDays := time.Date(y, m+1, 0, 0, 0, 0, 0, loc).Day()
	if d > maxDays {
		t.Fatalf("day %d overflows maximum days %d in %v %v", d, maxDays, y, m)
	}
}

func TestAssetShouldWarn(t *testing.T) {
	w := &AssetWatcherModule{}

	// Case 1: Custom warn_days [30, 15, 7, 3, 1]
	asset30 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 30,
	}
	if !w.shouldWarn(asset30) {
		t.Errorf("expected warn at 30 days left")
	}

	asset15 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 15,
	}
	if !w.shouldWarn(asset15) {
		t.Errorf("expected warn at 15 days left")
	}

	asset20 := store.Asset{
		ID:       "test_domain",
		WarnDays: "[30, 15, 7, 3, 1]",
		DaysLeft: 20,
	}
	if w.shouldWarn(asset20) {
		t.Errorf("did not expect warn at 20 days left")
	}

	// Case 2: Critical window (<= 3 days)
	asset2 := store.Asset{
		ID:       "test_critical",
		WarnDays: "[7, 3, 1]",
		DaysLeft: 2,
	}
	if !w.shouldWarn(asset2) {
		t.Errorf("expected warn at critical 2 days left")
	}
}
