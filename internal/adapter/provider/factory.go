package provider

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/time/rate"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// ProviderConfig holds configuration for a single provider adapter.
type ProviderConfig struct {
	// Provider is the canonical provider name (e.g. "aws", "azure", "gcp").
	Provider string
	// Category is the service category (e.g. "compute", "storage", "network").
	Category string
	// RateLimit defines the token bucket rate limit for this provider.
	// Tokens per second. Zero means no rate limiting.
	RateLimitRPS float64
	// RateLimitBurst is the maximum burst size for the token bucket.
	RateLimitBurst int
	// Retry configures retry behavior for this provider's HTTP calls.
	Retry RetryConfig
	// MaxUnmappedRatio defines the maximum acceptable ratio of unmapped items
	// (unmapped / (unmapped + valid observations)) before the job fails as systemic breakdown.
	// Default is 0.05 (5%).
	MaxUnmappedRatio float64
}

// Adapter defines the contract for a provider adapter that retrieves pricing data.
type Adapter interface {
	Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error)
}

// JobKey uniquely identifies a (provider, category) ingestion job.
type JobKey struct {
	Provider string
	Category string
}

// Job represents a single (provider, category) ingestion unit
// that the orchestrator can execute.
type Job struct {
	Provider         string
	Category         string
	Adapter          Adapter
	Limiter          *rate.Limiter
	Retry            RetryConfig
	MaxUnmappedRatio float64
}

// Fetch executes the job's adapter Fetch, passing down the rate limiter,
// and enforces the unmapped item threshold check.
func (j Job) Fetch(ctx context.Context) (domain.FetchResult, error) {
	slog.InfoContext(ctx, "starting provider fetch",
		"provider", j.Provider,
		"category", j.Category,
	)

	start := time.Now()
	result, err := j.Adapter.Fetch(ctx, j.Limiter)
	elapsed := time.Since(start)

	if err != nil {
		slog.ErrorContext(ctx, "provider fetch failed",
			"provider", j.Provider,
			"category", j.Category,
			"elapsed", elapsed,
			"error", err,
		)
		return domain.FetchResult{}, err
	}

	// Enforce unmapped threshold check (Phase 3)
	total := result.UnmappedCount + len(result.Observations)
	if total > 0 {
		ratio := float64(result.UnmappedCount) / float64(total)
		maxRatio := j.MaxUnmappedRatio
		if maxRatio <= 0 {
			maxRatio = 0.05
		}

		if ratio > maxRatio {
			threshErr := fmt.Errorf("%w: unmapped item ratio %.2f%% exceeds threshold %.2f%% (%d unmapped / %d total items)",
				ErrPermanentFailure, ratio*100, maxRatio*100, result.UnmappedCount, total)
			slog.ErrorContext(ctx, "provider fetch failed due to unmapped threshold",
				"provider", j.Provider,
				"category", j.Category,
				"unmapped_count", result.UnmappedCount,
				"observation_count", len(result.Observations),
				"ratio", ratio,
				"threshold", maxRatio,
			)
			return domain.FetchResult{}, threshErr
		}

		if result.UnmappedCount > 0 {
			slog.WarnContext(ctx, "quarantined unmapped items during fetch",
				"provider", j.Provider,
				"category", j.Category,
				"unmapped_count", result.UnmappedCount,
				"observation_count", len(result.Observations),
				"ratio", ratio,
			)
		}
	}

	slog.InfoContext(ctx, "provider fetch completed",
		"provider", j.Provider,
		"category", j.Category,
		"observations", len(result.Observations),
		"unmapped", result.UnmappedCount,
		"elapsed", elapsed,
	)
	return result, nil
}

// Factory constructs configured provider jobs for the orchestrator.
type Factory struct {
	adapters map[JobKey]Adapter
	configs  map[JobKey]ProviderConfig
}

// NewFactory creates a new provider factory.
func NewFactory() *Factory {
	return &Factory{
		adapters: make(map[JobKey]Adapter),
		configs:  make(map[JobKey]ProviderConfig),
	}
}

// Register adds a provider adapter with its configuration to the factory.
func (f *Factory) Register(cfg ProviderConfig, adapter Adapter) {
	key := JobKey{Provider: cfg.Provider, Category: cfg.Category}
	f.adapters[key] = adapter
	f.configs[key] = cfg
}

// BuildJobs constructs all registered provider jobs with rate limiters.
func (f *Factory) BuildJobs() []Job {
	jobs := make([]Job, 0, len(f.adapters))
	for key, adapter := range f.adapters {
		cfg := f.configs[key]

		var limiter *rate.Limiter
		if cfg.RateLimitRPS > 0 {
			limiter = rate.NewLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)
		}

		// Apply defaults if retry config is zero-valued
		retryConfig := cfg.Retry
		if retryConfig.MaxAttempts == 0 {
			retryConfig = DefaultRetryConfig()
		}

		maxUnmappedRatio := cfg.MaxUnmappedRatio
		if maxUnmappedRatio <= 0 {
			maxUnmappedRatio = 0.05
		}

		jobs = append(jobs, Job{
			Provider:         cfg.Provider,
			Category:         cfg.Category,
			Adapter:          adapter,
			Limiter:          limiter,
			Retry:            retryConfig,
			MaxUnmappedRatio: maxUnmappedRatio,
		})
	}
	return jobs
}
