package provider_test

import (
	"context"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"golang.org/x/time/rate"
)

// stubAdapter is a test double that records calls and returns configured results.
type stubAdapter struct {
	fetchCount int
	result     domain.FetchResult
	err        error
}

func (s *stubAdapter) Fetch(ctx context.Context, limiter *rate.Limiter) (domain.FetchResult, error) {
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return domain.FetchResult{}, err
		}
	}
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

func TestJob_Fetch_WithoutLimiter(t *testing.T) {
	expected := domain.FetchResult{RawGCSPath: "test/path"}
	adapter := &stubAdapter{result: expected}

	job := provider.Job{
		Provider: "aws",
		Category: "compute",
		Adapter:  adapter,
		Limiter:  nil, // no rate limiting
	}

	result, err := job.Fetch(context.Background())
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

func TestJob_Fetch_WithLimiter(t *testing.T) {
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
	result, err := jobs[0].Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RawGCSPath != expected.RawGCSPath {
		t.Errorf("GCSPath = %q, want %q", result.RawGCSPath, expected.RawGCSPath)
	}
}

func TestJob_Fetch_CancelledContext(t *testing.T) {
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
	_, _ = jobs[0].Fetch(ctx)

	// Now cancel context before next attempt
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := jobs[0].Fetch(cancelCtx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestJob_Fetch_UnmappedRatio_WithinThreshold_Succeeds(t *testing.T) {
	// 2 unmapped out of 100 observations (2% <= 5% threshold)
	obs := make([]domain.PriceObservation, 98)
	adapter := &stubAdapter{
		result: domain.FetchResult{
			Observations:  obs,
			UnmappedCount: 2,
			RawGCSPath:    "test/path",
		},
	}

	f := provider.NewFactory()
	f.Register(provider.ProviderConfig{
		Provider:         "aws",
		Category:         "compute",
		MaxUnmappedRatio: 0.05,
	}, adapter)

	jobs := f.BuildJobs()
	result, err := jobs[0].Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected success when within unmapped threshold, got: %v", err)
	}
	if len(result.Observations) != 98 {
		t.Errorf("got %d observations, want 98", len(result.Observations))
	}
	if result.UnmappedCount != 2 {
		t.Errorf("got %d unmapped, want 2", result.UnmappedCount)
	}
}

func TestJob_Fetch_UnmappedRatio_ExceedsThreshold_Fails(t *testing.T) {
	// 10 unmapped out of 20 total items (50% > 5% threshold)
	obs := make([]domain.PriceObservation, 10)
	adapter := &stubAdapter{
		result: domain.FetchResult{
			Observations:  obs,
			UnmappedCount: 10,
			RawGCSPath:    "test/path",
		},
	}

	f := provider.NewFactory()
	f.Register(provider.ProviderConfig{
		Provider:         "azure",
		Category:         "compute",
		MaxUnmappedRatio: 0.05,
	}, adapter)

	jobs := f.BuildJobs()
	_, err := jobs[0].Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when unmapped ratio exceeds threshold, got nil")
	}
}

func TestJob_Fetch_3WayClassification_ThresholdBoundaries(t *testing.T) {
	tests := []struct {
		name          string
		obsCount      int
		unmappedCount int
		ignoredCount  int
		maxRatio      float64
		wantErr       bool
	}{
		{
			name:          "Zero in-scope items, only ignored items -> succeeds",
			obsCount:      0,
			unmappedCount: 0,
			ignoredCount:  10000,
			maxRatio:      0.05,
			wantErr:       false,
		},
		{
			name:          "Large ignored count does not skew denominator: 2 unmapped / 100 in-scope (2% <= 5%) -> succeeds",
			obsCount:      98,
			unmappedCount: 2,
			ignoredCount:  50000,
			maxRatio:      0.05,
			wantErr:       false,
		},
		{
			name:          "Exactly at 5% boundary: 5 unmapped / 100 in-scope (5.00% <= 5.00%) -> succeeds",
			obsCount:      95,
			unmappedCount: 5,
			ignoredCount:  1000,
			maxRatio:      0.05,
			wantErr:       false,
		},
		{
			name:          "Just under boundary: 4 unmapped / 100 in-scope (4.00% <= 5.00%) -> succeeds",
			obsCount:      96,
			unmappedCount: 4,
			ignoredCount:  1000,
			maxRatio:      0.05,
			wantErr:       false,
		},
		{
			name:          "Just over boundary: 6 unmapped / 100 in-scope (6.00% > 5.00%) -> fails",
			obsCount:      94,
			unmappedCount: 6,
			ignoredCount:  1000,
			maxRatio:      0.05,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := make([]domain.PriceObservation, tt.obsCount)
			adapter := &stubAdapter{
				result: domain.FetchResult{
					Observations:  obs,
					UnmappedCount: tt.unmappedCount,
					IgnoredCount:  tt.ignoredCount,
					RawGCSPath:    "test/path",
				},
			}

			f := provider.NewFactory()
			f.Register(provider.ProviderConfig{
				Provider:         "test",
				Category:         "compute",
				MaxUnmappedRatio: tt.maxRatio,
			}, adapter)

			jobs := f.BuildJobs()
			res, err := jobs[0].Fetch(context.Background())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if res.IgnoredCount != tt.ignoredCount {
					t.Errorf("got ignored count %d, want %d", res.IgnoredCount, tt.ignoredCount)
				}
				if res.UnmappedCount != tt.unmappedCount {
					t.Errorf("got unmapped count %d, want %d", res.UnmappedCount, tt.unmappedCount)
				}
				if len(res.Observations) != tt.obsCount {
					t.Errorf("got obs count %d, want %d", len(res.Observations), tt.obsCount)
				}
			}
		})
	}
}
