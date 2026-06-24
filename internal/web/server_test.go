package web

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

func setupTestServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := probe.NewEngine(db, cfg, logger)
	return NewServer(db, engine, cfg, logger)
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		WebPort: 8080,
	}
	server := setupTestServer(t, cfg)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServerIsRunning_BeforeStart(t *testing.T) {
	cfg := &config.Config{}
	server := setupTestServer(t, cfg)

	if server.IsRunning() {
		t.Error("server should not be running before Start()")
	}
}

func TestServerAddr_Private(t *testing.T) {
	cfg := &config.Config{
		WebPort:   9090,
		WebPublic: false,
	}
	server := setupTestServer(t, cfg)

	addr := server.Addr()
	if addr != "127.0.0.1:9090" {
		t.Errorf("expected 127.0.0.1:9090, got %s", addr)
	}
}

func TestServerAddr_Public(t *testing.T) {
	cfg := &config.Config{
		WebPort:   9090,
		WebPublic: true,
	}
	server := setupTestServer(t, cfg)

	addr := server.Addr()
	if addr != "0.0.0.0:9090" {
		t.Errorf("expected 0.0.0.0:9090, got %s", addr)
	}
}

func TestServerStartStop(t *testing.T) {
	cfg := &config.Config{
		WebPort:   18080,
		WebPublic: true,
	}
	server := setupTestServer(t, cfg)

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait briefly for server goroutine to bind
	time.Sleep(50 * time.Millisecond)

	if !server.IsRunning() {
		t.Error("server should be running after Start()")
	}

	server.Stop()

	// Allow shutdown goroutine to finish
	time.Sleep(50 * time.Millisecond)

	if server.IsRunning() {
		t.Error("server should not be running after Stop()")
	}
}

func TestServerStart_AlreadyRunning(t *testing.T) {
	cfg := &config.Config{
		WebPort:   18081,
		WebPublic: true,
	}
	server := setupTestServer(t, cfg)

	if err := server.Start(); err != nil {
		t.Fatalf("first Start() failed: %v", err)
	}
	defer server.Stop()

	time.Sleep(50 * time.Millisecond)

	err := server.Start()
	if err == nil {
		t.Error("expected error when starting already-running server")
	}
}

func TestServerStop_NotRunning(t *testing.T) {
	cfg := &config.Config{}
	server := setupTestServer(t, cfg)

	// Stop on a non-running server should be a no-op (no panic)
	server.Stop()

	if server.IsRunning() {
		t.Error("server should still not be running")
	}
}
