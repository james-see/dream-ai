package tui

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/rivo/tview"
)

// ActionsView provides actions for document management
type ActionsView struct {
	app      *App
	flex     *tview.Flex
	list     *tview.List
	info     *tview.TextView
	status   string
}

// NewActionsView creates a new actions view
func NewActionsView(app *App) *ActionsView {
	av := &ActionsView{
		app:    app,
		status: "Ready",
	}

	// Create list for actions
	av.list = tview.NewList().
		ShowSecondaryText(true).
		SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
			av.executeAction(index)
		})
	av.list.SetBorder(true).SetTitle(" Actions ")

	// Create info text view
	av.info = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	av.info.SetBorder(true).SetTitle(" Status ")

	// Create main flex layout
	av.flex = tview.NewFlex().
		AddItem(av.list, 0, 1, true).
		AddItem(av.info, 0, 1, false)

	// Populate actions
	av.populateActions()

	return av
}

// GetPrimitive returns the tview primitive
func (av *ActionsView) GetPrimitive() tview.Primitive {
	return av.flex
}

// populateActions populates the actions list
func (av *ActionsView) populateActions() {
	av.list.Clear()
	
	av.list.AddItem("Reprocess All Documents", "Force reprocess all documents (ignores hash check)", 'r', nil)
	av.list.AddItem("Process Images Only", "Process images from all documents with CLIP2", 'i', nil)
	av.list.AddItem("Reprocess Selected Document", "Reprocess the selected document from Documents view", 's', nil)
	av.list.AddItem("Clear All Chunks", "Delete all text chunks (keeps documents)", 'c', nil)
	av.list.AddItem("Clear All Images", "Delete all image records (keeps documents)", 'x', nil)
	av.list.AddItem("Rebuild Embeddings", "Regenerate embeddings for all chunks", 'e', nil)
	
	av.info.SetText("[white]Select an action to perform")
}

// executeAction executes the selected action
func (av *ActionsView) executeAction(index int) {
	ctx := context.Background()
	
	switch index {
	case 0: // Reprocess All Documents
		av.reprocessAllDocuments(ctx)
	case 1: // Process Images Only
		av.processImagesOnly(ctx)
	case 2: // Reprocess Selected Document
		av.info.SetText("[yellow]Go to Documents view, select a document, then come back here and select this action again")
	case 3: // Clear All Chunks
		av.clearAllChunks(ctx)
	case 4: // Clear All Images
		av.clearAllImages(ctx)
	case 5: // Rebuild Embeddings
		av.rebuildEmbeddings(ctx)
	}
}

// reprocessAllDocuments reprocesses all documents
func (av *ActionsView) reprocessAllDocuments(ctx context.Context) {
	// Run in goroutine to avoid blocking UI
	go func() {
		av.app.app.QueueUpdateDraw(func() {
			av.info.SetText("[yellow]Preparing to reprocess all documents...")
		})

		// Get all documents
		docs, err := av.app.db.GetAllDocuments(ctx)
		if err != nil {
			av.app.app.QueueUpdateDraw(func() {
				av.info.SetText(fmt.Sprintf("[red]Error: %v", err))
			})
			return
		}

		if len(docs) == 0 {
			av.app.app.QueueUpdateDraw(func() {
				av.info.SetText("[yellow]No documents found to reprocess")
			})
			return
		}

		totalProcessed := 0
		totalErrors := 0

		// Process each document
		for i, doc := range docs {
			progress := float64(i) / float64(len(docs))
			progressBar := av.renderProgressBar(progress)
			
			av.app.app.QueueUpdateDraw(func() {
				av.info.SetText(fmt.Sprintf("[yellow]Processing %d/%d: %s\n%s %.1f%%", 
					i+1, len(docs), filepath.Base(doc.FilePath), progressBar, progress*100))
			})

			// Delete existing chunks and images for this document first
			// (This forces reprocessing)
			if err := av.app.db.DeleteDocument(ctx, doc.ID); err == nil {
				// Recreate document and process
				if err := av.app.processor.ProcessDocument(ctx, doc.FilePath); err != nil {
					totalErrors++
				} else {
					totalProcessed++
				}
			} else {
				totalErrors++
			}
		}

		av.app.app.QueueUpdateDraw(func() {
			if totalErrors > 0 {
				av.info.SetText(fmt.Sprintf("[yellow]Processed %d documents, %d errors", totalProcessed, totalErrors))
			} else {
				av.info.SetText(fmt.Sprintf("[green]Successfully reprocessed %d documents!", totalProcessed))
			}
		})
	}()
}

// processImagesOnly processes images from all documents
func (av *ActionsView) processImagesOnly(ctx context.Context) {
	// Run in goroutine to avoid blocking UI
	go func() {
		av.app.app.QueueUpdateDraw(func() {
			av.info.SetText("[yellow]Scanning documents for images...")
		})

		// Get all documents
		docs, err := av.app.db.GetAllDocuments(ctx)
		if err != nil {
			av.app.app.QueueUpdateDraw(func() {
				av.info.SetText(fmt.Sprintf("[red]Error: %v", err))
			})
			return
		}

		// Count total images to process
		totalImagesToProcess := 0
		docImageCounts := make(map[uuid.UUID]int)
		for _, doc := range docs {
			images, err := av.app.db.GetImagesByDocument(ctx, doc.ID)
			if err == nil {
				count := 0
				for _, img := range images {
					if img.Embedding == nil {
						count++
					}
				}
				if count > 0 {
					docImageCounts[doc.ID] = count
					totalImagesToProcess += count
				}
			}
		}

		if totalImagesToProcess == 0 {
			av.app.app.QueueUpdateDraw(func() {
				av.info.SetText("[yellow]No images found that need processing")
			})
			return
		}

		totalProcessed := 0
		totalErrors := 0
		currentImage := 0

		for i, doc := range docs {
			if _, ok := docImageCounts[doc.ID]; ok {
				av.app.app.QueueUpdateDraw(func() {
					av.info.SetText(fmt.Sprintf("[yellow]Processing %d/%d images\nDocument %d/%d: %s\nProgress: %d/%d images", 
						currentImage+1, totalImagesToProcess, i+1, len(docs), filepath.Base(doc.FilePath), currentImage, totalImagesToProcess))
				})

				// Get images for this document
				images, err := av.app.db.GetImagesByDocument(ctx, doc.ID)
				if err != nil {
					continue
				}

				// Process each image that doesn't have an embedding
				for _, img := range images {
					if img.Embedding == nil {
						currentImage++
						av.app.app.QueueUpdateDraw(func() {
							progress := float64(currentImage) / float64(totalImagesToProcess)
							progressBar := av.renderProgressBar(progress)
							av.info.SetText(fmt.Sprintf("[yellow]Processing %d/%d images\nDocument: %s\nImage: %s\n%s %.1f%%", 
								currentImage, totalImagesToProcess, filepath.Base(doc.FilePath), filepath.Base(img.FilePath), progressBar, progress*100))
						})

						caption, embedding, err := av.app.imageEmb.ProcessImage(ctx, img.FilePath)
						if err == nil {
							// Update image with caption and embedding
							if err := av.app.db.UpdateImage(ctx, img.ID, caption, embedding); err == nil {
								totalProcessed++
							} else {
								totalErrors++
							}
						} else {
							totalErrors++
						}
					}
				}
			}
		}

		av.app.app.QueueUpdateDraw(func() {
			if totalErrors > 0 {
				av.info.SetText(fmt.Sprintf("[yellow]Processed %d images, %d errors", totalProcessed, totalErrors))
			} else {
				av.info.SetText(fmt.Sprintf("[green]Successfully processed %d images!", totalProcessed))
			}
		})
	}()
}

// renderProgressBar creates a text-based progress bar
func (av *ActionsView) renderProgressBar(progress float64) string {
	width := 30
	filled := int(progress * float64(width))
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}

// clearAllChunks deletes all chunks
func (av *ActionsView) clearAllChunks(ctx context.Context) {
	av.info.SetText("[yellow]Clearing all chunks...")
	av.app.app.ForceDraw()

	// This would require a new database method
	av.info.SetText("[red]Not implemented yet - would require DELETE FROM chunks")
}

// clearAllImages deletes all images
func (av *ActionsView) clearAllImages(ctx context.Context) {
	av.info.SetText("[yellow]Clearing all images...")
	av.app.app.ForceDraw()

	// This would require a new database method
	av.info.SetText("[red]Not implemented yet - would require DELETE FROM images")
}

// rebuildEmbeddings regenerates embeddings for all chunks
func (av *ActionsView) rebuildEmbeddings(ctx context.Context) {
	av.info.SetText("[yellow]Rebuilding embeddings...")
	av.app.app.ForceDraw()

	// This would require fetching all chunks and regenerating embeddings
	av.info.SetText("[red]Not implemented yet - would regenerate embeddings for all chunks")
}
