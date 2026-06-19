package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

func setupTestHandler(t *testing.T) (*Handler, *store.Store) {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		ProbeInterval:    5 * time.Minute,
		ProbeTimeout:     30 * time.Second,
		ProbeConcurrency: 3,
		DBPath:           ":memory:",
		WebPassword:      "test-password",
	}

	engine := probe.NewEngine(db, cfg, logger)
	handler := NewHandler(db, engine, cfg, logger)

	return handler, db
}

func createTestProviderInDB(t *testing.T, db *store.Store, name string) *model.Provider {
	t.Helper()
	provider := &model.Provider{
		Name:    name,
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	return provider
}

func TestHandleStatus(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()

	handler.handleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["interval"] != "5m0s" {
		t.Errorf("expected interval '5m0s', got %v", resp["interval"])
	}
}

func TestHandleListProviders(t *testing.T) {
	handler, db := setupTestHandler(t)

	createTestProviderInDB(t, db, "Provider A")
	createTestProviderInDB(t, db, "Provider B")

	req := httptest.NewRequest("GET", "/api/providers", nil)
	rec := httptest.NewRecorder()

	handler.handleListProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var providers []model.Provider
	json.NewDecoder(rec.Body).Decode(&providers)

	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestHandleCreateProvider(t *testing.T) {
	handler, db := setupTestHandler(t)
	_ = db

	body := `{"name": "New Provider", "base_url": "https://api.new.com", "api_key": "new-key", "api_type": "openai"}`
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.handleCreateProvider(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	var provider model.Provider
	json.NewDecoder(rec.Body).Decode(&provider)

	if provider.Name != "New Provider" {
		t.Errorf("expected name 'New Provider', got %q", provider.Name)
	}
}

func TestHandleCreateProviderInvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/api/providers", bytes.NewBufferString("invalid"))
	rec := httptest.NewRecorder()

	handler.handleCreateProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleUpdateProvider(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "Original")

	body := `{"name": "Updated", "base_url": "https://updated.com", "api_key": "updated-key", "api_type": "openai", "enabled": true}`
	req := httptest.NewRequest("PUT", "/api/providers/1", bytes.NewBufferString(body))
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.handleUpdateProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	updated, _ := db.GetProvider(provider.ID)
	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", updated.Name)
	}
}

func TestHandleUpdateProviderInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("PUT", "/api/providers/invalid", bytes.NewBufferString("{}"))
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleUpdateProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDeleteProvider(t *testing.T) {
	handler, db := setupTestHandler(t)
	createTestProviderInDB(t, db, "ToDelete")

	req := httptest.NewRequest("DELETE", "/api/providers/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.handleDeleteProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	_, err := db.GetProvider(1)
	if err == nil {
		t.Error("expected provider to be deleted")
	}
}

func TestHandleDeleteProviderInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("DELETE", "/api/providers/invalid", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleDeleteProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleListProbes(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")

	db.CreateProbe(&model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true})
	db.CreateProbe(&model.Probe{ProviderID: provider.ID, Model: "gpt-3.5-turbo", Enabled: true})

	req := httptest.NewRequest("GET", "/api/probes", nil)
	rec := httptest.NewRecorder()

	handler.handleListProbes(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var probes []model.ProbeWithProvider
	json.NewDecoder(rec.Body).Decode(&probes)

	if len(probes) != 2 {
		t.Errorf("expected 2 probes, got %d", len(probes))
	}
}

func TestHandleListProbesWithProviderID(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")

	db.CreateProbe(&model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true})

	req := httptest.NewRequest("GET", "/api/probes?provider_id=1", nil)
	rec := httptest.NewRecorder()

	handler.handleListProbes(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleCreateProbe(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")

	body := `{"provider_id": 1, "model": "gpt-4"}`
	req := httptest.NewRequest("POST", "/api/probes", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.handleCreateProbe(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	probes, _ := db.ListProbes(provider.ID)
	if len(probes) != 1 {
		t.Errorf("expected 1 probe, got %d", len(probes))
	}
}

func TestHandleDeleteProbe(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	db.CreateProbe(&model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true})

	req := httptest.NewRequest("DELETE", "/api/probes/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.handleDeleteProbe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleStats(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		StatusCode: 200,
		LatencyMs: 100,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/stats?hours=24", nil)
	rec := httptest.NewRecorder()

	handler.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var stats []model.ProviderStats
	json.NewDecoder(rec.Body).Decode(&stats)

	if len(stats) != 1 {
		t.Errorf("expected 1 provider stats, got %d", len(stats))
	}
}

func TestHandleExportCSV(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		StatusCode: 200,
		LatencyMs: 100,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/export/csv?hours=24", nil)
	rec := httptest.NewRecorder()

	handler.handleExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type 'text/csv', got %q", contentType)
	}
}

func TestHandleTriggerProbe(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/api/probe/trigger", nil)
	rec := httptest.NewRecorder()

	handler.handleTriggerProbe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "triggered" {
		t.Errorf("expected status 'triggered', got %q", resp["status"])
	}
}

func TestHandleLogin(t *testing.T) {
	handler, _ := setupTestHandler(t)

	body := `{"password": "test-password"}`
	req := httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.handleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestHandleLoginInvalidPassword(t *testing.T) {
	handler, _ := setupTestHandler(t)

	body := `{"password": "wrong-password"}`
	req := httptest.NewRequest("POST", "/api/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.handleLogin(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleLoginInvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/api/login", bytes.NewBufferString("invalid"))
	rec := httptest.NewRecorder()

	handler.handleLogin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	handler, _ := setupTestHandler(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/status"},
		{"GET", "/api/providers"},
		{"POST", "/api/providers"},
		{"GET", "/api/probes"},
		{"POST", "/api/probes"},
		{"GET", "/api/stats"},
		{"GET", "/api/export/csv"},
		{"POST", "/api/probe/trigger"},
		{"POST", "/api/login"},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code == http.StatusNotFound {
				t.Errorf("route %s %s not registered", route.method, route.path)
			}
		})
	}
}

func TestHandleFetchModels(t *testing.T) {
	// Create a mock server that simulates the models API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]string{
				{"id": "gpt-4"},
				{"id": "gpt-3.5-turbo"},
			},
		})
	}))
	defer server.Close()

	handler, db := setupTestHandler(t)
	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: server.URL,
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	db.CreateProvider(provider)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/providers/%d/models", provider.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", provider.ID))
	rec := httptest.NewRecorder()

	handler.handleFetchModels(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	models, ok := resp["models"].([]interface{})
	if !ok {
		t.Fatal("expected models array in response")
	}
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestHandleFetchModelsInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/providers/invalid/models", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleFetchModels(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleFetchModelsProviderNotFound(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/providers/999/models", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()

	handler.handleFetchModels(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleListProbesInvalidProviderID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/probes?provider_id=invalid", nil)
	rec := httptest.NewRecorder()

	handler.handleListProbes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateProbeInvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("POST", "/api/probes", bytes.NewBufferString("invalid"))
	rec := httptest.NewRecorder()

	handler.handleCreateProbe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDeleteProbeInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("DELETE", "/api/probes/invalid", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleDeleteProbe(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleStatsWithDays(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		StatusCode: 200,
		LatencyMs: 100,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/stats?days=7", nil)
	rec := httptest.NewRecorder()

	handler.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleExportCSVWithDays(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:   probe.ID,
		Status:    model.StatusSuccess,
		StatusCode: 200,
		LatencyMs: 100,
		CreatedAt: time.Now(),
	})

	req := httptest.NewRequest("GET", "/api/export/csv?days=30", nil)
	rec := httptest.NewRecorder()

	handler.handleExportCSV(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleUpdateProviderInvalidJSON(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("PUT", "/api/providers/1", bytes.NewBufferString("invalid"))
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.handleUpdateProvider(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateProviderDuplicateName(t *testing.T) {
	handler, db := setupTestHandler(t)
	createTestProviderInDB(t, db, "Duplicate")

	body := `{"name": "Duplicate", "base_url": "https://api.new.com", "api_key": "new-key", "api_type": "openai"}`
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.handleCreateProvider(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleFetchModelsServerDown(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://localhost:1", // Server not running
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	db.CreateProvider(provider)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/providers/%d/models", provider.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", provider.ID))
	rec := httptest.NewRecorder()

	handler.handleFetchModels(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestHandleStatsInvalidHours(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/stats?hours=invalid", nil)
	rec := httptest.NewRecorder()

	handler.handleStats(rec, req)

	// Should still return 200 with default query
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleExportCSVInvalidDays(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/export/csv?days=invalid", nil)
	rec := httptest.NewRecorder()

	handler.handleExportCSV(rec, req)

	// Should still return 200 with default query
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHandleListProbesWithInvalidProviderID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/probes?provider_id=invalid", nil)
	rec := httptest.NewRecorder()

	handler.handleListProbes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNewHandler(t *testing.T) {
	handler, _ := setupTestHandler(t)

	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if handler.store == nil {
		t.Error("expected store to be set")
	}
	if handler.engine == nil {
		t.Error("expected engine to be set")
	}
	if handler.config == nil {
		t.Error("expected config to be set")
	}
	if handler.logger == nil {
		t.Error("expected logger to be set")
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusOK, map[string]string{"test": "value"})

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["test"] != "value" {
		t.Errorf("expected test='value', got %q", resp["test"])
	}
}

func TestReadJSON(t *testing.T) {
	body := `{"name": "test"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))

	var result map[string]string
	err := readJSON(req, &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("expected name='test', got %q", result["name"])
	}
}

func TestReadJSONInvalid(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("invalid"))

	var result map[string]string
	err := readJSON(req, &result)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
