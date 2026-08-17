package dlq_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
)

func setupTest(t *testing.T) (*dlq.DLQ, *miniredis.Miniredis, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	d := dlq.New(client)
	return d, mr, func() {
		_ = client.Close()
		mr.Close()
	}
}

func TestRecord_CreatesNewEntry(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Record(ctx, "aws", "metrics", errors.New("timeout"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, err := d.Get(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", entry.ConsecutiveFailures)
	}
	if entry.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", entry.Status)
	}
	if entry.LastError != "timeout" {
		t.Errorf("expected error 'timeout', got %s", entry.LastError)
	}
}

func TestRecord_IncrementsOnSubsequentFailure(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	if err := d.Record(ctx, "aws", "metrics", errors.New("timeout")); err != nil {
		t.Fatalf("first record failed: %v", err)
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.Record(ctx, "aws", "metrics", errors.New("connection reset")); err != nil {
		t.Fatalf("second record failed: %v", err)
	}

	entry, err := d.Get(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", entry.ConsecutiveFailures)
	}
	if entry.LastError != "connection reset" {
		t.Errorf("expected error 'connection reset', got %s", entry.LastError)
	}
}

func TestRecord_MarksBlockedForAuthErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		errStr string
	}{
		{"http 401 unauthorized"},
		{"API returned 403 Forbidden"},
		{"authentication failed for user"},
		{"authorization denied"},
		{"invalid credentials provided"},
		{"bad config"},
	}

	for _, tt := range tests {
		t.Run(tt.errStr, func(t *testing.T) {
			mr := miniredis.RunT(t)
			defer mr.Close()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer func() { _ = client.Close() }()
			d2 := dlq.New(client)

			err := d2.Record(ctx, "aws", "metrics", errors.New(tt.errStr))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			entry, err := d2.Get(ctx, "aws", "metrics")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if entry.Status != "blocked" {
				t.Errorf("expected status 'blocked' for error %q, got %s", tt.errStr, entry.Status)
			}
		})
	}
}

func TestRecord_MarksFailedForTransientErrors(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Record(ctx, "gcp", "logs", errors.New("i/o timeout"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, err := d.Get(ctx, "gcp", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entry.Status != "failed" {
		t.Errorf("expected status 'failed', got %s", entry.Status)
	}
}

func TestClear_RemovesEntry(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	if err := d.Record(ctx, "aws", "metrics", errors.New("error")); err != nil {
		t.Fatalf("record failed: %v", err)
	}

	err := d.Clear(ctx, "aws", "metrics")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = d.Get(ctx, "aws", "metrics")
	if !errors.Is(err, dlq.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestClear_NoopIfNotExists(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	err := d.Clear(ctx, "aws", "metrics")
	if err != nil {
		t.Errorf("expected nil error on clear missing, got %v", err)
	}
}

func TestGet_ReturnsEntryNotFoundForMissing(t *testing.T) {
	d, _, teardown := setupTest(t)
	defer teardown()

	ctx := context.Background()
	_, err := d.Get(ctx, "aws", "metrics")
	if !errors.Is(err, dlq.ErrEntryNotFound) {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}
