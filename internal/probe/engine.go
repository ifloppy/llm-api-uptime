package probe

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
)

type Engine struct {
	store    *store.Store
	config   *config.Config
	logger   *slog.Logger
	stopCh   chan struct{}
	doneCh   chan struct{}
	running  bool
	mu       sync.Mutex
}

func NewEngine(store *store.Store, config *config.Config, logger *slog.Logger) *Engine {
	return &Engine{
		store:  store,
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.doneCh = make(chan struct{})
	e.mu.Unlock()

	go e.run()
}

func (e *Engine) Stop() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	close(e.stopCh)
	e.mu.Unlock()

	<-e.doneCh
}

func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Engine) run() {
	defer close(e.doneCh)

	e.logger.Info("probe engine started", "interval", e.config.ProbeInterval)
	e.probeAll()

	ticker := time.NewTicker(e.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			e.logger.Info("probe engine stopped")
			return
		case <-ticker.C:
			e.probeAll()
		}
	}
}

func (e *Engine) probeAll() {
	probes, err := e.store.GetEnabledProbes()
	if err != nil {
		e.logger.Error("failed to get enabled probes", "error", err)
		return
	}

	if len(probes) == 0 {
		e.logger.Debug("no enabled probes")
		return
	}

	e.logger.Info("starting probe cycle", "count", len(probes))

	providerGroups := make(map[int64][]model.ProbeWithProvider)
	for _, p := range probes {
		providerGroups[p.ProviderID] = append(providerGroups[p.ProviderID], p)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, e.config.ProbeConcurrency)

	for _, group := range providerGroups {
		wg.Add(1)
		go func(probes []model.ProbeWithProvider) {
			defer wg.Done()
			for _, p := range probes {
				semaphore <- struct{}{}
				e.probeOne(p)
				<-semaphore
			}
		}(group)
	}

	wg.Wait()
	e.logger.Info("probe cycle completed")
}

func (e *Engine) probeOne(p model.ProbeWithProvider) {
	ctx, cancel := context.WithTimeout(context.Background(), e.config.ProbeTimeout)
	defer cancel()

	var result *model.Result

	switch p.APIType {
	case model.APITypeOpenAI:
		result = probeOpenAI(ctx, p.ProviderURL, p.ProviderName, p.Model)
	case model.APITypeAnthropic:
		result = probeAnthropic(ctx, p.ProviderURL, p.ProviderName, p.Model)
	default:
		e.logger.Error("unknown api type", "type", p.APIType, "provider", p.ProviderName)
		return
	}

	result.ProbeID = p.ID
	result.CreatedAt = time.Now()

	if err := e.store.SaveResult(result); err != nil {
		e.logger.Error("failed to save result", "error", err, "probe_id", p.ID)
		return
	}

	if result.Status != model.StatusSuccess {
		e.logger.Warn("probe failed",
			"provider", p.ProviderName,
			"model", p.Model,
			"status", result.Status,
			"error", result.ErrorMessage,
			"request_id", result.RequestID,
		)
	} else {
		e.logger.Debug("probe success",
			"provider", p.ProviderName,
			"model", p.Model,
			"latency_ms", result.LatencyMs,
		)
	}
}

func (e *Engine) TriggerOnce() {
	e.logger.Info("manual probe triggered")
	go e.probeAll()
}
