package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// DashboardView shows overall status and statistics using tview
type DashboardView struct {
	app      *App
	flex     *tview.Flex
	status   *tview.TextView
	stats    *tview.TextView
	menu     *tview.List
	progress *tview.TextView
	
	statsData DashboardStats
}

// DashboardStats contains statistics about the system
type DashboardStats struct {
	TotalDocuments     int
	ProcessedDocuments int
	TotalChunks        int
	TotalImages        int
	TotalWords         int
	PagesWithImages    int
	TotalPages         int
	ProcessingStatus   string
	CurrentProgress    float64
}

// NewDashboardView creates a new dashboard view
func NewDashboardView(app *App) *DashboardView {
	dv := &DashboardView{
		app:       app,
		statsData: DashboardStats{ProcessingStatus: "Ready"},
	}

	// Create status text view
	dv.status = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	dv.status.SetBorder(true).SetTitle(" Status ")

	// Create stats text view
	dv.stats = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	dv.stats.SetBorder(true).SetTitle(" Statistics ")

	// Create progress text view
	dv.progress = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	dv.progress.SetBorder(true).SetTitle(" Progress ")

	// Create menu list
	dv.menu = tview.NewList().
		AddItem("Chat Console", "Start chatting about dreams and symbols", '1', func() {
			app.pages.SwitchToPage("chat")
		}).
		AddItem("Documents Manager", "Manage and process documents", '2', func() {
			app.pages.SwitchToPage("documents")
		}).
		AddItem("Model Selection", "Select Ollama model", '3', func() {
			app.pages.SwitchToPage("models")
		}).
		AddItem("Settings", "View application settings", '4', func() {
			app.pages.SwitchToPage("settings")
		}).
		AddItem("Actions", "Document processing actions", '5', func() {
			app.pages.SwitchToPage("actions")
		}).
		AddItem("Quit", "Press to exit", 'q', func() {
			app.app.Stop()
		})
	dv.menu.SetBorder(true).SetTitle(" Navigation ")

	// Create main flex layout
	dv.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(
			tview.NewFlex().
				AddItem(dv.status, 0, 1, false).
				AddItem(dv.progress, 0, 1, false),
			0, 1, false,
		).
		AddItem(
			tview.NewFlex().
				AddItem(dv.stats, 0, 1, false).
				AddItem(dv.menu, 0, 1, true),
			0, 2, true,
		)

	// Update stats periodically
	go dv.updateStatsLoop()

	return dv
}

// GetPrimitive returns the tview primitive
func (dv *DashboardView) GetPrimitive() tview.Primitive {
	return dv.flex
}

// updateStatsLoop updates statistics periodically
func (dv *DashboardView) updateStatsLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		dv.updateStats()
		dv.app.app.QueueUpdateDraw(func() {
			dv.render()
		})
	}
}

// updateStats fetches current statistics
func (dv *DashboardView) updateStats() {
	ctx := context.Background()
	stats := DashboardStats{
		ProcessingStatus: "Ready",
	}

	// Get document stats
	docs, err := dv.app.db.GetAllDocuments(ctx)
	if err == nil {
		stats.TotalDocuments = len(docs)
		for _, doc := range docs {
			if doc.ProcessedAt != nil {
				stats.ProcessedDocuments++
			}
		}
	}

		// Get chunk, image, word, and page stats
		totalChunks, totalImages, totalWords, totalPages, pagesWithImages, err := dv.app.db.GetStats(ctx)
		if err == nil {
			stats.TotalChunks = totalChunks
			stats.TotalImages = totalImages
			stats.TotalWords = totalWords
			stats.TotalPages = totalPages
			stats.PagesWithImages = pagesWithImages
		}

	dv.statsData = stats
}

// render updates the display
func (dv *DashboardView) render() {
	// Update status
	statusText := fmt.Sprintf("[green]●[white] %s", dv.statsData.ProcessingStatus)
	if dv.statsData.ProcessingStatus == "Processing..." {
		statusText = fmt.Sprintf("[yellow]●[white] %s", dv.statsData.ProcessingStatus)
	}
	dv.status.SetText(statusText)

	// Update progress
	if dv.statsData.CurrentProgress > 0 && dv.statsData.CurrentProgress < 1.0 {
		progressBar := dv.renderProgressBar(dv.statsData.CurrentProgress)
		progressText := fmt.Sprintf("%s\n%.1f%%", progressBar, dv.statsData.CurrentProgress*100)
		dv.progress.SetText(progressText)
	} else {
		dv.progress.SetText("No active processing")
	}

	// Update stats
	statsText := fmt.Sprintf(`Documents: [yellow]%d/%d[white] processed
Chunks: [yellow]%d[white]
Words: [yellow]%s[white]
Images: [yellow]%d[white]
Pages: [yellow]%d[white] total, [yellow]%d[white] with images`,
		dv.statsData.ProcessedDocuments,
		dv.statsData.TotalDocuments,
		dv.statsData.TotalChunks,
		formatNumber(dv.statsData.TotalWords),
		dv.statsData.TotalImages,
		dv.statsData.TotalPages,
		dv.statsData.PagesWithImages,
	)
	dv.stats.SetText(statsText)
}

// renderProgressBar creates a text-based progress bar
func (dv *DashboardView) renderProgressBar(progress float64) string {
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

// formatNumber formats large numbers with K/M suffixes
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}
