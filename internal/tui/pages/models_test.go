package pages

import (
	"testing"

	"llm-api-uptime/internal/store"
)

func TestNewModels(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	if models == nil {
		t.Fatal("expected non-nil models page")
	}
}

func TestModelsIsFormMode(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	if models.IsFormMode() {
		t.Error("models should not be in form mode by default")
	}
}

func TestModelsInit(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	cmd := models.Init()
	if cmd != nil {
		t.Error("expected nil Init command")
	}
}

func TestModelsSetSize(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	models.SetSize(80, 30)

	if models.width != 80 {
		t.Errorf("expected width 80, got %d", models.width)
	}
	if models.height != 30 {
		t.Errorf("expected height 30, got %d", models.height)
	}
}

func TestModelsView(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	models.SetSize(80, 24)

	view := models.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
