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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

func TestKubernetesScorer_ExactMatch(t *testing.T) {
	scorer := service.KubernetesScorer{}

	cand := domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: "kubernetes",
		SkuID:           "SKU-AWS-EKS-STD",
		PriceAmount:     decimal.RequireFromString("0.10"),
		KubernetesAttributes: domain.KubernetesAttributes{
			Tier: kubernetestieremap.TierStandard,
		},
	}

	target := service.MatchTarget{
		Category:       "kubernetes",
		KubernetesTier: kubernetestieremap.TierStandard,
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

func TestKubernetesScorer_TierMismatchPenalty(t *testing.T) {
	scorer := service.KubernetesScorer{}

	cand := domain.PriceObservation{
		Provider:        "azure",
		ServiceCategory: "kubernetes",
		SkuID:           "SKU-AZURE-AKS-FREE",
		PriceAmount:     decimal.Zero,
		KubernetesAttributes: domain.KubernetesAttributes{
			Tier: kubernetestieremap.TierFree,
		},
	}

	target := service.MatchTarget{
		Category:       "kubernetes",
		KubernetesTier: kubernetestieremap.TierStandard,
	}

	dist, _, eligible := scorer.Score(cand, target)
	if !eligible {
		t.Fatalf("expected eligible = true")
	}
	if dist != service.PenaltyKubernetesTier {
		t.Errorf("expected dist = %f, got %f", service.PenaltyKubernetesTier, dist)
	}
}

func TestMatchKubernetesObservations_TieBreak(t *testing.T) {
	obsList := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-Z-EKS",
			PriceAmount:     decimal.RequireFromString("0.10"),
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: kubernetestieremap.TierStandard,
			},
		},
		{
			Provider:        "aws",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-A-EKS",
			PriceAmount:     decimal.RequireFromString("0.10"),
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: kubernetestieremap.TierStandard,
			},
		},
	}

	target := service.MatchTarget{
		Category:       "kubernetes",
		KubernetesTier: kubernetestieremap.TierStandard,
	}

	res, err := service.MatchKubernetesObservations(obsList, target, service.KubernetesThresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil match result")
	}
	if res.Observation.SkuID != "SKU-A-EKS" {
		t.Errorf("expected tie-break to choose SKU-A-EKS, got %s", res.Observation.SkuID)
	}
	if res.MatchQuality != "exact" {
		t.Errorf("expected exact match quality, got %s", res.MatchQuality)
	}
}

func TestPricingService_KubernetesConditionalGKECredit(t *testing.T) {
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

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-GCP-GKE-CLUSTER",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.10"), // $0.10/hour base
			Unit:            "hour",
			PriceCurrency:   "USD",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: kubernetestieremap.TierStandard,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "kubernetes", "us-east4"), gcpObs, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	// 1. Zonal cluster topology -> GKE credit applied -> hourlyCost = max(0, 0.10 - (74.40 / 730)) = $0.00
	resZonal, err := svc.MatchAndCalculate(ctx, "gcp", "kubernetes", regionGroup, service.MatchTarget{
		Category:        "kubernetes",
		KubernetesTier:  kubernetestieremap.TierStandard,
		ClusterTopology: domain.ClusterTopologyZonal,
	})
	if err != nil {
		t.Fatalf("MatchAndCalculate (zonal) failed: %v", err)
	}
	if !resZonal.HourlyCost.IsZero() {
		t.Errorf("expected zonal hourly cost 0.00 (credit applied), got %s", resZonal.HourlyCost)
	}
	if !resZonal.MonthlyCost.IsZero() {
		t.Errorf("expected zonal monthly cost 0.00, got %s", resZonal.MonthlyCost)
	}
	if len(resZonal.Warnings) != 0 {
		t.Errorf("expected 0 warnings for zonal topology, got %v", resZonal.Warnings)
	}

	// 2. Autopilot cluster topology -> GKE credit applied -> hourlyCost = $0.00
	resAutopilot, err := svc.MatchAndCalculate(ctx, "gcp", "kubernetes", regionGroup, service.MatchTarget{
		Category:        "kubernetes",
		KubernetesTier:  kubernetestieremap.TierStandard,
		ClusterTopology: domain.ClusterTopologyAutopilot,
	})
	if err != nil {
		t.Fatalf("MatchAndCalculate (autopilot) failed: %v", err)
	}
	if !resAutopilot.HourlyCost.IsZero() {
		t.Errorf("expected autopilot hourly cost 0.00 (credit applied), got %s", resAutopilot.HourlyCost)
	}
	if len(resAutopilot.Warnings) != 0 {
		t.Errorf("expected 0 warnings for autopilot topology, got %v", resAutopilot.Warnings)
	}

	// 3. Regional cluster topology -> GKE credit NOT applied -> hourlyCost = $0.10
	resRegional, err := svc.MatchAndCalculate(ctx, "gcp", "kubernetes", regionGroup, service.MatchTarget{
		Category:        "kubernetes",
		KubernetesTier:  kubernetestieremap.TierStandard,
		ClusterTopology: domain.ClusterTopologyRegional,
	})
	if err != nil {
		t.Fatalf("MatchAndCalculate (regional) failed: %v", err)
	}
	expectedHourly := decimal.RequireFromString("0.10")
	if !resRegional.HourlyCost.Equal(expectedHourly) {
		t.Errorf("expected regional hourly cost 0.10, got %s", resRegional.HourlyCost)
	}
	expectedMonthly := expectedHourly.Mul(service.HoursInMonth)
	if !resRegional.MonthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected regional monthly cost %s, got %s", expectedMonthly, resRegional.MonthlyCost)
	}
	if len(resRegional.Warnings) != 0 {
		t.Errorf("expected 0 warnings for regional topology, got %v", resRegional.Warnings)
	}

	// 4. Unspecified cluster topology ("") -> GKE credit NOT applied -> hourlyCost = $0.10, warning cluster_topology_unspecified present
	resUnspecified, err := svc.MatchAndCalculate(ctx, "gcp", "kubernetes", regionGroup, service.MatchTarget{
		Category:        "kubernetes",
		KubernetesTier:  kubernetestieremap.TierStandard,
		ClusterTopology: "",
	})
	if err != nil {
		t.Fatalf("MatchAndCalculate (unspecified) failed: %v", err)
	}
	if !resUnspecified.HourlyCost.Equal(expectedHourly) {
		t.Errorf("expected unspecified topology hourly cost 0.10 (no credit), got %s", resUnspecified.HourlyCost)
	}
	if !resUnspecified.MonthlyCost.Equal(expectedMonthly) {
		t.Errorf("expected unspecified topology monthly cost %s, got %s", expectedMonthly, resUnspecified.MonthlyCost)
	}
	foundWarning := false
	for _, w := range resUnspecified.Warnings {
		if w.Code == "cluster_topology_unspecified" && w.Provider == "gcp" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected cluster_topology_unspecified warning for GCP query without topology, got %v", resUnspecified.Warnings)
	}
}

func TestPricingService_KubernetesTierMismatchScoring(t *testing.T) {
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

	// Azure has only free tier available in this test scenario
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-AZURE-AKS-FREE",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.Zero,
			Unit:            "Hrs",
			PriceCurrency:   "USD",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: kubernetestieremap.TierFree,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "kubernetes", "eastus"), azureObs, cache.DefaultTTL)

	svc := service.NewPricingService(nil, rdb)

	// Query for standard tier -> should match Free tier with "close" quality and 20% delta
	res, err := svc.MatchAndCalculate(ctx, "azure", "kubernetes", regionGroup, service.MatchTarget{
		Category:       "kubernetes",
		KubernetesTier: kubernetestieremap.TierStandard,
	})
	if err != nil {
		t.Fatalf("MatchAndCalculate failed: %v", err)
	}
	if res.MatchResult.MatchQuality != "close" {
		t.Errorf("expected match_quality 'close', got %s", res.MatchResult.MatchQuality)
	}
	if res.MatchResult.MatchDeltaPct != 20.0 {
		t.Errorf("expected match_delta_pct 20.0, got %f", res.MatchResult.MatchDeltaPct)
	}
}
