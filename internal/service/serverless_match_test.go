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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
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
			Architecture:  serverlessarchmap.ArchX86_64,
			Tier:          domain.ServerlessTierConsumption,
			ComponentType: "composite",
		},
	}

	target := service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchX86_64,
		ServerlessTier:         domain.ServerlessTierConsumption,
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
			Architecture:  serverlessarchmap.ArchX86_64,
			Tier:          domain.ServerlessTierFlexConsumption,
			ComponentType: "composite",
		},
	}

	target := service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchX86_64,
		ServerlessTier:         domain.ServerlessTierConsumption,
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

	monthlyCost, hourlyCost := service.CalculateServerlessCost("aws", domain.ServerlessTierConsumption, reqPrice, "Requests", durPrice, 1_000_000, 512, 200)

	if !monthlyCost.IsZero() {
		t.Errorf("expected monthly cost $0.00 within free tier, got %s", monthlyCost)
	}
	if !hourlyCost.IsZero() {
		t.Errorf("expected hourly cost $0.00 within free tier, got %s", hourlyCost)
	}
}

func TestCalculateServerlessCost_BillableWorkload(t *testing.T) {
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

	monthlyCost, hourlyCost := service.CalculateServerlessCost("aws", domain.ServerlessTierConsumption, reqPrice, "Requests", durPrice, 10_000_000, 1024, 500)

	expectedMonthly := decimal.RequireFromString("78.46682")
	if !monthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected monthly cost %s, got %s", expectedMonthly, monthlyCost)
	}
	expectedHourly := expectedMonthly.Div(service.HoursInMonth)
	if !hourlyCost.Equal(expectedHourly) {
		t.Errorf("expected hourly cost %s, got %s", expectedHourly, hourlyCost)
	}
}

func TestMatchServerlessObservations_ArchitectureExclusion(t *testing.T) {
	// Azure obsList with only x86_64
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-REQ",
			PriceAmount:     decimal.RequireFromString("0.000002"),
			Unit:            "10",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
			},
		},
	}

	// 1. Querying x86_64 on Azure succeeds
	resX86, err := service.MatchServerlessObservations(azureObs, service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchX86_64,
		ServerlessTier:         domain.ServerlessTierConsumption,
	}, service.ServerlessThresholds)
	if err != nil {
		t.Fatalf("unexpected error for x86_64 query: %v", err)
	}
	if resX86 == nil {
		t.Fatalf("expected non-nil match for x86_64")
	}

	// 2. Querying arm64 on Azure returns ErrArchitectureUnsupported
	resARM, err := service.MatchServerlessObservations(azureObs, service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchARM64,
		ServerlessTier:         domain.ServerlessTierConsumption,
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
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
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
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
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
				Architecture:  serverlessarchmap.ArchARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
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
				Architecture:  serverlessarchmap.ArchARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
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
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
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
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
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
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-DUR",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000165"),
			Unit:            "s",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  serverlessarchmap.ArchX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "serverless", "us-east4"), gcpObs, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	// 1. Compare x86_64 workload across all 3 providers
	compX86, err := svc.Compare(ctx, "serverless", regionGroup, service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchX86_64,
		ServerlessTier:         domain.ServerlessTierConsumption,
		RequestsPerMonth:       5_000_000,
		MemoryMB:               512,
		ExecutionDurationMS:    200,
	})
	if err != nil {
		t.Fatalf("Compare x86_64 failed: %v", err)
	}
	if len(compX86.Results) != 3 {
		t.Fatalf("expected 3 provider results for x86_64, got %d", len(compX86.Results))
	}

	// 2. Compare arm64 workload -> AWS matches, Azure & GCP excluded with architecture_unsupported_excluded warning
	compARM, err := svc.Compare(ctx, "serverless", regionGroup, service.MatchTarget{
		Category:               "serverless",
		ServerlessArchitecture: serverlessarchmap.ArchARM64,
		ServerlessTier:         domain.ServerlessTierConsumption,
		RequestsPerMonth:       5_000_000,
		MemoryMB:               512,
		ExecutionDurationMS:    200,
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
