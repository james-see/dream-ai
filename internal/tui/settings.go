package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"
)

// SettingsView displays and allows editing settings using tview
type SettingsView struct {
	app      *App
	flex     *tview.Flex
	form     *tview.Form
	text     *tview.TextView
	docDirs  []string
}

// NewSettingsView creates a new settings view
func NewSettingsView(app *App) *SettingsView {
	sv := &SettingsView{
		app:     app,
		docDirs: make([]string, len(app.cfg.Paths.DocumentsDirs)),
	}
	copy(sv.docDirs, app.cfg.Paths.DocumentsDirs)

	// Create form for editing document directories
	sv.form = tview.NewForm().
		AddTextView("Document Directories", "Configure where to look for documents:", 0, 1, false, false).
		AddInputField("Directory 1", sv.getDocDir(0), 0, nil, func(text string) {
			sv.setDocDir(0, text)
		}).
		AddInputField("Directory 2", sv.getDocDir(1), 0, nil, func(text string) {
			sv.setDocDir(1, text)
		}).
		AddInputField("Directory 3", sv.getDocDir(2), 0, nil, func(text string) {
			sv.setDocDir(2, text)
		}).
		AddButton("Add Directory", func() {
			sv.addDocDir()
		}).
		AddButton("Save", func() {
			sv.saveSettings()
		}).
		AddButton("Reset to Defaults", func() {
			sv.resetToDefaults()
		})
	sv.form.SetBorder(true).SetTitle(" Document Directories ")

	// Create info text view
	sv.text = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	sv.text.SetBorder(true).SetTitle(" Current Settings ")

	// Create main flex layout
	sv.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(sv.form, 0, 1, true).
				AddItem(sv.text, 0, 1, false),
			0, 1, true,
		)

	sv.render()

	return sv
}

// GetPrimitive returns the tview primitive
func (sv *SettingsView) GetPrimitive() tview.Primitive {
	return sv.flex
}

// getDocDir gets a document directory at index, or returns empty string
func (sv *SettingsView) getDocDir(index int) string {
	if index < len(sv.docDirs) {
		return sv.docDirs[index]
	}
	return ""
}

// setDocDir sets a document directory at index
func (sv *SettingsView) setDocDir(index int, value string) {
	// Expand ~ in path
	if strings.HasPrefix(value, "~") {
		homeDir := os.Getenv("HOME")
		value = filepath.Join(homeDir, strings.TrimPrefix(value, "~"))
	}

	// Ensure we have enough space
	for len(sv.docDirs) <= index {
		sv.docDirs = append(sv.docDirs, "")
	}
	sv.docDirs[index] = value
}

// addDocDir adds a new empty directory field
func (sv *SettingsView) addDocDir() {
	sv.docDirs = append(sv.docDirs, "")
	sv.rebuildForm()
}

// saveSettings saves the settings
func (sv *SettingsView) saveSettings() {
	// Filter out empty directories
	filtered := []string{}
	for _, dir := range sv.docDirs {
		if strings.TrimSpace(dir) != "" {
			filtered = append(filtered, dir)
		}
	}

	// If no directories set, use defaults
	if len(filtered) == 0 {
		homeDir := os.Getenv("HOME")
		filtered = []string{
			filepath.Join(homeDir, ".config", "dream-ai", "documents"),
		}
	}

	sv.app.cfg.Paths.DocumentsDirs = filtered

	// Save to config file
	if err := sv.app.cfg.Save(); err != nil {
		sv.text.SetText(fmt.Sprintf("[red]Error saving settings: %v", err))
		return
	}

	sv.text.SetText("[green]Settings saved successfully!")
	sv.render()
}

// resetToDefaults resets to default directories
func (sv *SettingsView) resetToDefaults() {
	homeDir := os.Getenv("HOME")
	sv.docDirs = []string{
		filepath.Join(homeDir, ".config", "dream-ai", "documents"),
	}
	sv.rebuildForm()
	sv.text.SetText("[yellow]Reset to defaults. Press Save to apply.")
}

// rebuildForm rebuilds the form with current directories
func (sv *SettingsView) rebuildForm() {
	sv.form.Clear(true)
	sv.form.AddTextView("Document Directories", "Configure where to look for documents:", 0, 1, false, false)
	
	for i := 0; i < len(sv.docDirs) || i < 3; i++ {
		idx := i
		sv.form.AddInputField(fmt.Sprintf("Directory %d", i+1), sv.getDocDir(i), 0, nil, func(text string) {
			sv.setDocDir(idx, text)
		})
	}
	
	sv.form.AddButton("Add Directory", func() {
		sv.addDocDir()
	}).
	AddButton("Save", func() {
		sv.saveSettings()
	}).
	AddButton("Reset to Defaults", func() {
		sv.resetToDefaults()
	})
}

// render updates the settings display
func (sv *SettingsView) render() {
	cfg := sv.app.cfg
	
	docDirsText := "None"
	if len(cfg.Paths.DocumentsDirs) > 0 {
		docDirsText = strings.Join(cfg.Paths.DocumentsDirs, "\n  ")
	}
	
	settingsText := fmt.Sprintf(`[white]Database:
  Connection: [cyan]%s[white]

Ollama:
  Base URL: [cyan]%s[white]
  Text Model: [cyan]%s[white]

CLIP2:
  Python Path: [cyan]%s[white]
  Script Path: [cyan]%s[white]

Document Directories:
  [cyan]%s[white]

Images Directory:
  [cyan]%s[white]

Processing:
  Chunk Size: [cyan]%d[white]
  Chunk Overlap: [cyan]%d[white]

RAG:
  Top K: [cyan]5[white]
  Max Context Length: [cyan]2000[white]`,
		cfg.Database.ConnectionString,
		cfg.Ollama.BaseURL,
		cfg.Embeddings.TextModel,
		cfg.CLIP2.PythonPath,
		cfg.CLIP2.ScriptPath,
		docDirsText,
		cfg.Paths.ImageDir,
		cfg.Processing.ChunkSize,
		cfg.Processing.ChunkOverlap,
	)

	sv.text.SetText(settingsText)
}
