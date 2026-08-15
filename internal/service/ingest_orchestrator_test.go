package service_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// --- Test helpers ---

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

// mockFetcher implements provider.Adapter for testing.
type mockFetcher struct {
	result    domain.FetchResult
	err       error
	callCount atomic.Int32
	delay     time.Duration
}

func (m *mockFetcher) Fetch(_ context.Context) (domain.FetchResult, error) {
	m.callCount.Add(1)
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.result, m.err
}

// failingFetcher always returns an error.
type failingFetcher struct {
	err       error
	callCount atomic.Int32
}

func (f *failingFetcher) Fetch(_ context.Context) (domain.FetchResult, error) {
	f.callCount.Add(1)
	return domain.FetchResult{}, f.err
}

func makeTestObservations(provider, region string, priceStr string) []domain.PriceObservation {
	price, _ := decimal.NewFromString(priceStr)
	return []domain.PriceObservation{
		{
			Provider:        provider,
			ServiceCategory: "compute",
			SkuID:           "SKU-TEST-001",
			DisplayName:     "test-instance",
			Region:          region,
			RegionGroup:     "us-east",
			Unit:            "Hrs",
			PriceAmount:     price,
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      domain.ComputeAttributes{VCPU: 4, RAMGB: 16, Family: "general"},
			FetchedAt:       time.Now().UTC(),
		},
	}
}

func parseDatabaseConfig(t *testing.T, dbURL string) config.DatabaseConfig {
	t.Helper()
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse db url: %v", err)
	}
	pwd, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}
	return config.DatabaseConfig{
		Host:            u.Hostname(),
		Port:            port,
		User:            u.User.Username(),
		Password:        pwd,
		Name:            strings.TrimPrefix(u.Path, "/"),
		SSLMode:         u.Query().Get("sslmode"),
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 60,
		ConnMaxIdleTime: 30,
	}
}

// --- Tests ---

func TestOrchestrator_RunAll_ParallelFanOut(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)

	// Create two adapters with a small delay to verify parallelism
	adapter1 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/test.json",
		},
		delay: 50 * time.Millisecond,
	}
	adapter2 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("azure", "eastus", "0.60"),
			RawGCSPath:   "raw/azure/compute/test.json",
		},
		delay: 50 * time.Millisecond,
	}

	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter1)
	factory.Register(provider.ProviderConfig{
		Provider: "azure",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter2)

	dlqSvc := dlq.New(redisClient)
	orch := service.NewOrchestrator(queries, redisClient, dlqSvc, factory, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})

	start := time.Now()
	results := orch.RunAll(ctx)
	elapsed := time.Since(start)

	// Both jobs should complete
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Check both succeeded
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("%s/%s failed: %v", r.Provider, r.Category, r.Err)
		}
		if r.InsertedCount != 1 {
			t.Errorf("%s/%s inserted %d, want 1", r.Provider, r.Category, r.InsertedCount)
		}
	}

	// Both adapters ran in parallel, so elapsed should be roughly 50ms, not 100ms.
	if elapsed > 300*time.Millisecond {
		t.Errorf("fan-out took %v; expected parallel execution to be faster", elapsed)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestOrchestrator_OneFailureDoesNotBlockOthers(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)

	successAdapter := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/test.json",
		},
	}
	failAdapter := &failingFetcher{err: errors.New("connection refused")}

	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, successAdapter)
	factory.Register(provider.ProviderConfig{
		Provider: "azure",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, failAdapter)

	dlqSvc := dlq.New(redisClient)
	orch := service.NewOrchestrator(queries, redisClient, dlqSvc, factory, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})

	results := orch.RunAll(ctx)

	var successCount, failCount int
	for _, r := range results {
		if r.Err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
	if failCount != 1 {
		t.Errorf("expected 1 failure, got %d", failCount)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestOrchestrator_LockPreventsOverlap(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)

	// Pre-acquire the lock for aws:compute
	lock, err := cache.AcquireIngestionLock(ctx, redisClient, "aws", "compute", 30*time.Second)
	if err != nil {
		t.Fatalf("pre-acquire lock: %v", err)
	}
	defer func() { _ = lock.Release(ctx) }()

	adapter := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/test.json",
		},
	}

	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter)

	dlqSvc := dlq.New(redisClient)
	orch := service.NewOrchestrator(queries, redisClient, dlqSvc, factory, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})

	results := orch.RunAll(ctx)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Job should be skipped, not failed
	if !results[0].Skipped {
		t.Error("expected job to be skipped due to lock")
	}
	if results[0].Err != nil {
		t.Errorf("skipped job should not have error, got: %v", results[0].Err)
	}

	// Adapter should NOT have been called
	if adapter.callCount.Load() != 0 {
		t.Errorf("adapter should not have been called, but was called %d times", adapter.callCount.Load())
	}
}

func TestOrchestrator_DLQ_RecordOnFailure_ClearOnSuccess(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)
	dlqSvc := dlq.New(redisClient)

	// Phase 1: Fail the job → DLQ entry created
	failAdapter := &failingFetcher{err: errors.New("upstream timeout")}
	factory1 := provider.NewFactory()
	factory1.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, failAdapter)

	orch1 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory1, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	orch1.RunAll(ctx)

	entry, err := dlqSvc.Get(ctx, "aws", "compute")
	if err != nil {
		t.Fatalf("expected DLQ entry after failure, got error: %v", err)
	}
	if entry.ConsecutiveFailures != 1 {
		t.Errorf("consecutive_failures = %d, want 1", entry.ConsecutiveFailures)
	}
	if entry.Status != "failed" {
		t.Errorf("status = %q, want %q", entry.Status, "failed")
	}

	// Phase 2: Succeed → DLQ entry cleared
	successAdapter := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/test.json",
		},
	}
	factory2 := provider.NewFactory()
	factory2.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, successAdapter)

	orch2 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory2, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	orch2.RunAll(ctx)

	_, err = dlqSvc.Get(ctx, "aws", "compute")
	if !errors.Is(err, dlq.ErrEntryNotFound) {
		t.Errorf("expected DLQ entry to be cleared after success, got: %v", err)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestOrchestrator_DLQ_BlockedForAuthErrors(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)
	dlqSvc := dlq.New(redisClient)

	// Auth error should be marked as "blocked"
	failAdapter := &failingFetcher{err: errors.New("http 401: authentication failed")}
	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, failAdapter)

	orch := service.NewOrchestrator(queries, redisClient, dlqSvc, factory, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	orch.RunAll(ctx)

	entry, err := dlqSvc.Get(ctx, "aws", "compute")
	if err != nil {
		t.Fatalf("expected DLQ entry, got error: %v", err)
	}
	if entry.Status != "blocked" {
		t.Errorf("status = %q, want %q for auth error", entry.Status, "blocked")
	}
}

func TestOrchestrator_AnomalyDetection_FlagsSuspiciousPrice(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)
	dlqSvc := dlq.New(redisClient)

	// Phase 1: Insert a baseline price of $0.50
	adapter1 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/baseline.json",
		},
	}
	factory1 := provider.NewFactory()
	factory1.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter1)

	orch1 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory1, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	results1 := orch1.RunAll(ctx)
	if results1[0].Err != nil {
		t.Fatalf("baseline insertion failed: %v", results1[0].Err)
	}
	if results1[0].AnomalyCount != 0 {
		t.Errorf("baseline should have 0 anomalies, got %d", results1[0].AnomalyCount)
	}

	// Phase 2: Insert price of $50.00 (100x increase → should flag anomaly)
	adapter2 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "50.00"),
			RawGCSPath:   "raw/aws/compute/anomaly.json",
		},
	}
	factory2 := provider.NewFactory()
	factory2.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter2)

	orch2 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory2, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	results2 := orch2.RunAll(ctx)
	if results2[0].Err != nil {
		t.Fatalf("anomaly insertion failed: %v", results2[0].Err)
	}
	if results2[0].AnomalyCount != 1 {
		t.Errorf("expected 1 anomaly, got %d", results2[0].AnomalyCount)
	}

	// Verify the anomaly row exists in DB with pending_review status
	rows, err := queries.GetPriceObservations(ctx, store.GetPriceObservationsParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		RegionGroup:     "us-east",
	})
	if err != nil {
		t.Fatalf("GetPriceObservations: %v", err)
	}

	var foundAnomaly bool
	for _, row := range rows {
		if row.AnomalyStatus.Valid && row.AnomalyStatus.String == "pending_review" {
			foundAnomaly = true
			break
		}
	}
	if !foundAnomaly {
		t.Error("expected at least one row with anomaly_status = 'pending_review'")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestOrchestrator_AnomalyDetection_NoFlagForNormalPrice(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	_, redisClient := setupTestRedis(t)
	dlqSvc := dlq.New(redisClient)

	// Phase 1: Baseline price $0.50
	adapter1 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/baseline.json",
		},
	}
	factory1 := provider.NewFactory()
	factory1.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter1)

	orch1 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory1, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	orch1.RunAll(ctx)

	// Phase 2: Price $0.55 (10% increase → NOT anomalous, within 10x threshold)
	adapter2 := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.55"),
			RawGCSPath:   "raw/aws/compute/normal.json",
		},
	}
	factory2 := provider.NewFactory()
	factory2.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter2)

	orch2 := service.NewOrchestrator(queries, redisClient, dlqSvc, factory2, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})
	results2 := orch2.RunAll(ctx)
	if results2[0].Err != nil {
		t.Fatalf("normal price insertion failed: %v", results2[0].Err)
	}
	if results2[0].AnomalyCount != 0 {
		t.Errorf("expected 0 anomalies for normal price change, got %d", results2[0].AnomalyCount)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestOrchestrator_WorksWithoutRedis(t *testing.T) {
	dbURL := testDatabaseURL(t)
	ctx := context.Background()

	pool, err := store.NewPool(ctx, parseDatabaseConfig(t, dbURL))
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)

	adapter := &mockFetcher{
		result: domain.FetchResult{
			Observations: makeTestObservations("aws", "us-east-1", "0.50"),
			RawGCSPath:   "raw/aws/compute/test.json",
		},
	}

	factory := provider.NewFactory()
	factory.Register(provider.ProviderConfig{
		Provider: "aws",
		Category: "compute",
		Retry:    provider.RetryConfig{MaxAttempts: 1, BaseDelay: time.Millisecond},
	}, adapter)

	// No Redis, no DLQ
	orch := service.NewOrchestrator(queries, nil, nil, factory, service.OrchestratorConfig{
		MaxConcurrency: 5,
		LockTTL:        30 * time.Second,
	})

	results := orch.RunAll(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("unexpected error: %v", results[0].Err)
	}
	if results[0].InsertedCount != 1 {
		t.Errorf("inserted = %d, want 1", results[0].InsertedCount)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE sku_id = 'SKU-TEST-001'")
}

func TestDefaultOrchestratorConfig(t *testing.T) {
	cfg := service.DefaultOrchestratorConfig()
	if cfg.MaxConcurrency <= 0 {
		t.Errorf("expected MaxConcurrency > 0, got %d", cfg.MaxConcurrency)
	}
	if cfg.LockTTL <= 0 {
		t.Errorf("expected LockTTL > 0, got %v", cfg.LockTTL)
	}
}

func TestNewOrchestrator_InitializesCleanly(t *testing.T) {
	factory := provider.NewFactory()
	cfg := service.DefaultOrchestratorConfig()

	// Should not panic without tracer/meter
	orch := service.NewOrchestrator(nil, nil, nil, factory, cfg)
	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}
