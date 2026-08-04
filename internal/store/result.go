package store

import (
	"database/sql"
	"fmt"
	"llm-api-uptime/internal/model"
	"sort"
	"strings"
	"time"
)

// parseTimeString parses a time string that may include Go's monotonic clock reading.
// Go's time.Time.String() format: "2006-01-02 15:04:05.999999999 -0700 MST m=+0.000000001"
func parseTimeString(s string) (time.Time, error) {
	// Strip monotonic clock part if present
	if idx := strings.Index(s, " m=+"); idx > 0 {
		s = s[:idx]
	}

	formats := []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05-07:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse time: %s", s)
}

func (s *Store) SaveResult(r *model.Result) error {
	result, err := s.db.Exec(
		`INSERT INTO results (probe_id, status, status_code, latency_ms, ttft_ms,
		 prompt_tokens, completion_tokens, total_tokens, tps, tps_exclude_ttft,
		 error_code, error_message, request_id, raw_error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ProbeID, r.Status, r.StatusCode, r.LatencyMs, r.TTFTMs,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.TPS, r.TPSExcludeTTFT,
		r.ErrorCode, r.ErrorMessage, r.RequestID, r.RawError, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	id, _ := result.LastInsertId()
	r.ID = id
	return nil
}

func (s *Store) ClearResults() error {
	_, err := s.db.Exec("DELETE FROM results")
	return err
}

func (s *Store) DeleteResult(id int64) error {
	_, err := s.db.Exec("DELETE FROM results WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete result: %w", err)
	}
	return nil
}

func (s *Store) GetResultCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM results").Scan(&count)
	return count, err
}

func (s *Store) GetLastProbeTime() (*time.Time, error) {
	var lastTimeStr sql.NullString
	err := s.db.QueryRow("SELECT MAX(created_at) FROM results").Scan(&lastTimeStr)
	if err != nil {
		return nil, err
	}
	if !lastTimeStr.Valid || lastTimeStr.String == "" {
		return nil, nil
	}

	t, err := parseTimeString(lastTimeStr.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) GetResultsForProbe(probeID int64, limit int, statusFilter string) ([]model.Result, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, probe_id, status, status_code, latency_ms, ttft_ms,
		       prompt_tokens, completion_tokens, total_tokens, tps, tps_exclude_ttft,
		       error_code, error_message, request_id, raw_error, created_at
		FROM results
		WHERE probe_id = ?
	`
	args := []interface{}{probeID}

	if statusFilter == "failed" {
		query += ` AND status != 'success'`
	} else if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	query += `
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get results: %w", err)
	}
	defer rows.Close()

	var results []model.Result
	for rows.Next() {
		var r model.Result
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.Status, &r.StatusCode, &r.LatencyMs, &r.TTFTMs,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.TPS, &r.TPSExcludeTTFT,
			&r.ErrorCode, &r.ErrorMessage, &r.RequestID, &r.RawError, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Store) GetResultsForProbePage(probeID int64, limit, offset int, statusFilter string) ([]model.Result, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT id, probe_id, status, status_code, latency_ms, ttft_ms,
		       prompt_tokens, completion_tokens, total_tokens, tps, tps_exclude_ttft,
		       error_code, error_message, request_id, raw_error, created_at
		FROM results
		WHERE probe_id = ?
	`
	args := []interface{}{probeID}

	if statusFilter == "failed" {
		query += ` AND status != 'success'`
	} else if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get results page: %w", err)
	}
	defer rows.Close()

	var results []model.Result
	for rows.Next() {
		var r model.Result
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.Status, &r.StatusCode, &r.LatencyMs, &r.TTFTMs,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.TPS, &r.TPSExcludeTTFT,
			&r.ErrorCode, &r.ErrorMessage, &r.RequestID, &r.RawError, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *Store) GetResultsCount(probeID int64, statusFilter string) (int, error) {
	query := `SELECT COUNT(*) FROM results WHERE probe_id = ?`
	args := []interface{}{probeID}

	if statusFilter == "failed" {
		query += ` AND status != 'success'`
	} else if statusFilter != "" {
		query += ` AND status = ?`
		args = append(args, statusFilter)
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (s *Store) GetHourlySummary(probeID int64, hours int) ([]model.HourlySummary, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	rows, err := s.db.Query(`
		SELECT created_at, status
		FROM results
		WHERE probe_id = ?
		ORDER BY created_at
	`, probeID)
	if err != nil {
		return nil, fmt.Errorf("get hourly summary: %w", err)
	}
	defer rows.Close()

	type rawResult struct {
		CreatedAt time.Time
		Status    string
	}
	var results []rawResult
	for rows.Next() {
		var createdAtStr string
		var status string
		if err := rows.Scan(&createdAtStr, &status); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		createdAt, err := parseTimeString(createdAtStr)
		if err != nil {
			continue // skip unparseable rows
		}
		if createdAt.After(since) {
			results = append(results, rawResult{CreatedAt: createdAt, Status: status})
		}
	}

	hourMap := make(map[string]*model.HourlySummary)
	for _, r := range results {
		hourKey := r.CreatedAt.Format("2006-01-02 15:00:00")
		if _, ok := hourMap[hourKey]; !ok {
			hourMap[hourKey] = &model.HourlySummary{Hour: hourKey}
		}
		hourMap[hourKey].Total++
		if r.Status != "success" {
			hourMap[hourKey].Failed++
		}
	}

	summaries := make([]model.HourlySummary, 0, len(hourMap))
	for _, hs := range hourMap {
		summaries = append(summaries, *hs)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Hour < summaries[j].Hour
	})

	return summaries, nil
}

func (s *Store) GetDailySummary(probeID int64, days int) ([]model.DailySummary, error) {
	if days <= 0 {
		days = 7
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := today.AddDate(0, 0, -(days - 1))
	tomorrow := today.AddDate(0, 0, 1)
	// Format as string for SQLite comparison - Go monotonic clock format
	// uses "2006-01-02 15:04:05.999999999 -0700 MST m=+..." which SQLite
	// can't parse, but string comparison works on the prefix.
	sinceStr := since.Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT
			substr(created_at, 1, 10) as day,
			COUNT(*) as total,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failed,
			AVG(CASE WHEN status = 'success' THEN latency_ms ELSE NULL END) as avg_latency,
			AVG(CASE WHEN status = 'success' AND ttft_ms > 0 THEN ttft_ms ELSE NULL END) as avg_ttft,
			AVG(CASE WHEN status = 'success' AND tps > 0 THEN tps ELSE NULL END) as avg_tps,
			AVG(CASE WHEN status = 'success' AND tps_exclude_ttft > 0 THEN tps_exclude_ttft ELSE NULL END) as avg_tps_excl
		FROM results
		WHERE probe_id = ? AND created_at >= ? AND created_at < ?
		GROUP BY substr(created_at, 1, 10)
		ORDER BY day DESC
	`, probeID, sinceStr, tomorrow.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("get daily summary: %w", err)
	}
	defer rows.Close()

	summaries := make([]model.DailySummary, 0)
	for rows.Next() {
		var ds model.DailySummary
		var failed int
		var avgLatency, avgTTFT, avgTPS, avgTPSExcl sql.NullFloat64

		if err := rows.Scan(&ds.Date, &ds.Total, &failed, &avgLatency, &avgTTFT, &avgTPS, &avgTPSExcl); err != nil {
			return nil, fmt.Errorf("scan daily summary: %w", err)
		}
		ds.Failed = failed
		if ds.Total > 0 {
			ds.Success = float64(ds.Total-ds.Failed) / float64(ds.Total) * 100
		}
		if avgLatency.Valid {
			ds.AvgLatencyMs = avgLatency.Float64
		}
		if avgTTFT.Valid {
			ds.AvgTTFTMs = avgTTFT.Float64
		}
		if avgTPS.Valid {
			ds.AvgTPS = avgTPS.Float64
		}
		if avgTPSExcl.Valid {
			ds.AvgTPSExcludeTTFT = avgTPSExcl.Float64
		}
		summaries = append(summaries, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily summary: %w", err)
	}

	return summaries, nil
}

func (s *Store) GetDailyStats(days int) ([]model.ProviderDailyStats, error) {
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.GetDailyStatsRange(today.AddDate(0, 0, -(days - 1)), today)
}

func (s *Store) GetDailyStatsRange(from, to time.Time) ([]model.ProviderDailyStats, error) {
	// Format as string for SQLite comparison - Go monotonic clock format
	// uses "2006-01-02 15:04:05.999999999 -0700 MST m=+..." which SQLite
	// can't parse, but string comparison works on the prefix.
	sinceStr := from.Format("2006-01-02 15:04:05")
	endStr := to.AddDate(0, 0, 1).Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT
			pr.name,
			p.id,
			p.model,
			substr(r.created_at, 1, 10) AS day,
			COUNT(r.id) AS total,
			SUM(CASE WHEN r.status != 'success' THEN 1 ELSE 0 END) AS failed,
			AVG(CASE WHEN r.status = 'success' THEN r.latency_ms ELSE NULL END) AS avg_latency,
			AVG(CASE WHEN r.status = 'success' AND r.ttft_ms > 0 THEN r.ttft_ms ELSE NULL END) AS avg_ttft,
			AVG(CASE WHEN r.status = 'success' AND r.tps > 0 THEN r.tps ELSE NULL END) AS avg_tps,
			AVG(CASE WHEN r.status = 'success' AND r.tps_exclude_ttft > 0 THEN r.tps_exclude_ttft ELSE NULL END) AS avg_tps_excl
		FROM probes p
		JOIN providers pr ON pr.id = p.provider_id
		LEFT JOIN results r ON r.probe_id = p.id AND r.created_at >= ? AND r.created_at < ?
		WHERE p.enabled = 1 AND pr.enabled = 1
		GROUP BY pr.id, pr.name, p.id, p.model, day
		ORDER BY pr.name COLLATE NOCASE, pr.id, p.model COLLATE NOCASE, p.id, day DESC
	`, sinceStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("get daily stats: %w", err)
	}
	defer rows.Close()

	stats := make([]model.ProviderDailyStats, 0)
	var currentProvider string
	var currentProbeID int64
	for rows.Next() {
		var providerName, modelName string
		var probeID int64
		var day sql.NullString
		var ds model.DailySummary
		var failed sql.NullInt64
		var avgLatency, avgTTFT, avgTPS, avgTPSExcl sql.NullFloat64
		if err := rows.Scan(&providerName, &probeID, &modelName, &day, &ds.Total, &failed,
			&avgLatency, &avgTTFT, &avgTPS, &avgTPSExcl); err != nil {
			return nil, fmt.Errorf("scan daily stats: %w", err)
		}

		if len(stats) == 0 || providerName != currentProvider {
			stats = append(stats, model.ProviderDailyStats{
				ProviderName: providerName,
				Models:       make([]model.ModelDailyStats, 0),
			})
			currentProvider = providerName
			currentProbeID = 0
		}
		provider := &stats[len(stats)-1]
		if len(provider.Models) == 0 || probeID != currentProbeID {
			provider.Models = append(provider.Models, model.ModelDailyStats{
				ProbeID: probeID,
				Model:   modelName,
				Daily:   make([]model.DailySummary, 0),
			})
			currentProbeID = probeID
		}
		if !day.Valid || ds.Total == 0 {
			continue
		}

		ds.Date = day.String
		ds.Failed = int(failed.Int64)
		ds.Success = float64(ds.Total-ds.Failed) / float64(ds.Total) * 100
		if avgLatency.Valid {
			ds.AvgLatencyMs = avgLatency.Float64
		}
		if avgTTFT.Valid {
			ds.AvgTTFTMs = avgTTFT.Float64
		}
		if avgTPS.Valid {
			ds.AvgTPS = avgTPS.Float64
		}
		if avgTPSExcl.Valid {
			ds.AvgTPSExcludeTTFT = avgTPSExcl.Float64
		}
		provider.Models[len(provider.Models)-1].Daily = append(provider.Models[len(provider.Models)-1].Daily, ds)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily stats: %w", err)
	}
	return stats, nil
}

func (s *Store) GetHourlyStats(hours int) ([]model.ProviderHourlyStats, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	sinceStr := since.Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT
			pr.name,
			p.id,
			p.model,
			r.created_at,
			r.status
		FROM probes p
		JOIN providers pr ON pr.id = p.provider_id
		LEFT JOIN results r ON r.probe_id = p.id AND r.created_at >= ?
		WHERE p.enabled = 1 AND pr.enabled = 1
		ORDER BY pr.name COLLATE NOCASE, pr.id, p.model COLLATE NOCASE, p.id, r.created_at
	`, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("get hourly stats: %w", err)
	}
	defer rows.Close()

	stats := make([]model.ProviderHourlyStats, 0)
	var currentProvider string
	var currentProbeID int64
	type hourAgg struct {
		total  int
		failed int
	}
	hourMaps := make(map[int64]map[string]*hourAgg)

	for rows.Next() {
		var providerName, modelName string
		var probeID int64
		var createdAtStr, status sql.NullString
		if err := rows.Scan(&providerName, &probeID, &modelName, &createdAtStr, &status); err != nil {
			return nil, fmt.Errorf("scan hourly stats: %w", err)
		}

		if len(stats) == 0 || providerName != currentProvider {
			stats = append(stats, model.ProviderHourlyStats{
				ProviderName: providerName,
				Models:       make([]model.ModelHourlyStats, 0),
			})
			currentProvider = providerName
			currentProbeID = 0
		}
		provider := &stats[len(stats)-1]
		if len(provider.Models) == 0 || probeID != currentProbeID {
			provider.Models = append(provider.Models, model.ModelHourlyStats{
				ProbeID: probeID,
				Model:   modelName,
				Hourly:  make([]model.HourlySummary, 0),
			})
			currentProbeID = probeID
			if hourMaps[probeID] == nil {
				hourMaps[probeID] = make(map[string]*hourAgg)
			}
		}
		if !createdAtStr.Valid || !status.Valid {
			continue
		}
		createdAt, err := parseTimeString(createdAtStr.String)
		if err != nil || createdAt.Before(since) {
			continue
		}
		hourKey := createdAt.Format("2006-01-02 15:00:00")
		agg := hourMaps[probeID][hourKey]
		if agg == nil {
			agg = &hourAgg{}
			hourMaps[probeID][hourKey] = agg
		}
		agg.total++
		if status.String != "success" {
			agg.failed++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hourly stats: %w", err)
	}

	for i := range stats {
		for j := range stats[i].Models {
			probeID := stats[i].Models[j].ProbeID
			keys := make([]string, 0, len(hourMaps[probeID]))
			for key := range hourMaps[probeID] {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			hourly := make([]model.HourlySummary, 0, len(keys))
			for _, key := range keys {
				agg := hourMaps[probeID][key]
				hourly = append(hourly, model.HourlySummary{
					Hour:   key,
					Total:  agg.total,
					Failed: agg.failed,
				})
			}
			stats[i].Models[j].Hourly = hourly
		}
	}
	return stats, nil
}

func (s *Store) GetStats(query model.StatsQuery) ([]model.ProviderStats, error) {
	now := time.Now()
	var since time.Time
	var until time.Time
	if !query.From.IsZero() {
		since = query.From
		until = query.To
	} else if query.Days > 0 {
		since = now.AddDate(0, 0, -query.Days)
	} else if query.Hours > 0 {
		since = now.Add(-time.Duration(query.Hours) * time.Hour)
	} else {
		since = now.Add(-24 * time.Hour)
	}

	// Format as string for SQLite comparison - Go monotonic clock format
	// uses "2006-01-02 15:04:05.999999999 -0700 MST m=+..." which SQLite
	// can't parse, but string comparison works on the prefix.
	sinceStr := since.Format("2006-01-02 15:04:05")
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayStartStr := todayStart.Format("2006-01-02 15:04:05")
	tomorrowStartStr := todayStart.AddDate(0, 0, 1).Format("2006-01-02 15:04:05")

	// Calendar range queries bound the aggregation window on both ends;
	// rolling-hour/day queries only need a lower bound.
	rangeEndSQL := ""
	args := []interface{}{sinceStr}
	if !query.From.IsZero() {
		rangeEndSQL = " AND created_at < ?"
		args = append(args, until.AddDate(0, 0, 1).Format("2006-01-02 15:04:05"))
	}
	args = append(args, todayStartStr, tomorrowStartStr)

	rows, err := s.db.Query(`
		SELECT
			p.id as probe_id,
			pr.name as provider_name,
			p.model,
			COALESCE(a.total_probes, 0),
			COALESCE(a.success_count, 0),
			COALESCE(a.error_count, 0),
			COALESCE(a.timeout_count, 0),
			COALESCE(a.empty_resp_count, 0),
			COALESCE(a.empty_content_count, 0),
			a.avg_latency,
			a.avg_ttft,
			a.avg_tps,
			a.avg_tps_exclude_ttft,
			COALESCE(t.today_total, 0),
			COALESCE(t.today_success_count, 0),
			COALESCE(latest.status, ''),
			COALESCE(latest.tps, 0),
			latest.created_at,
			latest.status_code,
			latest.error_code,
			latest.error_message,
			latest.request_id
		FROM probes p
		JOIN providers pr ON pr.id = p.provider_id
		LEFT JOIN (
			SELECT
				probe_id,
				COUNT(*) AS total_probes,
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count,
				SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
				SUM(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END) AS timeout_count,
				SUM(CASE WHEN status = 'empty_response' THEN 1 ELSE 0 END) AS empty_resp_count,
				SUM(CASE WHEN status = 'empty_content' THEN 1 ELSE 0 END) AS empty_content_count,
				AVG(CASE WHEN status = 'success' THEN latency_ms ELSE NULL END) AS avg_latency,
				AVG(CASE WHEN status = 'success' AND ttft_ms > 0 THEN ttft_ms ELSE NULL END) AS avg_ttft,
				AVG(CASE WHEN status = 'success' AND tps > 0 THEN tps ELSE NULL END) AS avg_tps,
				AVG(CASE WHEN status = 'success' AND tps_exclude_ttft > 0 THEN tps_exclude_ttft ELSE NULL END) AS avg_tps_exclude_ttft
			FROM results
			WHERE created_at >= ?` + rangeEndSQL + `
			GROUP BY probe_id
		) a ON a.probe_id = p.id
		LEFT JOIN (
			SELECT
				probe_id,
				COUNT(*) AS today_total,
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS today_success_count
			FROM results
			WHERE created_at >= ? AND created_at < ?
			GROUP BY probe_id
		) t ON t.probe_id = p.id
		LEFT JOIN results latest ON latest.id = (
			SELECT r2.id FROM results r2
			WHERE r2.probe_id = p.id
			ORDER BY r2.created_at DESC, r2.id DESC
			LIMIT 1
		)
		WHERE p.enabled = 1 AND pr.enabled = 1
		ORDER BY pr.name COLLATE NOCASE, pr.id, p.model COLLATE NOCASE, p.id
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	defer rows.Close()

	stats := make([]model.ProviderStats, 0)

	for rows.Next() {
		var ms model.ModelStats
		var providerName string
		var avgLatency sql.NullFloat64
		var avgTTFT sql.NullFloat64
		var avgTPS sql.NullFloat64
		var avgTPSExcludeTTFT sql.NullFloat64
		var latestResultTime sql.NullString
		var latestStatusCode sql.NullInt64
		var latestErrorCode sql.NullString
		var latestErrorMessage sql.NullString
		var latestRequestID sql.NullString

		err := rows.Scan(&ms.ProbeID, &providerName, &ms.Model, &ms.TotalProbes, &ms.SuccessCount,
			&ms.ErrorCount, &ms.TimeoutCount, &ms.EmptyRespCount, &ms.EmptyContentCount,
			&avgLatency, &avgTTFT, &avgTPS, &avgTPSExcludeTTFT,
			&ms.TodayTotal, &ms.TodaySuccessCount, &ms.LastStatus, &ms.LastTPS,
			&latestResultTime, &latestStatusCode, &latestErrorCode, &latestErrorMessage, &latestRequestID)
		if err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}

		if avgLatency.Valid {
			ms.AvgLatencyMs = avgLatency.Float64
		}
		if avgTTFT.Valid {
			ms.AvgTTFTMs = avgTTFT.Float64
		}
		if avgTPS.Valid {
			ms.AvgTPS = avgTPS.Float64
		}
		if avgTPSExcludeTTFT.Valid {
			ms.AvgTPSExcludeTTFT = avgTPSExcludeTTFT.Float64
		}
		ms.ProviderName = providerName
		ms.StartTime = since
		ms.EndTime = now
		if ms.TodayTotal > 0 {
			uptime := float64(ms.TodaySuccessCount) / float64(ms.TodayTotal) * 100
			ms.TodayUptime = &uptime
		}
		if latestResultTime.Valid {
			latestTime, err := parseTimeString(latestResultTime.String)
			if err != nil {
				return nil, fmt.Errorf("parse latest result time: %w", err)
			}
			ms.LatestResultTime = &latestTime
		}
		if latestStatusCode.Valid {
			ms.LatestStatusCode = int(latestStatusCode.Int64)
		}
		if latestRequestID.Valid {
			ms.LatestRequestID = latestRequestID.String
		}
		if ms.LastStatus != string(model.StatusSuccess) {
			if latestErrorCode.Valid {
				ms.LatestErrorCode = latestErrorCode.String
			}
			if latestErrorMessage.Valid {
				ms.LatestErrorMessage = latestErrorMessage.String
			}
		}

		if ms.TotalProbes > 0 {
			ms.SuccessRate = float64(ms.SuccessCount) / float64(ms.TotalProbes) * 100
		}

		if len(stats) == 0 || stats[len(stats)-1].ProviderName != providerName {
			stats = append(stats, model.ProviderStats{
				ProviderName: providerName,
				Models:       make([]model.ModelStats, 0),
			})
		}
		ps := &stats[len(stats)-1]
		ps.Models = append(ps.Models, ms)
		ps.TotalProbes += ms.TotalProbes
		ps.SuccessCount += ms.SuccessCount
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stats: %w", err)
	}

	for i := range stats {
		ps := &stats[i]
		if ps.TotalProbes > 0 {
			ps.SuccessRate = float64(ps.SuccessCount) / float64(ps.TotalProbes) * 100
		}
	}

	return stats, nil
}

func (s *Store) GetDowntimePeriods(probeID int64, since time.Time) ([]model.DowntimePeriod, error) {
	sinceStr := since.Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT created_at, status
		FROM results
		WHERE probe_id = ? AND created_at >= ?
		ORDER BY created_at
	`, probeID, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("get downtime periods: %w", err)
	}
	defer rows.Close()

	var periods []model.DowntimePeriod
	var currentStart *time.Time
	lastStatus := "success"

	for rows.Next() {
		var createdAtStr string
		var status string
		if err := rows.Scan(&createdAtStr, &status); err != nil {
			return nil, fmt.Errorf("scan downtime: %w", err)
		}

		createdAt, err := parseTimeString(createdAtStr)
		if err != nil {
			continue // skip unparseable rows
		}

		if status != "success" && lastStatus == "success" {
			currentStart = &createdAt
		} else if status == "success" && lastStatus != "success" && currentStart != nil {
			periods = append(periods, model.DowntimePeriod{
				Start: *currentStart,
				End:   createdAt,
			})
			currentStart = nil
		}
		lastStatus = status
	}

	if currentStart != nil {
		periods = append(periods, model.DowntimePeriod{
			Start: *currentStart,
			End:   time.Now(),
		})
	}

	return periods, nil
}
