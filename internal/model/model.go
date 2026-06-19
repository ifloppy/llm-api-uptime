package model

import "time"

type APIType string

const (
	APITypeOpenAI    APIType = "openai"
	APITypeAnthropic APIType = "anthropic"
)

type Provider struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"base_url"`
	APIKey    string    `json:"api_key"`
	APIType   APIType   `json:"api_type"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Probe struct {
	ID         int64     `json:"id"`
	ProviderID int64     `json:"provider_id"`
	Model      string    `json:"model"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

type ProbeStatus string

const (
	StatusSuccess      ProbeStatus = "success"
	StatusError        ProbeStatus = "error"
	StatusTimeout      ProbeStatus = "timeout"
	StatusEmptyResp    ProbeStatus = "empty_response"
)

type Result struct {
	ID            int64       `json:"id"`
	ProbeID       int64       `json:"probe_id"`
	Status        ProbeStatus `json:"status"`
	StatusCode    int         `json:"status_code"`
	LatencyMs     int         `json:"latency_ms"`
	ErrorCode     string      `json:"error_code"`
	ErrorMessage  string      `json:"error_message"`
	RequestID     string      `json:"request_id"`
	RawError      string      `json:"raw_error"`
	CreatedAt     time.Time   `json:"created_at"`
}

type ProbeWithProvider struct {
	Probe
	ProviderName string  `json:"provider_name"`
	ProviderURL  string  `json:"provider_url"`
	APIType      APIType `json:"api_type"`
}

type StatsQuery struct {
	Hours  int
	Days   int
}

type ModelStats struct {
	ProviderName   string    `json:"provider_name"`
	Model          string    `json:"model"`
	TotalProbes    int       `json:"total_probes"`
	SuccessCount   int       `json:"success_count"`
	ErrorCount     int       `json:"error_count"`
	TimeoutCount   int       `json:"timeout_count"`
	EmptyRespCount int       `json:"empty_resp_count"`
	SuccessRate    float64   `json:"success_rate"`
	AvgLatencyMs   float64   `json:"avg_latency_ms"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
}

type ProviderStats struct {
	ProviderName string       `json:"provider_name"`
	Models       []ModelStats `json:"models"`
	TotalProbes  int          `json:"total_probes"`
	SuccessCount int          `json:"success_count"`
	SuccessRate  float64      `json:"success_rate"`
}

type DowntimePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type CSVRow struct {
	ProviderName   string
	Model          string
	TimeRange      string
	TotalProbes    int
	SuccessCount   int
	ErrorCount     int
	TimeoutCount   int
	EmptyRespCount int
	SuccessRate    float64
	AvgLatencyMs   float64
	DowntimePeriods string
}

type ModelListResponse struct {
	Models []string `json:"models"`
	Error  string   `json:"error,omitempty"`
}
