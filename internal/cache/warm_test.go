package cache_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestBuildKey(t *testing.T) {
	key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	want := "v1:aws:compute:us-east-1"
	if key != want {
		t.Errorf("BuildKey() = %q, want %q", key, want)
	}
}

func TestWarmAndGet(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer s.Close()

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	key := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")

	// Test Get on missing key returns ErrCacheMiss
	_, err = cache.Get(ctx, rdb, key)
	if !errors.Is(err, cache.ErrCacheMiss) {
		t.Fatalf("Get missing key err = %v, want ErrCacheMiss", err)
	}

	sampleData := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-C5-XLARGE",
			DisplayName:     "c5.xlarge",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			Unit:            "Hrs",
			PriceAmount:     decimal.NewFromFloat(0.17),
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  8,
				Family: "c5",
			},
			FetchedAt: time.Now().UTC().Truncate(time.Second),
		},
	}

	// Warm cache
	if err := cache.Warm(ctx, rdb, key, sampleData, cache.DefaultTTL); err != nil {
		t.Fatalf("Warm() unexpected error: %v", err)
	}

	// Retrieve data
	got, err := cache.Get(ctx, rdb, key)
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("Get() returned %d items, want 1", len(got))
	}
	if got[0].SkuID != sampleData[0].SkuID {
		t.Errorf("SkuID = %q, want %q", got[0].SkuID, sampleData[0].SkuID)
	}
	if !got[0].PriceAmount.Equal(sampleData[0].PriceAmount) {
		t.Errorf("PriceAmount = %s, want %s", got[0].PriceAmount.String(), sampleData[0].PriceAmount.String())
	}

	// Verify TTL set in miniredis
	ttl := s.TTL(key)
	if ttl <= 0 || ttl > cache.DefaultTTL {
		t.Errorf("TTL = %v, want > 0 and <= %v", ttl, cache.DefaultTTL)
	}
}
