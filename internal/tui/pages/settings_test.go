package pages

import (
	"testing"

	"llm-api-uptime/internal/config"
)

func TestNewSettings(t *testing.T) {
	cfg := &config.Config{
		ProbeInterval:    5000000000,
		ProbeTimeout:     30000000000,
		ProbeConcurrency: 3,
		DBPath:           "./data/uptime.db",
		DataRetention:    720000000000,
		WebEnabled:       false,
		WebPort:          8080,
		WebPublic:        false,
		WebPassword:      "",
		LogLevel:         "info",
	}

	settings := NewSettings(cfg)
	if settings == nil {
		t.Fatal("expected non-nil settings page")
	}
}

func TestSettingsIsFormMode(t *testing.T) {
	cfg := &config.Config{}

	settings := NewSettings(cfg)
	if settings.IsFormMode() {
		t.Error("settings should not be in form mode")
	}
}

func TestSettingsInit(t *testing.T) {
	cfg := &config.Config{}

	settings := NewSettings(cfg)
	cmd := settings.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestSettingsSetSize(t *testing.T) {
	cfg := &config.Config{}

	settings := NewSettings(cfg)
	settings.SetSize(90, 45)

	if settings.width != 90 {
		t.Errorf("expected width 90, got %d", settings.width)
	}
	if settings.height != 45 {
		t.Errorf("expected height 45, got %d", settings.height)
	}
}

func TestSettingsView(t *testing.T) {
	cfg := &config.Config{
		DBPath:           ":memory:",
		ProbeInterval:    300000000000,
		ProbeTimeout:     30000000000,
		ProbeConcurrency: 3,
		DataRetention:    720000000000,
		WebEnabled:       false,
		WebPort:          8080,
		LogLevel:         "info",
	}

	settings := NewSettings(cfg)
	settings.SetSize(80, 24)

	view := settings.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestMaskPassword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty password", "", "(not set)"},
		{"non-empty password", "secret123", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskPassword(tt.in)
			if got != tt.want {
				t.Errorf("maskPassword(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
