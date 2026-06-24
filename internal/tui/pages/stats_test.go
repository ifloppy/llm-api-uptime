package pages

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbletea"

	"llm-api-uptime/internal/model"
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

func TestStatsViewEmpty(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	stats.SetSize(80, 24)

	view := stats.View()

	assertions := []struct {
		name string
		want string
	}{
		{"stats title", "Statistics"},
		{"no stats message", "No statistics available"},
		{"time range label", "24 Hours"},
		{"help text", "export CSV"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !statsContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestStatsUpdateTimeRange(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)
	stats.SetSize(80, 24)

	// Test 'w' key for 7 days
	wMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	model, _ := stats.Update(wMsg)
	s := model.(*Stats)
	if s.timeRange != 168 {
		t.Errorf("expected timeRange 168 after 'w', got %d", s.timeRange)
	}
	if s.message != "Showing last 7 days" {
		t.Errorf("expected message 'Showing last 7 days', got %q", s.message)
	}

	// Test 'm' key for 30 days
	mMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
	model, _ = s.Update(mMsg)
	s = model.(*Stats)
	if s.timeRange != 720 {
		t.Errorf("expected timeRange 720 after 'm', got %d", s.timeRange)
	}
	if s.message != "Showing last 30 days" {
		t.Errorf("expected message 'Showing last 30 days', got %q", s.message)
	}

	// Test 'h' key for 24 hours
	hMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	model, _ = s.Update(hMsg)
	s = model.(*Stats)
	if s.timeRange != 24 {
		t.Errorf("expected timeRange 24 after 'h', got %d", s.timeRange)
	}
	if s.message != "Showing last 24 hours" {
		t.Errorf("expected message 'Showing last 24 hours', got %q", s.message)
	}
}

func TestStatsUpdateClearConfirm(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	stats := NewStats(db)

	// Press 'c' to enter clear confirm
	cMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	model, _ := stats.Update(cMsg)
	s := model.(*Stats)

	if s.mode != "confirm" {
		t.Errorf("expected mode 'confirm' after 'c', got %q", s.mode)
	}
	if !s.IsFormMode() {
		t.Error("expected IsFormMode() true in confirm mode")
	}
}

func TestStatsViewWithResults(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create provider and probe
	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://test.com",
		APIKey:  "key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-4",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}

	// Save a result
	result := &model.Result{
		ProbeID:          probe.ID,
		Status:           model.StatusSuccess,
		StatusCode:       200,
		LatencyMs:        100,
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
		TPS:              50.0,
		CreatedAt:        time.Now(),
	}
	if err := db.SaveResult(result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	stats := NewStats(db)
	stats.SetSize(120, 40)

	view := stats.View()

	assertions := []struct {
		name string
		want string
	}{
		{"stats title", "Statistics"},
		{"provider name", "TestProvider"},
		{"model name", "gpt-4"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !statsContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

// statsContains is a helper to check if substr is in s.
func statsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
