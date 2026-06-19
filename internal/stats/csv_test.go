package stats

import (
	"bytes"
	"encoding/csv"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	return db
}

func createTestData(t *testing.T, db *store.Store) {
	t.Helper()

	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://localhost:1",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	probe1 := &model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-4",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe1); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}

	probe2 := &model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-3.5-turbo",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe2); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}

	now := time.Now()

	for i := 0; i < 10; i++ {
		db.SaveResult(&model.Result{
			ProbeID:   probe1.ID,
			Status:    model.StatusSuccess,
			StatusCode: 200,
			LatencyMs: 100 + i*10,
			CreatedAt: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	for i := 0; i < 5; i++ {
		status := model.StatusSuccess
		if i >= 3 {
			status = model.StatusError
		}
		db.SaveResult(&model.Result{
			ProbeID:      probe2.ID,
			Status:       status,
			StatusCode:   200,
			LatencyMs:    200,
			ErrorCode:    "test_error",
			ErrorMessage: "Test error message",
			RequestID:    "req-123",
			CreatedAt:    now.Add(-time.Duration(i) * time.Hour),
		})
	}
}

func TestExportCSV(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()
	createTestData(t, db)

	var buf bytes.Buffer
	query := model.StatsQuery{Hours: 24}

	err := ExportCSV(&buf, db, query)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("expected at least 2 rows (header + data), got %d", len(records))
	}

	header := records[0]
	expectedHeaders := []string{
		"Provider", "Model", "Time Range", "Total Probes",
		"Success", "Error", "Timeout", "Empty Response", "Empty Content",
		"Success Rate (%)", "Avg Latency (ms)", "Avg TPS",
		"Downtime Periods",
	}

	if len(header) != len(expectedHeaders) {
		t.Fatalf("expected %d columns, got %d", len(expectedHeaders), len(header))
	}

	for i, h := range header {
		if h != expectedHeaders[i] {
			t.Errorf("header[%d] = %q, want %q", i, h, expectedHeaders[i])
		}
	}
}

func TestExportCSVWithData(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()
	createTestData(t, db)

	var buf bytes.Buffer
	query := model.StatsQuery{Hours: 24}

	err := ExportCSV(&buf, db, query)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows (header + 2 models), got %d", len(records))
	}

	for _, row := range records[1:] {
		if row[0] != "TestProvider" {
			t.Errorf("expected provider name 'TestProvider', got %q", row[0])
		}
	}
}

func TestExportCSVEmptyData(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	var buf bytes.Buffer
	query := model.StatsQuery{Hours: 24}

	err := ExportCSV(&buf, db, query)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected only header row for empty data, got %d rows", len(records))
	}
}

func TestExportCSVWithDaysQuery(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()
	createTestData(t, db)

	var buf bytes.Buffer
	query := model.StatsQuery{Days: 7}

	err := ExportCSV(&buf, db, query)
	if err != nil {
		t.Fatalf("failed to export CSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("expected data rows for 7-day query, got %d", len(records))
	}
}
