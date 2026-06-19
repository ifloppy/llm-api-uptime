package pages

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui/components"
)

type Providers struct {
	store      *store.Store
	providers  []model.Provider
	cursor     int
	width      int
	height     int
	mode       string
	form       *components.Form
	confirm    *components.Confirm
	editing    *model.Provider
	message    string
	messageTyp string
}

func NewProviders(store *store.Store) *Providers {
	p := &Providers{
		store: store,
		mode:  "normal",
	}
	p.loadProviders()
	return p
}

func (p *Providers) loadProviders() {
	providers, err := p.store.ListProviders()
	if err != nil {
		p.providers = []model.Provider{}
		return
	}
	p.providers = providers
	if p.cursor >= len(p.providers) {
		p.cursor = len(p.providers) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *Providers) Init() tea.Cmd {
	return nil
}

func (p *Providers) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch p.mode {
	case "form":
		return p.updateForm(msg)
	case "confirm":
		return p.updateConfirm(msg)
	default:
		return p.updateNormal(msg)
	}
}

func (p *Providers) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case "down", "j":
			if p.cursor < len(p.providers)-1 {
				p.cursor++
			}
		case "a":
			p.showAddForm()
			return p, nil
		case "e":
			if len(p.providers) > 0 {
				p.showEditForm()
			}
			return p, nil
		case "d":
			if len(p.providers) > 0 {
				p.showDeleteConfirm()
			}
			return p, nil
		}
	}
	return p, nil
}

func (p *Providers) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := p.form.Update(msg)

	switch msg := msg.(type) {
	case components.FormSubmitMsg:
		return p.handleFormSubmit(msg)
	case components.FormCancelMsg:
		p.mode = "normal"
		p.form = nil
		return p, nil
	}

	p.form = m.(*components.Form)
	return p, cmd
}

func (p *Providers) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := p.confirm.Update(msg)

	switch msg := msg.(type) {
	case components.ConfirmMsg:
		if msg.Confirmed {
			p.handleDelete()
		}
		p.mode = "normal"
		p.confirm = nil
		return p, nil
	}

	p.confirm = m.(*components.Confirm)
	return p, cmd
}

func (p *Providers) showAddForm() {
	p.form = components.NewForm("Add Provider", []components.FormField{
		{Label: "Name", Placeholder: "My Provider"},
		{Label: "Base URL", Placeholder: "https://api.example.com"},
		{Label: "API Key", Placeholder: "sk-...", IsPassword: true},
		{Label: "API Type", IsSelect: true, Options: []string{"openai", "anthropic"}},
	})
	p.mode = "form"
	p.editing = nil
}

func (p *Providers) showEditForm() {
	provider := p.providers[p.cursor]
	p.form = components.NewForm("Edit Provider", []components.FormField{
		{Label: "Name", Placeholder: "My Provider", Value: provider.Name},
		{Label: "Base URL", Placeholder: "https://api.example.com", Value: provider.BaseURL},
		{Label: "API Key", Placeholder: "sk-...", Value: provider.APIKey, IsPassword: true},
		{Label: "API Type", IsSelect: true, Options: []string{"openai", "anthropic"}},
	})
	p.mode = "form"
	p.editing = &provider
}

func (p *Providers) showDeleteConfirm() {
	provider := p.providers[p.cursor]
	p.confirm = components.NewConfirm(fmt.Sprintf("Delete provider '%s'?", provider.Name))
	p.mode = "confirm"
}

func (p *Providers) handleFormSubmit(msg components.FormSubmitMsg) (tea.Model, tea.Cmd) {
	name := msg.Values["Name"]
	baseURL := msg.Values["Base URL"]
	apiKey := msg.Values["API Key"]
	apiType := msg.Values["API Type"]

	if name == "" || baseURL == "" || apiKey == "" {
		p.message = "All fields are required"
		p.messageTyp = "error"
		p.mode = "normal"
		p.form = nil
		return p, nil
	}

	if p.editing != nil {
		p.editing.Name = name
		p.editing.BaseURL = baseURL
		p.editing.APIKey = apiKey
		p.editing.APIType = model.APIType(apiType)
		if err := p.store.UpdateProvider(p.editing); err != nil {
			p.message = "Error: " + err.Error()
			p.messageTyp = "error"
		} else {
			p.message = "Provider updated!"
			p.messageTyp = "success"
		}
	} else {
		provider := &model.Provider{
			Name:    name,
			BaseURL: baseURL,
			APIKey:  apiKey,
			APIType: model.APIType(apiType),
			Enabled: true,
		}
		if err := p.store.CreateProvider(provider); err != nil {
			p.message = "Error: " + err.Error()
			p.messageTyp = "error"
		} else {
			p.message = "Provider added!"
			p.messageTyp = "success"
		}
	}

	p.loadProviders()
	p.mode = "normal"
	p.form = nil
	p.editing = nil
	return p, nil
}

func (p *Providers) handleDelete() {
	if p.cursor < len(p.providers) {
		provider := p.providers[p.cursor]
		if err := p.store.DeleteProvider(provider.ID); err != nil {
			p.message = "Error: " + err.Error()
			p.messageTyp = "error"
		} else {
			p.message = "Provider deleted!"
			p.messageTyp = "success"
		}
		p.loadProviders()
	}
}

func (p *Providers) View() string {
	if p.mode == "form" {
		return p.form.View()
	}
	if p.mode == "confirm" {
		return p.confirm.View()
	}

	title := TitleStyle.Width(p.width).Render("Providers")

	if len(p.providers) == 0 {
		content := MutedStyle.Render("No providers configured")
		help := HelpStyle.Render("a: add provider")
		return lipgloss.JoinVertical(lipgloss.Left, title, content, "\n"+help)
	}

	rows := []string{
		TableHeaderStyle.Width(p.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(4).Render("#"),
				TableCellStyle.Width(20).Render("Name"),
				TableCellStyle.Width(30).Render("Base URL"),
				TableCellStyle.Width(10).Render("Type"),
				TableCellStyle.Width(8).Render("Enabled"),
			),
		),
	}

	for i, provider := range p.providers {
		enabled := "No"
		if provider.Enabled {
			enabled = SuccessStyle.Render("Yes")
		}

		style := TableCellStyle
		if i == p.cursor {
			style = MenuActiveStyle
		}

		row := style.Width(p.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(4).Render(fmt.Sprintf("%d", i+1)),
				TableCellStyle.Width(20).Render(provider.Name),
				TableCellStyle.Width(30).Render(provider.BaseURL),
				TableCellStyle.Width(10).Render(string(provider.APIType)),
				TableCellStyle.Width(8).Render(enabled),
			),
		)
		rows = append(rows, row)
	}

	if p.message != "" {
		msgStyle := SuccessStyle
		if p.messageTyp == "error" {
			msgStyle = ErrorStyle
		}
		rows = append(rows, "\n"+msgStyle.Render(p.message))
		p.message = ""
	}

	help := HelpStyle.Render("↑/↓: navigate • a: add • e: edit • d: delete")
	rows = append(rows, "\n"+help)

	return lipgloss.JoinVertical(lipgloss.Left, title, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (p *Providers) SetSize(width, height int) {
	p.width = width
	p.height = height
}
