package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestCategorySchemaVersion(t *testing.T) {
	if got := cache.CategorySchemaVersion("network"); got != cache.SchemaVersionNetwork {
		t.Errorf("CategorySchemaVersion(network) = %q, want %q", got, cache.SchemaVersionNetwork)
	}
	if cache.SchemaVersionNetwork != "v2" {
		t.Errorf("SchemaVersionNetwork = %q, want v2", cache.SchemaVersionNetwork)
	}

	for _, cat := range []string{"compute", "storage", "database_rdbms", "database_nosql", "kubernetes", "serverless"} {
		if got := cache.CategorySchemaVersion(cat); got != cache.SchemaVersion {
			t.Errorf("CategorySchemaVersion(%s) = %q, want %q", cat, got, cache.SchemaVersion)
		}
	}
}

func TestNetworkCacheVersionIsolation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()

	// 1. Simulate legacy v1 network cache entry with old attribute format
	v1Key := cache.BuildKeyRaw("v1", "aws", "network", "us-east-1")
	v1Observations := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-LEGACY-V1",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			PriceAmount:       decimal.RequireFromString("0.09"),
			PriceCurrency:     "USD",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1, TransferType: "internet_egress"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	if err := cache.Warm(ctx, rdb, v1Key, v1Observations, cache.DefaultTTL); err != nil {
		t.Fatalf("cache.Warm v1 failed: %v", err)
	}

	// 2. Querying via CategorySchemaVersion("network") should construct a v2 key
	v2Key := cache.BuildKey(cache.CategorySchemaVersion("network"), "aws", "network", "us-east-1")
	if v2Key != "v2:aws:network:us-east-1" {
		t.Fatalf("unexpected v2Key: %s", v2Key)
	}

	// 3. Verify cache miss on v2Key: old v1 entry is never read as new shape
	_, err = cache.Get(ctx, rdb, v2Key)
	if err == nil {
		t.Fatalf("expected cache miss for %s, but read old v1 data", v2Key)
	}

	// 4. Warm v2 entry with new transfer taxonomy attributes
	v2Observations := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "network",
			SkuID:             "SKU-AWS-DIRECTCONNECT-V2",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			PriceAmount:       decimal.RequireFromString("0.02"),
			PriceCurrency:     "USD",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1, TransferType: "direct_connect_egress"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	if err := cache.Warm(ctx, rdb, v2Key, v2Observations, cache.DefaultTTL); err != nil {
		t.Fatalf("cache.Warm v2 failed: %v", err)
	}

	// 5. Verify v2Key returns fresh v2 observation
	gotV2, err := cache.Get(ctx, rdb, v2Key)
	if err != nil {
		t.Fatalf("cache.Get v2 failed: %v", err)
	}
	if len(gotV2) != 1 || gotV2[0].SkuID != "SKU-AWS-DIRECTCONNECT-V2" {
		t.Fatalf("unexpected cached v2 data: %+v", gotV2)
	}
	if gotV2[0].NetworkAttributes.TransferType != "direct_connect_egress" {
		t.Errorf("unexpected transfer type: %s", gotV2[0].NetworkAttributes.TransferType)
	}
}
