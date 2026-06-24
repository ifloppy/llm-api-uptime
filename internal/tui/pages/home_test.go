package pages

import (
	"log/slog"
	"testing"

	"github.com/charmbracelet/bubbletea"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

func TestNewHome(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	if home == nil {
		t.Fatal("expected non-nil home page")
	}
}

func TestHomeIsFormMode(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	if home.IsFormMode() {
		t.Error("home should not be in form mode")
	}
}

func TestHomeInit(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	cmd := home.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestHomeSetSize(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(100, 50)

	if home.width != 100 {
		t.Errorf("expected width 100, got %d", home.width)
	}
	if home.height != 50 {
		t.Errorf("expected height 50, got %d", home.height)
	}
}

func TestHomeView(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(80, 24)

	view := home.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestHomeViewContent(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(80, 24)

	view := home.View()

	// Verify key content is present
	assertions := []struct {
		name string
		want string
	}{
		{"dashboard title", "Dashboard"},
		{"engine status label", "Engine Status"},
		{"stopped status", "Stopped"},
		{"probe interval", "Probe Interval"},
		{"db path", "DB Path"},
		{"help text", "trigger probe"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !contains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestHomeUpdateTriggerKey(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	logger := slog.Default()
	engine := probe.NewEngine(db, cfg, logger)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(80, 24)

	// Send 't' key to trigger probe
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	model, cmd := home.Update(msg)

	if cmd != nil {
		t.Error("expected nil command from trigger key")
	}

	h := model.(*Home)
	if h.message != "Probe triggered!" {
		t.Errorf("expected message 'Probe triggered!', got %q", h.message)
	}
	if h.messageTyp != "success" {
		t.Errorf("expected messageTyp 'success', got %q", h.messageTyp)
	}
}

func TestHomeUpdateWebServerNil(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(80, 24)

	// Send 'w' key with nil webServer
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	model, cmd := home.Update(msg)

	if cmd != nil {
		t.Error("expected nil command")
	}

	h := model.(*Home)
	if h.message != "Web server not configured (WEB_ENABLED=true in .env)" {
		t.Errorf("expected web server not configured message, got %q", h.message)
	}
	if h.messageTyp != "error" {
		t.Errorf("expected messageTyp 'error', got %q", h.messageTyp)
	}
}

func TestHomeUpdateOtherKey(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)

	// Send an unhandled key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	model, cmd := home.Update(msg)

	if cmd != nil {
		t.Error("expected nil command for unhandled key")
	}
	if model != home {
		t.Error("expected same model returned")
	}
}

// contains is a helper to check if substr is in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
