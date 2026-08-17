package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	_ "github.com/joho/godotenv/autoload"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration loading error: %v\n", err)
		os.Exit(1)
	}

	// Use CLOUDVITTA_DATABASE_MIGRATION_HOST (direct endpoint) if set, fallback to DB host
	migrationHost := os.Getenv("CLOUDVITTA_DATABASE_MIGRATION_HOST")
	if migrationHost == "" {
		migrationHost = cfg.Database.Host
	}

	encodedPassword := url.QueryEscape(cfg.Database.Password)
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		encodedPassword,
		migrationHost,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		slog.Error("failed to connect to database for migrations", "host", migrationHost, "error", err)
		os.Exit(1)
	}
	defer func() { _ = conn.Close(ctx) }()

	slog.Info("connected to database for migrations", "host", migrationHost)

	// Ensure schema_migrations tracking table exists
	createTrackerSQL := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);`
	if _, err := conn.Exec(ctx, createTrackerSQL); err != nil {
		slog.Error("failed to initialize schema_migrations tracking table", "error", err)
		os.Exit(1)
	}

	// Read all SQL migration files in migrations directory
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		slog.Error("failed to list migration files", "error", err)
		os.Exit(1)
	}

	sort.Strings(files)

	for _, file := range files {
		filename := filepath.Base(file)

		var alreadyApplied bool
		checkErr := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&alreadyApplied)
		if checkErr != nil {
			slog.Error("failed to check migration status", "file", filename, "error", checkErr)
			os.Exit(1)
		}

		if alreadyApplied {
			slog.Info("migration already applied", "file", filename)
			continue
		}

		sqlBytes, readErr := os.ReadFile(file)
		if readErr != nil {
			slog.Error("failed to read migration file", "file", file, "error", readErr)
			os.Exit(1)
		}

		slog.Info("applying migration", "file", filename)

		// Execute SQL script and record applied status inside a transaction
		tx, txErr := conn.Begin(ctx)
		if txErr != nil {
			slog.Error("failed to begin transaction for migration", "file", filename, "error", txErr)
			os.Exit(1)
		}

		if _, execErr := tx.Exec(ctx, string(sqlBytes)); execErr != nil {
			_ = tx.Rollback(ctx)
			slog.Error("failed to execute migration", "file", filename, "error", execErr)
			os.Exit(1)
		}

		if _, recErr := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT (version) DO NOTHING", filename); recErr != nil {
			_ = tx.Rollback(ctx)
			slog.Error("failed to record applied migration", "file", filename, "error", recErr)
			os.Exit(1)
		}

		if commitErr := tx.Commit(ctx); commitErr != nil {
			slog.Error("failed to commit migration transaction", "file", filename, "error", commitErr)
			os.Exit(1)
		}

		slog.Info("migration applied successfully", "file", filename)
	}

	slog.Info("all database migrations complete")
}
