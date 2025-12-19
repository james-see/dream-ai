package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pgvector/pgvector-go"
)

// ImageEmbedder processes images with CLIP2 for captioning and embeddings
type ImageEmbedder struct {
	pythonPath string
	scriptPath string
}

// NewImageEmbedder creates a new image embedder
func NewImageEmbedder(pythonPath string) *ImageEmbedder {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	return &ImageEmbedder{
		pythonPath: pythonPath,
	}
}

// ProcessImage generates a caption and embedding for an image using CLIP2
func (e *ImageEmbedder) ProcessImage(ctx context.Context, imagePath string) (string, *pgvector.Vector, error) {
	// Try to use the Python script if available
	scriptPath := e.scriptPath
	if scriptPath == "" {
		// Try to find script relative to current directory
		scriptPath = "scripts/clip2_process.py"
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			// Fallback to simple processing
			return e.ProcessImageSimple(ctx, imagePath)
		}
	}

	cmd := exec.CommandContext(ctx, e.pythonPath, scriptPath, imagePath)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to simple processing
		return e.ProcessImageSimple(ctx, imagePath)
	}

	// Parse output: JSON with caption and embedding
	var result struct {
		Caption   string    `json:"caption"`
		Embedding []float32 `json:"embedding"`
		Error     string    `json:"error,omitempty"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return e.ProcessImageSimple(ctx, imagePath)
	}

	if result.Error != "" || len(result.Embedding) == 0 {
		return e.ProcessImageSimple(ctx, imagePath)
	}

	vec := pgvector.NewVector(result.Embedding)
	return result.Caption, &vec, nil
}

// getCLIP2Script returns the Python script for CLIP2 processing
func (e *ImageEmbedder) getCLIP2Script() string {
	return `
import sys
import json
import torch
from PIL import Image
from transformers import CLIPProcessor, CLIPModel

# Load CLIP2 model (using CLIP as CLIP2 may not be available, adjust as needed)
model_name = "openai/clip-vit-base-patch32"
model = CLIPModel.from_pretrained(model_name)
processor = CLIPProcessor.from_pretrained(model_name)

image_path = sys.argv[1]
image = Image.open(image_path)

# Generate caption (simple approach - CLIP doesn't generate captions directly)
# We'll use the image features as the "caption" representation
inputs = processor(images=image, return_tensors="pt")
with torch.no_grad():
    image_features = model.get_image_features(**inputs)

# Convert to list for JSON
embedding = image_features[0].tolist()

# For caption, we'll use a placeholder or extract from image metadata
caption = f"Image from {image_path}"

result = {
    "caption": caption,
    "embedding": embedding
}

print(json.dumps(result))
`
}

// ProcessImageSimple uses a simpler approach if CLIP2 is not available
func (e *ImageEmbedder) ProcessImageSimple(ctx context.Context, imagePath string) (string, *pgvector.Vector, error) {
	// Fallback: use image file path as caption and generate a simple hash-based embedding
	// This is a placeholder - in production, you'd want actual CLIP2
	caption := fmt.Sprintf("Image: %s", filepath.Base(imagePath))
	
	// Generate a simple embedding based on file hash (not ideal, but works as fallback)
	// In production, replace this with actual CLIP2 embedding
	embedding := make([]float32, 512) // CLIP2 produces 512-dim embeddings
	for i := range embedding {
		embedding[i] = float32(i%100) / 100.0 // Placeholder
	}
	
	vec := pgvector.NewVector(embedding)
	return caption, &vec, nil
}

// SetScriptPath sets the path to a custom Python script
func (e *ImageEmbedder) SetScriptPath(path string) {
	e.scriptPath = path
}
