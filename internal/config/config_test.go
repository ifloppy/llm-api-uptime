package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()

	cfg := Load()

	if cfg.ProbeInterval != 5*time.Minute {
		t.Errorf("ProbeInterval = %v, want 5m", cfg.ProbeInterval)
	}
	if cfg.ProbeTimeout != 30*time.Second {
		t.Errorf("ProbeTimeout = %v, want 30s", cfg.ProbeTimeout)
	}
	if cfg.ProbeRetries != 2 {
		t.Errorf("ProbeRetries = %d, want 2", cfg.ProbeRetries)
	}
	if cfg.ProbeConcurrency != 3 {
		t.Errorf("ProbeConcurrency = %d, want 3", cfg.ProbeConcurrency)
	}
	if cfg.DBPath != "./data/uptime.db" {
		t.Errorf("DBPath = %q, want ./data/uptime.db", cfg.DBPath)
	}
	if cfg.DataRetention != 720*time.Hour {
		t.Errorf("DataRetention = %v, want 720h", cfg.DataRetention)
	}
	if cfg.WebEnabled != false {
		t.Errorf("WebEnabled = %v, want false", cfg.WebEnabled)
	}
	if cfg.WebPort != 8080 {
		t.Errorf("WebPort = %d, want 8080", cfg.WebPort)
	}
	if cfg.WebPublic != false {
		t.Errorf("WebPublic = %v, want false", cfg.WebPublic)
	}
	if cfg.WebPassword != "" {
		t.Errorf("WebPassword = %q, want empty", cfg.WebPassword)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
	if !cfg.UpdateCheckEnabled {
		t.Error("UpdateCheckEnabled = false, want true")
	}
	if cfg.UpdateCheckInterval != 24*time.Hour {
		t.Errorf("UpdateCheckInterval = %v, want 24h", cfg.UpdateCheckInterval)
	}
	if !cfg.UpdateAutoStage {
		t.Error("UpdateAutoStage = false, want true")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Clearenv()

	os.Setenv("PROBE_INTERVAL", "10m")
	os.Setenv("PROBE_TIMEOUT", "60s")
	os.Setenv("PROBE_RETRIES", "4")
	os.Setenv("PROBE_CONCURRENCY", "5")
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("DATA_RETENTION", "168h")
	os.Setenv("WEB_ENABLED", "true")
	os.Setenv("WEB_PORT", "9090")
	os.Setenv("WEB_PUBLIC", "true")
	os.Setenv("WEB_PASSWORD", "secret123")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("UPDATE_CHECK_ENABLED", "false")
	os.Setenv("UPDATE_CHECK_INTERVAL", "2h")
	os.Setenv("UPDATE_AUTO_STAGE", "false")

	cfg := Load()

	if cfg.ProbeInterval != 10*time.Minute {
		t.Errorf("ProbeInterval = %v, want 10m", cfg.ProbeInterval)
	}
	if cfg.ProbeTimeout != 60*time.Second {
		t.Errorf("ProbeTimeout = %v, want 60s", cfg.ProbeTimeout)
	}
	if cfg.ProbeRetries != 4 {
		t.Errorf("ProbeRetries = %d, want 4", cfg.ProbeRetries)
	}
	if cfg.ProbeConcurrency != 5 {
		t.Errorf("ProbeConcurrency = %d, want 5", cfg.ProbeConcurrency)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want /tmp/test.db", cfg.DBPath)
	}
	if cfg.DataRetention != 168*time.Hour {
		t.Errorf("DataRetention = %v, want 168h", cfg.DataRetention)
	}
	if cfg.WebEnabled != true {
		t.Errorf("WebEnabled = %v, want true", cfg.WebEnabled)
	}
	if cfg.WebPort != 9090 {
		t.Errorf("WebPort = %d, want 9090", cfg.WebPort)
	}
	if cfg.WebPublic != true {
		t.Errorf("WebPublic = %v, want true", cfg.WebPublic)
	}
	if cfg.WebPassword != "secret123" {
		t.Errorf("WebPassword = %q, want secret123", cfg.WebPassword)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.UpdateCheckEnabled {
		t.Error("UpdateCheckEnabled = true, want false")
	}
	if cfg.UpdateCheckInterval != 2*time.Hour {
		t.Errorf("UpdateCheckInterval = %v, want 2h", cfg.UpdateCheckInterval)
	}
	if cfg.UpdateAutoStage {
		t.Error("UpdateAutoStage = true, want false")
	}
}

func TestLoadInvalidEnv(t *testing.T) {
	os.Clearenv()

	os.Setenv("PROBE_INTERVAL", "invalid")
	os.Setenv("PROBE_TIMEOUT", "invalid")
	os.Setenv("PROBE_RETRIES", "-1")
	os.Setenv("PROBE_CONCURRENCY", "abc")
	os.Setenv("WEB_PORT", "-1")
	os.Setenv("WEB_ENABLED", "yes")
	os.Setenv("UPDATE_CHECK_INTERVAL", "30s")
	os.Setenv("UPDATE_CHECK_ENABLED", "invalid")
	os.Setenv("UPDATE_AUTO_STAGE", "invalid")

	cfg := Load()

	if cfg.ProbeInterval != 5*time.Minute {
		t.Errorf("ProbeInterval should default on invalid value, got %v", cfg.ProbeInterval)
	}
	if cfg.ProbeTimeout != 30*time.Second {
		t.Errorf("ProbeTimeout should default on invalid value, got %v", cfg.ProbeTimeout)
	}
	if cfg.ProbeRetries != 2 {
		t.Errorf("ProbeRetries should default on invalid value, got %d", cfg.ProbeRetries)
	}
	if cfg.ProbeConcurrency != 3 {
		t.Errorf("ProbeConcurrency should default on invalid value, got %d", cfg.ProbeConcurrency)
	}
	if cfg.WebPort != 8080 {
		t.Errorf("WebPort should default on invalid value, got %d", cfg.WebPort)
	}
	if cfg.WebEnabled != false {
		t.Errorf("WebEnabled should default on invalid value, got %v", cfg.WebEnabled)
	}
	if cfg.UpdateCheckInterval != 24*time.Hour {
		t.Errorf("UpdateCheckInterval should default below minimum, got %v", cfg.UpdateCheckInterval)
	}
	if !cfg.UpdateCheckEnabled || !cfg.UpdateAutoStage {
		t.Error("invalid update booleans should preserve default true values")
	}
}

func TestWebAddr(t *testing.T) {
	tests := []struct {
		name     string
		public   bool
		port     int
		expected string
	}{
		{"private", false, 8080, "127.0.0.1:8080"},
		{"public", true, 8080, "0.0.0.0:8080"},
		{"custom port", false, 3000, "127.0.0.1:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WebPublic: tt.public,
				WebPort:   tt.port,
			}
			result := cfg.WebAddr()
			if result != tt.expected {
				t.Errorf("WebAddr() = %q, want %q", result, tt.expected)
			}
		})
	}
}
