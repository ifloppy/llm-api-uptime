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

	results, _ := store.GetResultsForProbe(probe.ID, 50)
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
