package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"llm-api-uptime/internal/buildinfo"
	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/restart"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui"
	"llm-api-uptime/internal/update"
	"llm-api-uptime/internal/web"
)

func main() {
	os.Exit(run(os.Args, os.Stdout))
}

func printVersion(args []string, output io.Writer) bool {
	if len(args) != 1 || (args[0] != "--version" && args[0] != "-version") {
		return false
	}
	build := buildinfo.Current()
	fmt.Fprintf(output, "%s (commit %s, built %s)\n", build.Version, build.Commit, build.BuildDate)
	return true
}

func run(args []string, output io.Writer) int {
	if printVersion(args[1:], output) {
		return 0
	}
	executable, executableErr := os.Executable()

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
		return 1
	}

	engine := probe.NewEngine(db, cfg, logger)
	engine.Start()
	updateContext, stopUpdates := context.WithCancel(context.Background())
	updater := update.NewChecker(
		update.WithInterval(cfg.UpdateCheckInterval),
		update.WithAutoStage(cfg.UpdateAutoStage),
	)
	if cfg.UpdateCheckEnabled {
		updater.Start(updateContext)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	type restartRequest struct{ accepted chan error }
	restartCh := make(chan restartRequest)
	shutdownStarted := make(chan struct{})
	requestRestart := func() error {
		if !restart.Supported() {
			return fmt.Errorf("automatic update restart is unsupported on this platform")
		}
		if executableErr != nil {
			return fmt.Errorf("resolve executable for restart: %w", executableErr)
		}
		request := restartRequest{accepted: make(chan error, 1)}
		select {
		case restartCh <- request:
		case <-shutdownStarted:
			return fmt.Errorf("shutdown is already in progress")
		}
		return <-request.accepted
	}

	var webServer *web.Server
	if cfg.WebEnabled {
		webServer = web.NewServer(db, engine, cfg, logger, web.WithUpdater(updater, requestRestart))
		if err := webServer.Start(); err != nil {
			logger.Error("failed to start web server", "error", err)
		}
	}

	app := tui.NewApp(db, engine, cfg, logger, webServer)
	tuiDone := make(chan error, 1)
	go func() { tuiDone <- app.Run() }()

	restartRequested := false
	var tuiErr error
	select {
	case tuiErr = <-tuiDone:
		close(shutdownStarted)
	case signal := <-sigCh:
		close(shutdownStarted)
		logger.Info("shutdown signal received", "signal", signal)
		app.Stop()
		tuiErr = <-tuiDone
	case request := <-restartCh:
		restartRequested = true
		request.accepted <- nil
		close(shutdownStarted)
		logger.Info("restart requested")
		app.Stop()
		tuiErr = <-tuiDone
	}

	stopUpdates()
	if cfg.UpdateCheckEnabled {
		updater.Wait()
	}
	if webServer != nil {
		webServer.Stop()
	}
	engine.Stop()
	if err := db.Close(); err != nil {
		logger.Error("failed to close database", "error", err)
		return 1
	}
	if tuiErr != nil {
		logger.Error("tui error", "error", tuiErr)
		return 1
	}
	if restartRequested {
		if err := restart.Exec(executable, args, os.Environ()); err != nil {
			backup := updater.Status().BackupPath
			logger.Error("failed to restart updated executable", "error", err, "backup", backup)
			if restoreErr := restart.Restore(executable, backup); restoreErr != nil {
				logger.Error("failed to restore previous executable", "error", restoreErr)
				return 1
			}
			if retryErr := restart.Exec(executable, args, os.Environ()); retryErr != nil {
				logger.Error("failed to restart restored executable", "error", retryErr)
				return 1
			}
		}
	}
	return 0
}
