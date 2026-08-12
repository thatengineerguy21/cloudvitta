package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/tern/v2/migrate"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate runs all pending tern migrations against the database.
// Migrations are embedded in the binary, so no CLI tool is required
// inside the Cloud Run container.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire connection for migration: %w", err)
	}
	defer conn.Release()

	migrator, err := migrate.NewMigrator(ctx, conn.Conn(), "schema_version")
	if err != nil {
		return fmt.Errorf("store: create migrator: %w", err)
	}

	// The embed.FS has files under "migrations/", so strip the prefix
	// to give tern a flat directory of .sql files.
	migrationsDir, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: sub migrations FS: %w", err)
	}

	if err := migrator.LoadMigrations(migrationsDir); err != nil {
		return fmt.Errorf("store: load migrations: %w", err)
	}

	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("store: run migrations: %w", err)
	}

	ver, err := migrator.GetCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("store: get migration version: %w", err)
	}

	slog.Info("migrations applied", "version", ver)
	return nil
}
