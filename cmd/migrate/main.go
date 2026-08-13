package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

	// Read all SQL migration files in migrations directory
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		slog.Error("failed to list migration files", "error", err)
		os.Exit(1)
	}

	sort.Strings(files)

	for _, file := range files {
		sqlBytes, readErr := os.ReadFile(file)
		if readErr != nil {
			slog.Error("failed to read migration file", "file", file, "error", readErr)
			os.Exit(1)
		}

		filename := filepath.Base(file)
		slog.Info("applying migration", "file", filename)

		// Execute SQL script
		if _, execErr := conn.Exec(ctx, string(sqlBytes)); execErr != nil {
			// Check if error is due to pre-existing table/index (idempotent run)
			if strings.Contains(execErr.Error(), "already exists") {
				slog.Info("migration already applied", "file", filename)
				continue
			}
			slog.Error("failed to execute migration", "file", filename, "error", execErr)
			os.Exit(1)
		}

		slog.Info("migration applied successfully", "file", filename)
	}

	slog.Info("all database migrations complete")
}
