package rag

import (
	"fmt"
	"strings"
)

// ContextBuilder builds context for LLM from retrieval results
type ContextBuilder struct {
	maxTokens int
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(maxTokens int) *ContextBuilder {
	if maxTokens <= 0 {
		maxTokens = 2000 // Default
	}
	return &ContextBuilder{
		maxTokens: maxTokens,
	}
}

// BuildContext creates a formatted context string from retrieval results
func (cb *ContextBuilder) BuildContext(result *RetrievalResult) string {
	var parts []string

	// Add text chunks
	if len(result.Chunks) > 0 {
		parts = append(parts, "## Relevant Text Excerpts:")
		for i, chunk := range result.Chunks {
			parts = append(parts, fmt.Sprintf("\n### Excerpt %d:", i+1))
			parts = append(parts, chunk.Content)
			parts = append(parts, "")
		}
	}

	// Add image information
	if len(result.Images) > 0 {
		parts = append(parts, "## Relevant Images:")
		for i, img := range result.Images {
			parts = append(parts, fmt.Sprintf("\n### Image %d:", i+1))
			if img.Caption != "" {
				parts = append(parts, fmt.Sprintf("Caption: %s", img.Caption))
			}
			parts = append(parts, fmt.Sprintf("Source: %s", img.FilePath))
			parts = append(parts, "")
		}
	}

	context := strings.Join(parts, "\n")
	
	// Truncate if too long (simple token estimation: ~4 chars per token)
	maxChars := cb.maxTokens * 4
	if len(context) > maxChars {
		context = context[:maxChars] + "\n\n[Context truncated...]"
	}

	return context
}

// BuildPrompt creates a complete prompt with context and user query
func (cb *ContextBuilder) BuildPrompt(context, userQuery string) string {
	var parts []string

	parts = append(parts, "You are an expert in dream interpretation and symbolic analysis.")
	parts = append(parts, "You have access to a knowledge base of symbols, dream meanings, and interpretations.")
	parts = append(parts, "")
	
	if context != "" {
		parts = append(parts, "## Knowledge Base Context:")
		parts = append(parts, context)
		parts = append(parts, "")
	}

	parts = append(parts, "## User Question:")
	parts = append(parts, userQuery)
	parts = append(parts, "")
	parts = append(parts, "Please provide a thoughtful, detailed response based on the context provided above.")
	parts = append(parts, "If the context doesn't contain relevant information, you can draw from your general knowledge,")
	parts = append(parts, "but please indicate when you're doing so.")

	return strings.Join(parts, "\n")
}

// GetChunkIDs extracts chunk IDs from retrieval result
func GetChunkIDs(result *RetrievalResult) []string {
	ids := make([]string, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		ids = append(ids, chunk.ID.String())
	}
	return ids
}

// GetImageIDs extracts image IDs from retrieval result
func GetImageIDs(result *RetrievalResult) []string {
	ids := make([]string, 0, len(result.Images))
	for _, img := range result.Images {
		ids = append(ids, img.ID.String())
	}
	return ids
}
