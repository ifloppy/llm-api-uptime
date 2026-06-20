package probe

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
)

func setupTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		ProbeInterval:    100 * time.Millisecond,
		ProbeTimeout:     1 * time.Second,
		ProbeConcurrency: 2,
	}

	engine := NewEngine(db, cfg, logger)
	return engine, db
}

func createTestSetup(t *testing.T, db *store.Store) {
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

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "test-model",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}
}

func TestNewEngine(t *testing.T) {
	engine, _ := setupTestEngine(t)

	if engine == nil {
		t.Fatal("expected engine to be created")
	}
	if engine.IsRunning() {
		t.Error("new engine should not be running")
	}
}

func TestEngineStartStop(t *testing.T) {
	engine, _ := setupTestEngine(t)

	engine.Start()
	if !engine.IsRunning() {
		t.Error("engine should be running after Start()")
	}

	engine.Start()
	if !engine.IsRunning() {
		t.Error("engine should still be running after double Start()")
	}

	engine.Stop()
	if engine.IsRunning() {
		t.Error("engine should not be running after Stop()")
	}

	engine.Stop()
	if engine.IsRunning() {
		t.Error("engine should still not be running after double Stop()")
	}
}

func TestEngineTriggerOnce(t *testing.T) {
	engine, db := setupTestEngine(t)
	createTestSetup(t, db)

	engine.TriggerOnce()
	time.Sleep(500 * time.Millisecond)

	results, err := db.GetResultsForProbe(1, 50)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result after TriggerOnce()")
	}
}

func TestEngineProbeWithNoProbes(t *testing.T) {
	engine, db := setupTestEngine(t)

	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://localhost:1",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	db.CreateProvider(provider)

	engine.TriggerOnce()
	time.Sleep(100 * time.Millisecond)

	results, err := db.GetResultsForProbe(1, 50)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results when no probes configured")
	}
}

func TestEngineProbeWithDisabledProvider(t *testing.T) {
	engine, db := setupTestEngine(t)

	provider := &model.Provider{
		Name:    "DisabledProvider",
		BaseURL: "http://localhost:1",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: false,
	}
	db.CreateProvider(provider)

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "test-model",
		Enabled:    true,
	}
	db.CreateProbe(probe)

	engine.TriggerOnce()
	time.Sleep(100 * time.Millisecond)

	results, err := db.GetResultsForProbe(probe.ID, 50)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results when provider is disabled")
	}
}

func TestEngineProbeWithDisabledProbe(t *testing.T) {
	engine, db := setupTestEngine(t)

	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://localhost:1",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	db.CreateProvider(provider)

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "test-model",
		Enabled:    false,
	}
	db.CreateProbe(probe)

	engine.TriggerOnce()
	time.Sleep(100 * time.Millisecond)

	results, err := db.GetResultsForProbe(probe.ID, 50)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results when probe is disabled")
	}
}

func TestEngineProbeUnknownAPIType(t *testing.T) {
	engine, db := setupTestEngine(t)

	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://localhost:1",
		APIKey:  "test-key",
		APIType: "unknown",
		Enabled: true,
	}
	db.CreateProvider(provider)

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "test-model",
		Enabled:    true,
	}
	db.CreateProbe(probe)

	engine.TriggerOnce()
	time.Sleep(100 * time.Millisecond)

	results, err := db.GetResultsForProbe(probe.ID, 50)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) != 0 {
		t.Error("expected no results for unknown API type")
	}
}
