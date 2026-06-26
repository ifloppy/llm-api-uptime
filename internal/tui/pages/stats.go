package pages

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"llm-api-uptime/internal/model"
	"llm-api-uptime/internal/stats"
	"llm-api-uptime/internal/store"
	"llm-api-uptime/internal/tui/components"
)

type Stats struct {
	store      *store.Store
	data       []model.ProviderStats
	cursor     int
	width      int
	height     int
	timeRange  int
	mode       string
	confirm    *components.Confirm
	message    string
	messageTyp string
}

func NewStats(store *store.Store) *Stats {
	s := &Stats{
		store:     store,
		timeRange: 24,
		mode:      "normal",
	}
	s.loadData()
	return s
}

func (s *Stats) loadData() {
	query := model.StatsQuery{Hours: s.timeRange}
	data, err := s.store.GetStats(query)
	if err != nil {
		s.data = []model.ProviderStats{}
		return
	}
	s.data = data

	totalModels := 0
	for _, ps := range s.data {
		totalModels += len(ps.Models)
	}
	if s.cursor >= totalModels {
		s.cursor = totalModels - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
}

func (s *Stats) Init() tea.Cmd {
	return nil
}

func (s *Stats) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if s.mode == "confirm" {
		return s.updateConfirm(msg)
	}
	return s.updateNormal(msg)
}

func (s *Stats) updateNormal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h":
			s.timeRange = 24
			s.loadData()
			s.message = "Showing last 24 hours"
			s.messageTyp = "success"
		case "w":
			s.timeRange = 168
			s.loadData()
			s.message = "Showing last 7 days"
			s.messageTyp = "success"
		case "m":
			s.timeRange = 720
			s.loadData()
			s.message = "Showing last 30 days"
			s.messageTyp = "success"
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			totalModels := 0
			for _, ps := range s.data {
				totalModels += len(ps.Models)
			}
			if s.cursor < totalModels-1 {
				s.cursor++
			}
		case "e":
			s.exportCSV()
		case "c":
			s.confirm = components.NewConfirm("Clear all statistics? This cannot be undone.")
			s.mode = "confirm"
			return s, nil
		}
	}
	return s, nil
}

func (s *Stats) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	cm, cmd := s.confirm.Update(msg)

	switch msg := msg.(type) {
	case components.ConfirmMsg:
		if msg.Confirmed {
			if err := s.store.ClearResults(); err != nil {
				s.message = "Error: " + err.Error()
				s.messageTyp = "error"
			} else {
				s.loadData()
				s.message = "Statistics cleared"
				s.messageTyp = "success"
			}
		}
		s.mode = "normal"
		s.confirm = nil
		return s, nil
	}

	s.confirm = cm.(*components.Confirm)
	return s, cmd
}

func (s *Stats) exportCSV() {
	var buf bytes.Buffer
	query := model.StatsQuery{Hours: s.timeRange}

	if err := stats.ExportCSV(&buf, s.store, query); err != nil {
		s.message = "Error exporting CSV: " + err.Error()
		s.messageTyp = "error"
		return
	}

	filename := fmt.Sprintf("uptime_report_%s.csv", time.Now().Format("20060102_150405"))
	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		s.message = "Error writing file: " + err.Error()
		s.messageTyp = "error"
		return
	}

	s.message = fmt.Sprintf("Exported to %s", filename)
	s.messageTyp = "success"
}

func (s *Stats) View() string {
	if s.mode == "confirm" {
		return s.confirm.View()
	}

	title := TitleStyle.Width(s.width).Render("Statistics")

	timeRangeLabel := "24 Hours"
	if s.timeRange == 168 {
		timeRangeLabel = "7 Days"
	} else if s.timeRange == 720 {
		timeRangeLabel = "30 Days"
	}

	timeRangeInfo := fmt.Sprintf("Time Range: %s", SuccessStyle.Render(timeRangeLabel))

	if len(s.data) == 0 {
		content := MutedStyle.Render("No statistics available")
		if s.message != "" {
			msgStyle := SuccessStyle
			if s.messageTyp == "error" {
				msgStyle = ErrorStyle
			}
			content = msgStyle.Render(s.message)
			s.message = ""
		}
		help := HelpStyle.Render("h: 24h • w: 7d • m: 30d • e: export CSV • c: clear stats")
		return lipgloss.JoinVertical(lipgloss.Left, title, timeRangeInfo, content, help)
	}

	rows := []string{
		TableHeaderStyle.Width(s.width).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				TableCellStyle.Width(4).Render(""),
				TableCellStyle.Width(20).Render("Provider"),
				TableCellStyle.Width(25).Render("Model"),
				TableCellStyle.Width(10).Render("Probes"),
				TableCellStyle.Width(10).Render("Success%"),
				TableCellStyle.Width(12).Render("Avg Latency"),
				TableCellStyle.Width(10).Render("Avg TPS"),
				TableCellStyle.Width(10).Render("TPS excl"),
			),
		),
	}

	idx := 0
	for _, ps := range s.data {
		for _, ms := range ps.Models {
			rateStyle := SuccessStyle
			if ms.SuccessRate < 99 {
				rateStyle = WarningStyle
			}
			if ms.SuccessRate < 95 {
				rateStyle = ErrorStyle
			}

			tpsStyle := SuccessStyle
			if ms.AvgTPS < 10 {
				tpsStyle = WarningStyle
			}
			if ms.AvgTPS < 1 {
				tpsStyle = ErrorStyle
			}

			icon := getStatusIcon(ms.LastStatus, ms.LastTPS)
			prefix := "  "
			style := TableCellStyle
			if idx == s.cursor {
				prefix = SuccessStyle.Render("▸ ")
				style = MenuActiveStyle
			}

			row := style.Width(s.width).Render(
				lipgloss.JoinHorizontal(lipgloss.Left,
					TableCellStyle.Width(4).Render(icon+prefix),
					TableCellStyle.Width(20).Render(ms.ProviderName),
					TableCellStyle.Width(25).Render(ms.Model),
					TableCellStyle.Width(10).Render(fmt.Sprintf("%d", ms.TotalProbes)),
					TableCellStyle.Width(10).Render(rateStyle.Render(fmt.Sprintf("%.1f%%", ms.SuccessRate))),
					TableCellStyle.Width(12).Render(fmt.Sprintf("%.0fms", ms.AvgLatencyMs)),
					TableCellStyle.Width(10).Render(tpsStyle.Render(fmt.Sprintf("%.2f", ms.AvgTPS))),
					TableCellStyle.Width(10).Render(tpsStyle.Render(fmt.Sprintf("%.2f", ms.AvgTPSExcludeTTFT))),
				),
			)
			rows = append(rows, row)
			idx++
		}
	}

	if s.message != "" {
		msgStyle := SuccessStyle
		if s.messageTyp == "error" {
			msgStyle = ErrorStyle
		}
		rows = append(rows, msgStyle.Render(s.message))
		s.message = ""
	}

	rows = append(rows, HelpStyle.Render("↑/↓: navigate • h: 24h • w: 7d • m: 30d • e: export CSV • c: clear stats"))

	return lipgloss.JoinVertical(lipgloss.Left, title, timeRangeInfo, lipgloss.JoinVertical(lipgloss.Left, rows...))
}

func (s *Stats) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Stats) IsFormMode() bool {
	return s.mode == "confirm"
}

func getStatusIcon(status string, tps float64) string {
	if status == "success" {
		if tps >= 1 {
			return "🟢"
		}
		return "🟡"
	}
	return "🔴"
}
