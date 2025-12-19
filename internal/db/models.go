package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// Document represents a processed document
type Document struct {
	ID         uuid.UUID
	FilePath   string
	FileHash   string
	FileType   string
	ProcessedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Chunk represents a text chunk with embedding
type Chunk struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	ChunkIndex int
	Content    string
	Embedding  *pgvector.Vector
	CreatedAt  time.Time
}

// Image represents an image with caption and embedding
type Image struct {
	ID         uuid.UUID
	DocumentID uuid.UUID
	ImageIndex int
	FilePath   string
	Caption    string
	Embedding  *pgvector.Vector
	CreatedAt  time.Time
}

// Conversation represents a chat interaction
type Conversation struct {
	ID              uuid.UUID
	UserMessage     string
	AssistantMessage string
	ModelName       string
	ContextChunkIDs []uuid.UUID
	ContextImageIDs []uuid.UUID
	CreatedAt       time.Time
}
