package pages

import (
	"testing"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

func TestNewHome(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	if home == nil {
		t.Fatal("expected non-nil home page")
	}
}

func TestHomeIsFormMode(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	if home.IsFormMode() {
		t.Error("home should not be in form mode")
	}
}

func TestHomeInit(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	cmd := home.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestHomeSetSize(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(100, 50)

	if home.width != 100 {
		t.Errorf("expected width 100, got %d", home.width)
	}
	if home.height != 50 {
		t.Errorf("expected height 50, got %d", home.height)
	}
}

func TestHomeView(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DBPath:        ":memory:",
		ProbeInterval: 300000000000,
	}
	engine := probe.NewEngine(db, cfg, nil)

	home := NewHome(db, engine, cfg, nil)
	home.SetSize(80, 24)

	view := home.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
