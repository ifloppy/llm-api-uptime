package pages

import (
	"testing"

	"github.com/charmbracelet/bubbletea"

	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui/components"
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

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	return db
}

func createTestProvider(t *testing.T, db *store.Store, name string) *model.Provider {
	t.Helper()
	provider := &model.Provider{
		Name:    name,
		BaseURL: "http://test.com",
		APIKey:  "test-key",
		APIType: model.APITypeOpenAI,
		Enabled: true,
	}
	if err := db.CreateProvider(provider); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	return provider
}

func createTestProbe(t *testing.T, db *store.Store, providerID int64, modelName string) *model.Probe {
	t.Helper()
	probe := &model.Probe{
		ProviderID: providerID,
		Model:      modelName,
		Enabled:    true,
	}
	if err := db.CreateProbe(probe); err != nil {
		t.Fatalf("failed to create probe: %v", err)
	}
	return probe
}

func TestProvidersFormSubmit(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	p := NewProviders(db)

	// Enter form mode
	p.showAddForm()
	if p.mode != "form" {
		t.Fatal("expected form mode")
	}

	// Simulate form submission
	msg := components.FormSubmitMsg{
		Values: map[string]string{
			"Name":       "NewProvider",
			"Base URL":   "http://test.com",
			"API Key":    "test-key",
			"API Type":   "openai",
			"Max Tokens": "10",
		},
	}
	result, _ := p.handleFormSubmit(msg)
	providers := result.(*Providers)

	// Verify provider was created
	if providers.mode != "normal" {
		t.Error("expected normal mode after submit")
	}
	if providers.messageTyp != "success" {
		t.Errorf("expected success message type, got %q", providers.messageTyp)
	}

	// Verify provider exists in DB
	list, _ := db.ListProviders()
	found := false
	for _, prov := range list {
		if prov.Name == "NewProvider" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected provider to be created")
	}
}

func TestProvidersFormSubmitInvalid(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	p := NewProviders(db)
	p.showAddForm()

	msg := components.FormSubmitMsg{
		Values: map[string]string{
			"Name":     "",
			"Base URL": "http://test.com",
			"API Key":  "test-key",
			"API Type": "openai",
		},
	}
	result, _ := p.handleFormSubmit(msg)
	providers := result.(*Providers)

	if providers.messageTyp != "error" {
		t.Error("expected error message for invalid input")
	}
	if providers.mode != "normal" {
		t.Errorf("expected normal mode after invalid submit, got %q", providers.mode)
	}
}

func TestProvidersFormCancel(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	p := NewProviders(db)
	p.showAddForm()

	result, _ := p.updateForm(components.FormCancelMsg{})
	providers := result.(*Providers)

	if providers.mode != "normal" {
		t.Error("expected normal mode after cancel")
	}
	if providers.form != nil {
		t.Error("expected form to be nil after cancel")
	}
}

func TestProvidersDeleteConfirm(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	provider := createTestProvider(t, db, "ToDelete")
	p := NewProviders(db)
	p.showDeleteConfirm()

	// Confirm delete
	result, _ := p.updateConfirm(components.ConfirmMsg{Confirmed: true})
	providers := result.(*Providers)

	if providers.mode != "normal" {
		t.Error("expected normal mode after confirm")
	}
	if providers.messageTyp != "success" {
		t.Errorf("expected success message type, got %q", providers.messageTyp)
	}

	// Verify provider was deleted
	_, err := db.GetProvider(provider.ID)
	if err == nil {
		t.Error("expected provider to be deleted")
	}
}

func TestProvidersEditFormSubmit(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	provider := createTestProvider(t, db, "OldName")
	p := NewProviders(db)
	p.showEditForm()

	msg := components.FormSubmitMsg{
		Values: map[string]string{
			"Name":       "UpdatedName",
			"Base URL":   "http://updated.com",
			"API Key":    "updated-key",
			"API Type":   "anthropic",
			"Max Tokens": "5",
		},
	}
	result, _ := p.handleFormSubmit(msg)
	providers := result.(*Providers)

	if providers.mode != "normal" {
		t.Error("expected normal mode after edit submit")
	}
	if providers.messageTyp != "success" {
		t.Errorf("expected success message type, got %q", providers.messageTyp)
	}

	// Verify provider was updated
	updated, err := db.GetProvider(provider.ID)
	if err != nil {
		t.Fatalf("failed to get updated provider: %v", err)
	}
	if updated.Name != "UpdatedName" {
		t.Errorf("expected name 'UpdatedName', got %q", updated.Name)
	}
	if updated.BaseURL != "http://updated.com" {
		t.Errorf("expected base URL 'http://updated.com', got %q", updated.BaseURL)
	}
}

func TestProvidersDeleteCancel(t *testing.T) {
	db := setupTestStore(t)
	defer db.Close()

	createTestProvider(t, db, "KeepMe")
	p := NewProviders(db)
	p.showDeleteConfirm()

	// Cancel delete
	result, _ := p.updateConfirm(components.ConfirmMsg{Confirmed: false})
	providers := result.(*Providers)

	if providers.mode != "normal" {
		t.Error("expected normal mode after cancel")
	}

	// Verify provider still exists
	list, _ := db.ListProviders()
	if len(list) != 1 {
		t.Errorf("expected 1 provider, got %d", len(list))
	}
}
