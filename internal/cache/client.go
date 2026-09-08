package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// ClientOption allows customizing the redis client configuration.
type ClientOption func(*clientConfig)

type clientConfig struct {
	disableTracing bool
}

// WithoutTracer disables OpenTelemetry tracing on the Redis client.
// Use this for bulk ingestion workloads to prevent multi-megabyte cache
// payload spans from exceeding OpenTelemetry trace payload limits.
func WithoutTracer() ClientOption {
	return func(c *clientConfig) {
		c.disableTracing = true
	}
}

// InstrumentClient attaches OpenTelemetry metrics hooks and optional tracing hooks to a redis.Client.
// When tracing is enabled, DB statement logging is explicitly disabled (WithDBStatement(false))
// to prevent multi-megabyte JSON payloads from being attached to span attributes.
func InstrumentClient(client *redis.Client, opts ...ClientOption) error {
	if client == nil {
		return nil
	}
	var cfg clientConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	if !cfg.disableTracing {
		if err := redisotel.InstrumentTracing(client, redisotel.WithDBStatement(false)); err != nil {
			return fmt.Errorf("cache: instrument tracing: %w", err)
		}
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return fmt.Errorf("cache: instrument metrics: %w", err)
	}
	return nil
}

// NewClient constructs a new redis.Client connected to the provided Redis URL.
// It attaches OpenTelemetry metrics and tracing hooks (unless configured with WithoutTracer)
// and verifies connectivity with a short ping.
func NewClient(redisURL string, opts ...ClientOption) (*redis.Client, error) {
	parsedOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis url failed (invalid format)")
	}

	client := redis.NewClient(parsedOpts)

	if err := InstrumentClient(client, opts...); err != nil {
		_ = client.Close()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: ping redis server: %w", err)
	}

	slog.Info("redis client connected", "addr", parsedOpts.Addr)

	return client, nil
}
