package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/stats"
	"llm-api-uptime/internal/store"
)

type Handler struct {
	store  *store.Store
	engine *probe.Engine
	config *config.Config
	logger *slog.Logger
}

func NewHandler(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		store:  store,
		engine: engine,
		config: config,
		logger: logger,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", h.handleStatus)
	mux.HandleFunc("GET /api/providers", h.handleListProviders)
	mux.HandleFunc("POST /api/providers", h.handleCreateProvider)
	mux.HandleFunc("PUT /api/providers/{id}", h.handleUpdateProvider)
	mux.HandleFunc("DELETE /api/providers/{id}", h.handleDeleteProvider)
	mux.HandleFunc("GET /api/providers/{id}/models", h.handleFetchModels)
	mux.HandleFunc("GET /api/probes", h.handleListProbes)
	mux.HandleFunc("POST /api/probes", h.handleCreateProbe)
	mux.HandleFunc("DELETE /api/probes/{id}", h.handleDeleteProbe)
	mux.HandleFunc("GET /api/stats", h.handleStats)
	mux.HandleFunc("DELETE /api/stats", h.handleClearStats)
	mux.HandleFunc("GET /api/export/csv", h.handleExportCSV)
	mux.HandleFunc("POST /api/probe/trigger", h.handleTriggerProbe)
	mux.HandleFunc("POST /api/login", h.handleLogin)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":  h.engine.IsRunning(),
		"interval": h.config.ProbeInterval.String(),
		"db_path":  h.config.DBPath,
	})
}

func (h *Handler) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListProviders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, providers)
}

func (h *Handler) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		APIType string `json:"api_type"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	provider := &model.Provider{
		Name:    p.Name,
		BaseURL: p.BaseURL,
		APIKey:  p.APIKey,
		APIType: model.APIType(p.APIType),
		Enabled: true,
	}

	if err := h.store.CreateProvider(provider); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, provider)
}

func (h *Handler) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var p struct {
		Name    string `json:"name"`
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		APIType string `json:"api_type"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	provider := &model.Provider{
		ID:      id,
		Name:    p.Name,
		BaseURL: p.BaseURL,
		APIKey:  p.APIKey,
		APIType: model.APIType(p.APIType),
		Enabled: p.Enabled,
	}

	if err := h.store.UpdateProvider(provider); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, provider)
}

func (h *Handler) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DeleteProvider(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) handleFetchModels(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	provider, err := h.store.GetProvider(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	models, err := probe.FetchModelList(r.Context(), *provider)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  err.Error(),
			"models": []string{},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models": models,
	})
}

func (h *Handler) handleListProbes(w http.ResponseWriter, r *http.Request) {
	providerIDStr := r.URL.Query().Get("provider_id")
	if providerIDStr != "" {
		providerID, err := strconv.ParseInt(providerIDStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider_id"})
			return
		}
		probes, err := h.store.ListProbes(providerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, probes)
		return
	}

	probes, err := h.store.GetEnabledProbes()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, probes)
}

func (h *Handler) handleCreateProbe(w http.ResponseWriter, r *http.Request) {
	var p struct {
		ProviderID int64  `json:"provider_id"`
		Model      string `json:"model"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	probe := &model.Probe{
		ProviderID: p.ProviderID,
		Model:      p.Model,
		Enabled:    true,
	}

	if err := h.store.CreateProbe(probe); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, probe)
}

func (h *Handler) handleDeleteProbe(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DeleteProbe(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	daysStr := r.URL.Query().Get("days")

	query := model.StatsQuery{}
	if hoursStr != "" {
		hours, err := strconv.Atoi(hoursStr)
		if err == nil {
			query.Hours = hours
		}
	}
	if daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err == nil {
			query.Days = days
		}
	}

	stats, err := h.store.GetStats(query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	daysStr := r.URL.Query().Get("days")

	query := model.StatsQuery{}
	if hoursStr != "" {
		hours, err := strconv.Atoi(hoursStr)
		if err == nil {
			query.Hours = hours
		}
	}
	if daysStr != "" {
		days, err := strconv.Atoi(daysStr)
		if err == nil {
			query.Days = days
		}
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=uptime_report.csv")

	if err := stats.ExportCSV(w, h.store, query); err != nil {
		h.logger.Error("failed to export csv", "error", err)
	}
}

func (h *Handler) handleTriggerProbe(w http.ResponseWriter, r *http.Request) {
	h.engine.TriggerOnce()
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (h *Handler) handleClearStats(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("clearing all statistics")
	if err := h.store.ClearResults(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.Password != h.config.WebPassword {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": req.Password})
}
