package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, client
}

func TestAcquireIngestionLock_Success(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	lock, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if lock == nil {
		t.Fatal("expected lock to not be nil")
	}

	val, err := mr.Get(lock.key)
	if err != nil {
		t.Fatalf("expected key to exist in redis, err: %v", err)
	}
	if val != lock.token {
		t.Errorf("expected token %s in redis, got %s", lock.token, val)
	}
}

func TestAcquireIngestionLock_AlreadyHeld(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	_, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	// second acquire should fail
	_, err = AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != ErrLockHeld {
		t.Errorf("expected ErrLockHeld, got: %v", err)
	}
}

func TestAcquireIngestionLock_DifferentCategories(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	_, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	// acquire different category should succeed
	_, err = AcquireIngestionLock(ctx, client, "aws", "database", time.Minute)
	if err != nil {
		t.Errorf("expected acquire for different category to succeed, got: %v", err)
	}
}

func TestRelease_RemovesLock(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	lock, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("expected release to succeed, got: %v", err)
	}

	// should be able to acquire again
	_, err = AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Errorf("expected acquire after release to succeed, got: %v", err)
	}
}

func TestRelease_OnlyOwnerCanRelease(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	lock, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	// Manually change token in Redis to simulate someone else owning it
	if err := mr.Set(lock.key, "someone-else-token"); err != nil {
		t.Fatalf("mr.Set: %v", err)
	}

	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("expected release to not return error even if it doesn't delete, got: %v", err)
	}

	// Key should still exist and belong to someone else
	val, err := mr.Get(lock.key)
	if err != nil {
		t.Fatalf("expected key to still exist, got err: %v", err)
	}
	if val != "someone-else-token" {
		t.Errorf("expected token to remain someone-else-token, got: %v", val)
	}
}

func TestRelease_ExpiredLock(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	lock, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	mr.FastForward(2 * time.Minute) // Expire the lock

	err = lock.Release(ctx)
	if err != nil {
		t.Fatalf("expected release of expired lock to succeed without error, got: %v", err)
	}
}

func TestLock_KeyFormat(t *testing.T) {
	mr, client := setupRedis(t)
	defer mr.Close()

	ctx := context.Background()
	lock, err := AcquireIngestionLock(ctx, client, "aws", "compute", time.Minute)
	if err != nil {
		t.Fatalf("expected first acquire to succeed, got: %v", err)
	}

	expectedKey := "ingestion:aws:compute:lock"
	if lock.key != expectedKey {
		t.Errorf("expected key %q, got %q", expectedKey, lock.key)
	}
}
