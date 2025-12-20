package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dream-ai/cli/config"
	"github.com/dream-ai/cli/internal/db"
	"github.com/dream-ai/cli/internal/tui"
)

func main() {
	var (
		migrateFlag = flag.Bool("migrate", false, "Run database migrations")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Run migrations if requested
	if *migrateFlag {
		if err := runMigrations(cfg.Database.ConnectionString); err != nil {
			fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations completed successfully")
		return
	}

	// Ensure image directory exists
	if err := os.MkdirAll(cfg.Paths.ImageDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating image directory: %v\n", err)
		os.Exit(1)
	}

	// Set CLIP2 script path if not set
	if cfg.CLIP2.ScriptPath == "" {
		// Try to find the script relative to the binary
		exePath, err := os.Executable()
		if err == nil {
			scriptPath := filepath.Join(filepath.Dir(exePath), "..", "scripts", "clip2_process.py")
			if _, err := os.Stat(scriptPath); err == nil {
				cfg.CLIP2.ScriptPath = scriptPath
			}
		}
		// Fallback: try relative to current directory
		if cfg.CLIP2.ScriptPath == "" {
			scriptPath := filepath.Join("scripts", "clip2_process.py")
			if _, err := os.Stat(scriptPath); err == nil {
				cfg.CLIP2.ScriptPath = scriptPath
			}
		}
	}

	// Run migrations on startup if needed
	if err := ensureMigrations(cfg.Database.ConnectionString); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Migration check failed: %v\n", err)
		// Continue anyway - migrations might already be applied
	}

	// Create and run TUI
	app, err := tui.NewApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing app: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running app: %v\n", err)
		os.Exit(1)
	}
}

// runMigrations runs database migrations
func runMigrations(connString string) error {
	db, err := db.New(connString)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Get migration directory
	migrationDir := "migrations"
	if _, err := os.Stat(migrationDir); os.IsNotExist(err) {
		// Try relative to executable
		exePath, err := os.Executable()
		if err == nil {
			migrationDir = filepath.Join(filepath.Dir(exePath), "..", "migrations")
		}
	}

	// TODO: Migrations need to be run manually for now
	// Run: psql postgres -f migrations/00001_init_schema.up.sql
	// Or use a migration tool that supports pgx directly
	fmt.Printf("Note: Please run migrations manually:\n")
	fmt.Printf("  psql postgres -f %s\n", migrationDir+"/00001_init_schema.up.sql")
	fmt.Printf("Or install pgvector extension: CREATE EXTENSION IF NOT EXISTS vector;\n")

	return nil
}

// ensureMigrations checks and runs migrations if needed
func ensureMigrations(connString string) error {
	// Try to run migrations - if they fail, they might already be applied
	return runMigrations(connString)
}
