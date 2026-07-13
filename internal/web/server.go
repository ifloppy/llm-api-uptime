package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/update"
)

type Option func(*Server)

// WithUpdater exposes updater status and wires a validated restart request.
func WithUpdater(updater interface{ Status() update.Status }, restart func() error) Option {
	return func(server *Server) {
		server.updater = updater
		server.restart = restart
	}
}

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	store    *store.Store
	engine   *probe.Engine
	config   *config.Config
	logger   *slog.Logger
	listener net.Listener
	server   *http.Server
	running  bool
	stopping bool
	stopDone chan struct{}
	updater  updateStatusProvider
	restart  func() error
	mu       sync.Mutex
}

func NewServer(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger, options ...Option) *Server {
	server := &Server{
		store:  store,
		engine: engine,
		config: config,
		logger: logger,
	}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *Server) Start() error {
	s.mu.Lock()
	if s.running || s.stopping {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true

	mux := http.NewServeMux()

	handler := NewHandler(s.store, s.engine, s.config, s.logger, withHandlerUpdater(s.updater, s.restart))
	handler.RegisterRoutes(mux)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("create static fs: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, staticFS, "index.html")
			return
		}

		if r.URL.Path == "/login.html" {
			http.ServeFileFS(w, r, staticFS, "login.html")
			return
		}

		fileServer.ServeHTTP(w, r)
	})

	authMiddleware := AuthMiddleware(s.config.WebPassword, s.config.WebGuestEnabled)
	wrappedMux := authMiddleware(mux)

	addr := s.config.WebAddr()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.logger.Info("web server started", "addr", addr, "public", s.config.WebPublic)

	if s.config.WebPassword != "" {
		s.logger.Info("authentication enabled")
		if s.config.WebGuestEnabled {
			s.logger.Info("guest access enabled (read-only)")
		}
	} else {
		s.logger.Warn("authentication disabled - not recommended for public access")
	}

	httpServer := &http.Server{Handler: wrappedMux}
	s.listener = listener
	s.server = httpServer
	s.mu.Unlock()

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("web server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	if s.stopping {
		done := s.stopDone
		s.mu.Unlock()
		<-done
		return
	}
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.stopping = true
	s.stopDone = make(chan struct{})
	stopDone := s.stopDone
	httpServer := s.server
	s.mu.Unlock()

	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := httpServer.Shutdown(ctx); err != nil {
			s.logger.Warn("web server shutdown incomplete", "error", err)
			_ = httpServer.Close()
		}
		cancel()
	}

	s.mu.Lock()
	s.stopping = false
	s.stopDone = nil
	s.server = nil
	s.listener = nil
	close(stopDone)
	s.mu.Unlock()
	s.logger.Info("web server stopped")
}

func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) Addr() string {
	return s.config.WebAddr()
}
