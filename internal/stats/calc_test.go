package stats

import (
	"llm-api-uptime/internal/model"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"30 seconds", 30 * time.Second, "30s"},
		{"90 seconds", 90 * time.Second, "2m"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"30 minutes", 30 * time.Minute, "30m"},
		{"1 hour", 1 * time.Hour, "1.0h"},
		{"2.5 hours", 2*time.Hour + 30*time.Minute, "2.5h"},
		{"1 day", 24 * time.Hour, "1.0d"},
		{"3 days", 72 * time.Hour, "3.0d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestFormatTimeRange(t *testing.T) {
	start := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2024, 1, 15, 14, 45, 0, 0, time.UTC)

	result := FormatTimeRange(start, end)
	expected := "2024-01-15 10:30 ~ 2024-01-15 14:45"

	if result != expected {
		t.Errorf("FormatTimeRange() = %q, want %q", result, expected)
	}
}

func TestFormatDowntimePeriods(t *testing.T) {
	t.Run("empty periods", func(t *testing.T) {
		result := FormatDowntimePeriods([]model.DowntimePeriod{})
		if result != "" {
			t.Errorf("FormatDowntimePeriods([]) = %q, want empty string", result)
		}
	})

	t.Run("single period", func(t *testing.T) {
		periods := []model.DowntimePeriod{
			{
				Start: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
		}
		result := FormatDowntimePeriods(periods)
		expected := "2024-01-15 10:00 ~ 2024-01-15 11:00"
		if result != expected {
			t.Errorf("FormatDowntimePeriods() = %q, want %q", result, expected)
		}
	})

	t.Run("multiple periods", func(t *testing.T) {
		periods := []model.DowntimePeriod{
			{
				Start: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			},
			{
				Start: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
				End:   time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC),
			},
		}
		result := FormatDowntimePeriods(periods)
		expected := "2024-01-15 10:00 ~ 2024-01-15 11:00; 2024-01-15 14:00 ~ 2024-01-15 15:00"
		if result != expected {
			t.Errorf("FormatDowntimePeriods() = %q, want %q", result, expected)
		}
	})
}

func TestCalculateAvailability(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		success  int
		expected float64
	}{
		{"100% success", 100, 100, 100.0},
		{"50% success", 100, 50, 50.0},
		{"0% success", 100, 0, 0.0},
		{"99.9% success", 1000, 999, 99.9},
		{"zero total", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateAvailability(tt.total, tt.success)
			if result != tt.expected {
				t.Errorf("CalculateAvailability(%d, %d) = %f, want %f", tt.total, tt.success, result, tt.expected)
			}
		})
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		expected string
	}{
		{"excellent", 99.95, "🟢"},
		{"good", 99.5, "🟡"},
		{"fair", 97.0, "🟠"},
		{"poor", 90.0, "🔴"},
		{"critical", 50.0, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusEmoji(tt.rate)
			if result != tt.expected {
				t.Errorf("GetStatusEmoji(%f) = %q, want %q", tt.rate, result, tt.expected)
			}
		})
	}
}

func TestGetStatusText(t *testing.T) {
	tests := []struct {
		name     string
		rate     float64
		expected string
	}{
		{"excellent", 99.95, "Excellent"},
		{"good", 99.5, "Good"},
		{"fair", 97.0, "Fair"},
		{"poor", 90.0, "Poor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStatusText(tt.rate)
			if result != tt.expected {
				t.Errorf("GetStatusText(%f) = %q, want %q", tt.rate, result, tt.expected)
			}
		})
	}
}
