package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dream-ai/cli/internal/ollama"
	"github.com/google/uuid"
)

// ChatView handles the chat interface
type ChatView struct {
	app         *App
	model       string
	messages    []Message
	input       string
	cursor      int
	width       int
	height      int
	loading     bool
	scrollOffset int
}

// Message represents a chat message
type Message struct {
	ID       string
	Role     string // "user" or "assistant"
	Content  string
	Streaming bool
}

// NewChatView creates a new chat view
func NewChatView(app *App, defaultModel string) *ChatView {
	return &ChatView{
		app:      app,
		model:    defaultModel,
		messages: []Message{},
		width:    80,
		height:   24,
	}
}

// Init initializes the chat view
func (cv *ChatView) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (cv *ChatView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cv.width = msg.Width
		cv.height = msg.Height
		return cv, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if cv.input != "" && !cv.loading {
				return cv, cv.sendMessage()
			}
		case "backspace":
			if len(cv.input) > 0 {
				cv.input = cv.input[:len(cv.input)-1]
			}
		case "ctrl+l":
			cv.messages = []Message{}
			return cv, nil
		default:
			if !cv.loading && msg.Type == tea.KeyRunes {
				cv.input += msg.String()
			}
		}
	case streamingMsg:
		// Update streaming message
		if len(cv.messages) > 0 && cv.messages[len(cv.messages)-1].Streaming {
			cv.messages[len(cv.messages)-1].Content += msg.text
			return cv, nil
		}
	case messageCompleteMsg:
		cv.loading = false
		if len(cv.messages) > 0 {
			cv.messages[len(cv.messages)-1].Streaming = false
		}
		return cv, nil
	}
	return cv, nil
}

// View renders the chat view
func (cv *ChatView) View() string {
	var lines []string

	// Render messages
	availableHeight := cv.height - 4 // Leave space for input
	startIdx := 0
	if len(cv.messages) > availableHeight {
		startIdx = len(cv.messages) - availableHeight
	}

	for i := startIdx; i < len(cv.messages); i++ {
		msg := cv.messages[i]
		lines = append(lines, cv.renderMessage(msg))
	}

	// Render input
	inputLine := lipgloss.NewStyle().
		Width(cv.width - 2).
		Render(fmt.Sprintf("> %s", cv.input))

	if cv.loading {
		inputLine += " [thinking...]"
	}

	lines = append(lines, "")
	lines = append(lines, inputLine)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderMessage renders a single message
func (cv *ChatView) renderMessage(msg Message) string {
	var style lipgloss.Style
	if msg.Role == "user" {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)
	} else {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	}

	rolePrefix := "You: "
	if msg.Role == "assistant" {
		rolePrefix = "AI: "
	}

	content := style.Render(rolePrefix + msg.Content)
	return lipgloss.NewStyle().Width(cv.width - 2).Render(content)
}

// sendMessage sends a message and gets a response
func (cv *ChatView) sendMessage() tea.Cmd {
	userMsg := cv.input
	cv.input = ""
	cv.loading = true

	// Add user message
	cv.messages = append(cv.messages, Message{
		ID:      uuid.New().String(),
		Role:    "user",
		Content: userMsg,
	})

	// Add placeholder for assistant message
	assistantMsg := Message{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   "",
		Streaming: true,
	}
	cv.messages = append(cv.messages, assistantMsg)

	return func() tea.Msg {
		return cv.generateResponse(userMsg)
	}
}

// generateResponse generates a response using RAG
func (cv *ChatView) generateResponse(query string) tea.Msg {
	ctx := context.Background()

	// Retrieve relevant context
	result, err := cv.app.retriever.Retrieve(ctx, query)
	if err != nil {
		return messageCompleteMsg{error: err}
	}

	// Build context
	context := cv.app.contextBuilder.BuildContext(result)
	prompt := cv.app.contextBuilder.BuildPrompt(context, query)

	// Generate response with streaming
	var response strings.Builder
	err = cv.app.ollamaClient.GenerateStream(ctx, &ollama.GenerateRequest{
		Model:  cv.model,
		Prompt: prompt,
		Stream: true,
	}, func(chunk string) {
		response.WriteString(chunk)
		// In a real implementation, you'd send streaming updates here
	})

	if err != nil {
		return messageCompleteMsg{error: err}
	}

	// Update the last message
	if len(cv.messages) > 0 {
		cv.messages[len(cv.messages)-1].Content = response.String()
		cv.messages[len(cv.messages)-1].Streaming = false
	}

	return messageCompleteMsg{}
}

// streamingMsg represents a streaming text chunk
type streamingMsg struct {
	text string
}

// messageCompleteMsg signals that message generation is complete
type messageCompleteMsg struct {
	error error
}
