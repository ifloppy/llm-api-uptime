package pages

import (
	"testing"

	"github.com/charmbracelet/bubbletea"

	"llm-api-uptime/internal/model"
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

func TestModelsViewEmpty(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)
	models.SetSize(80, 24)

	view := models.View()

	assertions := []struct {
		name string
		want string
	}{
		{"models title", "Models"},
		{"no models message", "No models configured"},
		{"help text", "add model"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !modelsContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestModelsViewWithData(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create provider and probe
	provider := &model.Provider{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-test",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-4",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}

	models := NewModels(db)
	models.SetSize(120, 40)

	view := models.View()

	assertions := []struct {
		name string
		want string
	}{
		{"models title", "Models"},
		{"provider name", "OpenAI"},
		{"model name", "gpt-4"},
		{"help text", "navigate"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !modelsContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestModelsUpdateNavigation(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create provider
	provider := &model.Provider{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-test",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Create two probes
	for _, modelName := range []string{"gpt-4", "gpt-3.5-turbo"} {
		p := &model.Probe{
			ProviderID: provider.ID,
			Model:      modelName,
			Enabled:    true,
		}
		if err := db.CreateProbe(p); err != nil {
			t.Fatalf("failed to create probe: %v", err)
		}
	}

	models := NewModels(db)
	if models.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", models.cursor)
	}

	// Test down navigation
	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ := models.Update(downMsg)
	mod := m.(*Models)
	if mod.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", mod.cursor)
	}

	// Test up navigation
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	m, _ = mod.Update(upMsg)
	mod = m.(*Models)
	if mod.cursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", mod.cursor)
	}

	// Test up at boundary
	m, _ = mod.Update(upMsg)
	mod = m.(*Models)
	if mod.cursor != 0 {
		t.Errorf("expected cursor 0 at upper boundary, got %d", mod.cursor)
	}
}

func TestModelsUpdateDeleteConfirm(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create provider and probe
	provider := &model.Provider{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-test",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	probe := &model.Probe{
		ProviderID: provider.ID,
		Model:      "gpt-4",
		Enabled:    true,
	}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}

	models := NewModels(db)

	// Press 'd' to enter delete confirm
	delMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	m, _ := models.Update(delMsg)
	mod := m.(*Models)

	if mod.mode != "confirm" {
		t.Errorf("expected mode 'confirm' after 'd', got %q", mod.mode)
	}
	if !mod.IsFormMode() {
		t.Error("expected IsFormMode() true in confirm mode")
	}
}

func TestModelsUpdateAddNoProviders(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	models := NewModels(db)

	// Press 'a' with no providers
	addMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	m, _ := models.Update(addMsg)
	mod := m.(*Models)

	if mod.message != "No providers configured. Add a provider first." {
		t.Errorf("expected error message, got %q", mod.message)
	}
	if mod.messageTyp != "error" {
		t.Errorf("expected messageTyp 'error', got %q", mod.messageTyp)
	}
	if mod.mode != "normal" {
		t.Errorf("expected mode 'normal', got %q", mod.mode)
	}
}

func TestModelsUpdateAddWithProviders(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create provider
	provider := &model.Provider{
		Name:    "OpenAI",
		BaseURL: "https://api.openai.com",
		APIKey:  "sk-test",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	models := NewModels(db)

	// Press 'a' with providers
	addMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	m, _ := models.Update(addMsg)
	mod := m.(*Models)

	if mod.mode != "form" {
		t.Errorf("expected mode 'form' after 'a', got %q", mod.mode)
	}
	if !mod.IsFormMode() {
		t.Error("expected IsFormMode() true in form mode")
	}
}

// modelsContains is a helper to check if substr is in s.
func modelsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
