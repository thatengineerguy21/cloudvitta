package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// DefaultTTL defines the default expiration duration for cached pricing data (24 hours).
const DefaultTTL = 24 * time.Hour

// ErrCacheMiss indicates that the requested key does not exist in Redis.
var ErrCacheMiss = errors.New("cache: key not found")

// Warm JSON-marshals and stores the slice of PriceObservation values into Redis under key with the given TTL.
func Warm(ctx context.Context, client redis.Cmdable, key string, data []domain.PriceObservation, ttl time.Duration) error {
	if client == nil {
		return fmt.Errorf("cache: redis client is nil")
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("cache: marshal data for key %s: %w", key, err)
	}

	if err := client.Set(ctx, key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set key %s in redis: %w", key, err)
	}

	return nil
}

// Get retrieves and unmarshals a slice of PriceObservation values from Redis for key.
// Returns ErrCacheMiss if the key is missing.
func Get(ctx context.Context, client redis.Cmdable, key string) ([]domain.PriceObservation, error) {
	if client == nil {
		return nil, fmt.Errorf("cache: redis client is nil")
	}

	val, err := client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("cache: get key %s from redis: %w", key, err)
	}

	var observations []domain.PriceObservation
	if err := json.Unmarshal([]byte(val), &observations); err != nil {
		return nil, fmt.Errorf("cache: unmarshal payload for key %s: %w", key, err)
	}

	return observations, nil
}
