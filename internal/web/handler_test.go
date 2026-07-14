package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"llm-api-uptime/internal/buildinfo"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/update"
)

type fakeUpdater struct {
	status update.Status
}

func (u fakeUpdater) Status() update.Status { return u.status }

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
	if resp["version"] != buildinfo.Version || resp["commit"] != buildinfo.Commit || resp["build_date"] != buildinfo.BuildDate {
		t.Errorf("unexpected build fields: %#v", resp)
	}
}

func TestHandleUpdateStatus(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler.config.UpdateCheckEnabled = true
	handler.updater = fakeUpdater{status: update.Status{
		State:      update.StateRestartRequired,
		Current:    "v1.0.0",
		Latest:     "v1.1.0",
		ReleaseURL: "https://example.com/release",
	}}
	recorder := httptest.NewRecorder()
	handler.handleUpdateStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/update", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var response map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["status"] != string(update.StateRestartRequired) || response["current"] != "v1.0.0" || response["latest"] != "v1.1.0" || response["release_url"] != "https://example.com/release" || response["restart_required"] != true || response["enabled"] != true {
		t.Fatalf("unexpected update response: %#v", response)
	}
}

func TestHandleUpdateStatusRedactsGuestError(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler.config.UpdateCheckEnabled = true
	handler.updater = fakeUpdater{status: update.Status{
		State: update.StateError,
		Error: `replace executable C:\secret\llm-api-uptime.exe: access denied`,
	}}
	request := markGuest(httptest.NewRequest(http.MethodGet, "/api/update", nil))
	recorder := httptest.NewRecorder()
	handler.handleUpdateStatus(recorder, request)
	if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "access denied") {
		t.Fatalf("guest update response leaked internal error: %s", recorder.Body.String())
	}
}

func TestHandleUpdateRestartValidation(t *testing.T) {
	tests := []struct {
		name       string
		status     update.State
		guest      bool
		restartErr error
		wantStatus int
		wantCalls  int
	}{
		{name: "not staged", status: update.StateAvailable, wantStatus: http.StatusConflict},
		{name: "guest", status: update.StateRestartRequired, guest: true, wantStatus: http.StatusUnauthorized},
		{name: "platform unsupported", status: update.StateUnsupported, wantStatus: http.StatusNotImplemented},
		{name: "unsupported", status: update.StateRestartRequired, restartErr: fmt.Errorf("unsupported"), wantStatus: http.StatusServiceUnavailable, wantCalls: 1},
		{name: "accepted", status: update.StateRestartRequired, wantStatus: http.StatusAccepted, wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, _ := setupTestHandler(t)
			handler.updater = fakeUpdater{status: update.Status{State: test.status}}
			calls := 0
			handler.restart = func() error { calls++; return test.restartErr }
			request := httptest.NewRequest(http.MethodPost, "/api/update/restart", nil)
			if test.guest {
				request = markGuest(request)
			}
			recorder := httptest.NewRecorder()
			handler.handleUpdateRestart(recorder, request)
			if recorder.Code != test.wantStatus || calls != test.wantCalls {
				t.Fatalf("status/calls = %d/%d, want %d/%d", recorder.Code, calls, test.wantStatus, test.wantCalls)
			}
		})
	}
}

func TestHandleUpdateRestartRequiresPassword(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler.config.WebPassword = ""
	handler.updater = fakeUpdater{status: update.Status{State: update.StateRestartRequired}}
	calls := 0
	handler.restart = func() error { calls++; return nil }
	recorder := httptest.NewRecorder()
	handler.handleUpdateRestart(recorder, httptest.NewRequest(http.MethodPost, "/api/update/restart", nil))
	if recorder.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("status/calls = %d/%d, want %d/0", recorder.Code, calls, http.StatusForbidden)
	}
}

func TestUpdateRoutesGuestAndAuthenticatedRestart(t *testing.T) {
	handler, _ := setupTestHandler(t)
	handler.config.UpdateCheckEnabled = true
	handler.config.WebGuestEnabled = true
	handler.updater = fakeUpdater{status: update.Status{State: update.StateRestartRequired, Current: "v1.0.0", Latest: "v1.1.0"}}
	restarted := make(chan struct{}, 1)
	handler.restart = func() error {
		restarted <- struct{}{}
		return nil
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	server := AuthMiddleware(handler.config.WebPassword, true)(mux)

	guestGet := httptest.NewRecorder()
	server.ServeHTTP(guestGet, httptest.NewRequest(http.MethodGet, "/api/update", nil))
	if guestGet.Code != http.StatusOK {
		t.Fatalf("guest GET status = %d, want 200", guestGet.Code)
	}
	installPost := httptest.NewRecorder()
	installRequest := httptest.NewRequest(http.MethodPost, "/api/update", nil)
	installRequest.Header.Set("Authorization", "Bearer "+handler.config.WebPassword)
	server.ServeHTTP(installPost, installRequest)
	if installPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("authenticated POST /api/update status = %d, want 405 and no install endpoint", installPost.Code)
	}

	guestPost := httptest.NewRecorder()
	server.ServeHTTP(guestPost, httptest.NewRequest(http.MethodPost, "/api/update/restart", nil))
	if guestPost.Code != http.StatusUnauthorized {
		t.Fatalf("guest POST status = %d, want 401", guestPost.Code)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodPost, "/api/update/restart", nil)
	authenticatedRequest.Header.Set("Authorization", "Bearer "+handler.config.WebPassword)
	authenticatedPost := httptest.NewRecorder()
	server.ServeHTTP(authenticatedPost, authenticatedRequest)
	if authenticatedPost.Code != http.StatusAccepted {
		t.Fatalf("authenticated POST status = %d, want 202", authenticatedPost.Code)
	}
	select {
	case <-restarted:
	default:
		t.Fatal("restart signal was not sent")
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

func TestHandleListProbesGuestRedactsProviderSecrets(t *testing.T) {
	for _, rawURL := range []string{"/api/probes", "/api/probes?provider_id=1"} {
		t.Run(rawURL, func(t *testing.T) {
			handler, db := setupTestHandler(t)
			defer db.Close()
			provider := createTestProviderInDB(t, db, "TestProvider")
			if err := db.CreateProbe(&model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}); err != nil {
				t.Fatalf("CreateProbe: %v", err)
			}

			req := markGuest(httptest.NewRequest("GET", rawURL, nil))
			rec := httptest.NewRecorder()
			handler.handleListProbes(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			body := rec.Body.String()
			if bytes.Contains([]byte(body), []byte("test-key")) || bytes.Contains([]byte(body), []byte("https://api.example.com")) {
				t.Fatalf("guest response leaked a provider secret: %s", body)
			}
			var probes []map[string]interface{}
			if err := json.Unmarshal([]byte(body), &probes); err != nil {
				t.Fatalf("decode probes: %v", err)
			}
			if len(probes) != 1 {
				t.Fatalf("expected one probe, got %d", len(probes))
			}
			if _, ok := probes[0]["api_key"]; ok {
				t.Error("guest probe response must not include api_key")
			}
			if _, ok := probes[0]["provider_url"]; ok {
				t.Error("guest probe response must not include provider_url")
			}
		})
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
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
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

func TestHandleStatsGuestRedactsLatestDetails(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	if err := db.SaveResult(&model.Result{
		ProbeID:      probe.ID,
		Status:       model.StatusError,
		StatusCode:   503,
		ErrorCode:    "secret-error-code",
		ErrorMessage: "secret error message",
		RequestID:    "secret-request-id",
		CreatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	req := markGuest(httptest.NewRequest("GET", "/api/stats", nil))
	rec := httptest.NewRecorder()
	handler.handleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, secret := range []string{"secret-error-code", "secret error message", "secret-request-id"} {
		if bytes.Contains([]byte(body), []byte(secret)) {
			t.Errorf("guest stats leaked %q: %s", secret, body)
		}
	}
	var stats []model.ProviderStats
	if err := json.Unmarshal([]byte(body), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	got := stats[0].Models[0]
	if got.LastStatus != string(model.StatusError) || got.LatestStatusCode != 503 || got.LatestResultTime == nil {
		t.Errorf("guest should retain non-sensitive latest status: %+v", got)
	}
}

func TestHandleStatsReturnsEmptyArray(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	rec := httptest.NewRecorder()
	handler.handleStats(rec, httptest.NewRequest("GET", "/api/stats", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Errorf("empty stats response = status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestHandleDailyStats(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	if err := db.SaveResult(&model.Result{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 100, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}

	req := markGuest(httptest.NewRequest("GET", "/api/stats/daily?days=30", nil))
	rec := httptest.NewRecorder()
	handler.handleDailyStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stats []model.ProviderDailyStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode daily stats: %v", err)
	}
	if len(stats) != 1 || len(stats[0].Models) != 1 || len(stats[0].Models[0].Daily) != 1 {
		t.Fatalf("unexpected daily stats shape: %+v", stats)
	}
	if stats[0].Models[0].Daily[0].AvgLatencyMs != 100 {
		t.Errorf("daily metrics were not preserved: %+v", stats[0].Models[0].Daily[0])
	}
}

func TestHandleHourlyStats(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	if err := db.SaveResult(&model.Result{ProbeID: probe.ID, Status: model.StatusSuccess, LatencyMs: 100, CreatedAt: time.Now()}); err != nil {
		t.Fatalf("SaveResult: %v", err)
	}
	if err := db.SaveResult(&model.Result{ProbeID: probe.ID, Status: model.StatusError, LatencyMs: 50, CreatedAt: time.Now().Add(-10 * time.Minute)}); err != nil {
		t.Fatalf("SaveResult error: %v", err)
	}

	req := markGuest(httptest.NewRequest("GET", "/api/stats/hourly?hours=24", nil))
	rec := httptest.NewRecorder()
	handler.handleHourlyStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var stats []model.ProviderHourlyStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode hourly stats: %v", err)
	}
	if len(stats) != 1 || len(stats[0].Models) != 1 || len(stats[0].Models[0].Hourly) == 0 {
		t.Fatalf("unexpected hourly stats shape: %+v", stats)
	}
	total := 0
	failed := 0
	for _, hour := range stats[0].Models[0].Hourly {
		total += hour.Total
		failed += hour.Failed
	}
	if total != 2 || failed != 1 {
		t.Errorf("hourly totals = %d failed=%d, want 2 and 1", total, failed)
	}
}

func TestHandleGetDowntimePeriods(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	now := time.Now()
	for _, result := range []*model.Result{
		{ProbeID: probe.ID, Status: model.StatusSuccess, CreatedAt: now.Add(-20 * time.Minute)},
		{ProbeID: probe.ID, Status: model.StatusError, CreatedAt: now.Add(-10 * time.Minute)},
		{ProbeID: probe.ID, Status: model.StatusSuccess, CreatedAt: now},
	} {
		if err := db.SaveResult(result); err != nil {
			t.Fatalf("SaveResult: %v", err)
		}
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/downtime?hours=24", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	rec := httptest.NewRecorder()
	handler.handleGetDowntimePeriods(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var periods []model.DowntimePeriod
	if err := json.NewDecoder(rec.Body).Decode(&periods); err != nil {
		t.Fatalf("decode downtime: %v", err)
	}
	if len(periods) != 1 {
		t.Fatalf("expected 1 downtime period, got %+v", periods)
	}
}

func TestHandleExportCSV(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
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
		{"GET", "/api/stats/daily"},
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
		w.Header().Set("Content-Type", "application/json")
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
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
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
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
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

func TestHandleClearStats(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
	})

	req := httptest.NewRequest("DELETE", "/api/stats", nil)
	rec := httptest.NewRecorder()

	handler.handleClearStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "cleared" {
		t.Errorf("expected status 'cleared', got %q", resp["status"])
	}
}

func TestHandleDeleteResult(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
	})

	req := httptest.NewRequest("DELETE", "/api/results/1", nil)
	req.SetPathValue("id", "1")
	rec := httptest.NewRecorder()

	handler.handleDeleteResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["status"] != "deleted" {
		t.Errorf("expected status 'deleted', got %q", resp["status"])
	}
}

func TestHandleDeleteResultInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("DELETE", "/api/results/invalid", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleDeleteResult(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleGetProbeResults(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	db.SaveResult(&model.Result{
		ProbeID:    probe.ID,
		Status:     model.StatusSuccess,
		StatusCode: 200,
		LatencyMs:  100,
		CreatedAt:  time.Now(),
	})

	db.SaveResult(&model.Result{
		ProbeID:    probe.ID,
		Status:     model.StatusError,
		StatusCode: 500,
		LatencyMs:  200,
		CreatedAt:  time.Now(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/results", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	rec := httptest.NewRecorder()

	handler.handleGetProbeResults(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	results, ok := resp["results"].([]interface{})
	if !ok {
		t.Fatal("expected results array in response")
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if resp["total"].(float64) != 2 {
		t.Errorf("expected total 2, got %v", resp["total"])
	}
}

func TestHandleGetProbeResultsClampsLimit(t *testing.T) {
	handler, db := setupTestHandler(t)
	defer db.Close()
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/results?limit=10000", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	rec := httptest.NewRecorder()
	handler.handleGetProbeResults(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var response struct {
		Results []model.Result `json:"results"`
		Limit   int            `json:"limit"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Limit != 100 {
		t.Errorf("limit = %d, want 100", response.Limit)
	}
	if response.Results == nil {
		t.Error("results must be an empty array, not null")
	}
}

func TestHandleGetProbeResultsInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/probes/invalid/results", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleGetProbeResults(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleGetProbeResultsGuest(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/results", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	req = markGuest(req)
	rec := httptest.NewRecorder()

	handler.handleGetProbeResults(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleGetHourlySummary(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/hourly", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	rec := httptest.NewRecorder()

	handler.handleGetHourlySummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp []interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if len(resp) != 0 {
		t.Errorf("expected 0 summary entries, got %d", len(resp))
	}
}

func TestHandleGetHourlySummaryInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/probes/invalid/hourly", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleGetHourlySummary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleListProvidersGuest(t *testing.T) {
	handler, db := setupTestHandler(t)
	createTestProviderInDB(t, db, "Provider A")
	createTestProviderInDB(t, db, "Provider B")

	req := httptest.NewRequest("GET", "/api/providers", nil)
	req = markGuest(req)
	rec := httptest.NewRecorder()

	handler.handleListProviders(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var providers []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&providers)

	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	// Guest should NOT see base_url or api_key
	for _, p := range providers {
		if _, ok := p["base_url"]; ok {
			t.Error("guest should not see base_url field")
		}
		if _, ok := p["api_key"]; ok {
			t.Error("guest should not see api_key field")
		}
		// Guest SHOULD see id, name, api_type, max_tokens, enabled
		if _, ok := p["id"]; !ok {
			t.Error("guest should see id field")
		}
		if _, ok := p["name"]; !ok {
			t.Error("guest should see name field")
		}
	}
}

func TestHandleStatusGuest(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/status", nil)
	req = markGuest(req)
	rec := httptest.NewRecorder()

	handler.handleStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	guest, ok := resp["guest"].(bool)
	if !ok {
		t.Fatal("expected guest field in response")
	}
	if !guest {
		t.Error("expected guest to be true")
	}
}

func TestHandleGetDailySummary(t *testing.T) {
	handler, db := setupTestHandler(t)
	provider := createTestProviderInDB(t, db, "TestProvider")
	probe := &model.Probe{ProviderID: provider.ID, Model: "gpt-4", Enabled: true}
	db.CreateProbe(probe)

	// Create some results
	now := time.Now()
	for i := 0; i < 7; i++ {
		db.SaveResult(&model.Result{
			ProbeID:    probe.ID,
			Status:     model.StatusSuccess,
			StatusCode: 200,
			LatencyMs:  100,
			CreatedAt:  now.AddDate(0, 0, -i),
		})
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/probes/%d/daily?days=7", probe.ID), nil)
	req.SetPathValue("id", fmt.Sprintf("%d", probe.ID))
	rec := httptest.NewRecorder()

	handler.handleGetDailySummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var summary []model.DailySummary
	json.NewDecoder(rec.Body).Decode(&summary)

	if len(summary) == 0 {
		t.Fatal("expected non-empty summary")
	}
}

func TestHandleGetDailySummaryInvalidID(t *testing.T) {
	handler, _ := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/api/probes/invalid/daily", nil)
	req.SetPathValue("id", "invalid")
	rec := httptest.NewRecorder()

	handler.handleGetDailySummary(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
