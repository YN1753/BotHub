package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
)

type Asset struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"` // compute, domain, life
	Spec          string `json:"spec"`
	Price         string `json:"price"`
	Status        string `json:"status"`
	DueDate       string `json:"due_date"`
	AutoRenew     bool   `json:"auto_renew"`
	WarnDays      string `json:"warn_days"`
	LastCheckedAt string `json:"last_checked_at"`
	ExtraConfig   string `json:"extra_config,omitempty"`
	DaysLeft      int    `json:"days_left"`
	WarnFlag      bool   `json:"warn_flag"`
}

type PublicAsset struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Spec      string `json:"spec"`
	Price     string `json:"price"`
	Status    string `json:"status"`
	DueDate   string `json:"due_date"`
	AutoRenew bool   `json:"auto_renew"`
	DaysLeft  int    `json:"days_left"`
	WarnFlag  bool   `json:"warn_flag"`
}

func (s *Store) ListAssets() ([]Asset, error) {
	rows, err := s.db.Query(`
SELECT id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config
FROM assets ORDER BY CASE category WHEN 'compute' THEN 1 WHEN 'domain' THEN 2 WHEN 'life' THEN 3 ELSE 4 END, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Asset
	for rows.Next() {
		var a Asset
		var autoRenewInt int
		if err := rows.Scan(&a.ID, &a.Name, &a.Category, &a.Spec, &a.Price, &a.Status, &a.DueDate, &autoRenewInt, &a.WarnDays, &a.LastCheckedAt, &a.ExtraConfig); err == nil {
			a.AutoRenew = (autoRenewInt == 1)
			a.DaysLeft, _ = CalculateDaysLeft(a.DueDate)
			a.WarnFlag = (a.DaysLeft >= 0 && a.DaysLeft <= 7) || strings.Contains(a.Status, "到期")
			list = append(list, a)
		}
	}
	return list, rows.Err()
}

func (s *Store) GetAsset(id string) (*Asset, error) {
	var a Asset
	var autoRenewInt int
	err := s.db.QueryRow(`SELECT id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config FROM assets WHERE id = ?`, id).
		Scan(&a.ID, &a.Name, &a.Category, &a.Spec, &a.Price, &a.Status, &a.DueDate, &autoRenewInt, &a.WarnDays, &a.LastCheckedAt, &a.ExtraConfig)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.AutoRenew = (autoRenewInt == 1)
	a.DaysLeft, _ = CalculateDaysLeft(a.DueDate)
	a.WarnFlag = (a.DaysLeft >= 0 && a.DaysLeft <= 7) || strings.Contains(a.Status, "到期")
	return &a, nil
}

func (s *Store) SaveAsset(a Asset) error {
	autoInt := 0
	if a.AutoRenew {
		autoInt = 1
	}
	if a.WarnDays == "" {
		a.WarnDays = "[7, 3, 1]"
	}
	if a.ExtraConfig == "" {
		a.ExtraConfig = "{}"
	}
	if a.LastCheckedAt == "" {
		a.LastCheckedAt = NowShanghai()
	}
	_, err := s.db.Exec(`
INSERT INTO assets (id, name, category, spec, price, status, due_date, auto_renew, warn_days, last_checked_at, extra_config)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name, category = excluded.category, spec = excluded.spec, price = excluded.price,
	status = excluded.status, due_date = excluded.due_date, auto_renew = excluded.auto_renew,
	warn_days = excluded.warn_days, last_checked_at = excluded.last_checked_at, extra_config = excluded.extra_config`,
		a.ID, a.Name, a.Category, a.Spec, a.Price, a.Status, a.DueDate, autoInt, a.WarnDays, a.LastCheckedAt, a.ExtraConfig)
	return err
}

func (s *Store) DeleteAsset(id string) error {
	_, err := s.db.Exec(`DELETE FROM assets WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateAssetSyncResult(id, dueDate, status string, autoRenew bool, lastChecked string) error {
	autoInt := 0
	if autoRenew {
		autoInt = 1
	}
	_, err := s.db.Exec(`UPDATE assets SET due_date = ?, status = ?, auto_renew = ?, last_checked_at = ? WHERE id = ?`,
		dueDate, status, autoInt, lastChecked, id)
	return err
}

func (s *Store) GetPublicAssets() ([]PublicAsset, error) {
	list, err := s.ListAssets()
	if err != nil {
		return nil, err
	}
	out := make([]PublicAsset, 0, len(list))
	for _, a := range list {
		out = append(out, PublicAsset{
			ID: a.ID, Name: a.Name, Category: a.Category, Spec: a.Spec, Price: a.Price,
			Status: a.Status, DueDate: a.DueDate, AutoRenew: a.AutoRenew, DaysLeft: a.DaysLeft, WarnFlag: a.WarnFlag,
		})
	}
	return out, nil
}

func (s *Store) GetExpiringAssets(thresholdDays int) ([]Asset, error) {
	list, err := s.ListAssets()
	if err != nil {
		return nil, err
	}
	var expiring []Asset
	for _, a := range list {
		if a.DaysLeft >= 0 && a.DaysLeft <= thresholdDays {
			expiring = append(expiring, a)
		}
	}
	return expiring, nil
}

func CalculateDaysLeft(dueDateStr string) (int, error) {
	if strings.TrimSpace(dueDateStr) == "" {
		return 9999, nil
	}
	loc := ShanghaiLocation()
	formats := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	var t time.Time
	var err error
	for _, f := range formats {
		if t, err = time.ParseInLocation(f, dueDateStr, loc); err == nil {
			break
		}
	}
	if err != nil {
		return 0, err
	}
	return int(math.Ceil(t.Sub(time.Now().In(loc)).Hours() / 24.0)), nil
}

func ParseWarnDays(warnDaysStr string) []int {
	var days []int
	if err := json.Unmarshal([]byte(warnDaysStr), &days); err == nil && len(days) > 0 {
		return days
	}
	return []int{7, 3, 1}
}
