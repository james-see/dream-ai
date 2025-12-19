package documents

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/dream-ai/cli/internal/db"
	"github.com/dream-ai/cli/internal/embeddings"
)

// Processor handles document processing with incremental updates
type Processor struct {
	db         *db.DB
	textEmb    *embeddings.TextEmbedder
	imageEmb   *embeddings.ImageEmbedder
	pdfParser  *PDFParser
	epubParser *EPUBParser
	chunkSize  int
	chunkOverlap int
}

// NewProcessor creates a new document processor
func NewProcessor(
	db *db.DB,
	textEmb *embeddings.TextEmbedder,
	imageEmb *embeddings.ImageEmbedder,
	imageDir string,
	chunkSize, chunkOverlap int,
) *Processor {
	return &Processor{
		db:          db,
		textEmb:     textEmb,
		imageEmb:    imageEmb,
		pdfParser:   NewPDFParser(imageDir),
		epubParser:  NewEPUBParser(imageDir),
		chunkSize:   chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// ProcessDocument processes a document if it's new or changed
func (p *Processor) ProcessDocument(ctx context.Context, filePath string) error {
	// Compute file hash
	hash, err := computeFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	// Check if document already processed
	existingDoc, err := p.db.GetDocumentByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to check existing document: %w", err)
	}

	if existingDoc != nil {
		// Document already processed, skip
		return nil
	}

	// Determine file type
	fileType := strings.ToLower(filepath.Ext(filePath))
	if fileType == ".pdf" {
		fileType = "pdf"
	} else if fileType == ".epub" {
		fileType = "epub"
	} else {
		return fmt.Errorf("unsupported file type: %s", fileType)
	}

	// Create document record
	doc, err := p.db.CreateDocument(ctx, filePath, hash, fileType)
	if err != nil {
		return fmt.Errorf("failed to create document record: %w", err)
	}

	// Parse document
	var parsed *ParsedDocument
	if fileType == "pdf" {
		parsed, err = p.pdfParser.Parse(filePath)
	} else {
		parsed, err = p.epubParser.Parse(filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to parse document: %w", err)
	}

	// Process text chunks
	if err := p.processTextChunks(ctx, doc.ID, parsed.Text); err != nil {
		return fmt.Errorf("failed to process text chunks: %w", err)
	}

	// Process images (non-blocking - continue even if image processing fails)
	if err := p.processImages(ctx, doc.ID, parsed.Images); err != nil {
		// Log error but don't fail document processing
		fmt.Printf("Warning: failed to process images: %v\n", err)
	}

	// Mark document as processed
	if err := p.db.UpdateDocumentProcessed(ctx, doc.ID); err != nil {
		return fmt.Errorf("failed to update processed timestamp: %w", err)
	}

	return nil
}

// processTextChunks splits text into chunks and generates embeddings
func (p *Processor) processTextChunks(ctx context.Context, docID uuid.UUID, text string) error {
	chunks := p.splitText(text)
	if len(chunks) == 0 {
		return nil
	}

	// Generate embeddings for all chunks
	chunkData := make([]*db.Chunk, 0, len(chunks))
	for i, chunkText := range chunks {
		embedding, err := p.textEmb.Embed(ctx, chunkText)
		if err != nil {
			return fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}

		chunkData = append(chunkData, &db.Chunk{
			ID:         uuid.New(),
			DocumentID: docID,
			ChunkIndex: i,
			Content:    chunkText,
			Embedding:  embedding,
		})
	}

	// Insert chunks in batch
	return p.db.InsertChunksBatch(ctx, chunkData)
}

// processImages processes images with CLIP2 captioning and embeddings
func (p *Processor) processImages(ctx context.Context, docID uuid.UUID, images []ImageData) error {
	if len(images) == 0 {
		return nil
	}

	imageData := make([]*db.Image, 0, len(images))
	for _, img := range images {
		// Generate caption and embedding
		caption, embedding, err := p.imageEmb.ProcessImage(ctx, img.FilePath)
		if err != nil {
			// Log error but continue with other images
			fmt.Printf("Warning: failed to process image %s: %v\n", img.FilePath, err)
			continue
		}

		imageData = append(imageData, &db.Image{
			ID:         uuid.New(),
			DocumentID: docID,
			ImageIndex: img.Index,
			FilePath:   img.FilePath,
			Caption:    caption,
			Embedding:  embedding,
		})
	}

	if len(imageData) > 0 {
		return p.db.InsertImagesBatch(ctx, imageData)
	}
	return nil
}

// splitText splits text into chunks with overlap
func (p *Processor) splitText(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	currentChunk := []string{}
	currentSize := 0

	for _, word := range words {
		wordSize := len(word) + 1 // +1 for space
		if currentSize+wordSize > p.chunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, " "))
			
			// Keep overlap words for next chunk
			overlapWords := len(currentChunk) * p.chunkOverlap / 100
			if overlapWords > 0 && overlapWords < len(currentChunk) {
				currentChunk = currentChunk[len(currentChunk)-overlapWords:]
				currentSize = len(strings.Join(currentChunk, " "))
			} else {
				currentChunk = []string{}
				currentSize = 0
			}
		}
		currentChunk = append(currentChunk, word)
		currentSize += wordSize
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, " "))
	}

	return chunks
}

// computeFileHash computes SHA256 hash of a file
func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
