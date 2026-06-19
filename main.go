package main

import (
	"flag"
	"fmt"
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
	mode := flag.String("mode", "tui", "Run mode: tui or web")
	flag.Parse()

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

	go func() {
		<-sigCh
		logger.Info("shutting down...")
		engine.Stop()
		os.Exit(0)
	}()

	switch *mode {
	case "tui":
		app := tui.NewApp(db, engine, cfg, logger)
		if err := app.Run(); err != nil {
			logger.Error("tui error", "error", err)
			os.Exit(1)
		}
	case "web":
		cfg.WebEnabled = true
		srv := web.NewServer(db, engine, cfg, logger)
		logger.Info("starting web server", "addr", cfg.WebAddr())
		if err := srv.Start(); err != nil {
			logger.Error("web server error", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", *mode)
		fmt.Fprintf(os.Stderr, "Usage: llm-api-uptime -mode=tui|web\n")
		os.Exit(1)
	}
}
