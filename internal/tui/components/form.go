package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FormField struct {
	Label       string
	Placeholder string
	Value       string
	IsPassword  bool
	IsSelect    bool
	Options     []string
}

type FormSubmitMsg struct {
	Values map[string]string
}

type FormCancelMsg struct{}

type Form struct {
	fields      []FormField
	inputs      []textinput.Model
	selectIdx   int
	focusIdx    int
	title       string
	width       int
	focused     lipgloss.Style
	blurred     lipgloss.Style
	cursorStyle lipgloss.Style
}

func NewForm(title string, fields []FormField) *Form {
	m := &Form{
		fields:    fields,
		inputs:    make([]textinput.Model, len(fields)),
		title:     title,
		width:     60,
		focused:   lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		blurred:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		cursorStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
	}

	for i, field := range fields {
		t := textinput.New()
		t.Placeholder = field.Placeholder
		t.CharLimit = 256
		t.Width = 40

		if field.IsPassword {
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}

		if field.Value != "" {
			t.SetValue(field.Value)
		}

		if i == 0 {
			t.Focus()
			t.PromptStyle = m.focused
			t.TextStyle = m.focused
		} else {
			t.Blur()
			t.PromptStyle = m.blurred
			t.TextStyle = m.blurred
		}

		m.inputs[i] = t
	}

	return m
}

func (m *Form) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Form) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, func() tea.Msg { return FormCancelMsg{} }

		case "tab", "shift+tab", "up", "down":
			s := msg.String()

			if s == "up" || s == "shift+tab" {
				m.focusIdx--
			} else {
				m.focusIdx++
			}

			if m.focusIdx > len(m.fields)-1 {
				m.focusIdx = 0
			} else if m.focusIdx < 0 {
				m.focusIdx = len(m.fields) - 1
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIdx {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = m.focused
					m.inputs[i].TextStyle = m.focused
					continue
				}
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = m.blurred
				m.inputs[i].TextStyle = m.blurred
			}

			return m, tea.Batch(cmds...)

		case "enter":
			values := make(map[string]string)
			for i, field := range m.fields {
				if field.IsSelect && len(field.Options) > 0 {
					values[field.Label] = field.Options[m.selectIdx]
				} else {
					values[field.Label] = m.inputs[i].Value()
				}
			}
			return m, func() tea.Msg { return FormSubmitMsg{Values: values} }

		case "left", "right":
			if m.fields[m.focusIdx].IsSelect {
				if msg.String() == "left" {
					m.selectIdx--
				} else {
					m.selectIdx++
				}
				if m.selectIdx < 0 {
					m.selectIdx = len(m.fields[m.focusIdx].Options) - 1
				} else if m.selectIdx >= len(m.fields[m.focusIdx].Options) {
					m.selectIdx = 0
				}
			}
		}
	}

	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *Form) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *Form) View() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(m.title))
	b.WriteString("\n\n")

	for i, field := range m.fields {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(field.Label + ":"))
		b.WriteString(" ")

		if field.IsSelect && len(field.Options) > 0 {
			for j, opt := range field.Options {
				if j == m.selectIdx {
					b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("["+opt+"]"))
				} else {
					b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(opt))
				}
				if j < len(field.Options)-1 {
					b.WriteString(" ")
				}
			}
		} else {
			b.WriteString(m.inputs[i].View())
		}

		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("tab: next field • enter: submit • esc: cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Render(b.String())
}

func (m *Form) SetWidth(width int) {
	m.width = width
}
