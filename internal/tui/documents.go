package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dream-ai/cli/internal/db"
)

// DocumentsView handles document management
type DocumentsView struct {
	app      *App
	documents []*db.Document
	selected  int
	width     int
	height    int
	loading   bool
	errorMsg  string
}

// NewDocumentsView creates a new documents view
func NewDocumentsView(app *App) *DocumentsView {
	return &DocumentsView{
		app:      app,
		documents: []*db.Document{},
		width:    80,
		height:   24,
	}
}

// Init initializes the documents view
func (dv *DocumentsView) Init() tea.Cmd {
	return dv.loadDocuments
}

// Update handles updates
func (dv *DocumentsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		dv.width = msg.Width
		dv.height = msg.Height
		return dv, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if dv.selected < len(dv.documents)-1 {
				dv.selected++
			}
			return dv, nil
		case "k", "up":
			if dv.selected > 0 {
				dv.selected--
			}
			return dv, nil
		case "a":
			return dv, dv.promptAddDocument
		case "d":
			if len(dv.documents) > 0 && dv.selected >= 0 {
				return dv, dv.deleteDocument
			}
		case "r":
			return dv, dv.reloadDocuments
		case "p":
			if len(dv.documents) > 0 && dv.selected >= 0 {
				return dv, dv.processDocument
			}
		}
	case documentsLoadedMsg:
		dv.documents = msg.documents
		dv.loading = false
		return dv, nil
	case documentProcessedMsg:
		dv.errorMsg = ""
		return dv, dv.loadDocuments
	case errorMsg:
		dv.errorMsg = msg.error.Error()
		dv.loading = false
		return dv, nil
	}
	return dv, nil
}

// View renders the documents view
func (dv *DocumentsView) View() string {
	var lines []string

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Documents")

	lines = append(lines, title)
	lines = append(lines, "")

	if dv.loading {
		lines = append(lines, "Loading documents...")
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	if dv.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("Error: " + dv.errorMsg)
		lines = append(lines, errorStyle)
		lines = append(lines, "")
	}

	if len(dv.documents) == 0 {
		lines = append(lines, "No documents found. Press 'a' to add documents.")
	} else {
		// Render document list
		for i, doc := range dv.documents {
			style := lipgloss.NewStyle()
			if i == dv.selected {
				style = style.Bold(true).Foreground(lipgloss.Color("205"))
			}

			status := "○"
			if doc.ProcessedAt != nil {
				status = "✓"
			}

			name := filepath.Base(doc.FilePath)
			docLine := fmt.Sprintf("%s %s (%s)", status, name, doc.FileType)
			lines = append(lines, style.Render(docLine))
		}
	}

	lines = append(lines, "")
	help := "a: Add | d: Delete | p: Process | r: Reload | j/k: Navigate"
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(help))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// loadDocuments loads documents from database
func (dv *DocumentsView) loadDocuments() tea.Msg {
	ctx := context.Background()
	docs, err := dv.app.db.GetAllDocuments(ctx)
	if err != nil {
		return errorMsg{error: err}
	}
	return documentsLoadedMsg{documents: docs}
}

// promptAddDocument prompts user to add a document (simplified - in real app would use textinput)
func (dv *DocumentsView) promptAddDocument() tea.Msg {
	// Process documents from the configured directory
	ctx := context.Background()
	docDir := dv.app.cfg.Paths.DocumentsDir
	
	// Expand ~ in path
	if strings.HasPrefix(docDir, "~") {
		homeDir := os.Getenv("HOME")
		docDir = filepath.Join(homeDir, strings.TrimPrefix(docDir, "~"))
	}
	
	// Process all PDFs and EPUBs in the directory
	pdfFiles, err := filepath.Glob(filepath.Join(docDir, "*.pdf"))
	if err == nil {
		for _, file := range pdfFiles {
			if err := dv.app.processor.ProcessDocument(ctx, file); err != nil {
				// Continue processing other files even if one fails
				fmt.Printf("Warning: failed to process %s: %v\n", file, err)
			}
		}
	}
	
	epubFiles, err := filepath.Glob(filepath.Join(docDir, "*.epub"))
	if err == nil {
		for _, file := range epubFiles {
			if err := dv.app.processor.ProcessDocument(ctx, file); err != nil {
				// Continue processing other files even if one fails
				fmt.Printf("Warning: failed to process %s: %v\n", file, err)
			}
		}
	}

	return documentProcessedMsg{}
}

// deleteDocument deletes the selected document
func (dv *DocumentsView) deleteDocument() tea.Msg {
	if dv.selected < 0 || dv.selected >= len(dv.documents) {
		return nil
	}

	doc := dv.documents[dv.selected]
	ctx := context.Background()
	
	if err := dv.app.db.DeleteDocument(ctx, doc.ID); err != nil {
		return errorMsg{error: err}
	}

	return documentProcessedMsg{}
}

// reloadDocuments reloads the document list
func (dv *DocumentsView) reloadDocuments() tea.Msg {
	return dv.loadDocuments()
}

// processDocument processes the selected document
func (dv *DocumentsView) processDocument() tea.Msg {
	if dv.selected < 0 || dv.selected >= len(dv.documents) {
		return nil
	}

	doc := dv.documents[dv.selected]
	ctx := context.Background()
	
	if err := dv.app.processor.ProcessDocument(ctx, doc.FilePath); err != nil {
		return errorMsg{error: err}
	}

	return documentProcessedMsg{}
}

// documentsLoadedMsg signals documents have been loaded
type documentsLoadedMsg struct {
	documents []*db.Document
}

// documentProcessedMsg signals a document has been processed
type documentProcessedMsg struct{}

// errorMsg represents an error
type errorMsg struct {
	error error
}
