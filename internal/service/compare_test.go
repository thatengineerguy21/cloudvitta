package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/fx"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

func TestPricingService_Compare_Compute(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			PriceCurrency:   "USD",
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsObs, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	res, err := svc.Compare(ctx, "compute", regionGroup, service.MatchTarget{
		VCPU:         4,
		RAMGB:        16,
		Family:       "general_purpose",
		StrictFamily: true,
		Category:     "compute",
	})
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 compute comparison result, got 0")
	}
	if res.Results[0].Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", res.Results[0].Provider)
	}
	if res.Results[0].SkuID != "m5.xlarge" {
		t.Errorf("expected SkuID 'm5.xlarge', got %s", res.Results[0].SkuID)
	}
	if !res.Results[0].HourlyCost.Equal(decimal.RequireFromString("0.192")) {
		t.Errorf("expected hourly cost 0.192, got %s", res.Results[0].HourlyCost)
	}
}

func TestPricingService_Compare_Storage(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			PriceCurrency:   "USD",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsStorage, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	res, err := svc.Compare(ctx, "storage", regionGroup, service.MatchTarget{
		SizeGB:       500,
		StorageClass: "standard",
		Category:     "storage",
	})
	if err != nil {
		t.Fatalf("Compare storage failed: %v", err)
	}

	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 storage comparison result, got 0")
	}
	if res.Results[0].Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", res.Results[0].Provider)
	}
	expectedMonthly := decimal.RequireFromString("0.023").Mul(decimal.NewFromInt(500))
	if !res.Results[0].MonthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected monthly cost %s, got %s", expectedMonthly, res.Results[0].MonthlyCost)
	}
}

func TestPricingService_Compare_Network(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsNetwork := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "network",
			SkuID:           "data-transfer-out",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.090"),
			PriceCurrency:   "USD",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     100,
				TransferType: "internet_egress",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsNetwork, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	res, err := svc.Compare(ctx, "network", regionGroup, service.MatchTarget{
		EgressGB:     100,
		TransferType: "internet_egress",
		Category:     "network",
	})
	if err != nil {
		t.Fatalf("Compare network failed: %v", err)
	}

	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 network comparison result, got 0")
	}
	if res.Results[0].Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", res.Results[0].Provider)
	}
	expectedMonthly := decimal.RequireFromString("0.090").Mul(decimal.NewFromInt(100))
	if !res.Results[0].MonthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected monthly cost %s, got %s", expectedMonthly, res.Results[0].MonthlyCost)
	}
}

func TestPricingService_Compare_Kubernetes(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsK8s := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-AWS-EKS-STD",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.100"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: "standard",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), awsK8s, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	res, err := svc.Compare(ctx, "kubernetes", regionGroup, service.MatchTarget{
		KubernetesTier: "standard",
		Category:       "kubernetes",
	})
	if err != nil {
		t.Fatalf("Compare kubernetes failed: %v", err)
	}

	if len(res.Results) == 0 {
		t.Fatal("expected at least 1 kubernetes comparison result, got 0")
	}
	if res.Results[0].Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", res.Results[0].Provider)
	}
	if res.Results[0].MatchedKubernetes.Tier != "standard" {
		t.Errorf("expected matched tier 'standard', got %s", res.Results[0].MatchedKubernetes.Tier)
	}
	if !res.Results[0].HourlyCost.Equal(decimal.RequireFromString("0.100")) {
		t.Errorf("expected hourly cost 0.100, got %s", res.Results[0].HourlyCost)
	}
}

func TestPricingService_Compare_FXConversion(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// Provider offering price in CNY (e.g. 7.20 CNY/hour)
	cnyObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "cny.compute.1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("7.200"),
			PriceCurrency:   "CNY",
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), cnyObs, cache.DefaultTTL)

	// FX Service with 1 USD = 7.20 CNY
	fxSvc := fx.NewService(nil, nil, fx.WithInitialRates(map[string]fx.CachedRate{
		"USD": {
			Rate:   decimal.NewFromInt(1),
			Source: "base",
		},
		"CNY": {
			Rate:       decimal.RequireFromString("7.2000"),
			Source:     "frankfurter",
			RateDate:   "2026-08-24",
			FetchedAt:  time.Now().UTC(),
			IsFallback: true,
		},
	}))

	svc := service.NewPricingService(nil, rdb, service.WithFXService(fxSvc))

	res, err := svc.Compare(ctx, "compute", regionGroup, service.MatchTarget{
		VCPU:         4,
		RAMGB:        16,
		Family:       "general_purpose",
		StrictFamily: true,
		Category:     "compute",
	})
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if len(res.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res.Results))
	}

	item := res.Results[0]
	// Preserves original native currency and price amount
	if item.PriceCurrency != "CNY" {
		t.Errorf("expected native currency CNY, got %s", item.PriceCurrency)
	}
	if !item.PriceAmount.Equal(decimal.RequireFromString("7.200")) {
		t.Errorf("expected native price 7.200, got %s", item.PriceAmount)
	}
	// Hourly cost is normalized to USD (7.20 CNY / 7.20 = 1.00 USD)
	if !item.HourlyCost.Equal(decimal.RequireFromString("1.000")) {
		t.Errorf("expected normalized hourly USD 1.000, got %s", item.HourlyCost)
	}

	// Stale FX rate fallback warning should be populated
	var hasFallbackWarning bool
	for _, w := range res.Warnings {
		if w.Code == "stale_fx_rate" {
			hasFallbackWarning = true
			break
		}
	}
	if !hasFallbackWarning {
		t.Errorf("expected stale_fx_rate warning due to fallback rate")
	}
}
