package stats

import (
	"fmt"
	"llm-api-uptime/internal/model"
	"strings"
	"time"
)

func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}

func FormatTimeRange(start, end time.Time) string {
	layout := "2006-01-02 15:04"
	return fmt.Sprintf("%s ~ %s", start.Format(layout), end.Format(layout))
}

func FormatDowntimePeriods(periods []model.DowntimePeriod) string {
	if len(periods) == 0 {
		return ""
	}

	var parts []string
	for _, p := range periods {
		parts = append(parts, FormatTimeRange(p.Start, p.End))
	}
	return strings.Join(parts, "; ")
}

func CalculateAvailability(total, success int) float64 {
	if total == 0 {
		return 0
	}
	return float64(success) / float64(total) * 100
}

func GetStatusEmoji(rate float64) string {
	if rate >= 99.9 {
		return "🟢"
	}
	if rate >= 99 {
		return "🟡"
	}
	if rate >= 95 {
		return "🟠"
	}
	return "🔴"
}

func GetStatusText(rate float64) string {
	if rate >= 99.9 {
		return "Excellent"
	}
	if rate >= 99 {
		return "Good"
	}
	if rate >= 95 {
		return "Fair"
	}
	return "Poor"
}
