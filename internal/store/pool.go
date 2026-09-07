package store

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
)

// PoolOption allows customizing pgxpool configuration.
type PoolOption func(*pgxpool.Config)

// WithoutTracer disables OpenTelemetry tracing on the database connection pool.
// Use this for bulk ingestion workloads to prevent query spans
// from exceeding OpenTelemetry trace payload limits.
func WithoutTracer() PoolOption {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = nil
	}
}

// NewPoolConfig constructs a pgxpool.Config from the application's DatabaseConfig,
// attaching the OpenTelemetry otelpgx tracer and configuring connection limits.
func NewPoolConfig(cfg config.DatabaseConfig, opts ...PoolOption) (*pgxpool.Config, error) {
	hostPort := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	encodedPassword := url.QueryEscape(cfg.Password)

	// Data Source Name (DSN) format for PostgreSQL connection string.
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		cfg.User,
		encodedPassword,
		hostPort,
		cfg.Name,
		cfg.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: parse database URL: %w", err)
	}

	// Conservative pool sizing — Cloud Run scales horizontally,
	// so each instance keeps a small pool to avoid exhausting
	// the Neon connection budget.
	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second
	poolCfg.MaxConnIdleTime = time.Duration(cfg.ConnMaxIdleTime) * time.Second

	// Attach OpenTelemetry tracer for auto-instrumenting queries, batches, and transactions.
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	for _, opt := range opts {
		opt(poolCfg)
	}

	return poolCfg, nil
}

// NewPool creates a pgxpool.Pool from the application's DatabaseConfig.
// It attaches an OpenTelemetry tracer for observability, applies
// conservative pool limits, and verifies connectivity with a short ping.
func NewPool(ctx context.Context, cfg config.DatabaseConfig, opts ...PoolOption) (*pgxpool.Pool, error) {
	poolCfg, err := NewPoolConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

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
