package cache_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
)

func TestInstrumentClient_Miniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	if err := cache.InstrumentClient(client); err != nil {
		t.Fatalf("InstrumentClient failed: %v", err)
	}

	// Verify client operations succeed with instrumentation hooks attached
	ctx := context.Background()
	if err := client.Set(ctx, "test-key", "test-val", 0).Err(); err != nil {
		t.Fatalf("SET failed: %v", err)
	}

	val, err := client.Get(ctx, "test-key").Result()
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	if val != "test-val" {
		t.Errorf("expected 'test-val', got %q", val)
	}
}

func TestInstrumentClient_Nil(t *testing.T) {
	if err := cache.InstrumentClient(nil); err != nil {
		t.Errorf("expected nil error for nil client, got %v", err)
	}
}

func TestInstrumentClient_WithoutTracer(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	if err := cache.InstrumentClient(client, cache.WithoutTracer()); err != nil {
		t.Fatalf("InstrumentClient with WithoutTracer failed: %v", err)
	}

	ctx := context.Background()
	if err := client.Set(ctx, "no-trace-key", "no-trace-val", 0).Err(); err != nil {
		t.Fatalf("SET failed: %v", err)
	}
}
