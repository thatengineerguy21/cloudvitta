package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	// ErrLockHeld is returned when a lock is already held by another process.
	ErrLockHeld = errors.New("cache: lock already held")
)

// Lock represents a distributed Redis lock for ingestion job deduplication.
type Lock struct {
	client redis.Cmdable
	key    string
	token  string // random token for safe release
}

// AcquireIngestionLock attempts to acquire a lock for a given provider and category.
func AcquireIngestionLock(ctx context.Context, client redis.Cmdable, provider, category string, ttl time.Duration) (*Lock, error) {
	key := fmt.Sprintf("ingestion:%s:%s:lock", provider, category)
	token := uuid.New().String()

	acquired, err := client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrLockHeld
	}

	return &Lock{
		client: client,
		key:    key,
		token:  token,
	}, nil
}

// Release releases the lock using a Lua script to ensure safe deletion.
func (l *Lock) Release(ctx context.Context) error {
	script := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`
	err := l.client.Eval(ctx, script, []string{l.key}, l.token).Err()
	if err != nil && err != redis.Nil {
		return err
	}
	return nil
}
