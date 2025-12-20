package documents

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gen2brain/go-fitz"
)

// ParsedDocument contains extracted text and images from a document
type ParsedDocument struct {
	Text   string
	Images []ImageData
}

// ImageData contains image file path and data
type ImageData struct {
	Index    int
	FilePath string
	Data     []byte
}

// Parser interface for document parsing
type Parser interface {
	Parse(filePath string) (*ParsedDocument, error)
}

// PDFParser parses PDF files
type PDFParser struct {
	imageDir string
}

// NewPDFParser creates a new PDF parser
func NewPDFParser(imageDir string) *PDFParser {
	return &PDFParser{imageDir: imageDir}
}

// Parse extracts text and images from a PDF file
func (p *PDFParser) Parse(filePath string) (*ParsedDocument, error) {
	doc, err := fitz.New(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	var textParts []string
	var images []ImageData
	imageIndex := 0

	// Extract text and images from each page
	for i := 0; i < doc.NumPage(); i++ {
		text, err := doc.Text(i)
		if err == nil && strings.TrimSpace(text) != "" {
			textParts = append(textParts, text)
		}
		
		// Extract page as image using ImagePNG (returns []byte PNG data)
		// Use 150 DPI for reasonable quality/size balance
		imgData, err := doc.ImagePNG(i, 150.0)
		if err == nil && len(imgData) > 0 {
			// Save page as image
			baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			// Sanitize baseName for filesystem
			baseName = strings.ReplaceAll(baseName, " ", "_")
			baseName = strings.ReplaceAll(baseName, "/", "_")
			imgPath := filepath.Join(p.imageDir, fmt.Sprintf("pdf_%s_page%d.png", baseName, i))
			
			if err := os.WriteFile(imgPath, imgData, 0644); err == nil {
				images = append(images, ImageData{
					Index:    imageIndex,
					FilePath: imgPath,
					Data:     imgData,
				})
				imageIndex++
			}
		}
	}

	return &ParsedDocument{
		Text:   strings.Join(textParts, "\n\n"),
		Images: images,
	}, nil
}

// EPUBParser parses EPUB files using go-fitz (which supports EPUB)
type EPUBParser struct {
	imageDir string
}

// NewEPUBParser creates a new EPUB parser
func NewEPUBParser(imageDir string) *EPUBParser {
	return &EPUBParser{imageDir: imageDir}
}

// Parse extracts text and images from an EPUB file using go-fitz
func (p *EPUBParser) Parse(filePath string) (*ParsedDocument, error) {
	// Use go-fitz for EPUB parsing (it supports EPUB)
	doc, err := fitz.New(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB: %w", err)
	}
	defer doc.Close()

	var textParts []string
	var images []ImageData
	imageIndex := 0

	// Extract text and images from each page
	for i := 0; i < doc.NumPage(); i++ {
		text, err := doc.Text(i)
		if err == nil && strings.TrimSpace(text) != "" {
			textParts = append(textParts, text)
		}
		
		// Extract page as image using ImagePNG (returns []byte)
		// Use 150 DPI for reasonable quality/size balance
		imgData, err := doc.ImagePNG(i, 150.0)
		if err == nil && len(imgData) > 0 {
			// Save page as image
			baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			// Sanitize baseName for filesystem
			baseName = strings.ReplaceAll(baseName, " ", "_")
			baseName = strings.ReplaceAll(baseName, "/", "_")
			imgPath := filepath.Join(p.imageDir, fmt.Sprintf("pdf_%s_page%d.png", baseName, i))
			
			if err := os.WriteFile(imgPath, imgData, 0644); err == nil {
				images = append(images, ImageData{
					Index:    imageIndex,
					FilePath: imgPath,
					Data:     imgData,
				})
				imageIndex++
			}
		}
	}

	return &ParsedDocument{
		Text:   strings.Join(textParts, "\n\n"),
		Images: images,
	}, nil
}

// extractTextFromHTML performs basic HTML tag removal and entity decoding
func extractTextFromHTML(html string) string {
	// Simple approach: remove HTML tags and decode entities
	var result strings.Builder
	inTag := false
	for i, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			// Handle common HTML entities
			if r == '&' {
				// Check for common entities
				remaining := html[i:]
				if strings.HasPrefix(remaining, "&amp;") {
					result.WriteRune('&')
					continue
				} else if strings.HasPrefix(remaining, "&lt;") {
					result.WriteRune('<')
					continue
				} else if strings.HasPrefix(remaining, "&gt;") {
					result.WriteRune('>')
					continue
				} else if strings.HasPrefix(remaining, "&quot;") {
					result.WriteRune('"')
					continue
				} else if strings.HasPrefix(remaining, "&apos;") {
					result.WriteRune('\'')
					continue
				} else if strings.HasPrefix(remaining, "&nbsp;") {
					result.WriteRune(' ')
					continue
				}
			}
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// EPUBParserV2 uses zip-based parsing for better compatibility
type EPUBParserV2 struct {
	imageDir string
}

// NewEPUBParserV2 creates a new EPUB parser using zip
func NewEPUBParserV2(imageDir string) *EPUBParserV2 {
	return &EPUBParserV2{imageDir: imageDir}
}

// Parse extracts text and images from an EPUB file using zip
func (p *EPUBParserV2) Parse(filePath string) (*ParsedDocument, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open EPUB as zip: %w", err)
	}
	defer r.Close()

	var textParts []string
	var images []ImageData
	imageIndex := 0

	for _, f := range r.File {
		// Extract HTML/XHTML files
		if strings.HasSuffix(f.Name, ".html") || strings.HasSuffix(f.Name, ".xhtml") || strings.HasSuffix(f.Name, ".htm") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			html, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			text := extractTextFromHTML(string(html))
			if strings.TrimSpace(text) != "" {
				textParts = append(textParts, text)
			}
		}

		// Extract images
		if strings.HasPrefix(f.Name, "OEBPS/Images/") || strings.HasPrefix(f.Name, "images/") || strings.Contains(f.Name, ".jpg") || strings.Contains(f.Name, ".png") || strings.Contains(f.Name, ".jpeg") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			imgData, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}

			ext := filepath.Ext(f.Name)
			if ext == "" {
				ext = ".png"
			}
			imgPath := filepath.Join(p.imageDir, fmt.Sprintf("epub_%s_%d%s", filepath.Base(filePath), imageIndex, ext))
			if err := os.WriteFile(imgPath, imgData, 0644); err == nil {
				images = append(images, ImageData{
					Index:    imageIndex,
					FilePath: imgPath,
					Data:     imgData,
				})
				imageIndex++
			}
		}
	}

	return &ParsedDocument{
		Text:   strings.Join(textParts, "\n\n"),
		Images: images,
	}, nil
}
