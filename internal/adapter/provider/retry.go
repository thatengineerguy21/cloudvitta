package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// RetryConfig defines retry behavior for provider HTTP calls.
type RetryConfig struct {
	MaxAttempts int           // Max total attempts (default 3)
	BaseDelay   time.Duration // Initial backoff delay (default 5s)
	MaxDelay    time.Duration // Backoff cap (default 60s)
	Multiplier  float64       // Backoff multiplier (default 2.0)
	SleepFn     func(context.Context, time.Duration) error
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   5 * time.Second,
		MaxDelay:    60 * time.Second,
		Multiplier:  2.0,
		SleepFn: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

var (
	// ErrRetriesExhausted indicates all retry attempts failed.
	ErrRetriesExhausted = errors.New("provider: retries exhausted")
	// ErrPermanentFailure indicates a non-retryable error (auth, config).
	ErrPermanentFailure = errors.New("provider: permanent failure")
)

// HTTPError represents an HTTP error with a status code.
type HTTPError struct {
	StatusCode int
	RetryAfter time.Duration // parsed from Retry-After header, zero if not present
	Err        error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("http %d: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("http %d", e.StatusCode)
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

// Do executes fn with retry logic according to cfg.
// fn should return an error that may be an *HTTPError for status-aware retry behavior.
// Returns the result of fn on success, or a wrapped error on exhaustion/permanent failure.
func Do[T any](ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	if cfg.SleepFn == nil {
		cfg.SleepFn = DefaultRetryConfig().SleepFn
	}

	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}

		res, err := fn(ctx)
		if err == nil {
			return res, nil
		}
		lastErr = err

		var httpErr *HTTPError
		isHTTP := errors.As(err, &httpErr)

		if isHTTP {
			if httpErr.StatusCode == 401 || httpErr.StatusCode == 403 {
				var zero T
				return zero, fmt.Errorf("%w: %w", ErrPermanentFailure, err)
			}
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		var delay time.Duration
		if isHTTP && httpErr.StatusCode == 429 {
			if httpErr.RetryAfter > 0 {
				delay = httpErr.RetryAfter
				if delay > cfg.MaxDelay {
					delay = cfg.MaxDelay
				}
			} else {
				delay = calculateBackoff(cfg.BaseDelay, cfg.MaxDelay, cfg.Multiplier, attempt) * 2
			}
		} else {
			delay = calculateBackoff(cfg.BaseDelay, cfg.MaxDelay, cfg.Multiplier, attempt)
		}

		if err := cfg.SleepFn(ctx, delay); err != nil {
			var zero T
			return zero, err
		}
	}

	var zero T
	return zero, fmt.Errorf("%w: %w", ErrRetriesExhausted, lastErr)
}

func calculateBackoff(baseDelay, maxDelay time.Duration, multiplier float64, attempt int) time.Duration {
	m := 1.0
	for i := 0; i < attempt; i++ {
		m *= multiplier
	}

	val := float64(baseDelay) * m
	if val > float64(maxDelay) {
		val = float64(maxDelay)
	}

	return addJitter(time.Duration(val))
}

func addJitter(d time.Duration) time.Duration {
	jitterFraction := (rand.Float64() * 0.5) - 0.25 // [-0.25, 0.25)
	jitter := time.Duration(float64(d) * jitterFraction)
	return d + jitter
}
