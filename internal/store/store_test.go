package store

import (
	"llm-api-uptime/internal/model"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return store
}

func createTestProvider(t *testing.T, store *Store, name string) *model.Provider {
	t.Helper()
	provider := &model.Provider{
		Name:    name,
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := store.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	return provider
}

func createTestProbe(t *testing.T, store *Store, providerID int64, modelName string) *model.Probe {
	t.Helper()
	probe := &model.Probe{
		ProviderID: providerID,
		Model:      modelName,
		Enabled:    true,
	}
	if err := store.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}
	return probe
}

func TestCreateAndGetProvider(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")

	if provider.ID == 0 {
		t.Error("expected provider ID to be set")
	}
	if provider.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	fetched, err := store.GetProvider(provider.ID)
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}

	if fetched.Name != "TestProvider" {
		t.Errorf("Name = %q, want TestProvider", fetched.Name)
	}
	if fetched.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want https://api.example.com", fetched.BaseURL)
	}
	if fetched.APIType != model.APITypeOpenAI {
		t.Errorf("APIType = %q, want openai", fetched.APIType)
	}
}

func TestGetProviderNotFound(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	_, err := store.GetProvider(999)
	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}

func TestListProviders(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	createTestProvider(t, store, "Provider B")
	createTestProvider(t, store, "Provider A")
	createTestProvider(t, store, "Provider C")

	providers, err := store.ListProviders()
	if err != nil {
		t.Fatalf("failed to list providers: %v", err)
	}

	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}

	if providers[0].Name != "Provider A" {
		t.Errorf("expected sorted by name, got %q first", providers[0].Name)
	}
}

func TestUpdateProvider(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "Original")
	originalUpdatedAt := provider.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	provider.Name = "Updated"
	provider.Enabled = false
	if err := store.UpdateProvider(provider); err != nil {
		t.Fatalf("failed to update provider: %v", err)
	}

	fetched, _ := store.GetProvider(provider.ID)
	if fetched.Name != "Updated" {
		t.Errorf("Name = %q, want Updated", fetched.Name)
	}
	if fetched.Enabled != false {
		t.Error("expected Enabled to be false")
	}
	if !fetched.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestDeleteProvider(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "ToDelete")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	if err := store.DeleteProvider(provider.ID); err != nil {
		t.Fatalf("failed to delete provider: %v", err)
	}

	_, err := store.GetProvider(provider.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}

	_, err = store.GetProbe(probe.ID)
	if err == nil {
		t.Error("expected probe to be cascade deleted")
	}
}

func TestGetEnabledProviders(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	createTestProvider(t, store, "Enabled1")
	p2 := createTestProvider(t, store, "Disabled")
	p2.Enabled = false
	store.UpdateProvider(p2)
	createTestProvider(t, store, "Enabled2")

	providers, err := store.GetEnabledProviders()
	if err != nil {
		t.Fatalf("failed to get enabled providers: %v", err)
	}

	if len(providers) != 2 {
		t.Fatalf("expected 2 enabled providers, got %d", len(providers))
	}
}

func TestCreateAndGetProbe(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	if probe.ID == 0 {
		t.Error("expected probe ID to be set")
	}

	fetched, err := store.GetProbe(probe.ID)
	if err != nil {
		t.Fatalf("failed to get probe: %v", err)
	}

	if fetched.Model != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", fetched.Model)
	}
	if fetched.ProviderID != provider.ID {
		t.Errorf("ProviderID = %d, want %d", fetched.ProviderID, provider.ID)
	}
}

func TestListProbes(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	createTestProbe(t, store, provider.ID, "gpt-4")
	createTestProbe(t, store, provider.ID, "gpt-3.5-turbo")
	createTestProbe(t, store, provider.ID, "claude-3")

	probes, err := store.ListProbes(provider.ID)
	if err != nil {
		t.Fatalf("failed to list probes: %v", err)
	}

	if len(probes) != 3 {
		t.Fatalf("expected 3 probes, got %d", len(probes))
	}

	if probes[0].Model != "claude-3" {
		t.Errorf("expected sorted by model, got %q first", probes[0].Model)
	}
}

func TestGetEnabledProbes(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	createTestProbe(t, store, provider.ID, "gpt-4")

	disabledProbe := createTestProbe(t, store, provider.ID, "gpt-3.5-turbo")
	disabledProbe.Enabled = false
	store.UpdateProbe(disabledProbe)

	probes, err := store.GetEnabledProbes()
	if err != nil {
		t.Fatalf("failed to get enabled probes: %v", err)
	}

	if len(probes) != 1 {
		t.Fatalf("expected 1 enabled probe, got %d", len(probes))
	}

	if probes[0].ProviderName != "TestProvider" {
		t.Errorf("ProviderName = %q, want TestProvider", probes[0].ProviderName)
	}
}

func TestDuplicateProbe(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	createTestProbe(t, store, provider.ID, "gpt-4")

	err := store.CreateProbe(&model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-4",
		Enabled:    true,
	})
	if err == nil {
		t.Error("expected error for duplicate probe")
	}
}

func TestSaveResult(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	result := &model.Result{
		ProbeID:      probe.ID,
		Status:       model.StatusSuccess,
		StatusCode:   200,
		LatencyMs:    150,
		RequestID:    "req-123",
		CreatedAt:    time.Now(),
	}

	if err := store.SaveResult(result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	if result.ID == 0 {
		t.Error("expected result ID to be set")
	}

	result2 := &model.Result{
		ProbeID:       probe.ID,
		Status:        model.StatusError,
		StatusCode:    400,
		ErrorCode:     "model_not_found",
		ErrorMessage:  "Model not found",
		RequestID:     "req-456",
		RawError:      `{"error":{"code":"model_not_found"}}`,
		CreatedAt:     time.Now(),
	}

	if err := store.SaveResult(result2); err != nil {
		t.Fatalf("failed to save error result: %v", err)
	}
}

func TestGetStats(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	for i := 0; i < 10; i++ {
		status := model.StatusSuccess
		if i >= 8 {
			status = model.StatusError
		}
		store.SaveResult(&model.Result{
			ProbeID:   probe.ID,
			Status:    status,
			StatusCode: 200,
			LatencyMs: 100 + i*10,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	stats, err := store.GetStats(model.StatsQuery{Hours: 1})
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("expected 1 provider stats, got %d", len(stats))
	}

	ps := stats[0]
	if ps.ProviderName != "TestProvider" {
		t.Errorf("ProviderName = %q, want TestProvider", ps.ProviderName)
	}
	if ps.TotalProbes != 10 {
		t.Errorf("TotalProbes = %d, want 10", ps.TotalProbes)
	}
	if ps.SuccessCount != 8 {
		t.Errorf("SuccessCount = %d, want 8", ps.SuccessCount)
	}

	if len(ps.Models) != 1 {
		t.Fatalf("expected 1 model stats, got %d", len(ps.Models))
	}

	ms := ps.Models[0]
	if ms.SuccessRate != 80.0 {
		t.Errorf("SuccessRate = %f, want 80.0", ms.SuccessRate)
	}
}

func TestGetDowntimePeriods(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	statuses := []model.ProbeStatus{
		model.StatusSuccess,
		model.StatusSuccess,
		model.StatusError,
		model.StatusError,
		model.StatusSuccess,
		model.StatusSuccess,
		model.StatusTimeout,
		model.StatusSuccess,
	}

	for i, status := range statuses {
		store.SaveResult(&model.Result{
			ProbeID:   probe.ID,
			Status:    status,
			CreatedAt: now.Add(-time.Duration(len(statuses)-i) * time.Minute),
		})
	}

	periods, err := store.GetDowntimePeriods(probe.ID, now.Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("failed to get downtime periods: %v", err)
	}

	if len(periods) != 2 {
		t.Fatalf("expected 2 downtime periods, got %d", len(periods))
	}
}

func TestCleanup(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	store.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		CreatedAt: time.Now().Add(-48 * time.Hour),
	})
	store.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		CreatedAt: time.Now(),
	})

	if err := store.Cleanup(1); err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	results, _ := store.GetResultsForProbe(probe.ID, 50, "")
	if len(results) != 1 {
		t.Errorf("expected 1 result after cleanup, got %d", len(results))
	}
}

func TestListAllProbes(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider1 := createTestProvider(t, store, "Provider1")
	provider2 := createTestProvider(t, store, "Provider2")

	createTestProbe(t, store, provider1.ID, "gpt-4")
	createTestProbe(t, store, provider1.ID, "gpt-3.5-turbo")
	createTestProbe(t, store, provider2.ID, "claude-3")

	probes, err := store.ListAllProbes()
	if err != nil {
		t.Fatalf("failed to list all probes: %v", err)
	}

	if len(probes) != 3 {
		t.Fatalf("expected 3 probes, got %d", len(probes))
	}
}

func TestDeleteProbe(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	if err := store.DeleteProbe(probe.ID); err != nil {
		t.Fatalf("failed to delete probe: %v", err)
	}

	_, err := store.GetProbe(probe.ID)
	if err == nil {
		t.Error("expected probe to be deleted")
	}
}

func TestDeleteProbesByProvider(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	createTestProbe(t, store, provider.ID, "gpt-4")
	createTestProbe(t, store, provider.ID, "gpt-3.5-turbo")
	createTestProbe(t, store, provider.ID, "claude-3")

	if err := store.DeleteProbesByProvider(provider.ID); err != nil {
		t.Fatalf("failed to delete probes by provider: %v", err)
	}

	probes, err := store.ListProbes(provider.ID)
	if err != nil {
		t.Fatalf("failed to list probes: %v", err)
	}

	if len(probes) != 0 {
		t.Errorf("expected 0 probes after deletion, got %d", len(probes))
	}
}

func TestUpdateProbe(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	probe.Model = "gpt-4-turbo"
	probe.Enabled = false

	if err := store.UpdateProbe(probe); err != nil {
		t.Fatalf("failed to update probe: %v", err)
	}

	fetched, err := store.GetProbe(probe.ID)
	if err != nil {
		t.Fatalf("failed to get probe: %v", err)
	}

	if fetched.Model != "gpt-4-turbo" {
		t.Errorf("Model = %q, want gpt-4-turbo", fetched.Model)
	}
	if fetched.Enabled != false {
		t.Error("expected Enabled to be false")
	}
}

func TestGetProbeNotFound(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	_, err := store.GetProbe(999)
	if err == nil {
		t.Error("expected error for non-existent probe")
	}
}

func TestClearResults(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	for i := 0; i < 5; i++ {
		store.SaveResult(&model.Result{
			ProbeID:   probe.ID,
			Status:    model.StatusSuccess,
			StatusCode: 200,
			LatencyMs: 100,
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}

	count, err := store.GetResultCount()
	if err != nil {
		t.Fatalf("failed to get count: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 results, got %d", count)
	}

	if err := store.ClearResults(); err != nil {
		t.Fatalf("failed to clear results: %v", err)
	}

	count, err = store.GetResultCount()
	if err != nil {
		t.Fatalf("failed to get count after clear: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 results after clear, got %d", count)
	}
}

func TestDeleteResult(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	result := &model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		StatusCode: 200,
		LatencyMs: 100,
		CreatedAt: time.Now(),
	}
	if err := store.SaveResult(result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	count, _ := store.GetResultCount()
	if count != 1 {
		t.Fatalf("expected 1 result, got %d", count)
	}

	if err := store.DeleteResult(result.ID); err != nil {
		t.Fatalf("failed to delete result: %v", err)
	}

	count, _ = store.GetResultCount()
	if count != 0 {
		t.Errorf("expected 0 results after delete, got %d", count)
	}
}

func TestGetLastProbeTime(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	// No results: expect nil
	lastTime, err := store.GetLastProbeTime()
	if err != nil {
		t.Fatalf("failed to get last probe time: %v", err)
	}
	if lastTime != nil {
		t.Errorf("expected nil when no results, got %v", lastTime)
	}

	// Insert using raw SQL with SQLite-native datetime format.
	// GetLastProbeTime now parses the time string manually from sql.NullString.
	_, err = store.db.Exec(
		"INSERT INTO results (probe_id, status, status_code, latency_ms, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
		probe.ID, string(model.StatusSuccess), 200, 100,
	)
	if err != nil {
		t.Fatalf("failed to insert result: %v", err)
	}

	// Verify the result exists via count
	count, err := store.GetResultCount()
	if err != nil {
		t.Fatalf("failed to get count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 result, got %d", count)
	}

	// GetLastProbeTime should now succeed — scans as NullString and parses manually
	lastTime, err = store.GetLastProbeTime()
	if err != nil {
		t.Fatalf("GetLastProbeTime failed: %v", err)
	}
	if lastTime == nil {
		t.Error("expected time to be returned when results exist")
	}
}

func TestParseTimeString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "Go monotonic clock format",
			input: "2026-06-22 23:59:21.5433074 +0800 CST m=+21.487735701",
		},
		{
			name:  "Go time with timezone no monotonic",
			input: "2026-06-22 23:59:21.5433074 +0800 CST",
		},
		{
			name:  "Go time with timezone offset only",
			input: "2026-06-22 23:59:21 +0800",
		},
		{
			name:  "RFC3339",
			input: "2026-06-22T23:59:21+08:00",
		},
		{
			name:  "RFC3339Nano",
			input: "2026-06-22T23:59:21.5433074+08:00",
		},
		{
			name:  "SQLite datetime format",
			input: "2026-06-22 23:59:21",
		},
		{
			name:    "unparseable",
			input:   "not a time",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeString(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.IsZero() {
				t.Error("expected non-zero time")
			}
		})
	}
}

func TestGetLastProbeTimeWithMonotonicFormat(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	// Insert a row with Go's time.Time.String() format (including monotonic clock)
	goFormat := time.Now().Format("2006-01-02 15:04:05.999999999 -0700 MST")
	_, err := store.db.Exec(
		"INSERT INTO results (probe_id, status, status_code, latency_ms, created_at) VALUES (?, ?, ?, ?, ?)",
		probe.ID, string(model.StatusSuccess), 200, 100, goFormat,
	)
	if err != nil {
		t.Fatalf("failed to insert result: %v", err)
	}

	lastTime, err := store.GetLastProbeTime()
	if err != nil {
		t.Fatalf("GetLastProbeTime failed with Go format: %v", err)
	}
	if lastTime == nil {
		t.Fatal("expected time to be returned")
	}
	if lastTime.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestGetResultsForProbePage(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	for i := 0; i < 25; i++ {
		status := model.StatusSuccess
		if i < 2 {
			status = model.StatusError
		}
		store.SaveResult(&model.Result{
			ProbeID:   probe.ID,
			Status:    status,
			StatusCode: 200,
			LatencyMs: 100,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	// Page 1: limit 10, offset 0
	page1, err := store.GetResultsForProbePage(probe.ID, 10, 0, "")
	if err != nil {
		t.Fatalf("failed to get page 1: %v", err)
	}
	if len(page1) != 10 {
		t.Errorf("page 1: expected 10 results, got %d", len(page1))
	}

	// Page 2: limit 10, offset 10
	page2, err := store.GetResultsForProbePage(probe.ID, 10, 10, "")
	if err != nil {
		t.Fatalf("failed to get page 2: %v", err)
	}
	if len(page2) != 10 {
		t.Errorf("page 2: expected 10 results, got %d", len(page2))
	}

	// Page 3: limit 10, offset 20
	page3, err := store.GetResultsForProbePage(probe.ID, 10, 20, "")
	if err != nil {
		t.Fatalf("failed to get page 3: %v", err)
	}
	if len(page3) != 5 {
		t.Errorf("page 3: expected 5 results, got %d", len(page3))
	}

	// Filter by status "success"
	successPage, err := store.GetResultsForProbePage(probe.ID, 10, 0, "success")
	if err != nil {
		t.Fatalf("failed to get success results: %v", err)
	}
	if len(successPage) != 10 {
		t.Errorf("success filter: expected 10 results, got %d", len(successPage))
	}

	// Filter by status "failed" (non-success)
	failedPage, err := store.GetResultsForProbePage(probe.ID, 10, 0, "failed")
	if err != nil {
		t.Fatalf("failed to get failed results: %v", err)
	}
	if len(failedPage) != 2 {
		t.Errorf("failed filter: expected 2 results, got %d", len(failedPage))
	}
}

func TestGetResultsCount(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	for i := 0; i < 10; i++ {
		status := model.StatusSuccess
		if i >= 8 {
			status = model.StatusError
		}
		store.SaveResult(&model.Result{
			ProbeID:   probe.ID,
			Status:    status,
			StatusCode: 200,
			LatencyMs: 100,
			CreatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	count, err := store.GetResultsCount(probe.ID, "")
	if err != nil {
		t.Fatalf("failed to get all count: %v", err)
	}
	if count != 10 {
		t.Errorf("all count: expected 10, got %d", count)
	}

	count, err = store.GetResultsCount(probe.ID, "success")
	if err != nil {
		t.Fatalf("failed to get success count: %v", err)
	}
	if count != 8 {
		t.Errorf("success count: expected 8, got %d", count)
	}

	count, err = store.GetResultsCount(probe.ID, "failed")
	if err != nil {
		t.Fatalf("failed to get failed count: %v", err)
	}
	if count != 2 {
		t.Errorf("failed count: expected 2, got %d", count)
	}
}

func TestGetHourlySummary(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	// Insert results via SaveResult so timestamps use the driver's format
	for hour := 0; hour < 24; hour++ {
		for j := 0; j < 2; j++ {
			status := model.StatusSuccess
			if j == 1 && hour%3 == 0 {
				status = model.StatusError
			}
			err := store.SaveResult(&model.Result{
				ProbeID:   probe.ID,
				Status:    status,
				StatusCode: 200,
				LatencyMs: 100,
				CreatedAt: now.Add(-time.Duration(hour) * time.Hour),
			})
			if err != nil {
				t.Fatalf("failed to insert result for hour %d: %v", hour, err)
			}
		}
	}

	summaries, err := store.GetHourlySummary(probe.ID, 24)
	if err != nil {
		t.Fatalf("failed to get hourly summary: %v", err)
	}

	if len(summaries) == 0 {
		t.Fatal("expected non-empty summary")
	}

	for _, s := range summaries {
		if s.Total == 0 {
			t.Errorf("hour %s: expected non-zero total", s.Hour)
		}
	}
}

func TestGetDailySummary(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "TestProvider")
	probe := createTestProbe(t, store, provider.ID, "gpt-4")

	now := time.Now()
	// Insert results across 7 days
	for day := 0; day < 7; day++ {
		for j := 0; j < 3; j++ {
			status := model.StatusSuccess
			if j == 0 && day%2 == 0 {
				status = model.StatusError
			}
			store.SaveResult(&model.Result{
				ProbeID:   probe.ID,
				Status:    status,
				StatusCode: 200,
				LatencyMs: 100,
				CreatedAt: now.AddDate(0, 0, -day),
			})
		}
	}

	summaries, err := store.GetDailySummary(probe.ID, 7)
	if err != nil {
		t.Fatalf("failed to get daily summary: %v", err)
	}

	if len(summaries) == 0 {
		t.Fatal("expected non-empty summary")
	}

	for _, ds := range summaries {
		if ds.Total == 0 {
			t.Errorf("day %s: expected non-zero total", ds.Date)
		}
		if ds.Success < 0 || ds.Success > 100 {
			t.Errorf("day %s: invalid success rate %.1f", ds.Date, ds.Success)
		}
	}
}
