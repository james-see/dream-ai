package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// ModelInfo represents information about an Ollama model
type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// ListModelsResponse represents the response from listing models
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ModelSelector handles model selection logic
type ModelSelector struct {
	client *Client
}

// NewModelSelector creates a new model selector
func NewModelSelector(client *Client) *ModelSelector {
	return &ModelSelector{client: client}
}

// ListModels lists all available Ollama models
func (ms *ModelSelector) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", ms.client.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ms.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error: %d - %s", resp.StatusCode, string(body))
	}

	var result ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Models, nil
}

// SelectBestModel selects the best model for reasoning/thinking tasks
func (ms *ModelSelector) SelectBestModel(ctx context.Context) (string, error) {
	models, err := ms.ListModels(ctx)
	if err != nil {
		return "", err
	}

	if len(models) == 0 {
		return "", fmt.Errorf("no models available")
	}

	// Priority list for reasoning models (based on best practices)
	priorityModels := []string{
		"llama3.2",      // Strong reasoning capabilities
		"llama3.1",      // Good reasoning
		"qwen2.5",       // Good for analysis
		"mistral",       // Strong general performance
		"llama3",        // Fallback to llama3
		"llama2",        // Older but still good
	}

	// Try to find a model in priority order
	for _, priority := range priorityModels {
		for _, model := range models {
			modelName := strings.ToLower(model.Name)
			if strings.Contains(modelName, priority) {
				return model.Name, nil
			}
		}
	}

	// If no priority model found, return the largest model (usually best)
	sort.Slice(models, func(i, j int) bool {
		return models[i].Size > models[j].Size
	})

	return models[0].Name, nil
}

// GetDefaultModel returns the default model or selects the best one
func (ms *ModelSelector) GetDefaultModel(ctx context.Context, defaultModel string) (string, error) {
	if defaultModel != "" {
		// Verify the model exists
		models, err := ms.ListModels(ctx)
		if err != nil {
			return "", err
		}
		
		for _, model := range models {
			if model.Name == defaultModel {
				return defaultModel, nil
			}
		}
		// Model not found, fall through to select best
	}

	return ms.SelectBestModel(ctx)
}
