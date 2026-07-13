package tui

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui/pages"
	"llm-api-uptime/internal/web"
)

type page int

const (
	pageHome page = iota
	pageProviders
	pageModels
	pageStats
	pageSettings
)

type Page interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
	SetSize(width, height int)
	IsFormMode() bool
}

type App struct {
	store         *store.Store
	engine        *probe.Engine
	config        *config.Config
	logger        *slog.Logger
	webServer     *web.Server
	currentPage   page
	pages         map[page]Page
	width         int
	height        int
	quitting      bool
	program       *tea.Program
	stopRequested bool
	mu            sync.Mutex
}

func NewApp(store *store.Store, engine *probe.Engine, config *config.Config, logger *slog.Logger, webServer *web.Server) *App {
	app := &App{
		store:     store,
		engine:    engine,
		config:    config,
		logger:    logger,
		webServer: webServer,
		pages:     make(map[page]Page),
	}

	app.pages[pageHome] = pages.NewHome(store, engine, config, webServer)
	app.pages[pageProviders] = pages.NewProviders(store)
	app.pages[pageModels] = pages.NewModels(store)
	app.pages[pageStats] = pages.NewStats(store)
	app.pages[pageSettings] = pages.NewSettings(config)

	return app
}

func (a *App) Run() error {
	a.mu.Lock()
	if a.stopRequested {
		a.mu.Unlock()
		return nil
	}
	p := tea.NewProgram(a, tea.WithAltScreen())
	a.program = p
	a.mu.Unlock()
	_, err := p.Run()
	a.mu.Lock()
	a.program = nil
	a.mu.Unlock()
	return err
}

// Stop asks a running TUI program to exit. It is safe to call externally.
func (a *App) Stop() {
	a.mu.Lock()
	a.stopRequested = true
	p := a.program
	a.mu.Unlock()
	if p != nil {
		p.Quit()
	}
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		for _, p := range a.pages {
			p.SetSize(msg.Width-25, msg.Height-2)
		}
		return a, nil

	case tea.KeyMsg:
		// Only handle page navigation when not in form mode
		currentPage := a.pages[a.currentPage]
		if !currentPage.IsFormMode() {
			switch msg.String() {
			case "ctrl+c":
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
			}
		} else {
			// In form mode, only handle ctrl+c
			switch msg.String() {
			case "ctrl+c":
				a.quitting = true
				return a, tea.Quit
			}
		}
	}

	currentPage := a.pages[a.currentPage]
	m, cmd := currentPage.Update(msg)
	a.pages[a.currentPage] = m.(Page)

	return a, cmd
}

func (a *App) View() string {
	if a.quitting {
		return "Goodbye!\n"
	}

	menu := a.renderMenu()
	content := a.pages[a.currentPage].View()

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
		style := lipgloss.NewStyle().Padding(0, 2)
		if item.page == a.currentPage {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#7C3AED")).
				Padding(0, 2)
		}
		s += style.Render(fmt.Sprintf("[%s] %s", item.key, item.name)) + "\n"
	}

	s += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true).
		Render("ctrl+c: quit")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#374151")).
		Padding(1, 2).
		Width(20).
		Render(s)
}
