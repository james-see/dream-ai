package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dream-ai/cli/internal/ollama"
)

// ModelsView handles model selection
type ModelsView struct {
	app       *App
	models    []ollama.ModelInfo
	selected  int
	currentModel string
	width     int
	height    int
	loading   bool
	errorMsg  string
}

// NewModelsView creates a new models view
func NewModelsView(app *App, currentModel string) *ModelsView {
	return &ModelsView{
		app:          app,
		currentModel: currentModel,
		models:       []ollama.ModelInfo{},
		width:        80,
		height:       24,
	}
}

// Init initializes the models view
func (mv *ModelsView) Init() tea.Cmd {
	return mv.loadModels
}

// Update handles updates
func (mv *ModelsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		mv.width = msg.Width
		mv.height = msg.Height
		return mv, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if mv.selected < len(mv.models)-1 {
				mv.selected++
			}
			return mv, nil
		case "k", "up":
			if mv.selected > 0 {
				mv.selected--
			}
			return mv, nil
		case "enter", " ":
			if len(mv.models) > 0 && mv.selected >= 0 {
				return mv, mv.selectModel
			}
		case "r":
			return mv, mv.loadModels
		}
	case modelsLoadedMsg:
		mv.models = msg.models
		mv.loading = false
		// Find current model index
		for i, model := range mv.models {
			if model.Name == mv.currentModel {
				mv.selected = i
				break
			}
		}
		return mv, nil
	case modelSelectedMsg:
		mv.currentModel = msg.model
		mv.app.chatView.model = msg.model
		return mv, nil
	case errorMsg:
		mv.errorMsg = msg.error.Error()
		mv.loading = false
		return mv, nil
	}
	return mv, nil
}

// View renders the models view
func (mv *ModelsView) View() string {
	var lines []string

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Ollama Models")

	lines = append(lines, title)
	lines = append(lines, "")

	if mv.loading {
		lines = append(lines, "Loading models...")
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	if mv.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("Error: " + mv.errorMsg)
		lines = append(lines, errorStyle)
		lines = append(lines, "")
	}

	currentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)
	lines = append(lines, currentStyle.Render(fmt.Sprintf("Current: %s", mv.currentModel)))
	lines = append(lines, "")

	if len(mv.models) == 0 {
		lines = append(lines, "No models found. Make sure Ollama is running.")
	} else {
		// Render model list
		for i, model := range mv.models {
			style := lipgloss.NewStyle()
			if i == mv.selected {
				style = style.Bold(true).Foreground(lipgloss.Color("205"))
			}
			if model.Name == mv.currentModel {
				style = style.Foreground(lipgloss.Color("39"))
			}

			sizeMB := float64(model.Size) / (1024 * 1024)
			modelLine := fmt.Sprintf("%s %.2f MB", model.Name, sizeMB)
			lines = append(lines, style.Render(modelLine))
		}
	}

	lines = append(lines, "")
	help := "j/k: Navigate | Enter/Space: Select | r: Reload"
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(help))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// loadModels loads available models
func (mv *ModelsView) loadModels() tea.Msg {
	ctx := context.Background()
	models, err := mv.app.modelSelector.ListModels(ctx)
	if err != nil {
		return errorMsg{error: err}
	}
	return modelsLoadedMsg{models: models}
}

// selectModel selects the current model
func (mv *ModelsView) selectModel() tea.Msg {
	if mv.selected < 0 || mv.selected >= len(mv.models) {
		return nil
	}
	return modelSelectedMsg{model: mv.models[mv.selected].Name}
}

// modelsLoadedMsg signals models have been loaded
type modelsLoadedMsg struct {
	models []ollama.ModelInfo
}

// modelSelectedMsg signals a model has been selected
type modelSelectedMsg struct {
	model string
}
