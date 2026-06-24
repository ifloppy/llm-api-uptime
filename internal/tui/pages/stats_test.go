package pages

import (
	"testing"

	"llm-api-uptime/internal/store"
)

func TestNewStats(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	if stats == nil {
		t.Fatal("expected non-nil stats page")
	}
}

func TestStatsIsFormMode(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	if stats.IsFormMode() {
		t.Error("stats should not be in form mode by default")
	}
}

func TestStatsInit(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	cmd := stats.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestStatsSetSize(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	stats.SetSize(150, 60)

	if stats.width != 150 {
		t.Errorf("expected width 150, got %d", stats.width)
	}
	if stats.height != 60 {
		t.Errorf("expected height 60, got %d", stats.height)
	}
}

func TestStatsView(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	stats.SetSize(80, 24)

	view := stats.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestGetStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
		tps    float64
		want   string
	}{
		{"success with high tps", "success", 10.0, "🟢"},
		{"success with low tps", "success", 0.5, "🟡"},
		{"error status", "error", 0.0, "🔴"},
		{"timeout status", "timeout", 0.0, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStatusIcon(tt.status, tt.tps)
			if got != tt.want {
				t.Errorf("getStatusIcon(%q, %f) = %q, want %q", tt.status, tt.tps, got, tt.want)
			}
		})
	}
}
