package service_test

import (
	"testing"

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
