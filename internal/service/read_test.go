package service_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func TestPricingService_GetPrices_CacheHit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")

	cachedObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-TEST-1",
			DisplayName:     "t3.micro",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.0104"),
			PriceCurrency:   "USD",
		},
	}

	if err := cache.Warm(ctx, rdb, key, cachedObs, cache.DefaultTTL); err != nil {
		t.Fatalf("cache.Warm failed: %v", err)
	}

	// Passing nil queries to verify DB is never touched on cache hit
	svc := service.NewPricingService(nil, rdb)

	got, err := svc.GetPrices(ctx, "aws", "compute", "us-east-1")
	if err != nil {
		t.Fatalf("GetPrices unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d observations, want 1", len(got))
	}
	if got[0].SkuID != "SKU-TEST-1" {
		t.Errorf("SkuID = %q, want SKU-TEST-1", got[0].SkuID)
	}
}

func TestPricingService_GetPrices_AllCategories_CacheHit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	svc := service.NewPricingService(nil, rdb)

	categories := []string{
		"compute",
		"storage",
		"network",
		"database_rdbms",
		"database_nosql",
		"kubernetes",
		"serverless",
	}

	for _, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			skuID := "SKU-" + strings.ToUpper(cat) + "-1"
			key := cache.BuildKey(cache.SchemaVersion, "aws", cat, "us-east-1")

			cachedObs := []domain.PriceObservation{
				{
					Provider:        "aws",
					ServiceCategory: cat,
					SkuID:           skuID,
					DisplayName:     cat + " resource",
					Region:          "us-east-1",
					RegionGroup:     "us-east",
					PriceAmount:     decimal.RequireFromString("0.05"),
					PriceCurrency:   "USD",
				},
			}

			if err := cache.Warm(ctx, rdb, key, cachedObs, cache.DefaultTTL); err != nil {
				t.Fatalf("cache.Warm failed for %s: %v", cat, err)
			}

			got, err := svc.GetPrices(ctx, "aws", cat, "us-east-1")
			if err != nil {
				t.Fatalf("GetPrices unexpected error for %s: %v", cat, err)
			}

			if len(got) != 1 {
				t.Fatalf("got %d observations for %s, want 1", len(got), cat)
			}
			if got[0].SkuID != skuID {
				t.Errorf("SkuID = %q, want %q", got[0].SkuID, skuID)
			}
			if got[0].ServiceCategory != cat {
				t.Errorf("ServiceCategory = %q, want %q", got[0].ServiceCategory, cat)
			}
		})
	}
}

// spyDBTX implements a spy over store.DBTX to count database calls.
type spyDBTX struct {
	store.DBTX
	callCount int64
}

func (s *spyDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	atomic.AddInt64(&s.callCount, 1)
	return s.DBTX.Query(ctx, sql, args...)
}

func setupTestDBPool(t *testing.T, ctx context.Context, dbURL string) (*url.URL, *pgxpool.Pool) {
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

	cfg := config.DatabaseConfig{
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

	pool, err := store.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	return u, pool
}

func TestPricingService_GetPrices_SingleflightCollapse(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	dbURL := testDatabaseURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, pool := setupTestDBPool(t, ctx, dbURL)
	_ = u
	defer pool.Close()

	// Start a transaction that we will roll back at the end of the test.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("pool.Begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Wrap tx in our spy to verify singleflight works
	spy := &spyDBTX{DBTX: tx}
	queries := store.New(spy)

	var priceAmt pgtype.Numeric
	_ = priceAmt.Scan("0.05")

	now := time.Now().Truncate(time.Microsecond)
	params := store.InsertPriceObservationParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		SkuID:           "SKU-SINGLEFLIGHT",
		DisplayName:     "t3.medium",
		Region:          "us-east-1",
		RegionGroup:     "us-east",
		Unit:            "Hrs",
		PriceAmount:     priceAmt,
		PriceCurrency:   "USD",
		PricingModel:    "OnDemand",
		Attributes:      []byte(`{"vcpu":2,"ram_gb":4,"family":"t3"}`),
		RawResponseRef:  pgtype.Text{String: "raw/aws/2026-08-13.json", Valid: true},
		FetchedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		LastSeenAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}

	_, err = queries.InsertPriceObservation(ctx, params)
	if err != nil {
		t.Fatalf("InsertPriceObservation failed: %v", err)
	}

	svc := service.NewPricingService(queries, rdb)

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([][]domain.PriceObservation, numGoroutines)
	errorsList := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			results[workerID], errorsList[workerID] = svc.GetPrices(ctx, "aws", "compute", "us-east-1")
		}()
	}
	wg.Wait()

	for i := 0; i < numGoroutines; i++ {
		if errorsList[i] != nil {
			t.Fatalf("goroutine %d failed: %v", i, errorsList[i])
		}
		if len(results[i]) == 0 {
			t.Fatalf("goroutine %d returned 0 rows", i)
		}
	}

	// Verify key was warmed in Redis
	key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	cached, err := cache.Get(ctx, rdb, key)
	if err != nil {
		t.Fatalf("cache.Get after singleflight miss failed: %v", err)
	}
	if len(cached) == 0 {
		t.Fatalf("expected cached rows in Redis, got 0")
	}

	// Wait for singleflight context background jobs (warming) to finish
	time.Sleep(100 * time.Millisecond)

	// Singleflight must ensure exactly 1 database call happened
	// Note: callCount includes both the InsertPriceObservation and GetPriceObservations queries.
	// Since we inserted 1 row, callCount should be 2.
	if spy.callCount != 2 {
		t.Fatalf("expected exactly 2 DB calls (1 insert, 1 query), got %d", spy.callCount)
	}
}

func TestPricingService_GetPrices_CacheMissTTLFallback(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	dbURL := testDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, pool := setupTestDBPool(t, ctx, dbURL)
	_ = u
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("pool.Begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := store.New(tx)

	var priceAmt pgtype.Numeric
	_ = priceAmt.Scan("0.05")

	now := time.Now().Truncate(time.Microsecond)
	params := store.InsertPriceObservationParams{
		Provider:        "aws",
		ServiceCategory: "compute",
		SkuID:           "SKU-MISS",
		DisplayName:     "t3.miss",
		Region:          "us-east-1",
		RegionGroup:     "us-east",
		Unit:            "Hrs",
		PriceAmount:     priceAmt,
		PriceCurrency:   "USD",
		PricingModel:    "OnDemand",
		Attributes:      []byte(`{"vcpu":2,"ram_gb":4,"family":"t3"}`),
		RawResponseRef:  pgtype.Text{String: "raw/aws/miss.json", Valid: true},
		FetchedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		LastSeenAt:      pgtype.Timestamptz{Time: now, Valid: true},
	}

	_, err = queries.InsertPriceObservation(ctx, params)
	if err != nil {
		t.Fatalf("InsertPriceObservation failed: %v", err)
	}

	svc := service.NewPricingService(queries, rdb)

	// Fetch without cache warming first (trigger miss & fallback)
	got, err := svc.GetPrices(ctx, "aws", "compute", "us-east-1")
	if err != nil {
		t.Fatalf("GetPrices failed: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected rows from DB fallback, got 0")
	}

	// Wait for cache warming to complete in background
	time.Sleep(100 * time.Millisecond)

	// Verify key was set with TTL
	key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("rdb.TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %v", ttl)
	}
}

func TestPricingService_GetPrices_SingleflightCancellation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	dbURL := testDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, pool := setupTestDBPool(t, ctx, dbURL)
	_ = u
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("pool.Begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	spy := &spyDBTX{DBTX: tx}
	queries := store.New(spy)

	// We don't necessarily need to insert data; getting a valid empty result or any result is fine
	// as long as the second caller completes successfully despite the first caller canceling.
	svc := service.NewPricingService(queries, rdb)

	var wg sync.WaitGroup
	wg.Add(2)

	// Caller 1 context will be cancelled almost immediately
	ctx1, cancel1 := context.WithCancel(ctx)

	var err1 error
	go func() {
		defer wg.Done()
		_, err1 = svc.GetPrices(ctx1, "aws", "compute", "us-cancel-test")
	}()

	// Caller 2 context remains live
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()

	var err2 error
	go func() {
		defer wg.Done()
		// Sleep slightly to ensure Caller 1 starts the singleflight block first
		time.Sleep(10 * time.Millisecond)
		_, err2 = svc.GetPrices(ctx2, "aws", "compute", "us-cancel-test")
	}()

	// Cancel Caller 1 after giving it a chance to enter Singleflight DoChan
	time.Sleep(20 * time.Millisecond)
	cancel1()

	wg.Wait()

	if err1 == nil || !errors.Is(err1, context.Canceled) {
		t.Errorf("expected caller 1 to fail with context.Canceled, got %v", err1)
	}

	if err2 != nil {
		t.Errorf("expected caller 2 to succeed despite caller 1 cancellation, got error: %v", err2)
	}
}

func TestPricingService_GetPrices_UnsupportedCategory(t *testing.T) {
	svc := service.NewPricingService(nil, nil)

	_, err := svc.GetPrices(context.Background(), "aws", "unsupported_cat", "us-east")
	if err == nil || !errors.Is(err, service.ErrCategoryNotSupported) {
		t.Errorf("GetPrices with unsupported category error = %v, want ErrCategoryNotSupported", err)
	}

	_, err = svc.GetPrices(context.Background(), "unsupported_provider", "compute", "us-east")
	if err == nil || !errors.Is(err, service.ErrCategoryNotSupported) {
		t.Errorf("GetPrices with unsupported provider error = %v, want ErrCategoryNotSupported", err)
	}
}
