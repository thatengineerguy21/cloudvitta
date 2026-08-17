package auth

import (
	"sync"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// CachedRotation holds the newly minted TokenPair and the client's IdempotencyKey.
type CachedRotation struct {
	IdempotencyKey string
	TokenPair      domain.TokenPair
	ExpiresAt      time.Time
}

// RotationCache stores recent rotation results to support benign network retry replay (ADR 0007).
type RotationCache struct {
	mu    sync.RWMutex
	items map[string]CachedRotation
}

// NewRotationCache creates a new thread-safe in-memory RotationCache.
func NewRotationCache() *RotationCache {
	return &RotationCache{
		items: make(map[string]CachedRotation),
	}
}

// Put stores a token pair and its associated idempotency key under the rotated token's hash with a TTL.
func (c *RotationCache) Put(tokenHash string, idempotencyKey string, pair domain.TokenPair, now time.Time, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Prune expired entries when cache size grows to prevent unbounded memory growth
	if len(c.items) > 100 {
		for k, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, k)
			}
		}
	}

	c.items[tokenHash] = CachedRotation{
		IdempotencyKey: idempotencyKey,
		TokenPair:      pair,
		ExpiresAt:      now.Add(ttl),
	}
}

// Get retrieves a cached rotation entry if present and not expired relative to now.
func (c *RotationCache) Get(tokenHash string, now time.Time) (*CachedRotation, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[tokenHash]
	if !ok {
		return nil, false
	}
	if now.After(item.ExpiresAt) {
		return nil, false
	}
	itemCopy := item
	return &itemCopy, true
}

// Delete removes a token hash from the cache (used upon logout).
func (c *RotationCache) Delete(tokenHash string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, tokenHash)
}
