package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewForm(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
		{Label: "Email", Placeholder: "Enter email"},
	}

	form := NewForm("Test Form", fields)
	if form == nil {
		t.Fatal("expected non-nil form")
	}
}

func TestNewForm_EmptyFields(t *testing.T) {
	form := NewForm("Empty", []FormField{})
	if form == nil {
		t.Fatal("expected non-nil form even with empty fields")
	}
}

func TestForm_Init(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
	}
	form := NewForm("Test", fields)

	cmd := form.Init()
	if cmd == nil {
		t.Error("expected non-nil Init command (textinput.Blink)")
	}
}

func TestForm_View(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
		{Label: "Email", Placeholder: "Enter email"},
	}
	form := NewForm("Test Form", fields)

	view := form.View()
	if view == "" {
		t.Error("expected non-empty View output")
	}
}

func TestForm_SetWidth(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
	}
	form := NewForm("Test", fields)

	// Should not panic
	form.SetWidth(80)
	form.SetWidth(0)
}

func TestFormFieldTypes_Password(t *testing.T) {
	fields := []FormField{
		{Label: "Password", Placeholder: "Enter password", IsPassword: true},
	}
	form := NewForm("Test", fields)
	if form == nil {
		t.Fatal("expected non-nil form with password field")
	}

	view := form.View()
	if view == "" {
		t.Error("expected non-empty View for password field")
	}
}

func TestFormFieldTypes_Select(t *testing.T) {
	fields := []FormField{
		{Label: "Type", IsSelect: true, Options: []string{"openai", "anthropic"}},
	}
	form := NewForm("Test", fields)
	if form == nil {
		t.Fatal("expected non-nil form with select field")
	}

	view := form.View()
	if view == "" {
		t.Error("expected non-empty View for select field")
	}
}

func TestFormFieldTypes_Mixed(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
		{Label: "Password", Placeholder: "Enter password", IsPassword: true},
		{Label: "Type", IsSelect: true, Options: []string{"openai", "anthropic"}},
	}
	form := NewForm("Mixed", fields)
	if form == nil {
		t.Fatal("expected non-nil form with mixed fields")
	}
}

func TestFormField_WithValue(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name", Value: "prefilled"},
	}
	form := NewForm("Test", fields)
	if form == nil {
		t.Fatal("expected non-nil form with pre-filled value")
	}

	view := form.View()
	if view == "" {
		t.Error("expected non-empty View with pre-filled value")
	}
}

func TestForm_Submit(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
	}
	form := NewForm("Test", fields)

	// Simulate pressing enter
	result, cmd := form.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from submit")
	}

	// Execute the command to get the message
	msg := cmd()
	submitMsg, ok := msg.(FormSubmitMsg)
	if !ok {
		t.Fatalf("expected FormSubmitMsg, got %T", msg)
	}
	if _, exists := submitMsg.Values["Name"]; !exists {
		t.Error("expected 'Name' key in submitted values")
	}
}

func TestForm_Cancel_Esc(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
	}
	form := NewForm("Test", fields)

	result, cmd := form.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from cancel")
	}

	msg := cmd()
	if _, ok := msg.(FormCancelMsg); !ok {
		t.Fatalf("expected FormCancelMsg, got %T", msg)
	}
}

func TestForm_Cancel_CtrlC(t *testing.T) {
	fields := []FormField{
		{Label: "Name", Placeholder: "Enter name"},
	}
	form := NewForm("Test", fields)

	result, cmd := form.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from ctrl+c cancel")
	}

	msg := cmd()
	if _, ok := msg.(FormCancelMsg); !ok {
		t.Fatalf("expected FormCancelMsg, got %T", msg)
	}
}

func TestForm_Navigation_Tab(t *testing.T) {
	fields := []FormField{
		{Label: "First", Placeholder: "first"},
		{Label: "Second", Placeholder: "second"},
		{Label: "Third", Placeholder: "third"},
	}
	form := NewForm("Test", fields)

	// Tab through fields — should not panic
	for i := 0; i < len(fields)+2; i++ {
		result, _ := form.Update(tea.KeyMsg{Type: tea.KeyTab})
		if result == nil {
			t.Fatalf("expected non-nil result at tab %d", i)
		}
	}
}

func TestForm_Navigation_Arrows(t *testing.T) {
	fields := []FormField{
		{Label: "First", Placeholder: "first"},
		{Label: "Second", Placeholder: "second"},
	}
	form := NewForm("Test", fields)

	// Down arrow
	result, _ := form.Update(tea.KeyMsg{Type: tea.KeyDown})
	if result == nil {
		t.Error("expected non-nil result from down arrow")
	}

	// Up arrow
	result, _ = form.Update(tea.KeyMsg{Type: tea.KeyUp})
	if result == nil {
		t.Error("expected non-nil result from up arrow")
	}
}
