package pages

import (
	"testing"

	"llm-api-uptime/internal/store"
)

func TestNewProviders(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	if providers == nil {
		t.Fatal("expected non-nil providers page")
	}
}

func TestProvidersIsFormMode(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	if providers.IsFormMode() {
		t.Error("providers should not be in form mode by default")
	}
}

func TestProvidersInit(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	cmd := providers.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestProvidersSetSize(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	providers.SetSize(120, 40)

	if providers.width != 120 {
		t.Errorf("expected width 120, got %d", providers.width)
	}
	if providers.height != 40 {
		t.Errorf("expected height 40, got %d", providers.height)
	}
}

func TestProvidersView(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	providers.SetSize(80, 24)

	view := providers.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
