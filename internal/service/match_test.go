package service_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

func TestScoreComputeObservations(t *testing.T) {
	obsList := []domain.PriceObservation{
		{
			SkuID:      "exact-match",
			Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 16},
		},
		{
			SkuID:      "close-match",
			Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 20}, // 25% delta on RAM
		},
		{
			SkuID:      "far-match",
			Attributes: domain.ComputeAttributes{VCPU: 8, RAMGB: 32}, // 100% delta on both
		},
	}

	t.Run("Score non-strict", func(t *testing.T) {
		results := service.ScoreComputeObservations(4, 16, false, obsList)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		if results[0].Observation.SkuID != "exact-match" || results[0].MatchQuality != "exact" {
			t.Errorf("expected exact-match with quality exact, got %s, %s", results[0].Observation.SkuID, results[0].MatchQuality)
		}

		if results[1].Observation.SkuID != "close-match" || results[1].MatchQuality != "close" {
			t.Errorf("expected close-match with quality close, got %s, %s", results[1].Observation.SkuID, results[1].MatchQuality)
		}
	})

	t.Run("Score strict_family true", func(t *testing.T) {
		results := service.ScoreComputeObservations(4, 16, true, obsList)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		if results[0].Observation.SkuID != "exact-match" || results[0].MatchQuality != "exact" {
			t.Errorf("expected exact-match with quality exact, got %s, %s", results[0].Observation.SkuID, results[0].MatchQuality)
		}
	})

	t.Run("No parameters requested", func(t *testing.T) {
		results := service.ScoreComputeObservations(0, 0, false, obsList)
		if len(results) != 3 {
			t.Fatalf("expected 3 results when no params requested, got %d", len(results))
		}
		for _, r := range results {
			if r.MatchQuality != "exact" {
				t.Errorf("expected all qualities to be exact when no params requested, got %s for %s", r.MatchQuality, r.Observation.SkuID)
			}
		}
	})
}

func TestScoreStorageObservations(t *testing.T) {
	obsList := []domain.PriceObservation{
		{
			SkuID:             "s3-standard",
			PriceAmount:       decimal.RequireFromString("0.023"),
			StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: "standard"},
		},
		{
			SkuID:             "s3-ia",
			PriceAmount:       decimal.RequireFromString("0.0125"),
			StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: "infrequent_access"},
		},
	}

	t.Run("Score storage exact class", func(t *testing.T) {
		results := service.ScoreStorageObservations(500, "standard", obsList)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		if results[0].Observation.SkuID != "s3-standard" || results[0].MatchQuality != "exact" {
			t.Errorf("expected s3-standard with exact, got %s, %s", results[0].Observation.SkuID, results[0].MatchQuality)
		}
		expectedMonthly := decimal.RequireFromString("0.023").Mul(decimal.NewFromFloat(500))
		if !results[0].MonthlyCost.Equal(expectedMonthly) {
			t.Errorf("expected monthly cost %s, got %s", expectedMonthly, results[0].MonthlyCost)
		}

		if results[1].Observation.SkuID != "s3-ia" || results[1].MatchQuality != "close" {
			t.Errorf("expected s3-ia with close, got %s, %s", results[1].Observation.SkuID, results[1].MatchQuality)
		}
	})

	t.Run("Score storage default class", func(t *testing.T) {
		results := service.ScoreStorageObservations(100, "", obsList)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		for _, r := range results {
			if r.MatchQuality != "exact" {
				t.Errorf("expected exact when no class requested, got %s", r.MatchQuality)
			}
		}
	})
}
