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
}

// Adapter defines the contract for a provider adapter that retrieves pricing data.
type Adapter interface {
	Fetch(ctx context.Context) (domain.FetchResult, error)
}

// Job represents a single (provider, category) ingestion unit
// that the orchestrator can execute.
type Job struct {
	Provider string
	Category string
	Adapter  Adapter
	Limiter  *rate.Limiter
	Retry    RetryConfig
}

// Factory constructs configured provider jobs for the orchestrator.
type Factory struct {
	adapters map[string]Adapter // key: "provider:category"
	configs  map[string]ProviderConfig
}

// NewFactory creates a new provider factory.
func NewFactory() *Factory {
	return &Factory{
		adapters: make(map[string]Adapter),
		configs:  make(map[string]ProviderConfig),
	}
}

// Register adds a provider adapter with its configuration to the factory.
func (f *Factory) Register(cfg ProviderConfig, adapter Adapter) {
	key := fmt.Sprintf("%s:%s", cfg.Provider, cfg.Category)
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

		jobs = append(jobs, Job{
			Provider: cfg.Provider,
			Category: cfg.Category,
			Adapter:  adapter,
			Limiter:  limiter,
			Retry:    retryConfig,
		})
	}
	return jobs
}

// RateLimitedFetch wraps an adapter's Fetch call with rate limiting.
// If the job has a rate limiter, it waits for a token before calling Fetch.
func RateLimitedFetch(ctx context.Context, job Job) (domain.FetchResult, error) {
	if job.Limiter != nil {
		if err := job.Limiter.Wait(ctx); err != nil {
			return domain.FetchResult{}, fmt.Errorf("rate limit wait for %s/%s: %w", job.Provider, job.Category, err)
		}
	}

	slog.InfoContext(ctx, "starting provider fetch",
		"provider", job.Provider,
		"category", job.Category,
	)

	start := time.Now()
	result, err := job.Adapter.Fetch(ctx)
	elapsed := time.Since(start)

	if err != nil {
		slog.ErrorContext(ctx, "provider fetch failed",
			"provider", job.Provider,
			"category", job.Category,
			"elapsed", elapsed,
			"error", err,
		)
		return domain.FetchResult{}, err
	}

	slog.InfoContext(ctx, "provider fetch completed",
		"provider", job.Provider,
		"category", job.Category,
		"observations", len(result.Observations),
		"elapsed", elapsed,
	)
	return result, nil
}
