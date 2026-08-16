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
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

func TestPricingService_Calculate_CompleteAllCategories(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// 1. Warm Redis for AWS (compute, storage, network)
	awsCompute := []domain.PriceObservation{
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"), // $0.023 / GB-month
			PriceCurrency:   "USD",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", regionGroup), awsStorage, cache.DefaultTTL)

	awsNetwork := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "network",
			SkuID:           "data-transfer-out",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.090"), // $0.090 / GB
			PriceCurrency:   "USD",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     100,
				TransferType: "internet_egress",
			},
			FetchedAt: time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", regionGroup), awsNetwork, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Compute: &service.CalculateComputeTarget{
			VCPU:         4,
			RAMGB:        16,
			Family:       "general_purpose",
			StrictFamily: true,
		},
		Storage: &service.CalculateStorageTarget{
			SizeGB:       decimal.NewFromInt(500),
			StorageClass: "standard",
		},
		Network: &service.CalculateNetworkTarget{
			EgressGB:     decimal.NewFromInt(100),
			TransferType: "internet_egress",
		},
	}

	res, err := svc.Calculate(ctx, req)
	if err != nil {
		t.Fatalf("Calculate unexpected error: %v", err)
	}

	if len(res.Results) != 1 {
		t.Fatalf("got %d results, want 1 (aws)", len(res.Results))
	}

	awsRes := res.Results[0]
	if awsRes.Provider != "aws" {
		t.Errorf("Provider = %q, want 'aws'", awsRes.Provider)
	}
	if awsRes.Partial {
		t.Errorf("Partial = true, want false for complete match")
	}
	if awsRes.TotalNormalizedHourlyUSD == nil {
		t.Fatalf("TotalNormalizedHourlyUSD is nil, want populated total")
	}
	if awsRes.PartialTotalNormalizedHourlyUSD != nil {
		t.Errorf("PartialTotalNormalizedHourlyUSD = %v, want nil for complete match", awsRes.PartialTotalNormalizedHourlyUSD)
	}

	// Compute: 0.192
	// Storage: 500 * 0.023 / 730 = 11.5 / 730 = 0.0157534246575342
	// Network: 100 * 0.090 / 730 = 9 / 730 = 0.0123287671232877
	// Expected Total: 0.192 + (20.5 / 730)
	expectedStorageHourly := decimal.RequireFromString("0.023").Mul(decimal.NewFromInt(500)).Div(service.HoursInMonth)
	expectedNetworkHourly := decimal.RequireFromString("0.090").Mul(decimal.NewFromInt(100)).Div(service.HoursInMonth)
	expectedTotal := decimal.RequireFromString("0.192").Add(expectedStorageHourly).Add(expectedNetworkHourly)

	if !awsRes.TotalNormalizedHourlyUSD.Equal(expectedTotal) {
		t.Errorf("TotalNormalizedHourlyUSD = %s, want %s", awsRes.TotalNormalizedHourlyUSD.String(), expectedTotal.String())
	}

	if len(awsRes.Categories) != 3 {
		t.Errorf("got %d categories in map, want 3", len(awsRes.Categories))
	}
	if awsRes.Categories["compute"].SkuID != "m5.xlarge" {
		t.Errorf("Compute SkuID = %q, want m5.xlarge", awsRes.Categories["compute"].SkuID)
	}
	if awsRes.Categories["storage"].SkuID != "s3-standard" {
		t.Errorf("Storage SkuID = %q, want s3-standard", awsRes.Categories["storage"].SkuID)
	}
	if awsRes.Categories["network"].SkuID != "data-transfer-out" {
		t.Errorf("Network SkuID = %q, want data-transfer-out", awsRes.Categories["network"].SkuID)
	}
}

func TestPricingService_Calculate_PartialProvider_ADR0022(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// AWS: has compute and storage (missing network) -> partial
	awsCompute := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", regionGroup), awsStorage, cache.DefaultTTL)

	// AWS network is empty []

	// Azure: has compute, storage, and network -> complete
	azureCompute := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "Standard_D4s_v5",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", regionGroup), azureCompute, cache.DefaultTTL)

	azureStorage := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "storage",
			SkuID:           "blob-hot",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0184"),
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", regionGroup), azureStorage, cache.DefaultTTL)

	azureNetwork := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "network",
			SkuID:           "azure-egress",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.087"),
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     100,
				TransferType: "internet_egress",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", regionGroup), azureNetwork, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Compute: &service.CalculateComputeTarget{
			VCPU:  4,
			RAMGB: 16,
		},
		Storage: &service.CalculateStorageTarget{
			SizeGB:       decimal.NewFromInt(500),
			StorageClass: "standard",
		},
		Network: &service.CalculateNetworkTarget{
			EgressGB:     decimal.NewFromInt(100),
			TransferType: "internet_egress",
		},
	}

	res, err := svc.Calculate(ctx, req)
	if err != nil {
		t.Fatalf("Calculate unexpected error: %v", err)
	}

	var awsFound, azureFound bool
	for _, r := range res.Results {
		if r.Provider == "aws" {
			awsFound = true
			if !r.Partial {
				t.Errorf("AWS: Partial = false, want true for missing network category")
			}
			if r.TotalNormalizedHourlyUSD != nil {
				t.Errorf("AWS: TotalNormalizedHourlyUSD = %v, want nil per ADR 0022", r.TotalNormalizedHourlyUSD)
			}
			if r.PartialTotalNormalizedHourlyUSD == nil {
				t.Fatalf("AWS: PartialTotalNormalizedHourlyUSD is nil, want sum of available categories")
			}
			expectedAWSPartial := decimal.RequireFromString("0.192").Add(
				decimal.RequireFromString("0.023").Mul(decimal.NewFromInt(500)).Div(service.HoursInMonth),
			)
			if !r.PartialTotalNormalizedHourlyUSD.Equal(expectedAWSPartial) {
				t.Errorf("AWS: PartialTotalNormalizedHourlyUSD = %s, want %s", r.PartialTotalNormalizedHourlyUSD.String(), expectedAWSPartial.String())
			}
			if _, hasNetwork := r.Categories["network"]; hasNetwork {
				t.Errorf("AWS: categories should NOT contain 'network'")
			}
			if _, hasCompute := r.Categories["compute"]; !hasCompute {
				t.Errorf("AWS: categories should contain 'compute'")
			}
			if _, hasStorage := r.Categories["storage"]; !hasStorage {
				t.Errorf("AWS: categories should contain 'storage'")
			}
		}

		if r.Provider == "azure" {
			azureFound = true
			if r.Partial {
				t.Errorf("Azure: Partial = true, want false for complete match")
			}
			if r.TotalNormalizedHourlyUSD == nil {
				t.Fatalf("Azure: TotalNormalizedHourlyUSD is nil, want total")
			}
			if r.PartialTotalNormalizedHourlyUSD != nil {
				t.Errorf("Azure: PartialTotalNormalizedHourlyUSD = %v, want nil", r.PartialTotalNormalizedHourlyUSD)
			}
			if len(r.Categories) != 3 {
				t.Errorf("Azure: got %d categories, want 3", len(r.Categories))
			}
		}
	}

	if !awsFound || !azureFound {
		t.Errorf("expected both AWS and Azure in results, got aws=%v, azure=%v", awsFound, azureFound)
	}

	// Verify warnings contain explanation for missing AWS network data
	var awsNetworkWarnFound bool
	for _, w := range res.Warnings {
		if w.Provider == "aws" && (w.Code == "no_data_available" || w.Code == "fetch_failed" || w.Code == "no_match") {
			awsNetworkWarnFound = true
			break
		}
	}
	if !awsNetworkWarnFound {
		t.Errorf("expected warning for missing AWS network data, got warnings: %+v", res.Warnings)
	}
}

func TestPricingService_Calculate_NoCategoriesRequested(t *testing.T) {
	svc := service.NewPricingService(nil, nil)
	req := service.CalculateRequest{
		Region:   "us-east",
		Currency: "USD",
	}

	_, err := svc.Calculate(context.Background(), req)
	if err == nil {
		t.Fatalf("Calculate expected error when no categories requested, got nil")
	}
	if err != service.ErrNoCategoriesRequested {
		t.Errorf("err = %v, want ErrNoCategoriesRequested", err)
	}
}

func TestPricingService_Calculate_CurrencyWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	awsCompute := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			Attributes: domain.ComputeAttributes{
				VCPU:  4,
				RAMGB: 16,
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", regionGroup), awsCompute, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "EUR",
		Compute: &service.CalculateComputeTarget{
			VCPU:  4,
			RAMGB: 16,
		},
	}

	res, err := svc.Calculate(ctx, req)
	if err != nil {
		t.Fatalf("Calculate unexpected error: %v", err)
	}

	var currencyWarnFound bool
	for _, w := range res.Warnings {
		if w.Provider == "system" && w.Code == "currency_conversion_not_yet_supported" {
			currencyWarnFound = true
			break
		}
	}
	if !currencyWarnFound {
		t.Errorf("expected warning for non-USD currency, got: %+v", res.Warnings)
	}
}
