package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	Database struct {
		ConnectionString string `yaml:"connection_string"`
	} `yaml:"database"`
	Ollama struct {
		BaseURL      string `yaml:"base_url"`
		DefaultModel string `yaml:"default_model"`
	} `yaml:"ollama"`
	Embeddings struct {
		TextModel string `yaml:"text_model"`
	} `yaml:"embeddings"`
	Processing struct {
		ChunkSize    int `yaml:"chunk_size"`
		ChunkOverlap int `yaml:"chunk_overlap"`
		TopK         int `yaml:"top_k"`
	} `yaml:"processing"`
	CLIP2 struct {
		PythonPath string `yaml:"python_path"`
		ScriptPath string `yaml:"script_path"`
	} `yaml:"clip2"`
	Paths struct {
		DocumentsDir string `yaml:"documents_dir"`
		ImageDir     string `yaml:"image_dir"`
	} `yaml:"paths"`
}

// Load loads configuration from file or returns defaults
func Load() (*Config, error) {
	cfg := Default()
	
	configPath := filepath.Join(os.Getenv("HOME"), ".dream-ai", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save saves configuration to file
func (c *Config) Save() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".dream-ai")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// Default returns default configuration
func Default() *Config {
	cfg := &Config{}
	
	cfg.Database.ConnectionString = "postgres://postgres@localhost/postgres?sslmode=disable"
	cfg.Ollama.BaseURL = "http://localhost:11434"
	cfg.Ollama.DefaultModel = ""
	cfg.Embeddings.TextModel = "nomic-embed-text"
	cfg.Processing.ChunkSize = 512
	cfg.Processing.ChunkOverlap = 50
	cfg.Processing.TopK = 5
	cfg.CLIP2.PythonPath = "python3"
	cfg.CLIP2.ScriptPath = ""
	
	homeDir := os.Getenv("HOME")
	cfg.Paths.DocumentsDir = filepath.Join(homeDir, "documents")
	cfg.Paths.ImageDir = filepath.Join(os.TempDir(), "dream-ai-images")
	
	return cfg
}
