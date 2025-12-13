package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// Run executes all pending migrations in order.
// It creates a schema_migrations table to track which migrations have been applied.
// Migrations are idempotent (use IF NOT EXISTS) so they're safe to run multiple times.
func Run(ctx context.Context, db *sql.DB) error {
	slog.Info("checking database migrations")

	// Create schema_migrations table if it doesn't exist
	if err := createMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get list of applied migrations
	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get all migration files
	files, err := getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to read migration files: %w", err)
	}

	// Run pending migrations
	pending := 0
	for _, file := range files {
		if applied[file] {
			slog.Debug("migration already applied", "file", file)
			continue
		}

		if err := runMigration(ctx, db, file); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", file, err)
		}
		pending++
	}

	if pending == 0 {
		slog.Info("database schema is up to date")
	} else {
		slog.Info("migrations completed", "applied", pending)
	}

	return nil
}

func createMigrationsTable(ctx context.Context, db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
	_, err := db.ExecContext(ctx, query)
	return err
}

func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

func getMigrationFiles() ([]string, error) {
	entries, err := migrationFiles.ReadDir("sql")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	// Sort to ensure consistent order (001_xxx.sql, 002_xxx.sql, etc.)
	sort.Strings(files)
	return files, nil
}

func runMigration(ctx context.Context, db *sql.DB, filename string) error {
	slog.Info("applying migration", "file", filename)
	start := time.Now()

	// Read migration file
	content, err := migrationFiles.ReadFile(filepath.Join("sql", filename))
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Execute migration in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Run the migration SQL
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record the migration
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version) VALUES ($1)",
		filename,
	); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("migration applied", "file", filename, "duration", time.Since(start))
	return nil
}
