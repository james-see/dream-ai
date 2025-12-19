package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dream-ai/cli/config"
	"github.com/dream-ai/cli/internal/db"
	"github.com/dream-ai/cli/internal/documents"
	"github.com/dream-ai/cli/internal/embeddings"
	"github.com/dream-ai/cli/internal/ollama"
	"github.com/dream-ai/cli/internal/rag"
)

// ViewType represents different TUI views
type ViewType int

const (
	ViewChat ViewType = iota
	ViewDocuments
	ViewModels
	ViewSettings
)

// App represents the main TUI application
type App struct {
	db          *db.DB
	processor   *documents.Processor
	retriever   *rag.Retriever
	contextBuilder *rag.ContextBuilder
	ollamaClient *ollama.Client
	modelSelector *ollama.ModelSelector
	textEmb     *embeddings.TextEmbedder
	imageEmb    *embeddings.ImageEmbedder
	cfg         *config.Config
	
	currentView ViewType
	chatView    *ChatView
	docsView    *DocumentsView
	modelsView  *ModelsView
	settingsView *SettingsView
	
	width  int
	height int
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
	retriever := rag.NewRetriever(database, textEmb, cfg.Processing.TopK)
	contextBuilder := rag.NewContextBuilder(2000)

	// Initialize Ollama
	ollamaClient := ollama.NewClient(cfg.Ollama.BaseURL)
	modelSelector := ollama.NewModelSelector(ollamaClient)

	// Get default model
	ctx := context.Background()
	defaultModel, err := modelSelector.GetDefaultModel(ctx, cfg.Ollama.DefaultModel)
	if err != nil {
		return nil, fmt.Errorf("failed to get default model: %w", err)
	}

	app := &App{
		db:            database,
		processor:    processor,
		retriever:    retriever,
		contextBuilder: contextBuilder,
		ollamaClient:  ollamaClient,
		modelSelector: modelSelector,
		textEmb:       textEmb,
		imageEmb:      imageEmb,
		cfg:           cfg,
		currentView:   ViewChat,
	}

	// Initialize views
	app.chatView = NewChatView(app, defaultModel)
	app.docsView = NewDocumentsView(app)
	app.modelsView = NewModelsView(app, defaultModel)
	app.settingsView = NewSettingsView(app)

	return app, nil
}

// Init initializes the TUI
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.chatView.Init(),
		a.docsView.Init(),
		a.modelsView.Init(),
		a.settingsView.Init(),
	)
}

// Update handles updates
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "1":
			a.currentView = ViewChat
			return a, nil
		case "2":
			a.currentView = ViewDocuments
			return a, nil
		case "3":
			a.currentView = ViewModels
			return a, nil
		case "4":
			a.currentView = ViewSettings
			return a, nil
		}
	}

	// Delegate to current view
	switch a.currentView {
	case ViewChat:
		return a.chatView.Update(msg)
	case ViewDocuments:
		return a.docsView.Update(msg)
	case ViewModels:
		return a.modelsView.Update(msg)
	case ViewSettings:
		return a.settingsView.Update(msg)
	}

	return a, nil
}

// View renders the TUI
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Initializing..."
	}

	var view string
	switch a.currentView {
	case ViewChat:
		view = a.chatView.View()
	case ViewDocuments:
		view = a.docsView.View()
	case ViewModels:
		view = a.modelsView.View()
	case ViewSettings:
		view = a.settingsView.View()
	}

	// Add header and footer
	header := a.renderHeader()
	footer := a.renderFooter()
	
	contentHeight := a.height - lipgloss.Height(header) - lipgloss.Height(footer)
	content := lipgloss.NewStyle().
		Width(a.width).
		Height(contentHeight).
		Render(view)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// renderHeader renders the application header
func (a *App) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("Dream AI - Symbol & Dream Interpretation")

	viewTabs := lipgloss.JoinHorizontal(lipgloss.Left,
		a.renderTab("Chat", ViewChat == a.currentView),
		a.renderTab("Documents", ViewDocuments == a.currentView),
		a.renderTab("Models", ViewModels == a.currentView),
		a.renderTab("Settings", ViewSettings == a.currentView),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(a.width).Render(title),
		lipgloss.NewStyle().Width(a.width).Render(viewTabs),
		lipgloss.NewStyle().Width(a.width).Border(lipgloss.NormalBorder(), false, false, true, false).Render(""),
	)
}

// renderTab renders a tab
func (a *App) renderTab(name string, active bool) string {
	style := lipgloss.NewStyle().Padding(0, 1)
	if active {
		style = style.Bold(true).Foreground(lipgloss.Color("205"))
	} else {
		style = style.Foreground(lipgloss.Color("240"))
	}
	return style.Render(name)
}

// renderFooter renders the application footer
func (a *App) renderFooter() string {
	help := "Press 1-4 to switch views | q to quit"
	return lipgloss.NewStyle().
		Width(a.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		Foreground(lipgloss.Color("240")).
		Render(help)
}

// Close cleans up resources
func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}
