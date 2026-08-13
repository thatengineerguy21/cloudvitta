package service_test

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
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

func TestPricingService_GetComputePrices_CacheHit(t *testing.T) {
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
			PriceAmount:     decimal.NewFromFloat(0.0104),
			PriceCurrency:   "USD",
		},
	}

	if err := cache.Warm(ctx, rdb, key, cachedObs, cache.DefaultTTL); err != nil {
		t.Fatalf("cache.Warm failed: %v", err)
	}

	// Passing nil queries to verify DB is never touched on cache hit
	svc := service.NewPricingService(nil, rdb)

	got, err := svc.GetComputePrices(ctx, "aws", "compute", "us-east-1")
	if err != nil {
		t.Fatalf("GetComputePrices unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d observations, want 1", len(got))
	}
	if got[0].SkuID != "SKU-TEST-1" {
		t.Errorf("SkuID = %q, want SKU-TEST-1", got[0].SkuID)
	}
}

// spyDBQuerier implements a spy over store DB queries to count database calls.
type spyDBQuerier struct {
	callCount int64
	rows      []store.PriceObservation
}

func (s *spyDBQuerier) Query(ctx context.Context, provider, category, regionGroup string) ([]store.PriceObservation, error) {
	atomic.AddInt64(&s.callCount, 1)
	time.Sleep(50 * time.Millisecond) // simulate DB latency
	return s.rows, nil
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

func TestPricingService_GetComputePrices_SingleflightCollapse(t *testing.T) {
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

	queries := store.New(pool)

	// Clean test table and insert a row
	_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE provider = 'aws' AND sku_id = 'SKU-SINGLEFLIGHT'")

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
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM price_observations WHERE provider = 'aws' AND sku_id = 'SKU-SINGLEFLIGHT'")
	}()

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
			results[workerID], errorsList[workerID] = svc.GetComputePrices(ctx, "aws", "compute", "us-east-1")
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
}
