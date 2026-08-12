package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

// NewPool creates a pgxpool.Pool from the application's DatabaseConfig.
// It attaches an OpenTelemetry tracer for observability, applies
// conservative pool limits, and verifies connectivity with a short ping.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("store: parse database URL: %w", err)
	}

	// Attach OpenTelemetry tracing to every connection.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	// Conservative pool sizing — Cloud Run scales horizontally,
	// so each instance keeps a small pool to avoid exhausting
	// the Neon connection budget.
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second
	poolCfg.MaxConnIdleTime = time.Duration(cfg.ConnMaxIdleTimeSeconds) * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("store: create connection pool: %w", err)
	}

	// Fail fast: verify the database is reachable before returning.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping database: %w", err)
	}

	slog.Info("database pool connected",
		"max_conns", poolCfg.MaxConns,
		"min_conns", poolCfg.MinConns,
	)

	return pool, nil
}
