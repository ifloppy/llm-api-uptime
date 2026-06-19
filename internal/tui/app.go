package tui

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

type page int

const (
	pageHome page = iota
	pageProviders
	pageModels
	pageStats
	pageSettings
)

type App struct {
	store       *store.Store
	engine      *probe.Engine
	config      *config.Config
	logger      *slog.Logger
	currentPage page
	width       int
	height      int
	quitting    bool
}

func NewApp(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger) *App {
	return &App{
		store:  store,
		engine: engine,
		config: config,
		logger: logger,
	}
}

func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			a.quitting = true
			return a, tea.Quit
		case "1":
			a.currentPage = pageHome
			return a, nil
		case "2":
			a.currentPage = pageProviders
			return a, nil
		case "3":
			a.currentPage = pageModels
			return a, nil
		case "4":
			a.currentPage = pageStats
			return a, nil
		case "5":
			a.currentPage = pageSettings
			return a, nil
		case "t":
			if a.currentPage == pageHome {
				a.engine.TriggerOnce()
				return a, nil
			}
		}
	}

	return a, nil
}

func (a *App) View() string {
	if a.quitting {
		return "Goodbye!\n"
	}

	menu := a.renderMenu()
	content := a.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, menu, content)
}

func (a *App) renderMenu() string {
	items := []struct {
		key  string
		name string
		page page
	}{
		{"1", "Home", pageHome},
		{"2", "Providers", pageProviders},
		{"3", "Models", pageModels},
		{"4", "Stats", pageStats},
		{"5", "Settings", pageSettings},
	}

	var s string
	for _, item := range items {
		style := MenuItemStyle
		if item.page == a.currentPage {
			style = MenuActiveStyle
		}
		s += style.Render(fmt.Sprintf("[%s] %s", item.key, item.name)) + "\n"
	}

	s += "\n" + HelpStyle.Render("q: quit")
	if a.currentPage == pageHome {
		s += "\n" + HelpStyle.Render("t: trigger probe")
	}

	return BoxStyle.Width(20).Render(s)
}

func (a *App) renderContent() string {
	var content string
	width := a.width - 25

	switch a.currentPage {
	case pageHome:
		content = a.renderHome(width)
	case pageProviders:
		content = a.renderProviders(width)
	case pageModels:
		content = a.renderModels(width)
	case pageStats:
		content = a.renderStats(width)
	case pageSettings:
		content = a.renderSettings(width)
	}

	return content
}

func (a *App) renderHome(width int) string {
	title := TitleStyle.Width(width).Render("Dashboard")
	
	status := "Stopped"
	if a.engine.IsRunning() {
		status = "Running"
	}

	statusStyle := ErrorStyle
	if a.engine.IsRunning() {
		statusStyle = SuccessStyle
	}

	info := fmt.Sprintf(
		"Engine Status: %s\nProbe Interval: %s\nDB Path: %s",
		statusStyle.Render(status),
		a.config.ProbeInterval,
		a.config.DBPath,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, BoxStyle.Width(width).Render(info))
}

func (a *App) renderProviders(width int) string {
	title := TitleStyle.Width(width).Render("Providers")
	
	providers, err := a.store.ListProviders()
	if err != nil {
		return lipgloss.JoinVertical(lipgloss.Left, title, ErrorStyle.Render("Error: "+err.Error()))
	}

	if len(providers) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No providers configured"))
	}

	header := TableHeaderStyle.Width(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			TableCellStyle.Width(5).Render("ID"),
			TableCellStyle.Width(20).Render("Name"),
			TableCellStyle.Width(30).Render("Base URL"),
			TableCellStyle.Width(10).Render("Type"),
			TableCellStyle.Width(8).Render("Enabled"),
		),
	)

	rows := []string{header}
	for _, p := range providers {
		enabled := "No"
		if p.Enabled {
			enabled = SuccessStyle.Render("Yes")
		}
		row := TableCellStyle.Width(width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(5).Render(fmt.Sprintf("%d", p.ID)),
				TableCellStyle.Width(20).Render(p.Name),
				TableCellStyle.Width(30).Render(p.BaseURL),
				TableCellStyle.Width(10).Render(string(p.APIType)),
				TableCellStyle.Width(8).Render(enabled),
			),
		)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (a *App) renderModels(width int) string {
	title := TitleStyle.Width(width).Render("Models")
	
	probes, err := a.store.GetEnabledProbes()
	if err != nil {
		return lipgloss.JoinVertical(lipgloss.Left, title, ErrorStyle.Render("Error: "+err.Error()))
	}

	if len(probes) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No models configured"))
	}

	header := TableHeaderStyle.Width(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			TableCellStyle.Width(20).Render("Provider"),
			TableCellStyle.Width(30).Render("Model"),
			TableCellStyle.Width(8).Render("Enabled"),
		),
	)

	rows := []string{header}
	for _, p := range probes {
		enabled := "No"
		if p.Enabled {
			enabled = SuccessStyle.Render("Yes")
		}
		row := TableCellStyle.Width(width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(20).Render(p.ProviderName),
				TableCellStyle.Width(30).Render(p.Model),
				TableCellStyle.Width(8).Render(enabled),
			),
		)
		rows = append(rows, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (a *App) renderStats(width int) string {
	title := TitleStyle.Width(width).Render("Statistics")
	
	stats, err := a.store.GetStats(struct{ Hours, Days int }{Hours: 24})
	if err != nil {
		return lipgloss.JoinVertical(lipgloss.Left, title, ErrorStyle.Render("Error: "+err.Error()))
	}

	if len(stats) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, MutedStyle.Render("No statistics available"))
	}

	header := TableHeaderStyle.Width(width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left,
			TableCellStyle.Width(20).Render("Provider"),
			TableCellStyle.Width(25).Render("Model"),
			TableCellStyle.Width(10).Render("Probes"),
			TableCellStyle.Width(10).Render("Success%"),
			TableCellStyle.Width(12).Render("Avg Latency"),
		),
	)

	rows := []string{header}
	for _, ps := range stats {
		for _, ms := range ps.Models {
			rateStyle := SuccessStyle
			if ms.SuccessRate < 99 {
				rateStyle = WarningStyle
			}
			if ms.SuccessRate < 95 {
				rateStyle = ErrorStyle
			}

			row := TableCellStyle.Width(width).Render(
				lipgloss.JoinHorizontal(lipgloss.Left,
					TableCellStyle.Width(20).Render(ms.ProviderName),
					TableCellStyle.Width(25).Render(ms.Model),
					TableCellStyle.Width(10).Render(fmt.Sprintf("%d", ms.TotalProbes)),
					TableCellStyle.Width(10).Render(rateStyle.Render(fmt.Sprintf("%.1f%%", ms.SuccessRate))),
					TableCellStyle.Width(12).Render(fmt.Sprintf("%.0fms", ms.AvgLatencyMs)),
				),
			)
			rows = append(rows, row)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (a *App) renderSettings(width int) string {
	title := TitleStyle.Width(width).Render("Settings")

	settings := fmt.Sprintf(
		"Probe Interval:    %s\n"+
		"Probe Timeout:     %s\n"+
		"Probe Concurrency: %d\n"+
		"DB Path:           %s\n"+
		"Data Retention:    %s\n"+
		"Web Enabled:       %v\n"+
		"Web Port:          %d\n"+
		"Web Public:        %v\n"+
		"Web Password:      %s\n"+
		"Log Level:         %s",
		a.config.ProbeInterval,
		a.config.ProbeTimeout,
		a.config.ProbeConcurrency,
		a.config.DBPath,
		a.config.DataRetention,
		a.config.WebEnabled,
		a.config.WebPort,
		a.config.WebPublic,
		maskPassword(a.config.WebPassword),
		a.config.LogLevel,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, BoxStyle.Width(width).Render(settings))
}

func maskPassword(p string) string {
	if p == "" {
		return "(not set)"
	}
	return "****"
}
