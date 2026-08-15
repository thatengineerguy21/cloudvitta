package provider_test

import (
	"context"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// stubAdapter is a test double that records calls and returns configured results.
type stubAdapter struct {
	fetchCount int
	result     domain.FetchResult
	err        error
}

func (s *stubAdapter) Fetch(_ context.Context) (domain.FetchResult, error) {
	s.fetchCount++
	return s.result, s.err
}

func TestFactory_Register_And_BuildJobs(t *testing.T) {
	f := provider.NewFactory()
	adapter := &stubAdapter{result: domain.FetchResult{RawGCSPath: "test/path"}}

	f.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "compute",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
	}, adapter)

	jobs := f.BuildJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	job := jobs[0]
	if job.Provider != "aws" {
		t.Errorf("provider = %q, want %q", job.Provider, "aws")
	}
	if job.Category != "compute" {
		t.Errorf("category = %q, want %q", job.Category, "compute")
	}
	if job.Limiter == nil {
		t.Error("expected rate limiter to be set")
	}
	if job.Adapter == nil {
		t.Error("expected adapter to be set")
	}
}

func TestFactory_BuildJobs_NoRateLimit(t *testing.T) {
	f := provider.NewFactory()
	adapter := &stubAdapter{}

	f.Register(provider.ProviderConfig{
		Provider: "gcp",
		Category: "compute",
		// No RateLimitRPS set
	}, adapter)

	jobs := f.BuildJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Limiter != nil {
		t.Error("expected no rate limiter when RPS is 0")
	}
}

func TestFactory_BuildJobs_DefaultRetryConfig(t *testing.T) {
	f := provider.NewFactory()
	adapter := &stubAdapter{}

	f.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		// No Retry config set (zero-valued)
	}, adapter)

	jobs := f.BuildJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	defaults := provider.DefaultRetryConfig()
	if jobs[0].Retry.MaxAttempts != defaults.MaxAttempts {
		t.Errorf("MaxAttempts = %d, want default %d", jobs[0].Retry.MaxAttempts, defaults.MaxAttempts)
	}
}

func TestFactory_MultipleProviders(t *testing.T) {
	f := provider.NewFactory()

	f.Register(provider.ProviderConfig{Provider: "aws", Category: "compute"}, &stubAdapter{})
	f.Register(provider.ProviderConfig{Provider: "azure", Category: "compute"}, &stubAdapter{})
	f.Register(provider.ProviderConfig{Provider: "aws", Category: "storage"}, &stubAdapter{})

	jobs := f.BuildJobs()
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify all jobs are distinct
	seen := make(map[string]bool)
	for _, j := range jobs {
		key := j.Provider + ":" + j.Category
		if seen[key] {
			t.Errorf("duplicate job for %s", key)
		}
		seen[key] = true
	}
}

func TestRateLimitedFetch_WithoutLimiter(t *testing.T) {
	expected := domain.FetchResult{RawGCSPath: "test/path"}
	adapter := &stubAdapter{result: expected}

	job := provider.Job{
		Provider: "aws",
		Category: "compute",
		Adapter:  adapter,
		Limiter:  nil, // no rate limiting
	}

	result, err := provider.RateLimitedFetch(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RawGCSPath != expected.RawGCSPath {
		t.Errorf("GCSPath = %q, want %q", result.RawGCSPath, expected.RawGCSPath)
	}
	if adapter.fetchCount != 1 {
		t.Errorf("fetch count = %d, want 1", adapter.fetchCount)
	}
}

func TestRateLimitedFetch_WithLimiter(t *testing.T) {
	expected := domain.FetchResult{RawGCSPath: "test/path"}
	adapter := &stubAdapter{result: expected}

	f := provider.NewFactory()
	f.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "compute",
		RateLimitRPS:   100, // high rate so test doesn't block
		RateLimitBurst: 10,
	}, adapter)

	jobs := f.BuildJobs()
	result, err := provider.RateLimitedFetch(context.Background(), jobs[0])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RawGCSPath != expected.RawGCSPath {
		t.Errorf("GCSPath = %q, want %q", result.RawGCSPath, expected.RawGCSPath)
	}
}

func TestRateLimitedFetch_CancelledContext(t *testing.T) {
	adapter := &stubAdapter{}

	f := provider.NewFactory()
	f.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "compute",
		RateLimitRPS:   0.001, // extremely low rate
		RateLimitBurst: 1,
	}, adapter)

	jobs := f.BuildJobs()

	// Exhaust the burst
	ctx := context.Background()
	_, _ = provider.RateLimitedFetch(ctx, jobs[0])

	// Now cancel context before next attempt
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := provider.RateLimitedFetch(cancelCtx, jobs[0])
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}
