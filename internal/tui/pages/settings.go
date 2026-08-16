package pages

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/config"
)

type Settings struct {
	config *config.Config
	width  int
	height int
}

func NewSettings(config *config.Config) *Settings {
	return &Settings{
		config: config,
	}
}

func (s *Settings) Init() tea.Cmd {
	return nil
}

func (s *Settings) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

func (s *Settings) View() string {
	title := TitleStyle.Width(s.width).Render("Settings")

	settings := fmt.Sprintf(
		"Probe Interval:    %s\n"+
			"Probe Timeout:     %s\n"+
			"Probe Retries:     %d\n"+
			"Probe Concurrency: %d\n"+
			"DB Path:           %s\n"+
			"Data Retention:    %s\n"+
			"Web Enabled:       %v\n"+
			"Web Port:          %d\n"+
			"Web Public:        %v\n"+
			"Web Password:      %s\n"+
			"Log Level:         %s",
		s.config.ProbeInterval,
		s.config.ProbeTimeout,
		s.config.ProbeRetries,
		s.config.ProbeConcurrency,
		s.config.DBPath,
		s.config.DataRetention,
		s.config.WebEnabled,
		s.config.WebPort,
		s.config.WebPublic,
		maskPassword(s.config.WebPassword),
		s.config.LogLevel,
	)

	help := HelpStyle.Render("Settings are read-only. Use environment variables to change.")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		BoxStyle.Width(s.width).Render(settings),
		"\n"+help,
	)
}

func (s *Settings) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Settings) IsFormMode() bool {
	return false
}

func maskPassword(p string) string {
	if p == "" {
		return "(not set)"
	}
	return "****"
}
