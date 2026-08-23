package service_test

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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			Region:          "us-east-1",
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsStorage, cache.DefaultTTL)

	awsNetwork := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "network",
			SkuID:           "data-transfer-out",
			Region:          "us-east-1",
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), awsNetwork, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Compute: &domain.ComputeAttributes{
			VCPU:   4,
			RAMGB:  16,
			Family: "general_purpose",
		},
		Storage: &domain.StorageAttributes{
			SizeGB:       500,
			StorageClass: "standard",
		},
		Network: &domain.NetworkAttributes{
			EgressGB:     100,
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsCompute, cache.DefaultTTL)

	awsStorage := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), awsStorage, cache.DefaultTTL)

	// AWS network is empty []

	// Azure: has compute, storage, and network -> complete
	azureCompute := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "Standard_D4s_v5",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.192"),
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "general_purpose",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "eastus"), azureCompute, cache.DefaultTTL)

	azureStorage := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "storage",
			SkuID:           "blob-hot",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0184"),
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       500,
				StorageClass: "standard",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "eastus"), azureStorage, cache.DefaultTTL)

	azureNetwork := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "network",
			SkuID:           "azure-egress",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.087"),
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     100,
				TransferType: "internet_egress",
			},
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "network", "eastus"), azureNetwork, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Compute: &domain.ComputeAttributes{
			VCPU:  4,
			RAMGB: 16,
		},
		Storage: &domain.StorageAttributes{
			SizeGB:       500,
			StorageClass: "standard",
		},
		Network: &domain.NetworkAttributes{
			EgressGB:     100,
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), awsCompute, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "EUR",
		Compute: &domain.ComputeAttributes{
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
		if w.Provider == "system" && w.Code == "non_usd_currency_unsupported" {
			currencyWarnFound = true
			break
		}
	}
	if !currencyWarnFound {
		t.Errorf("expected warning for non-USD currency, got: %+v", res.Warnings)
	}
}

func TestPricingService_Calculate_SevenCategories(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"
	now := time.Now().UTC()

	// 1. AWS Compute
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "c5.xlarge",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.170"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			Attributes:      domain.ComputeAttributes{VCPU: 4, RAMGB: 8, Family: "compute_optimized"},
			FetchedAt:       now,
		},
	}, cache.DefaultTTL)

	// 2. AWS Storage
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.023"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       100,
				StorageClass: "standard",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// 3. AWS Network
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "network", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "network",
			SkuID:           "data-transfer-out",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.090"),
			PriceCurrency:   "USD",
			Unit:            "GB",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     50,
				TransferType: "internet_egress",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// 4. AWS Database RDBMS (Instance + Storage)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_rdbms", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-4VCORE",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.260"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-STORAGE-GP3",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.115"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// 5. AWS Database NoSQL (Read + Write + Storage)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "database_nosql", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-READ",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.00013"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-WRITE",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.00065"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				WriteUnits:    1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-STORAGE",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.25"),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// 6. AWS Kubernetes
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-AWS-EKS-STANDARD",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.100"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: domain.KubernetesTierStandard,
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	// 7. AWS Serverless
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "serverless", "us-east-1"), []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-LAMBDA-REQ-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.20"),
			PriceCurrency:   "USD",
			Unit:            "per_million_requests",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  "x86_64",
				Tier:          "consumption",
				ComponentType: "request_fee",
				Unit:          "per_million_requests",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-LAMBDA-DUR-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000166667"),
			PriceCurrency:   "USD",
			Unit:            "per_gb_second",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  "x86_64",
				Tier:          "consumption",
				ComponentType: "duration_fee",
				Unit:          "per_gb_second",
			},
			FetchedAt: now,
		},
	}, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Compute: &domain.ComputeAttributes{
			VCPU:   4,
			RAMGB:  8,
			Family: "compute_optimized",
		},
		Storage: &domain.StorageAttributes{
			SizeGB:       100,
			StorageClass: "standard",
		},
		Network: &domain.NetworkAttributes{
			EgressGB:     50,
			TransferType: "internet_egress",
		},
		DatabaseRDBMS: &domain.DatabaseRDBMSAttributes{
			Engine:    "postgresql",
			VCPU:      4,
			RAMGB:     16,
			StorageGB: 100,
		},
		DatabaseNoSQL: &domain.DatabaseNoSQLAttributes{
			DataModel:   "document",
			PricingMode: "provisioned",
			ReadUnits:   100,
			WriteUnits:  20,
			StorageGB:   50,
		},
		Kubernetes: &domain.KubernetesAttributes{
			Tier: domain.KubernetesTierStandard,
		},
		Serverless: &service.ServerlessWorkload{
			Architecture:        "x86_64",
			Tier:                "consumption",
			RequestsPerMonth:    decimal.RequireFromString("10000000"),
			MemoryMB:            decimal.RequireFromString("512"),
			ExecutionDurationMS: decimal.RequireFromString("200"),
		},
	}

	res, err := svc.Calculate(ctx, req)
	if err != nil {
		t.Fatalf("Calculate unexpected error: %v", err)
	}

	var awsRes *service.CalculateProviderResult
	for i := range res.Results {
		if res.Results[i].Provider == "aws" {
			awsRes = &res.Results[i]
			break
		}
	}

	if awsRes == nil {
		t.Fatalf("AWS result not found in results: %+v", res.Results)
	}

	if awsRes.Partial {
		t.Errorf("AWS: Partial = true, want false for all 7 categories matched")
	}
	if awsRes.TotalNormalizedHourlyUSD == nil {
		t.Fatalf("AWS: TotalNormalizedHourlyUSD is nil, want populated total")
	}
	if awsRes.PartialTotalNormalizedHourlyUSD != nil {
		t.Errorf("AWS: PartialTotalNormalizedHourlyUSD = %v, want nil for complete match", awsRes.PartialTotalNormalizedHourlyUSD)
	}
	if len(awsRes.Categories) != 7 {
		t.Errorf("AWS: got %d categories, want 7", len(awsRes.Categories))
	}
	for _, expectedCat := range []string{"compute", "storage", "network", "database_rdbms", "database_nosql", "kubernetes", "serverless"} {
		if _, ok := awsRes.Categories[expectedCat]; !ok {
			t.Errorf("AWS: missing category %q in result map", expectedCat)
		}
	}
}

func TestPricingService_Calculate_ArchitectureUnsupportedWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"

	// GCP only supports x86_64 for Cloud Functions
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "serverless", "us-east4"), []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CF-REQ",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.40"),
			PriceCurrency:   "USD",
			Unit:            "per_million_requests",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  "x86_64",
				Tier:          "consumption",
				ComponentType: "request_fee",
			},
			FetchedAt: time.Now().UTC(),
		},
	}, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	req := service.CalculateRequest{
		Region:   regionGroup,
		Currency: "USD",
		Serverless: &service.ServerlessWorkload{
			Architecture:        "arm64", // arm64 is unsupported on GCP FaaS
			Tier:                "consumption",
			RequestsPerMonth:    decimal.RequireFromString("1000000"),
			MemoryMB:            decimal.RequireFromString("512"),
			ExecutionDurationMS: decimal.RequireFromString("200"),
		},
	}

	res, err := svc.Calculate(ctx, req)
	if err != nil {
		t.Fatalf("Calculate unexpected error: %v", err)
	}

	var gcpArchWarnFound bool
	for _, w := range res.Warnings {
		if w.Provider == "gcp" && w.Code == "architecture_unsupported_excluded" {
			gcpArchWarnFound = true
			break
		}
	}
	if !gcpArchWarnFound {
		t.Errorf("expected warning 'architecture_unsupported_excluded' for GCP, got: %+v", res.Warnings)
	}
}

func TestResolveAliasedField(t *testing.T) {
	type spec struct {
		Name  string
		Value int
	}

	t.Run("BothNil_ReturnsNil", func(t *testing.T) {
		res, err := service.ResolveAliasedField[spec](nil, nil, "alias", "canonical")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != nil {
			t.Errorf("expected nil, got %+v", res)
		}
	})

	t.Run("CanonicalOnly_ReturnsCanonical", func(t *testing.T) {
		canon := &spec{Name: "postgres", Value: 10}
		res, err := service.ResolveAliasedField[spec](nil, canon, "alias", "canonical")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != canon {
			t.Errorf("expected canon %+v, got %+v", canon, res)
		}
	})

	t.Run("AliasOnly_ReturnsAlias", func(t *testing.T) {
		alias := &spec{Name: "postgres", Value: 10}
		res, err := service.ResolveAliasedField[spec](alias, nil, "alias", "canonical")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != alias {
			t.Errorf("expected alias %+v, got %+v", alias, res)
		}
	})

	t.Run("BothIdentical_ReturnsCanonical", func(t *testing.T) {
		alias := &spec{Name: "postgres", Value: 10}
		canon := &spec{Name: "postgres", Value: 10}
		res, err := service.ResolveAliasedField[spec](alias, canon, "alias", "canonical")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != canon {
			t.Errorf("expected canon %+v, got %+v", canon, res)
		}
	})

	t.Run("BothDiffering_ReturnsConflictingFieldsError", func(t *testing.T) {
		alias := &spec{Name: "mysql", Value: 5}
		canon := &spec{Name: "postgres", Value: 10}
		res, err := service.ResolveAliasedField[spec](alias, canon, "database", "database_rdbms")
		if err == nil {
			t.Fatalf("expected error for conflicting fields, got result %+v", res)
		}
		if !errors.Is(err, service.ErrConflictingFields) {
			t.Errorf("expected error wrapping ErrConflictingFields, got: %v", err)
		}
		expectedMsg := "conflicting category fields: both 'database' and 'database_rdbms' were provided with conflicting values; provide only one when values differ"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})
}
