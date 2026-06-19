package pages

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/probe"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui/components"
)

type Models struct {
	store      *store.Store
	probes     []model.ProbeWithProvider
	providers  []model.Provider
	cursor     int
	width      int
	height     int
	mode       string
	form       *components.Form
	confirm    *components.Confirm
	message    string
	messageTyp string
}

func NewModels(store *store.Store) *Models {
	m := &Models{
		store: store,
		mode:  "normal",
	}
	m.loadData()
	return m
}

func (m *Models) loadData() {
	probes, err := m.store.GetEnabledProbes()
	if err != nil {
		m.probes = []model.ProbeWithProvider{}
	} else {
		m.probes = probes
	}

	providers, err := m.store.ListProviders()
	if err != nil {
		m.providers = []model.Provider{}
	} else {
		m.providers = providers
	}

	if m.cursor >= len(m.probes) {
		m.cursor = len(m.probes) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Models) Init() tea.Cmd {
	return nil
}

func (m *Models) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case "form":
		return m.updateForm(msg)
	case "confirm":
		return m.updateConfirm(msg)
	default:
		return m.updateNormal(msg)
	}
}

func (m *Models) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.probes)-1 {
				m.cursor++
			}
		case "a":
			m.showAddForm()
			return m, nil
		case "f":
			m.showFetchForm()
			return m, nil
		case "d":
			if len(m.probes) > 0 {
				m.showDeleteConfirm()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Models) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	fm, cmd := m.form.Update(msg)

	switch msg := msg.(type) {
	case components.FormSubmitMsg:
		return m.handleFormSubmit(msg)
	case components.FormCancelMsg:
		m.mode = "normal"
		m.form = nil
		return m, nil
	}

	m.form = fm.(*components.Form)
	return m, cmd
}

func (m *Models) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	cm, cmd := m.confirm.Update(msg)

	switch msg := msg.(type) {
	case components.ConfirmMsg:
		if msg.Confirmed {
			m.handleDelete()
		}
		m.mode = "normal"
		m.confirm = nil
		return m, nil
	}

	m.confirm = cm.(*components.Confirm)
	return m, cmd
}

func (m *Models) showAddForm() {
	if len(m.providers) == 0 {
		m.message = "No providers configured. Add a provider first."
		m.messageTyp = "error"
		return
	}

	providerNames := make([]string, len(m.providers))
	for i, p := range m.providers {
		providerNames[i] = p.Name
	}

	m.form = components.NewForm("Add Model", []components.FormField{
		{Label: "Provider", IsSelect: true, Options: providerNames},
		{Label: "Model Name", Placeholder: "gpt-4"},
	})
	m.mode = "form"
}

func (m *Models) showFetchForm() {
	if len(m.providers) == 0 {
		m.message = "No providers configured. Add a provider first."
		m.messageTyp = "error"
		return
	}

	providerNames := make([]string, len(m.providers))
	for i, p := range m.providers {
		providerNames[i] = p.Name
	}

	m.form = components.NewForm("Fetch Models from Provider", []components.FormField{
		{Label: "Provider", IsSelect: true, Options: providerNames},
	})
	m.mode = "form"
}

func (m *Models) showDeleteConfirm() {
	if m.cursor < len(m.probes) {
		p := m.probes[m.cursor]
		m.confirm = components.NewConfirm(fmt.Sprintf("Delete model '%s' from %s?", p.Model, p.ProviderName))
		m.mode = "confirm"
	}
}

func (m *Models) handleFormSubmit(msg components.FormSubmitMsg) (tea.Model, tea.Cmd) {
	providerName := msg.Values["Provider"]

	var selectedProvider *model.Provider
	for _, p := range m.providers {
		if p.Name == providerName {
			selectedProvider = &p
			break
		}
	}

	if selectedProvider == nil {
		m.message = "Provider not found"
		m.messageTyp = "error"
		m.mode = "normal"
		m.form = nil
		return m, nil
	}

	modelName := msg.Values["Model Name"]

	if modelName != "" {
		newProbe := &model.Probe{
			ProviderID: selectedProvider.ID,
			Model:      modelName,
			Enabled:    true,
		}
		if err := m.store.CreateProbe(newProbe); err != nil {
			m.message = "Error: " + err.Error()
			m.messageTyp = "error"
		} else {
			m.message = fmt.Sprintf("Model '%s' added!", modelName)
			m.messageTyp = "success"
		}
	} else {
		m.message = "Fetching models..."
		m.messageTyp = "success"
		m.mode = "normal"
		m.form = nil

		go func() {
			models, err := probe.FetchModelList(context.Background(), *selectedProvider)
			if err != nil {
				m.message = "Error fetching models: " + err.Error()
				m.messageTyp = "error"
				return
			}

			for _, modelName := range models {
				newProbe := &model.Probe{
					ProviderID: selectedProvider.ID,
					Model:      modelName,
					Enabled:    true,
				}
				m.store.CreateProbe(newProbe)
			}

			m.message = fmt.Sprintf("Fetched %d models!", len(models))
			m.messageTyp = "success"
			m.loadData()
		}()

		return m, nil
	}

	m.loadData()
	m.mode = "normal"
	m.form = nil
	return m, nil
}

func (m *Models) handleDelete() {
	if m.cursor < len(m.probes) {
		p := m.probes[m.cursor]
		if err := m.store.DeleteProbe(p.ID); err != nil {
			m.message = "Error: " + err.Error()
			m.messageTyp = "error"
		} else {
			m.message = "Model deleted!"
			m.messageTyp = "success"
		}
		m.loadData()
	}
}

func (m *Models) View() string {
	if m.mode == "form" {
		return m.form.View()
	}
	if m.mode == "confirm" {
		return m.confirm.View()
	}

	title := TitleStyle.Width(m.width).Render("Models")

	if len(m.probes) == 0 {
		content := MutedStyle.Render("No models configured")
		if m.message != "" {
			msgStyle := SuccessStyle
			if m.messageTyp == "error" {
				msgStyle = ErrorStyle
			}
			content = msgStyle.Render(m.message)
			m.message = ""
		}
		help := HelpStyle.Render("a: add model • f: fetch from provider")
		return lipgloss.JoinVertical(lipgloss.Left, title, content, help)
	}

	rows := []string{
		TableHeaderStyle.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(2).Render(""),
				TableCellStyle.Width(4).Render("#"),
				TableCellStyle.Width(20).Render("Provider"),
				TableCellStyle.Width(30).Render("Model"),
				TableCellStyle.Width(8).Render("Enabled"),
			),
		),
	}

	for i, p := range m.probes {
		enabled := "No"
		if p.Enabled {
			enabled = SuccessStyle.Render("Yes")
		}

		prefix := "  "
		style := TableCellStyle
		if i == m.cursor {
			prefix = SuccessStyle.Render("▸ ")
			style = MenuActiveStyle
		}

		row := style.Width(m.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(2).Render(prefix),
				TableCellStyle.Width(4).Render(fmt.Sprintf("%d", i+1)),
				TableCellStyle.Width(20).Render(p.ProviderName),
				TableCellStyle.Width(30).Render(p.Model),
				TableCellStyle.Width(8).Render(enabled),
			),
		)
		rows = append(rows, row)
	}

	if m.message != "" {
		msgStyle := SuccessStyle
		if m.messageTyp == "error" {
			msgStyle = ErrorStyle
		}
		rows = append(rows, msgStyle.Render(m.message))
		m.message = ""
	}

	rows = append(rows, HelpStyle.Render("↑/↓: navigate • a: add • f: fetch • d: delete"))

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (m *Models) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *Models) IsFormMode() bool {
	return m.mode == "form" || m.mode == "confirm"
}
