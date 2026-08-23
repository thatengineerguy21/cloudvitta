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

func TestServerlessScorer_ExactMatch(t *testing.T) {
	scorer := service.ServerlessScorer{}

	cand := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "serverless",
		SkuID:           "SKU-AWS-LAMBDA-X86",
		PriceAmount:     decimal.RequireFromString("0.10"),
		ServerlessRateAttributes: domain.ServerlessRateAttributes{
			Architecture:  domain.ArchitectureX86_64,
			Tier:          domain.ServerlessTierConsumption,
			ComponentType: "composite",
		},
	}

	target := service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture: domain.ArchitectureX86_64,
			Tier:         domain.ServerlessTierConsumption,
		},
	}

	dist, missing, eligible := scorer.Score(cand, target)
	if !eligible {
		t.Fatalf("expected eligible = true")
	}
	if dist != 0.0 {
		t.Errorf("expected dist = 0.0, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected 0 missing attrs, got %v", missing)
	}
}

func TestServerlessScorer_TierMismatchPenalty(t *testing.T) {
	scorer := service.ServerlessScorer{}

	cand := domain.PriceObservation{
		Provider:        "azure",
		ServiceCategory: "serverless",
		SkuID:           "SKU-AZURE-FLEX",
		PriceAmount:     decimal.RequireFromString("0.20"),
		ServerlessRateAttributes: domain.ServerlessRateAttributes{
			Architecture:  domain.ArchitectureX86_64,
			Tier:          domain.ServerlessTierFlexConsumption,
			ComponentType: "composite",
		},
	}

	target := service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture: domain.ArchitectureX86_64,
			Tier:         domain.ServerlessTierConsumption,
		},
	}

	dist, _, eligible := scorer.Score(cand, target)
	if !eligible {
		t.Fatalf("expected eligible = true")
	}
	if dist != service.PenaltyServerlessTier {
		t.Errorf("expected dist = %f, got %f", service.PenaltyServerlessTier, dist)
	}
}

func TestCalculateServerlessCost_FreeTierNetting(t *testing.T) {
	// 1,000,000 requests, 512 MB, 200 ms:
	// Total GB-s = 1M * (512/1024) * (200/1000) = 100,000 GB-s.
	// Within AWS/Azure monthly free tier (1M req, 400k GB-s) -> cost should be $0.00.
	reqPrice := decimal.RequireFromString("0.20")         // $0.20 per 1M requests
	durPrice := decimal.RequireFromString("0.0000166667") // $0.0000166667 per GB-s

	workload := service.ServerlessWorkload{
		Architecture:        domain.ArchitectureX86_64,
		Tier:                domain.ServerlessTierConsumption,
		RequestsPerMonth:    decimal.RequireFromString("1000000"),
		MemoryMB:            decimal.RequireFromString("512"),
		ExecutionDurationMS: decimal.RequireFromString("200"),
	}

	monthlyCost, hourlyCost := service.CalculateServerlessCost("aws", domain.ServerlessTierConsumption, reqPrice, domain.UnitPerMillionRequests, durPrice, workload)

	if !monthlyCost.IsZero() {
		t.Errorf("expected monthly cost $0.00 within free tier, got %s", monthlyCost)
	}
	if !hourlyCost.IsZero() {
		t.Errorf("expected hourly cost $0.00 within free tier, got %s", hourlyCost)
	}
}

func TestCalculateServerlessCost_BillableWorkload_PrecisionDriftFree(t *testing.T) {
	// 10,000,000 requests, 1024 MB (1 GB), 500 ms (0.5 s):
	// Monthly GB-s = 10,000,000 * 1.0 * 0.5 = 5,000,000 GB-s.
	// Billable requests = 10M - 1M = 9M.
	// Request cost = (9M / 1M) * $0.20 = $1.80.
	// Billable GB-s = 5,000,000 - 400,000 = 4,600,000 GB-s.
	// Duration cost = 4,600,000 * 0.0000166667 = $76.66682.
	// Monthly cost = 1.80 + 76.66682 = $78.46682.
	// Hourly cost = $78.46682 / 730 = $0.10748879...
	reqPrice := decimal.RequireFromString("0.20")
	durPrice := decimal.RequireFromString("0.0000166667")

	workload := service.ServerlessWorkload{
		Architecture:        domain.ArchitectureX86_64,
		Tier:                domain.ServerlessTierConsumption,
		RequestsPerMonth:    decimal.RequireFromString("10000000"),
		MemoryMB:            decimal.RequireFromString("1024"),
		ExecutionDurationMS: decimal.RequireFromString("500"),
	}

	monthlyCost, hourlyCost := service.CalculateServerlessCost("aws", domain.ServerlessTierConsumption, reqPrice, domain.UnitPerMillionRequests, durPrice, workload)

	expectedMonthly := decimal.RequireFromString("78.46682")
	if !monthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected monthly cost %s, got %s", expectedMonthly, monthlyCost)
	}
	expectedHourly := expectedMonthly.Div(service.HoursInMonth)
	if !hourlyCost.Equal(expectedHourly) {
		t.Errorf("expected hourly cost %s, got %s", expectedHourly, hourlyCost)
	}
}

func TestCalculateServerlessCost_GCPDurationSplit_1stGen(t *testing.T) {
	// GCP 1st Gen:
	// 5,000,000 requests, 1024 MB (1 GB), 1000 ms (1 s):
	// Total GB-s = 5M * (1024/1024) * 1.0 = 5,000,000 GB-s.
	// 1024 MB maps to 1.400 GHz in 1st Gen table.
	// Total GHz-s = 5M * 1.400 * 1.0 = 7,000,000 GHz-s.
	//
	// Free Tier allowances:
	// Invocations: 2,000,000 calls. Billable = 5M - 2M = 3M.
	// Memory: 400,000 GB-s. Billable = 5M - 400k = 4,600,000 GB-s.
	// CPU: 200,000 GHz-s. Billable = 7M - 200k = 6,800,000 GHz-s.
	//
	// Rates:
	// Invocations: $0.0000004 per call (per_request). Cost = 3,000,000 * 0.0000004 = $1.20.
	// Memory: $0.0000025 per GB-s. Cost = 4,600,000 * 0.0000025 = $11.50.
	// CPU: $0.0000100 per GHz-s. Cost = 6,800,000 * 0.0000100 = $68.00.
	// Total Monthly = 1.20 + 11.50 + 68.00 = $80.70.
	components := service.ServerlessRateComponents{
		Provider:    "gcp",
		Tier:        domain.ServerlessTier1stGen,
		ReqPrice:    decimal.RequireFromString("0.0000004"),
		ReqUnit:     domain.UnitPerRequest,
		DurMemPrice: decimal.RequireFromString("0.0000025"),
		DurMemUnit:  domain.UnitPerGBSecond,
		DurCPUPrice: decimal.RequireFromString("0.0000100"),
		DurCPUUnit:  domain.UnitPerGHzSecond,
	}

	workload := service.ServerlessWorkload{
		Architecture:        domain.ArchitectureX86_64,
		Tier:                domain.ServerlessTier1stGen,
		RequestsPerMonth:    decimal.RequireFromString("5000000"),
		MemoryMB:            decimal.RequireFromString("1024"),
		ExecutionDurationMS: decimal.RequireFromString("1000"),
	}

	monthlyCost, hourlyCost := components.CalculateCost(workload)
	expectedMonthly := decimal.RequireFromString("80.70")
	if !monthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected GCP 1st Gen monthly cost %s, got %s", expectedMonthly, monthlyCost)
	}

	expectedHourly := expectedMonthly.Div(service.HoursInMonth)
	if !hourlyCost.Equal(expectedHourly) {
		t.Errorf("expected hourly cost %s, got %s", expectedHourly, hourlyCost)
	}
}

func TestCalculateServerlessCost_GCPDurationSplit_2ndGen(t *testing.T) {
	// GCP 2nd Gen:
	// 5,000,000 requests, 1769 MB (1 vCPU), 1000 ms (1 s):
	// Memory (GiB) = 1769 / 1024 GiB.
	// Total GiB-s = 5,000,000 * (1769/1024) * 1.0 = 8637695.3125 GiB-s.
	// vCPU = 1769 / 1769 = 1.0 vCPU.
	// Total vCPU-s = 5,000,000 * 1.0 * 1.0 = 5,000,000 vCPU-s.
	//
	// Free Tier allowances:
	// Invocations: 2,000,000 calls. Billable = 3M.
	// Memory: 360,000 GiB-s. Billable = 8637695.3125 - 360,000 = 8277695.3125 GiB-s.
	// CPU: 180,000 vCPU-s. Billable = 5,000,000 - 180,000 = 4,820,000 vCPU-s.
	components := service.ServerlessRateComponents{
		Provider:    "gcp",
		Tier:        domain.ServerlessTier2ndGen,
		ReqPrice:    decimal.RequireFromString("0.0000004"),
		ReqUnit:     domain.UnitPerRequest,
		DurMemPrice: decimal.RequireFromString("0.0000025"),
		DurMemUnit:  domain.UnitPerGiBSecond,
		DurCPUPrice: decimal.RequireFromString("0.0000240"),
		DurCPUUnit:  domain.UnitPerVCPUSecond,
	}

	workload := service.ServerlessWorkload{
		Architecture:        domain.ArchitectureX86_64,
		Tier:                domain.ServerlessTier2ndGen,
		RequestsPerMonth:    decimal.RequireFromString("5000000"),
		MemoryMB:            decimal.RequireFromString("1769"),
		ExecutionDurationMS: decimal.RequireFromString("1000"),
	}

	monthlyCost, _ := components.CalculateCost(workload)
	if monthlyCost.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("expected positive monthly cost for GCP 2nd Gen, got %s", monthlyCost)
	}
}

func TestCalculateServerlessCost_CuratedUnitRoutingWithoutMagnitudeHeuristic(t *testing.T) {
	// Construct a case where an invocation rate is priced unusually low (e.g. $0.005 per 1M requests),
	// which is less than the old magnitude threshold of 0.01.
	// If unit is domain.UnitPerMillionRequests, it MUST divide by 1M, not treat as per-single-request.
	workload := service.ServerlessWorkload{
		Architecture:        domain.ArchitectureX86_64,
		Tier:                domain.ServerlessTierConsumption,
		RequestsPerMonth:    decimal.RequireFromString("2000000"), // 1M billable after free tier
		MemoryMB:            decimal.RequireFromString("512"),
		ExecutionDurationMS: decimal.RequireFromString("200"),
	}

	components := service.ServerlessRateComponents{
		Provider: "aws",
		Tier:     domain.ServerlessTierConsumption,
		ReqPrice: decimal.RequireFromString("0.005"), // $0.005 per 1M requests (< 0.01 threshold)
		ReqUnit:  domain.UnitPerMillionRequests,
		DurPrice: decimal.Zero,
	}

	monthlyCost, _ := components.CalculateCost(workload)
	expectedCost := decimal.RequireFromString("0.005") // 1M billable / 1M * 0.005 = 0.005
	if !monthlyCost.Equal(expectedCost) {
		t.Errorf("expected unit-driven monthly cost %s, got %s (heuristic magnitude failed)", expectedCost, monthlyCost)
	}
}

func TestMatchServerlessObservations_ArchitectureExclusion(t *testing.T) {
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-REQ",
			PriceAmount:     decimal.RequireFromString("0.000002"),
			Unit:            "10",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPer10Requests,
			},
		},
	}

	// 1. Querying x86_64 on Azure succeeds
	resX86, err := service.MatchServerlessObservations(azureObs, service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture: domain.ArchitectureX86_64,
			Tier:         domain.ServerlessTierConsumption,
		},
	}, service.ServerlessThresholds)
	if err != nil {
		t.Fatalf("unexpected error for x86_64 query: %v", err)
	}
	if resX86 == nil {
		t.Fatalf("expected non-nil match for x86_64")
	}

	// 2. Querying arm64 on Azure returns ErrArchitectureUnsupported
	resARM, err := service.MatchServerlessObservations(azureObs, service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture: domain.ArchitectureARM64,
			Tier:         domain.ServerlessTierConsumption,
		},
	}, service.ServerlessThresholds)
	if err != service.ErrArchitectureUnsupported {
		t.Errorf("expected ErrArchitectureUnsupported for arm64 query against Azure, got err=%v res=%v", err, resARM)
	}
}

func TestPricingService_ServerlessCompare_CrossCloud(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	regionGroup := "us-east"
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.20"),
			Unit:            "Requests",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000166667"),
			Unit:            "Seconds",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-ARM",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.20"),
			Unit:            "Requests",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-ARM",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000133334"),
			Unit:            "Seconds",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "serverless", "us-east-1"), awsObs, cache.DefaultTTL)

	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-REQ",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.000002"),
			Unit:            "10",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPer10Requests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-DUR",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.000016"),
			Unit:            "1 GB Second",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "serverless", "eastus"), azureObs, cache.DefaultTTL)

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-REQ",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000004"),
			Unit:            "Calls",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerRequest,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CPU",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000100"),
			Unit:            "s",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeCPU,
				Unit:          domain.UnitPerGHzSecond,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-MEM",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000025"),
			Unit:            "GiBy.s",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeMemory,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "serverless", "us-east4"), gcpObs, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	// 1. Compare x86_64 workload across all 3 providers
	compX86, err := svc.Compare(ctx, "serverless", regionGroup, service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture:        domain.ArchitectureX86_64,
			Tier:                domain.ServerlessTierConsumption,
			RequestsPerMonth:    decimal.RequireFromString("5000000"),
			MemoryMB:            decimal.RequireFromString("512"),
			ExecutionDurationMS: decimal.RequireFromString("200"),
		},
	})
	if err != nil {
		t.Fatalf("Compare x86_64 failed: %v", err)
	}
	if len(compX86.Results) != 3 {
		t.Fatalf("expected 3 provider results for x86_64, got %d", len(compX86.Results))
	}

	// 2. Compare arm64 workload -> AWS matches, Azure & GCP excluded with architecture_unsupported_excluded warning
	compARM, err := svc.Compare(ctx, "serverless", regionGroup, service.MatchTarget{
		Category: "serverless",
		ServerlessWorkload: service.ServerlessWorkload{
			Architecture:        domain.ArchitectureARM64,
			Tier:                domain.ServerlessTierConsumption,
			RequestsPerMonth:    decimal.RequireFromString("5000000"),
			MemoryMB:            decimal.RequireFromString("512"),
			ExecutionDurationMS: decimal.RequireFromString("200"),
		},
	})
	if err != nil {
		t.Fatalf("Compare arm64 failed: %v", err)
	}
	if len(compARM.Results) != 1 {
		t.Fatalf("expected 1 result (AWS) for arm64, got %d", len(compARM.Results))
	}
	if compARM.Results[0].Provider != "aws" {
		t.Errorf("expected AWS result for arm64, got %s", compARM.Results[0].Provider)
	}

	var azureExcluded, gcpExcluded bool
	for _, w := range compARM.Warnings {
		if w.Code == "architecture_unsupported_excluded" {
			if w.Provider == "azure" {
				azureExcluded = true
			}
			if w.Provider == "gcp" {
				gcpExcluded = true
			}
		}
	}
	if !azureExcluded {
		t.Errorf("expected architecture_unsupported_excluded warning for Azure")
	}
	if !gcpExcluded {
		t.Errorf("expected architecture_unsupported_excluded warning for GCP")
	}
}
