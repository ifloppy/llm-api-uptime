package pages

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
)

type Home struct {
	store   *store.Store
	engine  *probe.Engine
	config  *config.Config
	width   int
	height  int
	message string
}

func NewHome(store *store.Store, engine *probe.Engine, config *config.Config) *Home {
	return &Home{
		store:  store,
		engine: engine,
		config: config,
	}
}

func (h *Home) Init() tea.Cmd {
	return nil
}

func (h *Home) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			h.engine.TriggerOnce()
			h.message = "Probe triggered!"
			return h, nil
		}
	}
	return h, nil
}

func (h *Home) View() string {
	title := TitleStyle.Width(h.width).Render("Dashboard")

	status := "Stopped"
	if h.engine.IsRunning() {
		status = "Running"
	}

	statusStyle := ErrorStyle
	if h.engine.IsRunning() {
		statusStyle = SuccessStyle
	}

	info := fmt.Sprintf(
		"Engine Status: %s\nProbe Interval: %s\nDB Path: %s",
		statusStyle.Render(status),
		h.config.ProbeInterval,
		h.config.DBPath,
	)

	if h.message != "" {
		info += "\n\n" + SuccessStyle.Render(h.message)
		h.message = ""
	}

	help := HelpStyle.Render("t: trigger probe")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		BoxStyle.Width(h.width).Render(info),
		"\n"+help,
	)
}

func (h *Home) SetSize(width, height int) {
	h.width = width
	h.height = height
}
