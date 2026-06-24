package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewConfirm(t *testing.T) {
	confirm := NewConfirm("Are you sure?")
	if confirm == nil {
		t.Fatal("expected non-nil confirm")
	}
}

func TestNewConfirm_EmptyMessage(t *testing.T) {
	confirm := NewConfirm("")
	if confirm == nil {
		t.Fatal("expected non-nil confirm even with empty message")
	}
}

func TestConfirm_Init(t *testing.T) {
	confirm := NewConfirm("Are you sure?")
	cmd := confirm.Init()
	if cmd != nil {
		t.Error("expected nil Init command for Confirm")
	}
}

func TestConfirm_View(t *testing.T) {
	confirm := NewConfirm("Are you sure?")
	view := confirm.View()
	if view == "" {
		t.Error("expected non-empty View output")
	}
}

func TestConfirm_View_ContainsMessage(t *testing.T) {
	msg := "Delete all data?"
	confirm := NewConfirm(msg)
	view := confirm.View()

	if view == "" {
		t.Fatal("expected non-empty view")
	}
	// The view should contain the message text
	if len(view) < len(msg) {
		t.Error("view should be at least as long as the message")
	}
}

func TestConfirm_SetWidth(t *testing.T) {
	confirm := NewConfirm("Test")

	// Should not panic
	confirm.SetWidth(80)
	confirm.SetWidth(0)
	confirm.SetWidth(200)
}

func TestConfirm_ConfirmYes(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from 'y' press")
	}

	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if !confirmMsg.Confirmed {
		t.Error("expected Confirmed=true for 'y' key")
	}
}

func TestConfirm_ConfirmYes_Upper(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from 'Y' press")
	}

	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if !confirmMsg.Confirmed {
		t.Error("expected Confirmed=true for 'Y' key")
	}
}

func TestConfirm_CancelNo(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from 'n' press")
	}

	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if confirmMsg.Confirmed {
		t.Error("expected Confirmed=false for 'n' key")
	}
}

func TestConfirm_CancelEsc(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from esc press")
	}

	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if confirmMsg.Confirmed {
		t.Error("expected Confirmed=false for esc key")
	}
}

func TestConfirm_CancelCtrlC(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd == nil {
		t.Fatal("expected non-nil command from ctrl+c press")
	}

	msg := cmd()
	confirmMsg, ok := msg.(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", msg)
	}
	if confirmMsg.Confirmed {
		t.Error("expected Confirmed=false for ctrl+c key")
	}
}

func TestConfirm_UnrelatedKey(t *testing.T) {
	confirm := NewConfirm("Are you sure?")

	// Pressing an unrelated key should not produce a command
	result, cmd := confirm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if result == nil {
		t.Fatal("expected non-nil result from Update")
	}
	if cmd != nil {
		t.Error("expected nil command for unrelated key 'x'")
	}
}
