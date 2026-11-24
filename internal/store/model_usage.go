package store

import (
	"math"
	"strings"
	"time"
)

type ModelUsageRecord struct {
	TargetID         string  `json:"target_id"`
	ProviderID       string  `json:"provider_id"`
	ModelID          string  `json:"model_id"`
	Source           string  `json:"source"`
	TaskID           string  `json:"task_id"`
	BotID            string  `json:"bot_id"`
	PromptTokens     int     `json:"prompt_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	LatencyMs        int64   `json:"latency_ms"`
	Success          bool    `json:"success"`
	CreatedAt        string  `json:"created_at"`
}

type ModelUsageModelStat struct {
	TargetID     string  `json:"target_id"`
	ModelID      string  `json:"model_id"`
	Calls        int     `json:"calls"`
	SuccessCalls int     `json:"success_calls"`
	Tokens       int     `json:"tokens"`
	CachedTokens int     `json:"cached_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatency   int64   `json:"avg_latency_ms"`
}

type ModelUsageCallerStat struct {
	BotID   string  `json:"bot_id"`
	TaskID  string  `json:"task_id"`
	Source  string  `json:"source"`
	Calls   int     `json:"calls"`
	Tokens  int     `json:"tokens"`
	CostUSD float64 `json:"cost_usd"`
}

type ModelUsageSummary struct {
	Days                  int                    `json:"days"`
	TotalCalls            int                    `json:"total_calls"`
	SuccessCalls          int                    `json:"success_calls"`
	TotalPromptTokens     int                    `json:"total_prompt_tokens"`
	TotalCachedTokens     int                    `json:"total_cached_tokens"`
	TotalCompletionTokens int                    `json:"total_completion_tokens"`
	TotalTokens           int                    `json:"total_tokens"`
	CacheHitRate          float64                `json:"cache_hit_rate"` // 0..1
	TotalCostUSD          float64                `json:"total_cost_usd"`
	TotalCostCNY          float64                `json:"total_cost_cny"`
	TopModels             []ModelUsageModelStat  `json:"top_models"`
	TopCallers            []ModelUsageCallerStat `json:"top_callers"`
}

type modelPricePerM struct {
	InputUSD  float64
	OutputUSD float64
	CacheMult float64 // fraction of input price for cached prompt reads
}

// BuiltinModelPricePerM estimates USD per 1M input/output tokens for mainstream LLM families.
func BuiltinModelPricePerM(modelID string) (inputUSD, outputUSD, cacheMult float64) {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	if idx := strings.LastIndex(lower, "/"); idx >= 0 {
		lower = lower[idx+1:]
	}
	switch {
	case strings.Contains(lower, "claude") && strings.Contains(lower, "opus"):
		return 15.0, 75.0, 0.10
	case strings.Contains(lower, "claude") && strings.Contains(lower, "haiku"):
		return 0.80, 4.0, 0.10
	case strings.Contains(lower, "claude"):
		return 3.0, 15.0, 0.10
	case strings.Contains(lower, "gpt-5") && strings.Contains(lower, "mini"):
		return 0.25, 2.0, 0.10
	case strings.Contains(lower, "gpt-5") && strings.Contains(lower, "nano"):
		return 0.05, 0.40, 0.10
	case strings.Contains(lower, "gpt-5"):
		return 1.25, 10.0, 0.10
	case strings.Contains(lower, "gpt-4.1-mini"):
		return 0.40, 1.60, 0.25
	case strings.Contains(lower, "gpt-4.1-nano"):
		return 0.10, 0.40, 0.25
	case strings.Contains(lower, "gpt-4.1"):
		return 2.0, 8.0, 0.25
	case strings.Contains(lower, "gpt-4o-mini"):
		return 0.15, 0.60, 0.50
	case strings.Contains(lower, "gpt-4o"):
		return 2.50, 10.0, 0.50
	case strings.Contains(lower, "o3-mini") || strings.Contains(lower, "o4-mini"):
		return 1.10, 4.40, 0.25
	case strings.HasPrefix(lower, "o3") || strings.Contains(lower, "-o3"):
		return 2.0, 8.0, 0.25
	case strings.Contains(lower, "gemini") && strings.Contains(lower, "pro"):
		return 1.25, 10.0, 0.25
	case strings.Contains(lower, "gemini") && strings.Contains(lower, "lite"):
		return 0.10, 0.40, 0.25
	case strings.Contains(lower, "gemini") && strings.Contains(lower, "flash"):
		return 0.30, 2.50, 0.25
	case strings.Contains(lower, "deepseek") && (strings.Contains(lower, "reasoner") || strings.Contains(lower, "r1")):
		return 0.55, 2.19, 0.25
	case strings.Contains(lower, "deepseek"):
		return 0.27, 1.10, 0.25
	case strings.Contains(lower, "grok") && strings.Contains(lower, "mini"):
		return 0.30, 0.50, 0.25
	case strings.Contains(lower, "grok"):
		return 3.0, 15.0, 0.25
	case strings.Contains(lower, "qwen"):
		return 0.40, 1.20, 0.25
	case strings.Contains(lower, "kimi") || strings.Contains(lower, "moonshot"):
		return 0.60, 2.50, 0.25
	case strings.Contains(lower, "glm"):
		return 0.50, 2.0, 0.25
	default:
		return 1.0, 3.0, 0.25
	}
}

// EstimateModelCostUSD computes estimated USD spend accounting for cached prompt token discounts.
func EstimateModelCostUSD(modelID string, promptTokens, cachedTokens, completionTokens int, customInPerM, customOutPerM float64) float64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	inPrice, outPrice, cacheMult := BuiltinModelPricePerM(modelID)
	if customInPerM > 0 {
		inPrice = customInPerM
	}
	if customOutPerM > 0 {
		outPrice = customOutPerM
	}
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > promptTokens {
		cachedTokens = promptTokens
	}
	uncachedPrompt := promptTokens - cachedTokens
	usd := (float64(uncachedPrompt)*inPrice + float64(cachedTokens)*inPrice*cacheMult + float64(completionTokens)*outPrice) / 1_000_000.0
	return math.Round(usd*1_000_000) / 1_000_000
}

func (s *Store) RecordModelUsage(rec ModelUsageRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	now := strings.TrimSpace(rec.CreatedAt)
	if now == "" {
		now = NowShanghai()
	}
	source := strings.TrimSpace(rec.Source)
	if source == "" {
		source = "chat"
	}
	okInt := 0
	if rec.Success {
		okInt = 1
	}
	costUSD := rec.CostUSD
	if costUSD <= 0 && (rec.PromptTokens > 0 || rec.CompletionTokens > 0) {
		costUSD = EstimateModelCostUSD(rec.ModelID, rec.PromptTokens, rec.CachedTokens, rec.CompletionTokens, 0, 0)
	}
	_, err := s.db.Exec(
		`INSERT INTO model_usage_logs (target_id, provider_id, model_id, tier, source, task_id, bot_id, prompt_tokens, cached_tokens, completion_tokens, cost_usd, latency_ms, success, created_at)
		 VALUES (?, ?, ?, 'unified', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(rec.TargetID),
		strings.TrimSpace(rec.ProviderID),
		strings.TrimSpace(rec.ModelID),
		source,
		strings.TrimSpace(rec.TaskID),
		strings.TrimSpace(rec.BotID),
		rec.PromptTokens,
		rec.CachedTokens,
		rec.CompletionTokens,
		costUSD,
		rec.LatencyMs,
		okInt,
		now,
	)
	return err
}

func (s *Store) usdToCNYRate() float64 {
	if s != nil {
		if snap := s.GetFXRatesCNY(); snap.Rates != nil {
			if r := snap.Rates["USD"]; r > 0 {
				return r
			}
		}
	}
	return 7.24
}

// SummarizeModelUsage aggregates unified model call, token, cache hit, and estimated cost usage over the past N days (Asia/Shanghai).
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
		COALESCE(SUM(prompt_tokens), 0),
		COALESCE(SUM(cached_tokens), 0),
		COALESCE(SUM(completion_tokens), 0),
		COALESCE(SUM(cost_usd), 0)
		FROM model_usage_logs WHERE created_at >= ?`, since).
		Scan(&summary.TotalCalls, &summary.SuccessCalls, &summary.TotalPromptTokens, &summary.TotalCachedTokens, &summary.TotalCompletionTokens, &summary.TotalCostUSD)
	if err != nil {
		return summary, err
	}
	summary.TotalTokens = summary.TotalPromptTokens + summary.TotalCompletionTokens
	if summary.TotalPromptTokens > 0 {
		summary.CacheHitRate = math.Round((float64(summary.TotalCachedTokens)/float64(summary.TotalPromptTokens))*1000) / 1000
	}

	// Backfill historical rows that had cost_usd == 0 by estimating per-model totals
	rows, err := s.db.Query(`SELECT target_id, model_id, COUNT(*) AS calls,
		COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0) AS success_calls,
		COALESCE(SUM(prompt_tokens), 0) AS p_tok,
		COALESCE(SUM(cached_tokens), 0) AS c_tok,
		COALESCE(SUM(completion_tokens), 0) AS comp_tok,
		COALESCE(SUM(cost_usd), 0) AS cost_usd,
		COALESCE(CAST(AVG(CASE WHEN latency_ms > 0 THEN latency_ms END) AS INTEGER), 0) AS avg_lat
		FROM model_usage_logs
		WHERE created_at >= ? AND model_id != ''
		GROUP BY target_id, model_id
		ORDER BY calls DESC, (p_tok + comp_tok) DESC
		LIMIT 12`, since)
	if err == nil {
		defer rows.Close()
		recalcUSD := 0.0
		for rows.Next() {
			var st ModelUsageModelStat
			var pTok, compTok int
			if scanErr := rows.Scan(&st.TargetID, &st.ModelID, &st.Calls, &st.SuccessCalls, &pTok, &st.CachedTokens, &compTok, &st.CostUSD, &st.AvgLatency); scanErr == nil {
				st.Tokens = pTok + compTok
				if st.CostUSD <= 0 && st.Tokens > 0 {
					st.CostUSD = EstimateModelCostUSD(st.ModelID, pTok, st.CachedTokens, compTok, 0, 0)
				}
				st.CostUSD = math.Round(st.CostUSD*10000) / 10000
				recalcUSD += st.CostUSD
				summary.TopModels = append(summary.TopModels, st)
			}
		}
		if summary.TotalCostUSD <= 0 && recalcUSD > 0 {
			summary.TotalCostUSD = recalcUSD
		}
	}
	summary.TotalCostUSD = math.Round(summary.TotalCostUSD*10000) / 10000
	summary.TotalCostCNY = math.Round(summary.TotalCostUSD*s.usdToCNYRate()*100) / 100

	callerRows, cErr := s.db.Query(`SELECT bot_id, task_id, source,
		COUNT(*) AS calls,
		COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens,
		COALESCE(SUM(cost_usd), 0) AS cost_usd
		FROM model_usage_logs
		WHERE created_at >= ?
		GROUP BY bot_id, task_id, source
		ORDER BY calls DESC, tokens DESC
		LIMIT 12`, since)
	if cErr == nil {
		defer callerRows.Close()
		for callerRows.Next() {
			var cs ModelUsageCallerStat
			if scanErr := callerRows.Scan(&cs.BotID, &cs.TaskID, &cs.Source, &cs.Calls, &cs.Tokens, &cs.CostUSD); scanErr == nil {
				cs.CostUSD = math.Round(cs.CostUSD*10000) / 10000
				summary.TopCallers = append(summary.TopCallers, cs)
			}
		}
	}
	return summary, nil
}
