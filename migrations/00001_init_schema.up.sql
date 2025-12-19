-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Documents table
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_path TEXT NOT NULL UNIQUE,
    file_hash TEXT NOT NULL,
    file_type TEXT NOT NULL CHECK (file_type IN ('pdf', 'epub')),
    processed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_hash ON documents(file_hash);
CREATE INDEX idx_documents_path ON documents(file_path);

-- Text chunks table
CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding vector(768), -- nomic-embed-text produces 768-dim embeddings
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chunks_document ON chunks(document_id);
CREATE INDEX idx_chunks_embedding ON chunks USING ivfflat (embedding vector_cosine_ops);

-- Images table
CREATE TABLE images (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    image_index INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    caption TEXT,
    embedding vector(512), -- CLIP2 produces 512-dim embeddings
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_document ON images(document_id);
CREATE INDEX idx_images_embedding ON images USING ivfflat (embedding vector_cosine_ops);

-- Conversations table
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_message TEXT NOT NULL,
    assistant_message TEXT NOT NULL,
    model_name TEXT NOT NULL,
    context_chunk_ids UUID[],
    context_image_ids UUID[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_conversations_created ON conversations(created_at);
