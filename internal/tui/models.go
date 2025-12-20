package tui

import (
	"context"
	"fmt"

	"github.com/dream-ai/cli/internal/ollama"
	"github.com/rivo/tview"
)

// ModelsView manages model selection using tview
type ModelsView struct {
	app      *App
	flex     *tview.Flex
	list     *tview.List
	info     *tview.TextView
	models   []ollama.ModelInfo
	current  string
}

// NewModelsView creates a new models view
func NewModelsView(app *App, currentModel string) *ModelsView {
	mv := &ModelsView{
		app:     app,
		current: currentModel,
		models:  []ollama.ModelInfo{},
	}

	// Create list for models
	mv.list = tview.NewList().
		ShowSecondaryText(true).
		SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
			mv.selectModel(index)
		})
	mv.list.SetBorder(true).SetTitle(" Available Models ")

	// Create info text view
	mv.info = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	mv.info.SetBorder(true).SetTitle(" Model Info ")

	// Create main flex layout
	mv.flex = tview.NewFlex().
		AddItem(mv.list, 0, 1, true).
		AddItem(mv.info, 0, 1, false)

	// Load models
	mv.reloadModels()

	return mv
}

// GetPrimitive returns the tview primitive
func (mv *ModelsView) GetPrimitive() tview.Primitive {
	return mv.flex
}

// reloadModels reloads the model list
func (mv *ModelsView) reloadModels() {
	ctx := context.Background()
	models, err := mv.app.modelSelector.ListModels(ctx)
	if err != nil {
		mv.info.SetText(fmt.Sprintf("[red]Error loading models: %v", err))
		return
	}

	mv.models = models
	mv.list.Clear()

	for i, model := range models {
		isCurrent := ""
		if model.Name == mv.current {
			isCurrent = " [green]← Current"
		}
		
		mainText := fmt.Sprintf("%d. %s%s", i+1, model.Name, isCurrent)
		secondaryText := fmt.Sprintf("Size: %s", formatModelSize(model.Size))
		
		mv.list.AddItem(mainText, secondaryText, 0, nil)
	}

	if len(models) == 0 {
		mv.info.SetText("[yellow]No models found. Make sure Ollama is running.")
	} else {
		mv.info.SetText(fmt.Sprintf("[white]Total: %d models available", len(models)))
	}
}

// selectModel selects a model
func (mv *ModelsView) selectModel(index int) {
	if index < 0 || index >= len(mv.models) {
		return
	}

	model := mv.models[index]
	mv.current = model.Name
	mv.app.chatView.model = model.Name
	
	mv.reloadModels()
	mv.info.SetText(fmt.Sprintf("[green]Selected model: %s", model.Name))
}

// formatModelSize formats model size
func formatModelSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}
