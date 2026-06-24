package pages

import (
	"testing"

	"github.com/charmbracelet/bubbletea"

	"llm-api-uptime/internal/model"
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

func TestProvidersViewEmpty(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)
	providers.SetSize(80, 24)

	view := providers.View()

	assertions := []struct {
		name string
		want string
	}{
		{"providers title", "Providers"},
		{"no providers message", "No providers configured"},
		{"help text", "add provider"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !testContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestProvidersViewWithData(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create test provider
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

	providers := NewProviders(db)
	providers.SetSize(120, 40)

	view := providers.View()

	assertions := []struct {
		name string
		want string
	}{
		{"providers title", "Providers"},
		{"provider name", "OpenAI"},
		{"provider url", "https://api.openai.com"},
		{"provider type", "openai"},
		{"help text", "navigate"},
	}

	for _, a := range assertions {
		t.Run(a.name, func(t *testing.T) {
			if !testContains(view, a.want) {
				t.Errorf("view missing %q\nGot: %s", a.want, view)
			}
		})
	}
}

func TestProvidersUpdateNavigation(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create two providers
	for i, name := range []string{"Alpha", "Beta"} {
		p := &model.Provider{
			Name:    name,
			BaseURL: "http://test.com",
			APIKey:  "key",
			APIType: model.APITypeOpenAI,
			Enabled: true,
		}
		if err := db.CreateProvider(p); err != nil {
			t.Fatalf("failed to create provider %d: %v", i, err)
		}
	}

	providers := NewProviders(db)
	if providers.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", providers.cursor)
	}

	// Test down navigation
	downMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	model, _ := providers.Update(downMsg)
	p := model.(*Providers)
	if p.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", p.cursor)
	}

	// Test up navigation
	upMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	model, _ = p.Update(upMsg)
	p = model.(*Providers)
	if p.cursor != 0 {
		t.Errorf("expected cursor 0 after up, got %d", p.cursor)
	}

	// Test up at boundary (should stay at 0)
	model, _ = p.Update(upMsg)
	p = model.(*Providers)
	if p.cursor != 0 {
		t.Errorf("expected cursor 0 at upper boundary, got %d", p.cursor)
	}

	// Navigate to end and test boundary
	model, _ = p.Update(downMsg)
	p = model.(*Providers)
	model, _ = p.Update(downMsg)
	p = model.(*Providers)
	if p.cursor != 1 {
		t.Errorf("expected cursor 1 at lower boundary, got %d", p.cursor)
	}
}

func TestProvidersUpdateAddForm(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	providers := NewProviders(db)

	// Press 'a' to enter add form
	addMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	model, _ := providers.Update(addMsg)
	p := model.(*Providers)

	if p.mode != "form" {
		t.Errorf("expected mode 'form' after 'a', got %q", p.mode)
	}
	if !p.IsFormMode() {
		t.Error("expected IsFormMode() true after 'a'")
	}
}

func TestProvidersUpdateDeleteConfirm(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create a provider
	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://test.com",
		APIKey:  "key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	providers := NewProviders(db)

	// Press 'd' to enter delete confirm
	delMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	model, _ := providers.Update(delMsg)
	p := model.(*Providers)

	if p.mode != "confirm" {
		t.Errorf("expected mode 'confirm' after 'd', got %q", p.mode)
	}
	if !p.IsFormMode() {
		t.Error("expected IsFormMode() true in confirm mode")
	}
}

func TestProvidersUpdateEditForm(t *testing.T) {
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer db.Close()

	// Create a provider
	provider := &model.Provider{
		Name:    "TestProvider",
		BaseURL: "http://test.com",
		APIKey:  "key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	providers := NewProviders(db)

	// Press 'e' to enter edit form
	editMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	model, _ := providers.Update(editMsg)
	p := model.(*Providers)

	if p.mode != "form" {
		t.Errorf("expected mode 'form' after 'e', got %q", p.mode)
	}
	if p.editing == nil {
		t.Error("expected editing to be set")
	}
}

// testContains is a helper to check if substr is in s.
func testContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
