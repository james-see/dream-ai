#!/usr/bin/env python3
"""
CLIP2 Image Processing Script
Processes images and generates captions and embeddings using CLIP2
"""

import sys
import json
import torch
from PIL import Image

try:
    from transformers import CLIPProcessor, CLIPModel
    HAS_CLIP = True
except ImportError:
    HAS_CLIP = False
    print("Warning: transformers not installed. Install with: pip install transformers torch pillow", file=sys.stderr)

def process_image(image_path):
    """Process an image and return caption and embedding"""
    if not HAS_CLIP:
        # Fallback: return placeholder
        return {
            "caption": f"Image: {image_path}",
            "embedding": [0.0] * 512
        }
    
    try:
        # Load CLIP model (using CLIP ViT-B/32 as base)
        # Note: For CLIP2 specifically, you may need a different model
        model_name = "openai/clip-vit-base-patch32"
        model = CLIPModel.from_pretrained(model_name)
        processor = CLIPProcessor.from_pretrained(model_name)
        
        # Load and process image
        image = Image.open(image_path).convert("RGB")
        inputs = processor(images=image, return_tensors="pt")
        
        # Generate image features
        with torch.no_grad():
            image_features = model.get_image_features(**inputs)
        
        # Normalize and convert to list
        image_features = image_features / image_features.norm(dim=-1, keepdim=True)
        embedding = image_features[0].tolist()
        
        # Generate a simple caption (CLIP doesn't generate captions directly)
        # In production, you might want to use a captioning model
        caption = f"Image from {image_path}"
        
        return {
            "caption": caption,
            "embedding": embedding
        }
    except Exception as e:
        return {
            "caption": f"Error processing image: {str(e)}",
            "embedding": [0.0] * 512
        }

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps({"error": "Usage: clip2_process.py <image_path>"}), file=sys.stderr)
        sys.exit(1)
    
    image_path = sys.argv[1]
    result = process_image(image_path)
    print(json.dumps(result))
