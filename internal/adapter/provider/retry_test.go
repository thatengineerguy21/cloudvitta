package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.MaxAttempts = 3

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		return "success", nil
	}

	res, err := provider.Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res != "success" {
		t.Errorf("expected 'success', got %v", res)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_SuccessAfterTransientFailure(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 1 * time.Millisecond

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", &provider.HTTPError{StatusCode: 500, Err: errors.New("server error")}
		}
		return "success", nil
	}

	res, err := provider.Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res != "success" {
		t.Errorf("expected 'success', got %v", res)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_429_RespectsRetryAfter(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 1 * time.Millisecond

	var slept time.Duration
	cfg.SleepFn = func(ctx context.Context, d time.Duration) error {
		slept = d
		return nil
	}

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", &provider.HTTPError{StatusCode: 429, RetryAfter: 50 * time.Millisecond, Err: errors.New("rate limited")}
		}
		return "success", nil
	}

	_, err := provider.Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
	if slept != 50*time.Millisecond {
		t.Errorf("expected to sleep for 50ms (RetryAfter), got %v", slept)
	}
}

func TestDo_429_WithoutRetryAfter(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond // 10ms
	cfg.MaxAttempts = 2

	var slept time.Duration
	cfg.SleepFn = func(ctx context.Context, d time.Duration) error {
		slept = d
		return nil
	}

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", &provider.HTTPError{StatusCode: 429, Err: errors.New("rate limited")}
		}
		return "success", nil
	}

	_, err := provider.Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if slept < 15*time.Millisecond || slept > 25*time.Millisecond {
		t.Errorf("expected sleep to be doubled normal backoff (15-25ms), got %v", slept)
	}
}

func TestDo_401_FailsFast(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.MaxAttempts = 3

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		return "", &provider.HTTPError{StatusCode: 401, Err: errors.New("unauthorized")}
	}

	_, err := provider.Do(ctx, cfg, fn)
	if !errors.Is(err, provider.ErrPermanentFailure) {
		t.Fatalf("expected ErrPermanentFailure, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (fail fast), got %d", attempts)
	}
}

func TestDo_403_FailsFast(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.MaxAttempts = 3

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		return "", &provider.HTTPError{StatusCode: 403, Err: errors.New("forbidden")}
	}

	_, err := provider.Do(ctx, cfg, fn)
	if !errors.Is(err, provider.ErrPermanentFailure) {
		t.Fatalf("expected ErrPermanentFailure, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (fail fast), got %d", attempts)
	}
}

func TestDo_ExhaustsRetries(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 1 * time.Millisecond
	cfg.MaxAttempts = 3

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		return "", &provider.HTTPError{StatusCode: 500, Err: errors.New("server error")}
	}

	_, err := provider.Do(ctx, cfg, fn)
	if !errors.Is(err, provider.ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		cancel() // cancel before next retry
		return "", errors.New("transient error")
	}

	_, err := provider.Do(ctx, cfg, fn)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_TimeoutError_IsRetryable(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 1 * time.Millisecond
	cfg.MaxAttempts = 2

	attempts := 0
	fn := func(ctx context.Context) (string, error) {
		attempts++
		if attempts == 1 {
			return "", context.DeadlineExceeded
		}
		return "success", nil
	}

	res, err := provider.Do(ctx, cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res != "success" {
		t.Errorf("expected 'success', got %v", res)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestDo_BackoffIncreases(t *testing.T) {
	ctx := context.Background()
	cfg := provider.DefaultRetryConfig()
	cfg.BaseDelay = 10 * time.Millisecond
	cfg.Multiplier = 2.0
	cfg.MaxAttempts = 3

	var sleeps []time.Duration
	cfg.SleepFn = func(ctx context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}

	fn := func(ctx context.Context) (string, error) {
		return "", errors.New("always fail")
	}

	_, _ = provider.Do(ctx, cfg, fn)

	if len(sleeps) != 2 { // 3 attempts -> 2 sleeps
		t.Fatalf("expected 2 sleeps, got %d", len(sleeps))
	}

	if sleeps[0] < 7*time.Millisecond || sleeps[0] > 13*time.Millisecond {
		t.Errorf("expected first sleep ~10ms, got %v", sleeps[0])
	}
	if sleeps[1] < 15*time.Millisecond || sleeps[1] > 25*time.Millisecond {
		t.Errorf("expected second sleep ~20ms, got %v", sleeps[1])
	}
}

func TestDo_UnmappedErrors_ArePermanentFailure(t *testing.T) {
	unmappedErrors := []struct {
		name string
		err  error
	}{
		{"unmapped product", regionmap.ErrUnmappedRegion},
		{"unmapped region", catalogmap.ErrUnmappedProduct},
		{"unmapped storage class", storageclassmap.ErrUnmappedStorageClass},
		{"unmapped transfer type", transfertypemap.ErrUnmappedTransferType},
	}

	for _, tt := range unmappedErrors {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := provider.DefaultRetryConfig()
			cfg.MaxAttempts = 3

			attempts := 0
			fn := func(ctx context.Context) (string, error) {
				attempts++
				return "", tt.err
			}

			_, err := provider.Do(ctx, cfg, fn)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, provider.ErrPermanentFailure) {
				t.Errorf("expected ErrPermanentFailure, got %v", err)
			}
			if attempts != 1 {
				t.Errorf("expected 1 attempt (no retries for unmapped errors), got %d", attempts)
			}
		})
	}
}
