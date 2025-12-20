package tui

import (
	"context"
	"fmt"

	"github.com/dream-ai/cli/config"
	"github.com/dream-ai/cli/internal/db"
	"github.com/dream-ai/cli/internal/documents"
	"github.com/dream-ai/cli/internal/embeddings"
	"github.com/dream-ai/cli/internal/ollama"
	"github.com/dream-ai/cli/internal/rag"
	"github.com/rivo/tview"
	"github.com/gdamore/tcell/v2"
)

// App represents the main TUI application using tview
type App struct {
	app            *tview.Application
	pages          *tview.Pages
	db             *db.DB
	processor      *documents.Processor
	retriever      *rag.Retriever
	contextBuilder *rag.ContextBuilder
	ollamaClient   *ollama.Client
	modelSelector  *ollama.ModelSelector
	textEmb        *embeddings.TextEmbedder
	imageEmb       *embeddings.ImageEmbedder
	cfg            *config.Config
	
	// Views
	dashboardView *DashboardView
	chatView      *ChatView
	documentsView *DocumentsView
	modelsView    *ModelsView
	settingsView  *SettingsView
	actionsView   *ActionsView
}

// NewApp creates a new TUI application
func NewApp(cfg *config.Config) (*App, error) {
	// Initialize database
	database, err := db.New(cfg.Database.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize embeddings
	textEmb := embeddings.NewTextEmbedder(cfg.Ollama.BaseURL, cfg.Embeddings.TextModel)
	imageEmb := embeddings.NewImageEmbedder(cfg.CLIP2.PythonPath)
	if cfg.CLIP2.ScriptPath != "" {
		imageEmb.SetScriptPath(cfg.CLIP2.ScriptPath)
	}

	// Initialize document processor
	processor := documents.NewProcessor(
		database,
		textEmb,
		imageEmb,
		cfg.Paths.ImageDir,
		cfg.Processing.ChunkSize,
		cfg.Processing.ChunkOverlap,
	)

	// Initialize RAG components
	retriever := rag.NewRetriever(database, textEmb, 5) // Default topK
	contextBuilder := rag.NewContextBuilder(2000) // Default max context length

	// Initialize Ollama client
	ollamaClient := ollama.NewClient(cfg.Ollama.BaseURL)
	modelSelector := ollama.NewModelSelector(ollamaClient)

	// Select default model
	ctx := context.Background()
	defaultModel, err := modelSelector.SelectBestModel(ctx)
	if err != nil {
		defaultModel = "llama3.2" // Fallback
	}

	app := &App{
		db:             database,
		processor:      processor,
		retriever:      retriever,
		contextBuilder: contextBuilder,
		ollamaClient:   ollamaClient,
		modelSelector:  modelSelector,
		textEmb:        textEmb,
		imageEmb:       imageEmb,
		cfg:            cfg,
	}

	// Initialize tview application
	app.app = tview.NewApplication()
	app.pages = tview.NewPages()

	// Initialize views
	app.dashboardView = NewDashboardView(app)
	app.chatView = NewChatView(app, defaultModel)
	app.documentsView = NewDocumentsView(app)
	app.modelsView = NewModelsView(app, defaultModel)
	app.settingsView = NewSettingsView(app)
	app.actionsView = NewActionsView(app)

	// Add pages
	app.pages.AddPage("dashboard", app.dashboardView.GetPrimitive(), true, true)
	app.pages.AddPage("chat", app.chatView.GetPrimitive(), true, false)
	app.pages.AddPage("documents", app.documentsView.GetPrimitive(), true, false)
	app.pages.AddPage("models", app.modelsView.GetPrimitive(), true, false)
	app.pages.AddPage("settings", app.settingsView.GetPrimitive(), true, false)
	app.pages.AddPage("actions", app.actionsView.GetPrimitive(), true, false)
	app.pages.AddPage("actions", app.actionsView.GetPrimitive(), true, false)

	// Set root
	app.app.SetRoot(app.pages, true).SetFocus(app.pages)
	
	// Set focus to chat input when switching to chat page
	app.pages.SetChangedFunc(func() {
		name, _ := app.pages.GetFrontPage()
		if name == "chat" {
			app.app.SetFocus(app.chatView.input)
		}
	})

	// Set up global key handlers
	app.setupGlobalKeys()

	return app, nil
}

// setupGlobalKeys sets up global keyboard shortcuts
func (a *App) setupGlobalKeys() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Get the currently focused primitive
		focused := a.app.GetFocus()
		
		// Don't intercept keys if user is typing in an input field
		// Check if focused widget is an input field (InputField or similar)
		if focused != nil {
			// Get the primitive name/type to check if it's an input
			// tview.InputField and other input widgets will handle their own keys
			// We only want to intercept navigation keys when NOT in an input field
			
			// Check if it's a chat input field by checking the current page
			name, _ := a.pages.GetFrontPage()
			if name == "chat" {
				// In chat view, let the chat view handle input
				// Only intercept Esc and Ctrl+C
				switch event.Key() {
				case tcell.KeyCtrlC:
					a.app.Stop()
					return nil
				case tcell.KeyEsc:
					a.pages.SwitchToPage("dashboard")
					return nil
				}
				// Let all other keys pass through to chat input
				return event
			}
		}

		switch event.Key() {
		case tcell.KeyCtrlC:
			a.app.Stop()
			return nil
		case tcell.KeyEsc:
			name, _ := a.pages.GetFrontPage()
			if name == "dashboard" {
				a.app.Stop()
				return nil
			}
			// Return to dashboard
			a.pages.SwitchToPage("dashboard")
			return nil
		}

		// Number keys for navigation (only when not in input field)
		switch event.Rune() {
		case '0':
			a.pages.SwitchToPage("dashboard")
			return nil
		case '1':
			a.pages.SwitchToPage("chat")
			return nil
		case '2':
			a.pages.SwitchToPage("documents")
			return nil
		case '3':
			a.pages.SwitchToPage("models")
			return nil
		case '4':
			a.pages.SwitchToPage("settings")
			return nil
		case '5':
			a.pages.SwitchToPage("actions")
			return nil
		}

		return event
	})
}

// Run starts the TUI application
func (a *App) Run() error {
	return a.app.Run()
}
