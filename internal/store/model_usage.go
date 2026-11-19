package store

import (
	"strings"
	"time"
)

type ModelUsageRecord struct {
	TargetID         string `json:"target_id"`
	ProviderID       string `json:"provider_id"`
	ModelID          string `json:"model_id"`
	Tier             string `json:"tier"` // free | paid
	Source           string `json:"source"`
	TaskID           string `json:"task_id"`
	BotID            string `json:"bot_id"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	Success          bool   `json:"success"`
	CreatedAt        string `json:"created_at"`
}

type ModelUsageModelStat struct {
	TargetID     string `json:"target_id"`
	ModelID      string `json:"model_id"`
	Tier         string `json:"tier"`
	Calls        int    `json:"calls"`
	SuccessCalls int    `json:"success_calls"`
	FreeCalls    int    `json:"free_calls"`
	PaidCalls    int    `json:"paid_calls"`
	Tokens       int    `json:"tokens"`
	AvgLatency   int64  `json:"avg_latency_ms"`
}

type ModelUsageCallerStat struct {
	BotID     string `json:"bot_id"`
	TaskID    string `json:"task_id"`
	Source    string `json:"source"`
	FreeCalls int    `json:"free_calls"`
	PaidCalls int    `json:"paid_calls"`
	Tokens    int    `json:"tokens"`
}

type ModelUsageSummary struct {
	Days         int                    `json:"days"`
	TotalCalls   int                    `json:"total_calls"`
	SuccessCalls int                    `json:"success_calls"`
	FreeCalls    int                    `json:"free_calls"`
	PaidCalls    int                    `json:"paid_calls"`
	FreeTokens   int                    `json:"free_tokens"`
	PaidTokens   int                    `json:"paid_tokens"`
	TopModels    []ModelUsageModelStat  `json:"top_models"`
	TopCallers   []ModelUsageCallerStat `json:"top_callers"`
}

func (s *Store) RecordModelUsage(rec ModelUsageRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	now := strings.TrimSpace(rec.CreatedAt)
	if now == "" {
		now = NowShanghai()
	}
	tier := NormalizeModelTier(rec.Tier)
	source := strings.TrimSpace(rec.Source)
	if source == "" {
		source = "chat"
	}
	okInt := 0
	if rec.Success {
		okInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO model_usage_logs (target_id, provider_id, model_id, tier, source, task_id, bot_id, prompt_tokens, completion_tokens, latency_ms, success, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(rec.TargetID),
		strings.TrimSpace(rec.ProviderID),
		strings.TrimSpace(rec.ModelID),
		tier,
		source,
		strings.TrimSpace(rec.TaskID),
		strings.TrimSpace(rec.BotID),
		rec.PromptTokens,
		rec.CompletionTokens,
		rec.LatencyMs,
		okInt,
		now,
	)
	return err
}

// CountTaskPaidCallsToday returns the number of paid model calls made today (Asia/Shanghai) by the given task.
func (s *Store) CountTaskPaidCallsToday(taskID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	tid := strings.TrimSpace(taskID)
	if tid == "" {
		return 0, nil
	}
	dayPrefix := time.Now().In(ShanghaiLocation()).Format("2006-01-02") + "%"
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM model_usage_logs WHERE task_id = ? AND tier = ? AND success = 1 AND created_at LIKE ?`,
		tid, ModelTierPaid, dayPrefix,
	).Scan(&count)
	return count, err
}

// SummarizeModelUsage aggregates Free vs Paid call & token usage over the past N days (Asia/Shanghai).
func (s *Store) SummarizeModelUsage(days int) (ModelUsageSummary, error) {
	if days <= 0 {
		days = 7
	}
	summary := ModelUsageSummary{
		Days:       days,
		TopModels:  []ModelUsageModelStat{},
		TopCallers: []ModelUsageCallerStat{},
	}
	if s == nil || s.db == nil {
		return summary, nil
	}
	since := time.Now().In(ShanghaiLocation()).AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	err := s.db.QueryRow(`SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN tier = 'free' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN tier = 'paid' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN tier = 'free' THEN prompt_tokens + completion_tokens ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN tier = 'paid' THEN prompt_tokens + completion_tokens ELSE 0 END), 0)
		FROM model_usage_logs WHERE created_at >= ?`, since).
		Scan(&summary.TotalCalls, &summary.SuccessCalls, &summary.FreeCalls, &summary.PaidCalls, &summary.FreeTokens, &summary.PaidTokens)
	if err != nil {
		return summary, err
	}

	rows, err := s.db.Query(`SELECT target_id, model_id, tier, COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(CASE WHEN tier = 'free' THEN 1 ELSE 0 END), 0) AS free_calls,
		COALESCE(SUM(CASE WHEN tier = 'paid' THEN 1 ELSE 0 END), 0) AS paid_calls,
		COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens,
		COALESCE(CAST(AVG(CASE WHEN latency_ms > 0 THEN latency_ms END) AS INTEGER), 0) AS avg_lat
		FROM model_usage_logs
		WHERE created_at >= ? AND model_id != ''
		GROUP BY target_id, model_id, tier
		ORDER BY calls DESC, tokens DESC
		LIMIT 12`, since)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var st ModelUsageModelStat
			if scanErr := rows.Scan(&st.TargetID, &st.ModelID, &st.Tier, &st.Calls, &st.SuccessCalls, &st.FreeCalls, &st.PaidCalls, &st.Tokens, &st.AvgLatency); scanErr == nil {
				summary.TopModels = append(summary.TopModels, st)
			}
		}
	}

	callerRows, cErr := s.db.Query(`SELECT bot_id, task_id, source,
		COALESCE(SUM(CASE WHEN tier = 'free' THEN 1 ELSE 0 END), 0) AS free_calls,
		COALESCE(SUM(CASE WHEN tier = 'paid' THEN 1 ELSE 0 END), 0) AS paid_calls,
		COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens
		FROM model_usage_logs
		WHERE created_at >= ?
		GROUP BY bot_id, task_id, source
		ORDER BY (free_calls + paid_calls) DESC, tokens DESC
		LIMIT 12`, since)
	if cErr == nil {
		defer callerRows.Close()
		for callerRows.Next() {
			var cs ModelUsageCallerStat
			if scanErr := callerRows.Scan(&cs.BotID, &cs.TaskID, &cs.Source, &cs.FreeCalls, &cs.PaidCalls, &cs.Tokens); scanErr == nil {
				summary.TopCallers = append(summary.TopCallers, cs)
			}
		}
	}
	return summary, nil
}
