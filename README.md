# Dream AI - Symbol & Dream Interpretation CLI

A powerful command-line tool for dream interpretation and symbolic analysis using RAG (Retrieval-Augmented Generation) with pgvector embeddings, Ollama models, and CLIP2 image captioning.

## Features

- **RAG-powered Conversations**: Ask questions about symbols and dreams using your personal knowledge base
- **Document Processing**: Automatically process PDF and EPUB documents with incremental updates
- **Image Understanding**: Extract and understand images from documents using CLIP2
- **Model Selection**: Choose from available Ollama models optimized for reasoning tasks
- **Beautiful TUI**: Intuitive terminal user interface built with Bubbletea
- **Vector Search**: Fast semantic search using pgvector

## Requirements

- **PostgreSQL** (14+) with pgvector extension
- **Ollama** running locally with models installed
- **Python 3** (for CLIP2 image processing, optional)
- **Go** 1.21 or later

## Installation

### 1. Install PostgreSQL with pgvector

```bash
# macOS
brew install postgresql@14
brew install pgvector

# Ubuntu/Debian
sudo apt install postgresql-14 postgresql-14-pgvector

# Create database
createdb postgres
psql postgres -c "CREATE EXTENSION vector;"
```

### 2. Install Ollama

Visit [ollama.ai](https://ollama.ai) and install Ollama, then pull a recommended model:

```bash
ollama pull llama3.2
# or
ollama pull qwen2.5
```

### 3. Install Python dependencies (for CLIP2)

```bash
pip install transformers torch pillow
```

### 4. Build Dream AI

```bash
git clone <repository-url>
cd dream-ai
go mod download
make build
```

Or install directly:

```bash
make install
```

## Usage

### First Run

1. **Run migrations** to set up the database schema:

```bash
./bin/dream-ai -migrate
# or
make migrate
```

2. **Start the application**:

```bash
./bin/dream-ai
# or
make run
```

### Using the TUI

The application provides four main views:

- **Chat (Press 1)**: Main conversation interface for asking questions
- **Documents (Press 2)**: Manage and process documents
- **Models (Press 3)**: Select and switch between Ollama models
- **Settings (Press 4)**: View application settings

#### Chat View

- Type your question and press Enter
- The system will retrieve relevant context from your documents
- Responses stream in real-time

#### Documents View

- **a**: Add documents from the configured directory
- **d**: Delete selected document
- **p**: Process/reprocess selected document
- **r**: Reload document list
- **j/k**: Navigate up/down

#### Models View

- **j/k**: Navigate models
- **Enter/Space**: Select model
- **r**: Reload model list

### Adding Documents

Place your PDF and EPUB files in the documents directory (default: `~/documents`), then:

1. Go to Documents view (Press 2)
2. Press 'a' to add documents
3. Documents will be automatically processed

The system uses incremental processing - only new or changed documents are processed.

## Configuration

Configuration is stored in `~/.dream-ai/config.yaml`. Default values:

```yaml
database:
  connection_string: "postgres://postgres@localhost/postgres?sslmode=disable"

ollama:
  base_url: "http://localhost:11434"
  default_model: ""  # Auto-selects best model

embeddings:
  text_model: "nomic-embed-text"

processing:
  chunk_size: 512
  chunk_overlap: 50
  top_k: 5

clip2:
  python_path: "python3"
  script_path: ""  # Auto-detected

paths:
  documents_dir: "~/documents"
  image_dir: "/tmp/dream-ai-images"
```

## Architecture

```
└── dream-ai/
    ├── cmd/dream-ai/          # Main CLI entry point
    ├── internal/
    │   ├── db/                # Database layer (pgvector, migrations)
    │   ├── embeddings/        # Embedding generation (text + CLIP2)
    │   ├── rag/              # RAG retrieval and context building
    │   ├── ollama/           # Ollama client integration
    │   ├── documents/        # Document parsing (PDF, EPUB)
    │   └── tui/              # Bubbletea TUI components
    ├── migrations/            # SQL migrations for schema
    └── scripts/              # Helper scripts (CLIP2)
```

## Model Selection

The application automatically selects the best available model for reasoning tasks, prioritizing:

1. `llama3.2` / `llama3.1` (strong reasoning)
2. `qwen2.5` (good for analysis)
3. `mistral` variants
4. Largest available model (fallback)

## Development

```bash
# Run tests
make test

# Build
make build

# Run with migrations
make migrate && make run
```

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please open an issue or submit a pull request.

## Troubleshooting

### Database Connection Issues

- Ensure PostgreSQL is running: `pg_isready`
- Check connection string in config
- Verify pgvector extension: `psql postgres -c "\dx"`

### Ollama Issues

- Ensure Ollama is running: `ollama list`
- Check Ollama URL in config (default: http://localhost:11434)
- Verify models are installed: `ollama list`

### CLIP2 Issues

- Install Python dependencies: `pip install transformers torch pillow`
- Check Python path in config
- Image processing will fallback gracefully if CLIP2 is unavailable

### Document Processing Issues

- Check file permissions
- Ensure documents directory exists
- Check logs for specific error messages
