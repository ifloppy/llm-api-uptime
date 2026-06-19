package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	store  *store.Store
	engine *probe.Engine
	config *config.Config
	logger *slog.Logger
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
	mux := http.NewServeMux()

	handler := NewHandler(s.store, s.engine, s.config, s.logger)
	handler.RegisterRoutes(mux)

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("create static fs: %w", err)
	}

	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, staticFS, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	authMiddleware := AuthMiddleware(s.config.WebPassword)
	wrappedMux := authMiddleware(mux)

	addr := s.config.WebAddr()
	s.logger.Info("web server starting", "addr", addr, "public", s.config.WebPublic)

	if s.config.WebPassword != "" {
		s.logger.Info("authentication enabled")
	} else {
		s.logger.Warn("authentication disabled - not recommended for public access")
	}

	return http.ListenAndServe(addr, wrappedMux)
}
