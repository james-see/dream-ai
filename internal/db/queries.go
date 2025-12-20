package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// GetDocumentByHash retrieves a document by its file hash
func (db *DB) GetDocumentByHash(ctx context.Context, hash string) (*Document, error) {
	var doc Document
	err := db.pool.QueryRow(ctx,
		`SELECT id, file_path, file_hash, file_type, processed_at, error_message, created_at, updated_at
		 FROM documents WHERE file_hash = $1`,
		hash,
	).Scan(
		&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
		&doc.ProcessedAt, &doc.ErrorMessage, &doc.CreatedAt, &doc.UpdatedAt,
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
		 RETURNING id, file_path, file_hash, file_type, processed_at, error_message, created_at, updated_at`,
		filePath, fileHash, fileType,
	).Scan(
		&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
		&doc.ProcessedAt, &doc.ErrorMessage, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}
	return &doc, nil
}

// UpdateDocumentProcessed updates the processed_at timestamp
func (db *DB) UpdateDocumentProcessed(ctx context.Context, docID uuid.UUID) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE documents SET processed_at = NOW(), error_message = NULL, updated_at = NOW() WHERE id = $1`,
		docID,
	)
	return err
}

// UpdateDocumentError updates the error_message for a document
func (db *DB) UpdateDocumentError(ctx context.Context, docID uuid.UUID, errorMsg string) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE documents SET error_message = $1, updated_at = NOW() WHERE id = $2`,
		errorMsg, docID,
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
// Note: This requires a 512-dim embedding (CLIP2), not 768-dim (text embeddings)
func (db *DB) SearchSimilarImages(ctx context.Context, embedding *pgvector.Vector, limit int) ([]*Image, error) {
	// Check embedding dimension - images use 512-dim, text uses 768-dim
	if embedding != nil && len(embedding.Slice()) != 512 {
		// Return empty result if dimension mismatch instead of error
		return []*Image{}, nil
	}

	rows, err := db.pool.Query(ctx,
		`SELECT id, document_id, image_index, file_path, caption, embedding, created_at
		 FROM images
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		embedding, limit,
	)
	if err != nil {
		// Check if it's a dimension mismatch error
		if strings.Contains(err.Error(), "different vector dimensions") {
			return []*Image{}, nil
		}
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

// GetDocumentByID retrieves a document by its ID
func (db *DB) GetDocumentByID(ctx context.Context, id uuid.UUID) (*Document, error) {
	var doc Document
	err := db.pool.QueryRow(ctx,
		`SELECT id, file_path, file_hash, file_type, processed_at, error_message, created_at, updated_at
		 FROM documents WHERE id = $1`,
		id,
	).Scan(
		&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileType,
		&doc.ProcessedAt, &doc.ErrorMessage, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document by ID: %w", err)
	}
	return &doc, nil
}

// GetAllDocuments retrieves all documents
func (db *DB) GetAllDocuments(ctx context.Context) ([]*Document, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, file_path, file_hash, file_type, processed_at, error_message, created_at, updated_at
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
			&doc.ProcessedAt, &doc.ErrorMessage, &doc.CreatedAt, &doc.UpdatedAt,
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

// GetImagesByDocument retrieves all images for a document
func (db *DB) GetImagesByDocument(ctx context.Context, docID uuid.UUID) ([]*Image, error) {
	rows, err := db.pool.Query(ctx,
		`SELECT id, document_id, image_index, file_path, caption, embedding, created_at
		 FROM images WHERE document_id = $1 ORDER BY image_index`,
		docID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get images: %w", err)
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

// UpdateImage updates an image with caption and embedding
func (db *DB) UpdateImage(ctx context.Context, imageID uuid.UUID, caption string, embedding *pgvector.Vector) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE images SET caption = $1, embedding = $2 WHERE id = $3`,
		caption, embedding, imageID,
	)
	return err
}

// GetStats retrieves statistics about the database
func (db *DB) GetStats(ctx context.Context) (totalChunks, totalImages, totalWords, totalPages, pagesWithImages int, err error) {
	// Get chunk count
	err = db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&totalChunks)
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("failed to get chunk count: %w", err)
	}

	// Get image count
	err = db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM images`).Scan(&totalImages)
	if err != nil {
		return totalChunks, 0, 0, 0, 0, fmt.Errorf("failed to get image count: %w", err)
	}

	// Estimate word count from chunks (rough estimate: ~5 chars per word)
	var totalChars int
	err = db.pool.QueryRow(ctx, `SELECT COALESCE(SUM(LENGTH(content)), 0) FROM chunks`).Scan(&totalChars)
	if err != nil {
		return totalChunks, totalImages, 0, 0, 0, fmt.Errorf("failed to get word count: %w", err)
	}
	totalWords = totalChars / 5 // Rough estimate

	// Estimate pages: count distinct documents and estimate pages per document
	// For PDFs: use chunk count as proxy (each page might generate multiple chunks)
	// For EPUBs: count HTML files processed
	var docCount int
	err = db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents`).Scan(&docCount)
	if err == nil && docCount > 0 {
		// Rough estimate: average 10 chunks per page for PDFs, 5 for EPUBs
		// This is a heuristic - actual page count would need to be tracked during parsing
		totalPages = totalChunks / 8 // Average estimate
		
		// Count documents that have images
		err = db.pool.QueryRow(ctx, 
			`SELECT COUNT(DISTINCT document_id) FROM images`).Scan(&pagesWithImages)
		if err != nil {
			pagesWithImages = 0
		}
		// This is actually documents with images, not pages - but it's a start
	}

	return totalChunks, totalImages, totalWords, totalPages, pagesWithImages, nil
}
