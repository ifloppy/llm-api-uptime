package stats

import (
	"encoding/csv"
	"fmt"
	"io"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
	"time"
)

func ExportCSV(w io.Writer, s *store.Store, query model.StatsQuery) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{
		"Provider", "Model", "Time Range", "Total Probes",
		"Success", "Error", "Timeout", "Empty Response", "Empty Content",
		"Success Rate (%)", "Avg Latency (ms)", "Avg TPS",
		"Downtime Periods",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	stats, err := s.GetStats(query)
	if err != nil {
		return fmt.Errorf("get stats: %w", err)
	}

	var since time.Time
	if query.Days > 0 {
		since = time.Now().AddDate(0, 0, -query.Days)
	} else if query.Hours > 0 {
		since = time.Now().Add(-time.Duration(query.Hours) * time.Hour)
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	for _, ps := range stats {
		for _, ms := range ps.Models {
			probes, err := s.ListProbes(0)
			if err != nil {
				continue
			}

			var probeID int64
			for _, p := range probes {
				if p.Model == ms.Model {
					probeID = p.ID
					break
				}
			}

			downtimeStr := ""
			if probeID > 0 {
				periods, err := s.GetDowntimePeriods(probeID, since)
				if err == nil {
					downtimeStr = FormatDowntimePeriods(periods)
				}
			}

			row := []string{
				ms.ProviderName,
				ms.Model,
				FormatTimeRange(ms.StartTime, ms.EndTime),
				fmt.Sprintf("%d", ms.TotalProbes),
				fmt.Sprintf("%d", ms.SuccessCount),
				fmt.Sprintf("%d", ms.ErrorCount),
				fmt.Sprintf("%d", ms.TimeoutCount),
				fmt.Sprintf("%d", ms.EmptyRespCount),
				fmt.Sprintf("%d", ms.EmptyContentCount),
				fmt.Sprintf("%.2f", ms.SuccessRate),
				fmt.Sprintf("%.0f", ms.AvgLatencyMs),
				fmt.Sprintf("%.2f", ms.AvgTPS),
				downtimeStr,
			}
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("write row: %w", err)
			}
		}
	}

	return nil
}
