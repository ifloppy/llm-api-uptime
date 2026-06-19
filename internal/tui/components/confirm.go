package components

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmMsg struct {
	Confirmed bool
}

type Confirm struct {
	message   string
	confirmed bool
	width     int
}

func NewConfirm(message string) *Confirm {
	return &Confirm{
		message: message,
		width:   50,
	}
}

func (c *Confirm) Init() tea.Cmd {
	return nil
}

func (c *Confirm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			return c, func() tea.Msg { return ConfirmMsg{Confirmed: true} }
		case "n", "N", "esc", "ctrl+c":
			return c, func() tea.Msg { return ConfirmMsg{Confirmed: false} }
		}
	}
	return c, nil
}

func (c *Confirm) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(c.width)

	msgStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	content := msgStyle.Render(c.message) + "\n\n"
	content += helpStyle.Render("y: confirm • n: cancel")

	return style.Render(content)
}

func (c *Confirm) SetWidth(width int) {
	c.width = width
}
