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

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

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
	mu       sync.Mutex
}

func NewServer(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger) *Server {
	return &Server{
		store:  store,
		engine: engine,
		config: config,
		logger: logger,
	}
}

func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	mux := http.NewServeMux()

	handler := NewHandler(s.store, s.engine, s.config, s.logger)
	handler.RegisterRoutes(mux)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.mu.Lock()
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
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.listener = listener

	s.logger.Info("web server started", "addr", addr, "public", s.config.WebPublic)

	if s.config.WebPassword != "" {
		s.logger.Info("authentication enabled")
		if s.config.WebGuestEnabled {
			s.logger.Info("guest access enabled (read-only)")
		}
	} else {
		s.logger.Warn("authentication disabled - not recommended for public access")
	}

	s.server = &http.Server{Handler: wrappedMux}

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("web server error", "error", err)
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.server != nil {
		s.server.Shutdown(context.Background())
	}

	s.running = false
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
