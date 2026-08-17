package auth_test

import (
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestRotationCache_PutAndGet(t *testing.T) {
	cache := auth.NewRotationCache()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	ttl := 10 * time.Second

	tokenHash := "token-hash-123456"
	idempotencyKey := "idemp-key-abc"
	pair := domain.TokenPair{
		AccessToken:  "access.jwt.token",
		RefreshToken: "refresh-raw-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}

	cache.Put(tokenHash, idempotencyKey, pair, now, ttl)

	// Immediate lookup
	got, ok := cache.Get(tokenHash, now)
	if !ok || got == nil {
		t.Fatalf("expected item in cache, got ok=false")
	}
	if got.IdempotencyKey != idempotencyKey {
		t.Errorf("IdempotencyKey = %q, want %q", got.IdempotencyKey, idempotencyKey)
	}
	if got.TokenPair.AccessToken != pair.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.TokenPair.AccessToken, pair.AccessToken)
	}

	// Lookup at 5 seconds (before TTL)
	got5s, ok5s := cache.Get(tokenHash, now.Add(5*time.Second))
	if !ok5s || got5s == nil {
		t.Errorf("expected item in cache at 5s, got ok=false")
	}

	// Lookup at 11 seconds (after TTL)
	_, ok11s := cache.Get(tokenHash, now.Add(11*time.Second))
	if ok11s {
		t.Errorf("expected item expired at 11s, got ok=true")
	}
}

func TestRotationCache_Delete(t *testing.T) {
	cache := auth.NewRotationCache()
	now := time.Now().UTC()
	tokenHash := "token-hash-delete-test"

	cache.Put(tokenHash, "key-1", domain.TokenPair{AccessToken: "token"}, now, 10*time.Second)

	if _, ok := cache.Get(tokenHash, now); !ok {
		t.Fatalf("expected item in cache before delete")
	}

	cache.Delete(tokenHash)

	if _, ok := cache.Get(tokenHash, now); ok {
		t.Errorf("expected item removed after delete, got ok=true")
	}
}
