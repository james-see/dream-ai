package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingsView handles application settings
type SettingsView struct {
	app      *App
	selected int
	width    int
	height   int
}

// NewSettingsView creates a new settings view
func NewSettingsView(app *App) *SettingsView {
	return &SettingsView{
		app:      app,
		selected: 0,
		width:    80,
		height:   24,
	}
}

// Init initializes the settings view
func (sv *SettingsView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (sv *SettingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		sv.width = msg.Width
		sv.height = msg.Height
		return sv, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if sv.selected < 7 {
				sv.selected++
			}
			return sv, nil
		case "k", "up":
			if sv.selected > 0 {
				sv.selected--
			}
			return sv, nil
		case "s":
			// Save settings
			if err := sv.app.cfg.Save(); err != nil {
				return sv, func() tea.Msg {
					return errorMsg{error: err}
				}
			}
			return sv, nil
		}
	}
	return sv, nil
}

// View renders the settings view
func (sv *SettingsView) View() string {
	var lines []string

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Settings")

	lines = append(lines, title)
	lines = append(lines, "")

	cfg := sv.app.cfg

	settings := []struct {
		label string
		value string
	}{
		{"Database", cfg.Database.ConnectionString},
		{"Ollama URL", cfg.Ollama.BaseURL},
		{"Default Model", cfg.Ollama.DefaultModel},
		{"Embedding Model", cfg.Embeddings.TextModel},
		{"Chunk Size", fmt.Sprintf("%d", cfg.Processing.ChunkSize)},
		{"Chunk Overlap", fmt.Sprintf("%d", cfg.Processing.ChunkOverlap)},
		{"Top K", fmt.Sprintf("%d", cfg.Processing.TopK)},
		{"Documents Dir", cfg.Paths.DocumentsDir},
	}

	for i, setting := range settings {
		style := lipgloss.NewStyle()
		if i == sv.selected {
			style = style.Bold(true).Foreground(lipgloss.Color("205"))
		}
		line := fmt.Sprintf("%s: %s", setting.label, setting.value)
		lines = append(lines, style.Render(line))
	}

	lines = append(lines, "")
	help := "j/k: Navigate | s: Save | Note: Settings are read-only in this view"
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(help))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
