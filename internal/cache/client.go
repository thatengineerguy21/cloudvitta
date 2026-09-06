package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// InstrumentClient attaches OpenTelemetry tracing and metrics hooks to a redis.Client.
func InstrumentClient(client *redis.Client) error {
	if client == nil {
		return nil
	}
	if err := redisotel.InstrumentTracing(client); err != nil {
		return fmt.Errorf("cache: instrument tracing: %w", err)
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		return fmt.Errorf("cache: instrument metrics: %w", err)
	}
	return nil
}

// NewClient constructs a new redis.Client connected to the provided Redis URL.
// It attaches OpenTelemetry tracing and metrics hooks and verifies connectivity
// with a short ping.
func NewClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: parse redis url failed (invalid format)")
	}

	client := redis.NewClient(opts)

	if err := InstrumentClient(client); err != nil {
		_ = client.Close()
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("cache: ping redis server: %w", err)
	}

	slog.Info("redis client connected", "addr", opts.Addr)

	return client, nil
}
