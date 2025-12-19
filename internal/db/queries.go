package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// GetDocumentByHash retrieves a document by its file hash
func (db *DB) GetDocumentByHash(ctx context.Context, hash string) (*Document, error) {
	var doc Document
	err := db.pool.QueryRow(ctx,
		`SELECT id, file_path, file_hash, file_type, processed_at, created_at, updated_at
		 FROM documents WHERE file_hash = $1`,
		hash,
	).Scan(
		&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
		&doc.ProcessedAt, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document by hash: %w", err)
	}
	return &doc, nil
}

// CreateDocument creates a new document record
func (db *DB) CreateDocument(ctx context.Context, filePath, fileHash, fileType string) (*Document, error) {
	var doc Document
	err := db.pool.QueryRow(ctx,
		`INSERT INTO documents (file_path, file_hash, file_type)
		 VALUES ($1, $2, $3)
		 RETURNING id, file_path, file_hash, file_type, processed_at, created_at, updated_at`,
		filePath, fileHash, fileType,
	).Scan(
		&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
		&doc.ProcessedAt, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}
	return &doc, nil
}

// UpdateDocumentProcessed updates the processed_at timestamp
func (db *DB) UpdateDocumentProcessed(ctx context.Context, docID uuid.UUID) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE documents SET processed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		docID,
	)
	return err
}

// InsertChunk inserts a text chunk with embedding
func (db *DB) InsertChunk(ctx context.Context, chunk *Chunk) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO chunks (id, document_id, chunk_index, content, embedding)
		 VALUES ($1, $2, $3, $4, $5)`,
		chunk.ID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content, chunk.Embedding,
	)
	return err
}

// InsertChunksBatch inserts multiple chunks in a transaction
func (db *DB) InsertChunksBatch(ctx context.Context, chunks []*Chunk) error {
	batch := &pgx.Batch{}
	for _, chunk := range chunks {
		batch.Queue(
			`INSERT INTO chunks (id, document_id, chunk_index, content, embedding)
			 VALUES ($1, $2, $3, $4, $5)`,
			chunk.ID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content, chunk.Embedding,
		)
	}
	br := db.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(chunks); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("failed to insert chunk %d: %w", i, err)
		}
	}
	return nil
}

// InsertImage inserts an image with caption and embedding
func (db *DB) InsertImage(ctx context.Context, img *Image) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO images (id, document_id, image_index, file_path, caption, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		img.ID, img.DocumentID, img.ImageIndex, img.FilePath, img.Caption, img.Embedding,
	)
	return err
}

// InsertImagesBatch inserts multiple images in a transaction
func (db *DB) InsertImagesBatch(ctx context.Context, images []*Image) error {
	batch := &pgx.Batch{}
	for _, img := range images {
		batch.Queue(
			`INSERT INTO images (id, document_id, image_index, file_path, caption, embedding)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			img.ID, img.DocumentID, img.ImageIndex, img.FilePath, img.Caption, img.Embedding,
		)
	}
	br := db.pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(images); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("failed to insert image %d: %w", i, err)
		}
	}
	return nil
}

// SearchSimilarChunks finds similar chunks using vector similarity
func (db *DB) SearchSimilarChunks(ctx context.Context, embedding *pgvector.Vector, limit int) ([]*Chunk, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, document_id, chunk_index, content, embedding, created_at
		 FROM chunks
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		embedding, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		var chunk Chunk
		if err := rows.Scan(
			&chunk.ID, &chunk.DocumentID, &chunk.ChunkIndex,
			&chunk.Content, &chunk.Embedding, &chunk.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}
		chunks = append(chunks, &chunk)
	}
	return chunks, rows.Err()
}

// SearchSimilarImages finds similar images using vector similarity
func (db *DB) SearchSimilarImages(ctx context.Context, embedding *pgvector.Vector, limit int) ([]*Image, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, document_id, image_index, file_path, caption, embedding, created_at
		 FROM images
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		embedding, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search images: %w", err)
	}
	defer rows.Close()

	var images []*Image
	for rows.Next() {
		var img Image
		if err := rows.Scan(
			&img.ID, &img.DocumentID, &img.ImageIndex,
			&img.FilePath, &img.Caption, &img.Embedding, &img.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan image: %w", err)
		}
		images = append(images, &img)
	}
	return images, rows.Err()
}

// SaveConversation saves a conversation record
func (db *DB) SaveConversation(ctx context.Context, conv *Conversation) error {
	_, err := db.pool.Exec(ctx,
		`INSERT INTO conversations (id, user_message, assistant_message, model_name, context_chunk_ids, context_image_ids)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		conv.ID, conv.UserMessage, conv.AssistantMessage, conv.ModelName,
		conv.ContextChunkIDs, conv.ContextImageIDs,
	)
	return err
}

// GetAllDocuments retrieves all documents
func (db *DB) GetAllDocuments(ctx context.Context) ([]*Document, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, file_path, file_hash, file_type, processed_at, created_at, updated_at
		 FROM documents ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents: %w", err)
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
			&doc.ProcessedAt, &doc.CreatedAt, &doc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}
		docs = append(docs, &doc)
	}
	return docs, rows.Err()
}

// DeleteDocument deletes a document and its associated chunks/images
func (db *DB) DeleteDocument(ctx context.Context, docID uuid.UUID) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM documents WHERE id = $1`, docID)
	return err
}
