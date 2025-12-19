package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pgvector/pgvector-go"
)

// TextEmbedder generates text embeddings using Ollama
type TextEmbedder struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewTextEmbedder creates a new text embedder
func NewTextEmbedder(baseURL, model string) *TextEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text" // Default embedding model
	}
	return &TextEmbedder{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{},
	}
}

// Embed generates an embedding for the given text
func (e *TextEmbedder) Embed(ctx context.Context, text string) (*pgvector.Vector, error) {
	// Clean and prepare text
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Prepare request
	url := fmt.Sprintf("%s/api/embeddings", e.baseURL)
	payload := map[string]interface{}{
		"model": e.model,
		"prompt": text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	// Convert to pgvector
	vec := pgvector.NewVector(result.Embedding)
	return &vec, nil
}

// EmbedBatch generates embeddings for multiple texts
func (e *TextEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]*pgvector.Vector, error) {
	embeddings := make([]*pgvector.Vector, 0, len(texts))
	for _, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text: %w", err)
		}
		embeddings = append(embeddings, emb)
	}
	return embeddings, nil
}
