package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"llm-api-uptime/internal/buildinfo"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/stats"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/update"
)

type updateStatusProvider interface {
	Status() update.Status
}

type HandlerOption func(*Handler)

func withHandlerUpdater(updater updateStatusProvider, restart func() error) HandlerOption {
	return func(handler *Handler) {
		handler.updater = updater
		handler.restart = restart
	}
}

type Handler struct {
	store   *store.Store
	engine  *probe.Engine
	config  *config.Config
	logger  *slog.Logger
	updater updateStatusProvider
	restart func() error
}

func NewHandler(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger, options ...HandlerOption) *Handler {
	handler := &Handler{
		store:  store,
		engine: engine,
		config: config,
		logger: logger,
	}
	for _, option := range options {
		option(handler)
	}
	return handler
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", h.handleStatus)
	mux.HandleFunc("GET /api/update", h.handleUpdateStatus)
	mux.HandleFunc("POST /api/update/restart", h.handleUpdateRestart)
	mux.HandleFunc("GET /api/providers", h.handleListProviders)
	mux.HandleFunc("POST /api/providers", h.handleCreateProvider)
	mux.HandleFunc("PUT /api/providers/{id}", h.handleUpdateProvider)
	mux.HandleFunc("DELETE /api/providers/{id}", h.handleDeleteProvider)
	mux.HandleFunc("GET /api/providers/{id}/models", h.handleFetchModels)
	mux.HandleFunc("GET /api/probes", h.handleListProbes)
	mux.HandleFunc("POST /api/probes", h.handleCreateProbe)
	mux.HandleFunc("DELETE /api/probes/{id}", h.handleDeleteProbe)
	mux.HandleFunc("GET /api/stats", h.handleStats)
	mux.HandleFunc("GET /api/stats/daily", h.handleDailyStats)
	mux.HandleFunc("DELETE /api/stats", h.handleClearStats)
	mux.HandleFunc("GET /api/probes/{id}/results", h.handleGetProbeResults)
	mux.HandleFunc("GET /api/probes/{id}/hourly", h.handleGetHourlySummary)
	mux.HandleFunc("GET /api/probes/{id}/daily", h.handleGetDailySummary)
	mux.HandleFunc("GET /api/export/csv", h.handleExportCSV)
	mux.HandleFunc("DELETE /api/results/{id}", h.handleDeleteResult)
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
	lastProbeTime, _ := h.store.GetLastProbeTime()
	build := buildinfo.Current()

	response := map[string]interface{}{
		"running":    h.engine.IsRunning(),
		"interval":   h.config.ProbeInterval.String(),
		"db_path":    h.config.DBPath,
		"guest":      isGuest(r),
		"version":    build.Version,
		"commit":     build.Commit,
		"build_date": build.BuildDate,
	}

	if lastProbeTime != nil {
		response["last_probe_time"] = lastProbeTime.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"enabled":          h.config.UpdateCheckEnabled,
		"status":           update.StateDisabled,
		"current":          buildinfo.Current().Version,
		"latest":           "",
		"release_url":      "",
		"restart_required": false,
		"restart_allowed":  h.config.WebPassword != "" && !isGuest(r),
	}
	if h.config.UpdateCheckEnabled && h.updater != nil {
		status := h.updater.Status()
		response["status"] = status.State
		response["current"] = status.Current
		response["latest"] = status.Latest
		response["release_url"] = status.ReleaseURL
		response["restart_required"] = status.State == update.StateRestartRequired
		if status.Error != "" && !isGuest(r) {
			response["error"] = status.Error
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) handleUpdateRestart(w http.ResponseWriter, r *http.Request) {
	if isGuest(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	if h.config.WebPassword == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "restart requires WEB_PASSWORD authentication"})
		return
	}
	if h.updater == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "no staged update is ready to restart"})
		return
	}
	status := h.updater.Status()
	if status.State == update.StateUnsupported {
		message := status.Error
		if message == "" {
			message = "automatic update restart is unsupported"
		}
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": message})
		return
	}
	if status.State != update.StateRestartRequired {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "no staged update is ready to restart"})
		return
	}
	if h.restart == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "restart is not supported"})
		return
	}
	if err := h.restart(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "restart_requested"})
}

func (h *Handler) handleListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.store.ListProviders()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if providers == nil {
		providers = make([]model.Provider, 0)
	}

	if isGuest(r) {
		type guestProvider struct {
			ID        int64         `json:"id"`
			Name      string        `json:"name"`
			APIType   model.APIType `json:"api_type"`
			MaxTokens int           `json:"max_tokens"`
			Enabled   bool          `json:"enabled"`
		}
		guest := make([]guestProvider, len(providers))
		for i, p := range providers {
			guest[i] = guestProvider{
				ID:        p.ID,
				Name:      p.Name,
				APIType:   p.APIType,
				MaxTokens: p.MaxTokens,
				Enabled:   p.Enabled,
			}
		}
		writeJSON(w, http.StatusOK, guest)
		return
	}

	writeJSON(w, http.StatusOK, providers)
}

func (h *Handler) handleCreateProvider(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		APIKey    string `json:"api_key"`
		APIType   string `json:"api_type"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if p.MaxTokens <= 0 {
		p.MaxTokens = 2
	}

	provider := &model.Provider{
		Name:      p.Name,
		BaseURL:   p.BaseURL,
		APIKey:    p.APIKey,
		APIType:   model.APIType(p.APIType),
		MaxTokens: p.MaxTokens,
		Enabled:   true,
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
		Name      string `json:"name"`
		BaseURL   string `json:"base_url"`
		APIKey    string `json:"api_key"`
		APIType   string `json:"api_type"`
		MaxTokens int    `json:"max_tokens"`
		Enabled   bool   `json:"enabled"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if p.MaxTokens <= 0 {
		p.MaxTokens = 2
	}

	provider := &model.Provider{
		ID:        id,
		Name:      p.Name,
		BaseURL:   p.BaseURL,
		APIKey:    p.APIKey,
		APIType:   model.APIType(p.APIType),
		MaxTokens: p.MaxTokens,
		Enabled:   p.Enabled,
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
		if probes == nil {
			probes = make([]model.Probe, 0)
		}
		if isGuest(r) {
			writeJSON(w, http.StatusOK, probes)
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
	if probes == nil {
		probes = make([]model.ProbeWithProvider, 0)
	}
	if isGuest(r) {
		type guestProbe struct {
			model.Probe
			ProviderName string        `json:"provider_name"`
			MaxTokens    int           `json:"max_tokens"`
			APIType      model.APIType `json:"api_type"`
		}
		guest := make([]guestProbe, len(probes))
		for i, p := range probes {
			guest[i] = guestProbe{
				Probe:        p.Probe,
				ProviderName: p.ProviderName,
				MaxTokens:    p.MaxTokens,
				APIType:      p.APIType,
			}
		}
		writeJSON(w, http.StatusOK, guest)
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
	if isGuest(r) {
		for i := range stats {
			for j := range stats[i].Models {
				stats[i].Models[j].LatestErrorCode = ""
				stats[i].Models[j].LatestErrorMessage = ""
				stats[i].Models[j].LatestRequestID = ""
			}
		}
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleDailyStats(w http.ResponseWriter, r *http.Request) {
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if n, err := strconv.Atoi(daysStr); err == nil && n > 0 {
			days = n
		}
	}

	stats, err := h.store.GetDailyStats(days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if isGuest(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

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
	if !h.engine.TriggerOnce() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "probe engine is stopping"})
		return
	}
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

func (h *Handler) handleDeleteResult(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DeleteResult(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) handleGetProbeResults(w http.ResponseWriter, r *http.Request) {
	if isGuest(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	page := 1
	if pageStr != "" {
		if n, err := strconv.Atoi(pageStr); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	statusFilter := r.URL.Query().Get("status")

	results, err := h.store.GetResultsForProbePage(id, limit, offset, statusFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if results == nil {
		results = make([]model.Result, 0)
	}

	total, err := h.store.GetResultsCount(id, statusFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"pages":   (total + limit - 1) / limit,
	})
}

func (h *Handler) handleGetHourlySummary(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
			hours = n
		}
	}

	summary, err := h.store.GetHourlySummary(id, hours)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) handleGetDailySummary(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		if n, err := strconv.Atoi(daysStr); err == nil && n > 0 {
			days = n
		}
	}

	summary, err := h.store.GetDailySummary(id, days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, summary)
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
