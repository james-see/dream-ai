package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dream-ai/cli/internal/db"
	"github.com/rivo/tview"
	"github.com/gdamore/tcell/v2"
)

// DocumentsView manages documents using tview
type DocumentsView struct {
	app      *App
	flex     *tview.Flex
	list     *tview.List
	info     *tview.TextView
	documents []*db.Document
}

// NewDocumentsView creates a new documents view
func NewDocumentsView(app *App) *DocumentsView {
	dv := &DocumentsView{
		app:       app,
		documents: []*db.Document{},
	}

	// Create list for documents
	dv.list = tview.NewList().
		ShowSecondaryText(true).
		SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
			dv.processSelected()
		}).
		SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
			dv.showDocumentInfo(index)
		})
	dv.list.SetBorder(true).SetTitle(" Documents ")

	// Create info text view
	dv.info = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	dv.info.SetBorder(true).SetTitle(" Info ")

	// Create main flex layout
	dv.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(dv.list, 0, 2, true).
				AddItem(dv.info, 0, 1, false),
			0, 1, true,
		).
		AddItem(
			tview.NewTextView().
				SetText("[yellow]a[white]: Add | [yellow]d[white]: Delete | [yellow]p[white]: Process | [yellow]r[white]: Reload").
				SetDynamicColors(true),
			1, 0, false,
		)

	// Set up input capture
	dv.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a', 'A':
			dv.addDocuments()
			return nil
		case 'd', 'D':
			dv.deleteSelected()
			return nil
		case 'p', 'P':
			dv.processSelected()
			return nil
		case 'r', 'R':
			dv.reloadDocuments()
			return nil
		}
		return event
	})

	// Load documents
	dv.reloadDocuments()

	return dv
}

// GetPrimitive returns the tview primitive
func (dv *DocumentsView) GetPrimitive() tview.Primitive {
	return dv.flex
}

// reloadDocuments reloads the document list
func (dv *DocumentsView) reloadDocuments() {
	ctx := context.Background()
	docs, err := dv.app.db.GetAllDocuments(ctx)
	if err != nil {
		dv.info.SetText(fmt.Sprintf("[red]Error loading documents: %v", err))
		return
	}

	dv.documents = docs
	dv.list.Clear()

	for i, doc := range docs {
		status := "[red]Not processed"
		if doc.ProcessedAt != nil {
			status = "[green]Processed"
		} else if doc.ErrorMessage != nil && *doc.ErrorMessage != "" {
			// Show error reason if available
			errorMsg := *doc.ErrorMessage
			// Truncate long error messages
			if len(errorMsg) > 50 {
				errorMsg = errorMsg[:47] + "..."
			}
			status = fmt.Sprintf("[red]Not processed: %s", errorMsg)
		}
		
		fileName := filepath.Base(doc.FilePath)
		mainText := fmt.Sprintf("%d. %s", i+1, fileName)
		secondaryText := fmt.Sprintf("%s | %s", doc.FileType, status)
		
		dv.list.AddItem(mainText, secondaryText, 0, nil)
	}

	if len(docs) == 0 {
		dv.info.SetText("[yellow]No documents found. Press 'a' to add documents from the configured directory.")
	} else {
		// Show info for currently selected document
		selected := dv.list.GetCurrentItem()
		if selected >= 0 && selected < len(docs) {
			dv.showDocumentInfo(selected)
		} else {
			dv.info.SetText(fmt.Sprintf("[white]Total: %d documents", len(docs)))
		}
	}
}

// addDocuments processes documents from all configured directories
func (dv *DocumentsView) addDocuments() {
	// Run processing in a goroutine to avoid blocking UI
	go func() {
		ctx := context.Background()
		docDirs := dv.app.cfg.Paths.DocumentsDirs
		if len(docDirs) == 0 {
			// Fallback to default directory
			homeDir := os.Getenv("HOME")
			docDirs = []string{
				filepath.Join(homeDir, ".config", "dream-ai", "documents"),
			}
		}

		dv.app.app.QueueUpdateDraw(func() {
			dv.info.SetText("[yellow]Scanning directories...")
		})

		totalProcessed := 0
		totalErrors := 0
		totalSkipped := 0
		var errorFiles []string
		var allFiles []string

		// Collect all files first
		for _, docDir := range docDirs {
			// Expand ~ in path
			if strings.HasPrefix(docDir, "~") {
				homeDir := os.Getenv("HOME")
				docDir = filepath.Join(homeDir, strings.TrimPrefix(docDir, "~"))
			}

			// Check if directory exists
			if _, err := os.Stat(docDir); os.IsNotExist(err) {
				continue
			}

			// Collect PDFs
			pdfFiles, _ := filepath.Glob(filepath.Join(docDir, "*.pdf"))
			allFiles = append(allFiles, pdfFiles...)

			// Collect EPUBs
			epubFiles, _ := filepath.Glob(filepath.Join(docDir, "*.epub"))
			allFiles = append(allFiles, epubFiles...)
		}

		if len(allFiles) == 0 {
			dv.app.app.QueueUpdateDraw(func() {
				dv.info.SetText("[yellow]No documents found in configured directories")
				dv.reloadDocuments()
			})
			return
		}

		// Process files
		for i, file := range allFiles {
			fileName := filepath.Base(file)
			dv.app.app.QueueUpdateDraw(func() {
				dv.info.SetText(fmt.Sprintf("[yellow]Processing %d/%d: %s...", i+1, len(allFiles), fileName))
			})

			if err := dv.processDocumentWithSuppressedWarnings(ctx, file); err != nil {
				// Check if it's a "already processed" skip (which is not an error)
				if strings.Contains(err.Error(), "already processed") || strings.Contains(err.Error(), "skip") {
					totalSkipped++
				} else {
					totalErrors++
					errorFiles = append(errorFiles, fmt.Sprintf("%s", fileName))
				}
			} else {
				totalProcessed++
			}
		}

		// Update UI with results
		dv.app.app.QueueUpdateDraw(func() {
			dv.reloadDocuments()
			
			var statusMsg string
			if totalProcessed > 0 || totalSkipped > 0 {
				parts := []string{}
				if totalProcessed > 0 {
					parts = append(parts, fmt.Sprintf("[green]Processed: %d", totalProcessed))
				}
				if totalSkipped > 0 {
					parts = append(parts, fmt.Sprintf("[yellow]Skipped (already processed): %d", totalSkipped))
				}
				if totalErrors > 0 {
					parts = append(parts, fmt.Sprintf("[red]Errors: %d", totalErrors))
					if len(errorFiles) > 0 {
						errorList := strings.Join(errorFiles[:min(5, len(errorFiles))], ", ")
						if len(errorFiles) > 5 {
							errorList += fmt.Sprintf(" (+%d more)", len(errorFiles)-5)
						}
						parts = append(parts, fmt.Sprintf("[red]Failed: %s", errorList))
					}
				}
				statusMsg = strings.Join(parts, "\n")
			} else if totalErrors > 0 {
				statusMsg = fmt.Sprintf("[red]Failed to process documents\nErrors: %s", strings.Join(errorFiles, ", "))
			} else {
				statusMsg = "[yellow]No documents found in configured directories"
			}
			dv.info.SetText(statusMsg)
		})
	}()
}

// deleteSelected deletes the selected document
func (dv *DocumentsView) deleteSelected() {
	selected := dv.list.GetCurrentItem()
	if selected < 0 || selected >= len(dv.documents) {
		return
	}

	doc := dv.documents[selected]
	ctx := context.Background()

	if err := dv.app.db.DeleteDocument(ctx, doc.ID); err != nil {
		dv.info.SetText(fmt.Sprintf("[red]Error deleting document: %v", err))
		return
	}

	dv.reloadDocuments()
	dv.info.SetText("[green]Document deleted successfully!")
}

// showDocumentInfo displays information about the selected document
func (dv *DocumentsView) showDocumentInfo(index int) {
	if index < 0 || index >= len(dv.documents) {
		return
	}

	doc := dv.documents[index]
	fileName := filepath.Base(doc.FilePath)
	
	var infoText strings.Builder
	infoText.WriteString(fmt.Sprintf("[white]File: [yellow]%s[white]\n", fileName))
	infoText.WriteString(fmt.Sprintf("Type: [cyan]%s[white]\n", doc.FileType))
	infoText.WriteString(fmt.Sprintf("Path: [gray]%s[white]\n", doc.FilePath))
	
	if doc.ProcessedAt != nil {
		infoText.WriteString(fmt.Sprintf("Status: [green]Processed[white]\n"))
		infoText.WriteString(fmt.Sprintf("Processed: [gray]%s[white]", doc.ProcessedAt.Format("2006-01-02 15:04:05")))
	} else {
		infoText.WriteString("Status: [red]Not processed[white]\n")
		if doc.ErrorMessage != nil && *doc.ErrorMessage != "" {
			infoText.WriteString(fmt.Sprintf("\n[red]Error:[white]\n%s", *doc.ErrorMessage))
		} else {
			infoText.WriteString("\n[yellow]No error information available")
		}
	}
	
	dv.info.SetText(infoText.String())
}

// processSelected processes the selected document
func (dv *DocumentsView) processSelected() {
	selected := dv.list.GetCurrentItem()
	if selected < 0 || selected >= len(dv.documents) {
		return
	}

	doc := dv.documents[selected]
	ctx := context.Background()

	dv.info.SetText(fmt.Sprintf("[yellow]Processing %s...", filepath.Base(doc.FilePath)))

	if err := dv.app.processor.ProcessDocument(ctx, doc.FilePath); err != nil {
		// Reload to get updated error message
		dv.reloadDocuments()
		// Show error in info pane
		if doc.ErrorMessage != nil && *doc.ErrorMessage != "" {
			dv.info.SetText(fmt.Sprintf("[red]Error: %s", *doc.ErrorMessage))
		} else {
			dv.info.SetText(fmt.Sprintf("[red]Error processing document: %v", err))
		}
		return
	}

	dv.reloadDocuments()
	dv.info.SetText("[green]Document processed successfully!")
}

// processDocumentWithSuppressedWarnings processes a document while suppressing PDF library warnings
func (dv *DocumentsView) processDocumentWithSuppressedWarnings(ctx context.Context, filePath string) error {
	// Save original stderr
	originalStderr := os.Stderr
	defer func() {
		os.Stderr = originalStderr
	}()
	
	// Create a pipe to capture stderr
	r, w, err := os.Pipe()
	if err != nil {
		// If pipe creation fails, just process normally
		return dv.app.processor.ProcessDocument(ctx, filePath)
	}
	
	// Redirect stderr to the pipe
	os.Stderr = w
	
	// Process document
	done := make(chan error, 1)
	go func() {
		err := dv.app.processor.ProcessDocument(ctx, filePath)
		w.Close()
		done <- err
	}()
	
	// Read and discard stderr output in background
	go func() {
		io.Copy(io.Discard, r)
		r.Close()
	}()
	
	// Wait for processing to complete
	err = <-done
	
	return err
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
