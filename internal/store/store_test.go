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
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  150,
		RequestID:  "req-123",
		CreatedAt:  time.Now(),
	}

	if err := store.SaveResult(result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	if result.ID == 0 {
		t.Error("expected result ID to be set")
	}

	result2 := &model.Result{
		ProbeID:      probe.ID,
		Status:       model.StatusError,
		StatusCode:   400,
		ErrorCode:    "model_not_found",
		ErrorMessage: "Model not found",
		RequestID:    "req-456",
		RawError:     `{"error":{"code":"model_not_found"}}`,
		CreatedAt:    time.Now(),
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
			ProbeID:    probe.ID,
			Status:     status,
			StatusCode: 200,
			LatencyMs:  100 + i*10,
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
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

func TestGetStatsIncludesEnabledProbesAndLatestDetails(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	zulu := createTestProvider(t, store, "Zulu")
	alpha := createTestProvider(t, store, "Alpha")
	emptyProbe := createTestProbe(t, store, alpha.ID, "a-empty")
	errorProbe := createTestProbe(t, store, alpha.ID, "z-error")
	successProbe := createTestProbe(t, store, zulu.ID, "m-success")

	disabledProbe := createTestProbe(t, store, alpha.ID, "disabled-probe")
	disabledProbe.Enabled = false
	if err := store.UpdateProbe(disabledProbe); err != nil {
		t.Fatalf("disable probe: %v", err)
	}
	disabledProvider := createTestProvider(t, store, "DisabledProvider")
	disabledProvider.Enabled = false
	if err := store.UpdateProvider(disabledProvider); err != nil {
		t.Fatalf("disable provider: %v", err)
	}
	createTestProbe(t, store, disabledProvider.ID, "disabled-provider-probe")

	now := time.Now()
	latestErrorTime := now
	if err := store.SaveResult(&model.Result{
		ProbeID:      errorProbe.ID,
		Status:       model.StatusError,
		StatusCode:   503,
		ErrorCode:    "overloaded",
		ErrorMessage: "try later",
		RequestID:    "req-latest",
		CreatedAt:    latestErrorTime,
	}); err != nil {
		t.Fatalf("save error result: %v", err)
	}
	// A later insertion with an older timestamp must not replace current status.
	if err := store.SaveResult(&model.Result{
		ProbeID:    errorProbe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		RequestID:  "req-old",
		CreatedAt:  now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("save stale success result: %v", err)
	}
	if err := store.SaveResult(&model.Result{
		ProbeID:      successProbe.ID,
		Status:       model.StatusSuccess,
		StatusCode:   204,
		ErrorCode:    "stale-code",
		ErrorMessage: "stale-message",
		RequestID:    "req-success",
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("save latest success: %v", err)
	}

	stats, err := store.GetStats(model.StatsQuery{Days: 2})
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 enabled providers, got %d", len(stats))
	}
	if stats[0].ProviderName != "Alpha" || stats[1].ProviderName != "Zulu" {
		t.Fatalf("providers not deterministic: %q, %q", stats[0].ProviderName, stats[1].ProviderName)
	}
	if len(stats[0].Models) != 2 {
		t.Fatalf("expected 2 enabled Alpha models, got %d", len(stats[0].Models))
	}
	if stats[0].Models[0].ProbeID != emptyProbe.ID || stats[0].Models[0].Model != "a-empty" {
		t.Fatalf("models not deterministic: %+v", stats[0].Models)
	}
	empty := stats[0].Models[0]
	if empty.TotalProbes != 0 || empty.TodayTotal != 0 || empty.TodayUptime != nil || empty.LatestResultTime != nil {
		t.Errorf("empty probe should have zero counts and null uptime/latest time: %+v", empty)
	}

	latestError := stats[0].Models[1]
	if latestError.LastStatus != string(model.StatusError) || latestError.LatestStatusCode != 503 {
		t.Errorf("latest result was not selected by timestamp: %+v", latestError)
	}
	if latestError.LatestErrorCode != "overloaded" || latestError.LatestErrorMessage != "try later" || latestError.LatestRequestID != "req-latest" {
		t.Errorf("missing latest error details: %+v", latestError)
	}
	if latestError.LatestResultTime == nil || !latestError.LatestResultTime.Equal(latestErrorTime) {
		t.Errorf("latest result time = %v, want %v", latestError.LatestResultTime, latestErrorTime)
	}
	if latestError.TodayTotal != 2 || latestError.TodaySuccessCount != 1 || latestError.TodayUptime == nil || *latestError.TodayUptime != 50 {
		t.Errorf("today stats = total %d, success %d, uptime %v", latestError.TodayTotal, latestError.TodaySuccessCount, latestError.TodayUptime)
	}

	latestSuccess := stats[1].Models[0]
	if latestSuccess.LatestErrorCode != "" || latestSuccess.LatestErrorMessage != "" {
		t.Errorf("latest success leaked stale error fields: %+v", latestSuccess)
	}
	if latestSuccess.LatestRequestID != "req-success" {
		t.Errorf("latest request ID = %q, want req-success", latestSuccess.LatestRequestID)
	}
}

func TestGetStatsTodayUsesLocalCalendarDay(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "Provider")
	probe := createTestProbe(t, store, provider.ID, "model")
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, result := range []*model.Result{
		{ProbeID: probe.ID, Status: model.StatusError, CreatedAt: today.Add(-time.Second)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, CreatedAt: today.Add(time.Second)},
	} {
		if err := store.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	stats, err := store.GetStats(model.StatsQuery{Days: 2})
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	got := stats[0].Models[0]
	if got.TodayTotal != 1 || got.TodaySuccessCount != 1 || got.TodayUptime == nil || *got.TodayUptime != 100 {
		t.Errorf("local-day stats = total %d success %d uptime %v", got.TodayTotal, got.TodaySuccessCount, got.TodayUptime)
	}
}

func TestGetStatsCalendarRange(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "Provider")
	probe := createTestProbe(t, store, provider.ID, "model")
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, result := range []*model.Result{
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 100, CreatedAt: today.AddDate(0, 0, -5).Add(12 * time.Hour)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 200, CreatedAt: today.AddDate(0, 0, -3).Add(12 * time.Hour)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 300, CreatedAt: today.AddDate(0, 0, -1).Add(12 * time.Hour)},
	} {
		if err := store.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	stats, err := store.GetStats(model.StatsQuery{
		From: today.AddDate(0, 0, -6),
		To:   today.AddDate(0, 0, -4),
	})
	if err != nil {
		t.Fatalf("GetStats range: %v", err)
	}
	if len(stats) != 1 || len(stats[0].Models) != 1 {
		t.Fatalf("unexpected stats shape: %+v", stats)
	}
	got := stats[0].Models[0]
	if got.TotalProbes != 1 || got.SuccessCount != 1 || got.AvgLatencyMs != 100 {
		t.Errorf("range aggregate = total %d success %d avg %v, want only the -5 day result", got.TotalProbes, got.SuccessCount, got.AvgLatencyMs)
	}
}

func TestGetDailyStatsRange(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	provider := createTestProvider(t, store, "Provider")
	probe := createTestProbe(t, store, provider.ID, "model")
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, result := range []*model.Result{
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 100, CreatedAt: today.AddDate(0, 0, -5).Add(12 * time.Hour)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 200, CreatedAt: today.AddDate(0, 0, -3).Add(12 * time.Hour)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 300, CreatedAt: today.AddDate(0, 0, -1).Add(12 * time.Hour)},
	} {
		if err := store.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	stats, err := store.GetDailyStatsRange(today.AddDate(0, 0, -6), today.AddDate(0, 0, -4))
	if err != nil {
		t.Fatalf("GetDailyStatsRange: %v", err)
	}
	if len(stats) != 1 || len(stats[0].Models) != 1 {
		t.Fatalf("unexpected daily stats shape: %+v", stats)
	}
	daily := stats[0].Models[0].Daily
	if len(daily) != 1 || daily[0].Date != today.AddDate(0, 0, -5).Format("2006-01-02") {
		t.Fatalf("range daily = %+v, want only the -5 day", daily)
	}
	if daily[0].Total != 1 || daily[0].Success != 100 {
		t.Errorf("daily aggregate = %+v", daily[0])
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
			ProbeID:    probe.ID,
			Status:     model.StatusSuccess,
			StatusCode: 200,
			LatencyMs:  100,
			CreatedAt:  time.Now().Add(-time.Duration(i) * time.Minute),
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
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
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
			ProbeID:    probe.ID,
			Status:     status,
			StatusCode: 200,
			LatencyMs:  100,
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
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
			ProbeID:    probe.ID,
			Status:     status,
			StatusCode: 200,
			LatencyMs:  100,
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
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
				ProbeID:    probe.ID,
				Status:     status,
				StatusCode: 200,
				LatencyMs:  100,
				CreatedAt:  now.Add(-time.Duration(hour) * time.Hour),
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
	for i := 1; i < len(summaries); i++ {
		if summaries[i-1].Hour > summaries[i].Hour {
			t.Fatalf("hourly output is not sorted: %q before %q", summaries[i-1].Hour, summaries[i].Hour)
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
				ProbeID:          probe.ID,
				Status:           status,
				StatusCode:       200,
				LatencyMs:        100,
				TTFTMs:           50,
				TPS:              15.5,
				TPSExcludeTTFT:   20.0,
				CompletionTokens: 30,
				CreatedAt:        now.AddDate(0, 0, -day),
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
		// On days with at least one success, avg latency/tps/ttft should be populated
		successCount := ds.Total - ds.Failed
		if successCount > 0 {
			if ds.AvgLatencyMs <= 0 {
				t.Errorf("day %s: expected positive avg_latency_ms, got %.1f", ds.Date, ds.AvgLatencyMs)
			}
			if ds.AvgTPS <= 0 {
				t.Errorf("day %s: expected positive avg_tps, got %.2f", ds.Date, ds.AvgTPS)
			}
			if ds.AvgTPSExcludeTTFT <= 0 {
				t.Errorf("day %s: expected positive avg_tps_exclude_ttft, got %.2f", ds.Date, ds.AvgTPSExcludeTTFT)
			}
			if ds.AvgTTFTMs <= 0 {
				t.Errorf("day %s: expected positive avg_ttft_ms, got %.1f", ds.Date, ds.AvgTTFTMs)
			}
		}
	}
}

func TestGetDailyStatsGroupedAndIncludesEmptyModels(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	zulu := createTestProvider(t, store, "Zulu")
	alpha := createTestProvider(t, store, "Alpha")
	emptyProbe := createTestProbe(t, store, alpha.ID, "a-empty")
	dataProbe := createTestProbe(t, store, alpha.ID, "b-data")
	createTestProbe(t, store, zulu.ID, "z-empty")

	now := time.Now()
	for _, result := range []*model.Result{
		{ProbeID: dataProbe.ID, Status: model.StatusSuccess, LatencyMs: 100, TTFTMs: 40, TPS: 10, TPSExcludeTTFT: 12, CreatedAt: now},
		{ProbeID: dataProbe.ID, Status: model.StatusError, LatencyMs: 999, TTFTMs: 999, TPS: 999, TPSExcludeTTFT: 999, CreatedAt: now},
		{ProbeID: dataProbe.ID, Status: model.StatusSuccess, LatencyMs: 200, TTFTMs: 60, TPS: 20, TPSExcludeTTFT: 24, CreatedAt: now.AddDate(0, 0, -1)},
	} {
		if err := store.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	stats, err := store.GetDailyStats(2)
	if err != nil {
		t.Fatalf("GetDailyStats: %v", err)
	}
	if len(stats) != 2 || stats[0].ProviderName != "Alpha" || stats[1].ProviderName != "Zulu" {
		t.Fatalf("unexpected provider grouping/order: %+v", stats)
	}
	if len(stats[0].Models) != 2 || stats[0].Models[0].ProbeID != emptyProbe.ID {
		t.Fatalf("unexpected model grouping/order: %+v", stats[0].Models)
	}
	if stats[0].Models[0].Daily == nil || len(stats[0].Models[0].Daily) != 0 {
		t.Errorf("empty model daily series should be []: %#v", stats[0].Models[0].Daily)
	}
	daily := stats[0].Models[1].Daily
	if len(daily) != 2 || daily[0].Date < daily[1].Date {
		t.Fatalf("unexpected daily series: %+v", daily)
	}
	if daily[0].Total != 2 || daily[0].Failed != 1 || daily[0].Success != 50 || daily[0].AvgLatencyMs != 100 {
		t.Errorf("today summary did not preserve metrics: %+v", daily[0])
	}
}

func TestGetHourlyStatsGroupedAndIncludesEmptyModels(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	zulu := createTestProvider(t, store, "Zulu")
	alpha := createTestProvider(t, store, "Alpha")
	emptyProbe := createTestProbe(t, store, alpha.ID, "a-empty")
	dataProbe := createTestProbe(t, store, alpha.ID, "b-data")
	createTestProbe(t, store, zulu.ID, "z-empty")

	now := time.Now()
	for _, result := range []*model.Result{
		{ProbeID: dataProbe.ID, Status: model.StatusSuccess, LatencyMs: 100, CreatedAt: now},
		{ProbeID: dataProbe.ID, Status: model.StatusError, LatencyMs: 200, CreatedAt: now.Add(-30 * time.Minute)},
		{ProbeID: dataProbe.ID, Status: model.StatusSuccess, LatencyMs: 150, CreatedAt: now.Add(-2 * time.Hour)},
	} {
		if err := store.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	stats, err := store.GetHourlyStats(24)
	if err != nil {
		t.Fatalf("GetHourlyStats: %v", err)
	}
	if len(stats) != 2 || stats[0].ProviderName != "Alpha" || stats[1].ProviderName != "Zulu" {
		t.Fatalf("unexpected provider grouping/order: %+v", stats)
	}
	if len(stats[0].Models) != 2 || stats[0].Models[0].ProbeID != emptyProbe.ID {
		t.Fatalf("unexpected model grouping/order: %+v", stats[0].Models)
	}
	if stats[0].Models[0].Hourly == nil || len(stats[0].Models[0].Hourly) != 0 {
		t.Errorf("empty model hourly series should be []: %#v", stats[0].Models[0].Hourly)
	}
	if len(stats[0].Models[1].Hourly) == 0 {
		t.Fatalf("expected hourly buckets for data model, got %+v", stats[0].Models[1].Hourly)
	}
	total := 0
	failed := 0
	for _, hour := range stats[0].Models[1].Hourly {
		total += hour.Total
		failed += hour.Failed
	}
	if total != 3 || failed != 1 {
		t.Errorf("hourly totals = %d failed=%d, want 3 and 1", total, failed)
	}
}
