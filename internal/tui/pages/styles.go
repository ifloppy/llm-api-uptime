package pages

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 2).
			MarginBottom(1)

	MenuActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 1)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(1, 2)

	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#4B5563")).
				Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)
)
