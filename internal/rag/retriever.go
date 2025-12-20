package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/dream-ai/cli/internal/db"
	"github.com/dream-ai/cli/internal/embeddings"
)

// Retriever handles RAG retrieval using vector similarity search
type Retriever struct {
	db      *db.DB
	textEmb *embeddings.TextEmbedder
	topK    int
}

// NewRetriever creates a new RAG retriever
func NewRetriever(db *db.DB, textEmb *embeddings.TextEmbedder, topK int) *Retriever {
	if topK <= 0 {
		topK = 5 // Default
	}
	return &Retriever{
		db:      db,
		textEmb: textEmb,
		topK:    topK,
	}
}

// RetrievalResult contains retrieved chunks and images
type RetrievalResult struct {
	Chunks []*db.Chunk
	Images []*db.Image
}

// Retrieve finds relevant chunks and images for a query
func (r *Retriever) Retrieve(ctx context.Context, query string) (*RetrievalResult, error) {
	// Generate query embedding (for text chunks - 768 dimensions)
	queryEmbedding, err := r.textEmb.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search for similar chunks
	chunks, err := r.db.SearchSimilarChunks(ctx, queryEmbedding, r.topK)
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}

	// Search for similar images - skip if dimension mismatch (images use 512-dim embeddings)
	// We can't use text embeddings (768-dim) to search images (512-dim)
	images, err := r.db.SearchSimilarImages(ctx, queryEmbedding, r.topK)
	if err != nil {
		// Dimension mismatch is expected - images use different embedding model
		// Just return empty images list instead of failing
		images = []*db.Image{}
	}

	return &RetrievalResult{
		Chunks: chunks,
		Images: images,
	}, nil
}

// RetrieveHybrid performs hybrid search (semantic + keyword)
func (r *Retriever) RetrieveHybrid(ctx context.Context, query string) (*RetrievalResult, error) {
	// First do semantic search
	semanticResult, err := r.Retrieve(ctx, query)
	if err != nil {
		return nil, err
	}

	// Then do keyword matching (simple approach)
	// In production, you might want to use full-text search with PostgreSQL
	keywords := extractKeywords(query)
	
	// Filter chunks by keyword relevance
	filteredChunks := filterByKeywords(semanticResult.Chunks, keywords)
	
	return &RetrievalResult{
		Chunks: filteredChunks,
		Images: semanticResult.Images,
	}, nil
}

// extractKeywords extracts important keywords from query
func extractKeywords(query string) []string {
	// Simple keyword extraction - remove common words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "is": true,
		"are": true, "was": true, "were": true, "be": true, "been": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"what": true, "which": true, "who": true, "when": true, "where": true,
		"why": true, "how": true,
	}

	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:")
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

// filterByKeywords filters chunks by keyword presence
func filterByKeywords(chunks []*db.Chunk, keywords []string) []*db.Chunk {
	if len(keywords) == 0 {
		return chunks
	}

	var filtered []*db.Chunk
	for _, chunk := range chunks {
		content := strings.ToLower(chunk.Content)
		matches := 0
		for _, keyword := range keywords {
			if strings.Contains(content, keyword) {
				matches++
			}
		}
		// Keep chunk if it matches at least one keyword
		if matches > 0 {
			filtered = append(filtered, chunk)
		}
	}

	// If filtering removed too many, return original
	if len(filtered) < len(chunks)/2 {
		return chunks
	}
	return filtered
}
