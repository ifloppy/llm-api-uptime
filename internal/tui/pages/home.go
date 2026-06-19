package pages

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/config"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/web"
)

type Home struct {
	store      *store.Store
	engine     *probe.Engine
	config     *config.Config
	webServer  *web.Server
	width      int
	height     int
	message    string
	messageTyp string
}

func NewHome(store *store.Store, engine *probe.Engine, config *config.Config, webServer *web.Server) *Home {
	return &Home{
		store:     store,
		engine:    engine,
		config:    config,
		webServer: webServer,
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
			h.messageTyp = "success"
			return h, nil
		case "w":
			if h.webServer != nil {
				if h.webServer.IsRunning() {
					h.webServer.Stop()
					h.message = "Web server stopped"
					h.messageTyp = "success"
				} else {
					if err := h.webServer.Start(); err != nil {
						h.message = "Failed to start web server: " + err.Error()
						h.messageTyp = "error"
					} else {
						h.message = "Web server started on " + h.webServer.Addr()
						h.messageTyp = "success"
					}
				}
			} else {
				h.message = "Web server not configured (WEB_ENABLED=true in .env)"
				h.messageTyp = "error"
			}
			return h, nil
		}
	}
	return h, nil
}

func (h *Home) View() string {
	title := TitleStyle.Width(h.width).Render("Dashboard")

	engineStatus := "Stopped"
	if h.engine.IsRunning() {
		engineStatus = "Running"
	}

	engineStatusStyle := ErrorStyle
	if h.engine.IsRunning() {
		engineStatusStyle = SuccessStyle
	}

	info := fmt.Sprintf(
		"Engine Status: %s\nProbe Interval: %s\nDB Path: %s",
		engineStatusStyle.Render(engineStatus),
		h.config.ProbeInterval,
		h.config.DBPath,
	)

	webInfo := ""
	if h.webServer != nil {
		webStatus := "Stopped"
		webStatusStyle := ErrorStyle
		if h.webServer.IsRunning() {
			webStatus = "Running"
			webStatusStyle = SuccessStyle
		}

		authStatus := "Disabled"
		if h.config.WebPassword != "" {
			authStatus = "Enabled"
		}

		webInfo = fmt.Sprintf(
			"\n\nWeb Server: %s\nWeb Address: %s\nWeb Auth: %s",
			webStatusStyle.Render(webStatus),
			SuccessStyle.Render(h.webServer.Addr()),
			WarningStyle.Render(authStatus),
		)
	} else if h.config.WebEnabled {
		webInfo = "\n\nWeb Server: Not initialized"
	}

	if h.message != "" {
		msgStyle := SuccessStyle
		if h.messageTyp == "error" {
			msgStyle = ErrorStyle
		}
		webInfo += "\n\n" + msgStyle.Render(h.message)
		h.message = ""
	}

	help := HelpStyle.Render("t: trigger probe • w: toggle web server")

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		BoxStyle.Width(h.width).Render(info+webInfo),
		"\n"+help,
	)
}

func (h *Home) SetSize(width, height int) {
	h.width = width
	h.height = height
}

func (h *Home) IsFormMode() bool {
	return false
}
