package store

import (
	"database/sql"
	"fmt"
	"llm-api-uptime/internal/model"
	"time"
)

func (s *Store) SaveResult(r *model.Result) error {
	result, err := s.db.Exec(
		`INSERT INTO results (probe_id, status, status_code, latency_ms, 
		 prompt_tokens, completion_tokens, total_tokens, tps,
		 error_code, error_message, request_id, raw_error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ProbeID, r.Status, r.StatusCode, r.LatencyMs,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.TPS,
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
	var lastTime sql.NullTime
	err := s.db.QueryRow("SELECT MAX(created_at) FROM results").Scan(&lastTime)
	if err != nil {
		return nil, err
	}
	if lastTime.Valid {
		return &lastTime.Time, nil
	}
	return nil, nil
}

func (s *Store) GetResultsForProbe(probeID int64, limit int, statusFilter string) ([]model.Result, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, probe_id, status, status_code, latency_ms,
		       prompt_tokens, completion_tokens, total_tokens, tps,
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
		ORDER BY created_at DESC
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
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.Status, &r.StatusCode, &r.LatencyMs,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.TPS,
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
		SELECT id, probe_id, status, status_code, latency_ms,
		       prompt_tokens, completion_tokens, total_tokens, tps,
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

	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get results page: %w", err)
	}
	defer rows.Close()

	var results []model.Result
	for rows.Next() {
		var r model.Result
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.Status, &r.StatusCode, &r.LatencyMs,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.TPS,
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
	query := `
		SELECT 
			strftime('%Y-%m-%d %H:00:00', created_at) as hour,
			COUNT(*) as total,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failed
		FROM results
		WHERE probe_id = ? AND created_at >= datetime('now', ?)
		GROUP BY strftime('%Y-%m-%d %H:00:00', created_at)
		ORDER BY hour DESC
	`
	rows, err := s.db.Query(query, probeID, fmt.Sprintf("-%d hours", hours))
	if err != nil {
		return nil, fmt.Errorf("get hourly summary: %w", err)
	}
	defer rows.Close()

	var summaries []model.HourlySummary
	for rows.Next() {
		var hs model.HourlySummary
		if err := rows.Scan(&hs.Hour, &hs.Total, &hs.Failed); err != nil {
			return nil, fmt.Errorf("scan hourly summary: %w", err)
		}
		summaries = append(summaries, hs)
	}
	return summaries, nil
}

func (s *Store) GetStats(query model.StatsQuery) ([]model.ProviderStats, error) {
	var since time.Time
	if query.Days > 0 {
		since = time.Now().AddDate(0, 0, -query.Days)
	} else if query.Hours > 0 {
		since = time.Now().Add(-time.Duration(query.Hours) * time.Hour)
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	rows, err := s.db.Query(`
		SELECT
			p.id as probe_id,
			pr.name as provider_name,
			p.model,
			COUNT(*) as total_probes,
			SUM(CASE WHEN r.status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN r.status = 'error' THEN 1 ELSE 0 END) as error_count,
			SUM(CASE WHEN r.status = 'timeout' THEN 1 ELSE 0 END) as timeout_count,
			SUM(CASE WHEN r.status = 'empty_response' THEN 1 ELSE 0 END) as empty_resp_count,
			SUM(CASE WHEN r.status = 'empty_content' THEN 1 ELSE 0 END) as empty_content_count,
			AVG(CASE WHEN r.status = 'success' THEN r.latency_ms ELSE NULL END) as avg_latency,
			AVG(CASE WHEN r.status = 'success' AND r.tps > 0 THEN r.tps ELSE NULL END) as avg_tps,
			COALESCE((SELECT r2.status FROM results r2 WHERE r2.probe_id = p.id ORDER BY r2.created_at DESC LIMIT 1), '') as last_status,
			COALESCE((SELECT r2.tps FROM results r2 WHERE r2.probe_id = p.id ORDER BY r2.created_at DESC LIMIT 1), 0) as last_tps
		FROM results r
		JOIN probes p ON r.probe_id = p.id
		JOIN providers pr ON p.provider_id = pr.id
		WHERE r.created_at >= ?
		GROUP BY p.id, pr.name, p.model
		ORDER BY pr.name, p.model
	`, since)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	defer rows.Close()

	var stats []model.ProviderStats
	providerMap := make(map[string]*model.ProviderStats)

	for rows.Next() {
		var ms model.ModelStats
		var providerName string
		var avgLatency sql.NullFloat64
		var avgTPS sql.NullFloat64

		err := rows.Scan(&ms.ProbeID, &providerName, &ms.Model, &ms.TotalProbes, &ms.SuccessCount,
			&ms.ErrorCount, &ms.TimeoutCount, &ms.EmptyRespCount, &ms.EmptyContentCount,
			&avgLatency, &avgTPS, &ms.LastStatus, &ms.LastTPS)
		if err != nil {
			return nil, fmt.Errorf("scan stats: %w", err)
		}

		if avgLatency.Valid {
			ms.AvgLatencyMs = avgLatency.Float64
		}
		if avgTPS.Valid {
			ms.AvgTPS = avgTPS.Float64
		}
		ms.ProviderName = providerName
		ms.StartTime = since
		ms.EndTime = time.Now()

		if ms.TotalProbes > 0 {
			ms.SuccessRate = float64(ms.SuccessCount) / float64(ms.TotalProbes) * 100
		}

		if ps, ok := providerMap[providerName]; ok {
			ps.Models = append(ps.Models, ms)
			ps.TotalProbes += ms.TotalProbes
			ps.SuccessCount += ms.SuccessCount
		} else {
			providerMap[providerName] = &model.ProviderStats{
				ProviderName: providerName,
				Models:       []model.ModelStats{ms},
				TotalProbes:  ms.TotalProbes,
				SuccessCount: ms.SuccessCount,
			}
		}
	}

	for _, ps := range providerMap {
		if ps.TotalProbes > 0 {
			ps.SuccessRate = float64(ps.SuccessCount) / float64(ps.TotalProbes) * 100
		}
		stats = append(stats, *ps)
	}

	return stats, nil
}

func (s *Store) GetDowntimePeriods(probeID int64, since time.Time) ([]model.DowntimePeriod, error) {
	rows, err := s.db.Query(`
		SELECT created_at, status
		FROM results
		WHERE probe_id = ? AND created_at >= ?
		ORDER BY created_at
	`, probeID, since)
	if err != nil {
		return nil, fmt.Errorf("get downtime periods: %w", err)
	}
	defer rows.Close()

	var periods []model.DowntimePeriod
	var currentStart *time.Time
	lastStatus := "success"

	for rows.Next() {
		var createdAt time.Time
		var status string
		if err := rows.Scan(&createdAt, &status); err != nil {
			return nil, fmt.Errorf("scan downtime: %w", err)
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
