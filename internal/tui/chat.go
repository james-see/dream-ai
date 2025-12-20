package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dream-ai/cli/internal/ollama"
	"github.com/dream-ai/cli/internal/rag"
	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
)

// ChatView handles the chat interface using tview
type ChatView struct {
	app      *App
	flex     *tview.Flex
	messages *tview.TextView
	input    *tview.TextArea
	model    string

	messagesData []Message
	loading      bool
}

// Message represents a chat message
type Message struct {
	Role    string
	Content string
	Sources []string // Document file paths used as sources
}

// NewChatView creates a new chat view
func NewChatView(app *App, defaultModel string) *ChatView {
	cv := &ChatView{
		app:          app,
		model:        defaultModel,
		messagesData: []Message{},
	}

	// Create messages text view
	cv.messages = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetScrollable(true)
	cv.messages.SetBorder(true).SetTitle(" Chat ")

	// Create input text area (supports multi-line and wrapping)
	cv.input = tview.NewTextArea().
		SetPlaceholder("Ask about dreams or symbols... (Ctrl+Enter to send)").
		SetWrap(true)

	// Handle Ctrl+Enter to send message
	cv.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter && event.Modifiers()&tcell.ModCtrl != 0 {
			cv.sendMessage()
			return nil
		}
		return event
	})

	// Create input container with label
	inputFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewTextView().SetText("> ").SetDynamicColors(false), 1, 0, false).
		AddItem(cv.input, 0, 1, true)
	inputFlex.SetBorder(false)

	// Create main flex layout
	cv.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(cv.messages, 0, 1, false).
		AddItem(inputFlex, 3, 0, true)

	return cv
}

// GetPrimitive returns the tview primitive
func (cv *ChatView) GetPrimitive() tview.Primitive {
	return cv.flex
}

// sendMessage sends a message and gets a response
func (cv *ChatView) sendMessage() {
	userMsg := cv.input.GetText()
	if strings.TrimSpace(userMsg) == "" || cv.loading {
		return
	}

	// Clear input
	cv.input.SetText("", false)
	cv.loading = true

	// Add user message
	cv.messagesData = append(cv.messagesData, Message{
		Role:    "user",
		Content: userMsg,
	})
	cv.renderMessages()

	// Add placeholder for assistant message
	cv.messagesData = append(cv.messagesData, Message{
		Role:    "assistant",
		Content: "[yellow]Thinking...",
	})
	cv.renderMessages()

	// Generate response asynchronously
	go cv.generateResponse(userMsg)
}

// generateResponse generates a response using RAG
func (cv *ChatView) generateResponse(query string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Retrieve relevant context
	result, err := cv.app.retriever.Retrieve(ctx, query)
	if err != nil {
		cv.app.app.QueueUpdateDraw(func() {
			cv.messagesData[len(cv.messagesData)-1].Content = fmt.Sprintf("[red]Error: %v", err)
			cv.loading = false
			cv.renderMessages()
		})
		return
	}

	// Build context
	context := cv.app.contextBuilder.BuildContext(result)
	prompt := cv.app.contextBuilder.BuildPrompt(context, query)

	// Generate response
	response, err := cv.app.ollamaClient.Generate(ctx, &ollama.GenerateRequest{
		Model:  cv.model,
		Prompt: prompt,
		Stream: false,
	})

	// Extract unique source documents from retrieval result
	sources := cv.extractSources(ctx, result)

	cv.app.app.QueueUpdateDraw(func() {
		if err != nil {
			cv.messagesData[len(cv.messagesData)-1].Content = fmt.Sprintf("[red]Error: %v", err)
			cv.messagesData[len(cv.messagesData)-1].Sources = nil
		} else {
			cv.messagesData[len(cv.messagesData)-1].Content = response
			cv.messagesData[len(cv.messagesData)-1].Sources = sources
		}
		cv.loading = false
		cv.renderMessages()
	})
}

// renderMessages updates the messages display
func (cv *ChatView) renderMessages() {
	var lines []string
	for _, msg := range cv.messagesData {
		var prefix string
		var color string
		if msg.Role == "user" {
			prefix = "You: "
			color = "[cyan]"
			lines = append(lines, fmt.Sprintf("%s%s%s[white]", color, prefix, msg.Content))
		} else {
			prefix = "AI: "
			color = "[white]"
			// Convert markdown to tview format and add content
			formattedContent := cv.formatMarkdown(msg.Content)
			lines = append(lines, fmt.Sprintf("%s%s%s[white]", color, prefix, formattedContent))

			// Add sources section if available
			if len(msg.Sources) > 0 {
				lines = append(lines, "")
				lines = append(lines, "[yellow]Sources Found:[white]")
				for _, source := range msg.Sources {
					lines = append(lines, fmt.Sprintf("  [gray]- %s[white]", source))
				}
			}
		}
	}
	cv.messages.SetText(strings.Join(lines, "\n"))
	cv.messages.ScrollToEnd()
}

// formatMarkdown converts markdown syntax to tview color codes
func (cv *ChatView) formatMarkdown(text string) string {
	// First, handle headers and lists (process line by line)
	lines := strings.Split(text, "\n")
	var formattedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Process headers first (before bold processing)
		if strings.HasPrefix(trimmed, "### ") {
			// Level 3 header
			headerText := strings.TrimPrefix(trimmed, "### ")
			formattedLines = append(formattedLines, fmt.Sprintf("[yellow]%s[white]", headerText))
			continue
		} else if strings.HasPrefix(trimmed, "## ") {
			// Level 2 header
			headerText := strings.TrimPrefix(trimmed, "## ")
			formattedLines = append(formattedLines, fmt.Sprintf("[yellow]%s[white]", headerText))
			continue
		} else if strings.HasPrefix(trimmed, "# ") {
			// Level 1 header
			headerText := strings.TrimPrefix(trimmed, "# ")
			formattedLines = append(formattedLines, fmt.Sprintf("[yellow]%s[white]", headerText))
			continue
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			// Bullet points - process bold within bullets
			bulletText := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			formattedBullet := cv.processBold(bulletText)
			formattedLines = append(formattedLines, fmt.Sprintf("  [gray]•[white] %s", formattedBullet))
			continue
		}

		// Process bold text in regular lines
		formattedLine := cv.processBold(line)
		formattedLines = append(formattedLines, formattedLine)
	}

	return strings.Join(formattedLines, "\n")
}

// processBold converts **bold** markdown to [yellow]bold[white] tview format
func (cv *ChatView) processBold(text string) string {
	// Find all ** pairs and replace them
	var result strings.Builder
	i := 0
	boldOpen := false

	for i < len(text) {
		if i < len(text)-1 && text[i] == '*' && text[i+1] == '*' {
			if boldOpen {
				result.WriteString("[white]")
			} else {
				result.WriteString("[yellow]")
			}
			boldOpen = !boldOpen
			i += 2
		} else {
			result.WriteByte(text[i])
			i++
		}
	}

	// If we ended with an open bold tag, close it
	if boldOpen {
		result.WriteString("[white]")
	}

	return result.String()
}

// extractSources extracts unique document file paths from retrieval result
func (cv *ChatView) extractSources(ctx context.Context, result *rag.RetrievalResult) []string {
	sourceMap := make(map[string]bool)
	var sources []string

	// Get document IDs from chunks
	docIDs := make(map[uuid.UUID]bool)
	for _, chunk := range result.Chunks {
		docIDs[chunk.DocumentID] = true
	}
	for _, img := range result.Images {
		docIDs[img.DocumentID] = true
	}

	// Fetch document file paths
	for docID := range docIDs {
		doc, err := cv.app.db.GetDocumentByID(ctx, docID)
		if err == nil && doc != nil {
			filePath := doc.FilePath
			if !sourceMap[filePath] {
				sourceMap[filePath] = true
				sources = append(sources, filepath.Base(filePath))
			}
		}
	}

	return sources
}
