package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui"
	"llm-api-uptime/internal/web"
)

func main() {
	godotenv.Load()

	cfg := config.Load()

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	db, err := store.New(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	engine := probe.NewEngine(db, cfg, logger)
	engine.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var webServer *web.Server
	if cfg.WebEnabled {
		webServer = web.NewServer(db, engine, cfg, logger)
		if err := webServer.Start(); err != nil {
			logger.Error("failed to start web server", "error", err)
		}
	}

	go func() {
		<-sigCh
		logger.Info("shutting down...")
		if webServer != nil && webServer.IsRunning() {
			webServer.Stop()
		}
		engine.Stop()
		os.Exit(0)
	}()

	app := tui.NewApp(db, engine, cfg, logger, webServer)
	if err := app.Run(); err != nil {
		logger.Error("tui error", "error", err)
		os.Exit(1)
	}
}
