package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ProbeInterval       time.Duration
	ProbeTimeout        time.Duration
	ProbeConcurrency    int
	DBPath              string
	DataRetention       time.Duration
	WebEnabled          bool
	WebPort             int
	WebPublic            bool
	WebPassword         string
	WebGuestEnabled     bool
	UpdateCheckEnabled  bool
	UpdateCheckInterval time.Duration
	UpdateAutoStage     bool
	UpdateHTTPProxy     string
	LogLevel            string
}

const minUpdateCheckInterval = time.Minute

func Load() *Config {
	cfg := &Config{
		ProbeInterval:       5 * time.Minute,
		ProbeTimeout:        30 * time.Second,
		ProbeConcurrency:    3,
		DBPath:              "./data/uptime.db",
		DataRetention:       720 * time.Hour,
		WebEnabled:          false,
		WebPort:             8080,
		WebPublic:           false,
		WebPassword:         "",
		WebGuestEnabled:     false,
		UpdateCheckEnabled:  true,
		UpdateCheckInterval: 24 * time.Hour,
		UpdateAutoStage:     true,
		UpdateHTTPProxy:     "",
		LogLevel:            "info",
	}

	if v := os.Getenv("PROBE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ProbeInterval = d
		}
	}

	if v := os.Getenv("PROBE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ProbeTimeout = d
		}
	}

	if v := os.Getenv("PROBE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ProbeConcurrency = n
		}
	}

	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}

	if v := os.Getenv("DATA_RETENTION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DataRetention = d
		}
	}

	if v := os.Getenv("WEB_ENABLED"); v != "" {
		cfg.WebEnabled = v == "true" || v == "1"
	}

	if v := os.Getenv("WEB_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.WebPort = n
		}
	}

	if v := os.Getenv("WEB_PUBLIC"); v != "" {
		cfg.WebPublic = v == "true" || v == "1"
	}

	if v := os.Getenv("WEB_PASSWORD"); v != "" {
		cfg.WebPassword = v
	}

	if v := os.Getenv("WEB_GUEST_ENABLED"); v != "" {
		cfg.WebGuestEnabled = v == "true" || v == "1"
	}

	if v := os.Getenv("UPDATE_CHECK_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.UpdateCheckEnabled = enabled
		}
	}

	if v := os.Getenv("UPDATE_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= minUpdateCheckInterval {
			cfg.UpdateCheckInterval = d
		}
	}

	if v := os.Getenv("UPDATE_AUTO_STAGE"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.UpdateAutoStage = enabled
		}
	}

	if v := os.Getenv("UPDATE_HTTP_PROXY"); v != "" {
		cfg.UpdateHTTPProxy = strings.TrimSpace(v)
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	return cfg
}

func (c *Config) WebAddr() string {
	host := "127.0.0.1"
	if c.WebPublic {
		host = "0.0.0.0"
	}
	return host + ":" + strconv.Itoa(c.WebPort)
}
