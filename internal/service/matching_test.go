package service_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// --- Compute scorer tests ---

func TestComputeScorer_ExactMatch(t *testing.T) {
	scorer := service.ComputeScorer{}
	target := service.MatchTarget{
		VCPU:     4,
		RAMGB:    16,
		Category: "compute",
	}

	obs := domain.PriceObservation{
		SkuID:      "exact-4-16",
		Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 16, Family: "general"},
	}

	dist, missing, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible")
	}
	if dist != 0 {
		t.Errorf("expected distance 0, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing attributes, got %v", missing)
	}
}

func TestComputeScorer_CloseMatch(t *testing.T) {
	scorer := service.ComputeScorer{}
	target := service.MatchTarget{
		VCPU:     4,
		RAMGB:    16,
		Category: "compute",
	}

	// 4 vCPU, 20 GB RAM: RAM delta = |20-16|/16 = 0.25
	obs := domain.PriceObservation{
		SkuID:      "close-4-20",
		Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 20, Family: "general"},
	}

	dist, _, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible")
	}
	// distance = 0 (vcpu) + 0.25 (ram) = 0.25
	if dist != 0.25 {
		t.Errorf("expected distance 0.25, got %f", dist)
	}
}

func TestComputeScorer_NoParamsRequestedAllExact(t *testing.T) {
	scorer := service.ComputeScorer{}
	target := service.MatchTarget{
		VCPU:     0,
		RAMGB:    0,
		Category: "compute",
	}

	obs := domain.PriceObservation{
		SkuID:      "any-sku",
		Attributes: domain.ComputeAttributes{VCPU: 8, RAMGB: 32, Family: "general"},
	}

	dist, _, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible when no params requested")
	}
	if dist != 0 {
		t.Errorf("expected distance 0 when no params, got %f", dist)
	}
}

func TestComputeScorer_CandidateLacksRequiredDimension(t *testing.T) {
	scorer := service.ComputeScorer{}
	target := service.MatchTarget{
		VCPU:     4,
		RAMGB:    16,
		Category: "compute",
	}

	obs := domain.PriceObservation{
		SkuID:      "no-ram",
		Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 0},
	}

	_, _, eligible := scorer.Score(obs, target)
	if eligible {
		t.Error("expected ineligible when candidate lacks required dimension")
	}
}

// --- Storage scorer tests ---

func TestStorageScorer_ExactMatch(t *testing.T) {
	scorer := service.StorageScorer{}
	target := service.MatchTarget{
		SizeGB:       100,
		StorageClass: "standard",
		Category:     "storage",
	}

	obs := domain.PriceObservation{
		SkuID:             "storage-exact",
		StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
	}

	dist, missing, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible")
	}
	if dist != 0 {
		t.Errorf("expected distance 0, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestStorageScorer_StorageClassOrdinalPenalty(t *testing.T) {
	scorer := service.StorageScorer{}

	tests := []struct {
		name        string
		targetClass string
		candClass   string
		wantPenalty float64 // only the class penalty term
		description string
	}{
		{
			name:        "same class (0 penalty)",
			targetClass: "standard",
			candClass:   "standard",
			wantPenalty: 0,
			description: "standard vs standard = 0 ordinal difference",
		},
		{
			name:        "one tier apart",
			targetClass: "standard",
			candClass:   "infrequent_access",
			wantPenalty: 0.5, // |0-1|/2 = 0.5
			description: "standard(0) vs infrequent_access(1) = 1/2",
		},
		{
			name:        "two tiers apart",
			targetClass: "standard",
			candClass:   "archive",
			wantPenalty: 1.0, // |0-2|/2 = 1.0
			description: "standard(0) vs archive(2) = 2/2",
		},
		{
			name:        "one tier apart reverse",
			targetClass: "archive",
			candClass:   "infrequent_access",
			wantPenalty: 0.5, // |2-1|/2 = 0.5
			description: "archive(2) vs infrequent_access(1) = 1/2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := service.MatchTarget{
				SizeGB:       100,
				StorageClass: tt.targetClass,
				Category:     "storage",
			}

			obs := domain.PriceObservation{
				SkuID: "test-sku",
				StorageAttributes: domain.StorageAttributes{
					SizeGB:       100, // same size, so size penalty = 0
					StorageClass: tt.candClass,
				},
			}

			dist, _, eligible := scorer.Score(obs, target)
			if !eligible {
				t.Fatal("expected eligible")
			}
			// With same size, distance = class penalty only
			if dist != tt.wantPenalty {
				t.Errorf("%s: expected penalty %f, got %f", tt.description, tt.wantPenalty, dist)
			}
		})
	}
}

func TestStorageScorer_TargetOmitsStorageClass(t *testing.T) {
	scorer := service.StorageScorer{}
	target := service.MatchTarget{
		SizeGB:       100,
		StorageClass: "", // omitted
		Category:     "storage",
	}

	obs := domain.PriceObservation{
		SkuID: "has-class",
		StorageAttributes: domain.StorageAttributes{
			SizeGB:       100,
			StorageClass: "archive",
		},
	}

	dist, missing, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible")
	}
	// Class term excluded entirely, size is exact match → distance = 0
	if dist != 0 {
		t.Errorf("expected distance 0 when target omits storage_class, got %f", dist)
	}
	// Should NOT record as missing — the caller didn't request this dimension
	if len(missing) != 0 {
		t.Errorf("expected no missing attributes when target omits class, got %v", missing)
	}
}

func TestStorageScorer_CandidateMissingStorageClass(t *testing.T) {
	scorer := service.StorageScorer{}
	target := service.MatchTarget{
		SizeGB:       100,
		StorageClass: "standard",
		Category:     "storage",
	}

	obs := domain.PriceObservation{
		SkuID: "no-class",
		StorageAttributes: domain.StorageAttributes{
			SizeGB:       100,
			StorageClass: "", // candidate lacks it
		},
	}

	dist, missing, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible — candidate passes required dimension guard on size_gb alone")
	}
	// Class term excluded from distance (not penalized), but recorded in missing_attributes
	if dist != 0 {
		t.Errorf("expected distance 0 (class excluded), got %f", dist)
	}
	if len(missing) != 1 || missing[0] != "storage_class" {
		t.Errorf("expected [storage_class] in missing, got %v", missing)
	}
}

// --- Network scorer tests ---

func TestNetworkScorer_ExactMatch(t *testing.T) {
	scorer := service.NetworkScorer{}
	target := service.MatchTarget{
		EgressGB:     500,
		TransferType: "internet_egress",
		Category:     "network",
	}

	obs := domain.PriceObservation{
		SkuID:             "net-exact",
		NetworkAttributes: domain.NetworkAttributes{EgressGB: 500, TransferType: "internet_egress"},
	}

	dist, missing, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible")
	}
	if dist != 0 {
		t.Errorf("expected distance 0, got %f", dist)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestNetworkScorer_TransferTypePenalty(t *testing.T) {
	scorer := service.NetworkScorer{}

	tests := []struct {
		name        string
		targetType  string
		candType    string
		wantPenalty float64
	}{
		{"same type", "intra_region", "intra_region", 0},
		{"one tier apart", "intra_region", "inter_region", 0.5},
		{"two tiers apart", "intra_region", "internet_egress", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := service.MatchTarget{
				EgressGB:     500,
				TransferType: tt.targetType,
				Category:     "network",
			}

			obs := domain.PriceObservation{
				SkuID: "test-net",
				NetworkAttributes: domain.NetworkAttributes{
					EgressGB:     500, // same egress
					TransferType: tt.candType,
				},
			}

			dist, _, eligible := scorer.Score(obs, target)
			if !eligible {
				t.Fatal("expected eligible")
			}
			if dist != tt.wantPenalty {
				t.Errorf("expected penalty %f, got %f", tt.wantPenalty, dist)
			}
		})
	}
}

func TestNetworkScorer_NoTransferTypeStillPassesGuard(t *testing.T) {
	scorer := service.NetworkScorer{}
	target := service.MatchTarget{
		EgressGB: 500,
		// TransferType omitted
		Category: "network",
	}

	obs := domain.PriceObservation{
		SkuID:             "net-no-type",
		NetworkAttributes: domain.NetworkAttributes{EgressGB: 500},
	}

	dist, _, eligible := scorer.Score(obs, target)
	if !eligible {
		t.Fatal("expected eligible — no transfer_type requested, egress_gb alone passes guard")
	}
	if dist != 0 {
		t.Errorf("expected distance 0, got %f", dist)
	}
}

// --- MatchObservations integration tests ---

func TestMatchObservations_PicksLowestDistance(t *testing.T) {
	obsList := []domain.PriceObservation{
		{SkuID: "far", Attributes: domain.ComputeAttributes{VCPU: 8, RAMGB: 32}},
		{SkuID: "close", Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 18}},
		{SkuID: "exact", Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 16}},
	}

	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	if result == nil {
		t.Fatal("expected a result")
	}
	if result.Observation.SkuID != "exact" {
		t.Errorf("expected exact, got %s", result.Observation.SkuID)
	}
	if result.MatchQuality != "exact" {
		t.Errorf("expected quality 'exact', got %s", result.MatchQuality)
	}
	if result.MatchDeltaPct != 0 {
		t.Errorf("expected delta 0, got %f", result.MatchDeltaPct)
	}
}

func TestMatchObservations_TieBreakOnSkuID(t *testing.T) {
	// Two candidates with identical distance. Tie-break by SKU ID lexicographically.
	obsList := []domain.PriceObservation{
		{
			SkuID:       "sku-z",
			Attributes:  domain.ComputeAttributes{VCPU: 4, RAMGB: 16},
			PriceAmount: decimal.NewFromFloat(0.50), // cheaper, but tie-break is NOT on price
		},
		{
			SkuID:       "sku-a",
			Attributes:  domain.ComputeAttributes{VCPU: 4, RAMGB: 16},
			PriceAmount: decimal.NewFromFloat(1.00), // more expensive
		},
	}

	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	if result == nil {
		t.Fatal("expected a result")
	}
	// sku-a < sku-z lexicographically, so sku-a wins despite being more expensive
	if result.Observation.SkuID != "sku-a" {
		t.Errorf("expected sku-a (lexicographic tie-break), got %s", result.Observation.SkuID)
	}
}

func TestMatchObservations_NoneThreshold(t *testing.T) {
	// All candidates are too far — should return nil
	obsList := []domain.PriceObservation{
		{SkuID: "very-far", Attributes: domain.ComputeAttributes{VCPU: 100, RAMGB: 200}},
	}

	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	if result != nil {
		t.Errorf("expected nil (none tier), got result with quality %s", result.MatchQuality)
	}
}

func TestMatchObservations_EmptyObsList(t *testing.T) {
	result := service.MatchObservations(service.ComputeScorer{}, nil, service.MatchTarget{}, service.ComputeThresholds)
	if result != nil {
		t.Error("expected nil for empty obs list")
	}
}

func TestMatchObservations_StrictFamilyFilters(t *testing.T) {
	obsList := []domain.PriceObservation{
		{SkuID: "general-4-16", Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 16, Family: "general"}},
		{SkuID: "memory-4-16", Attributes: domain.ComputeAttributes{VCPU: 4, RAMGB: 16, Family: "memory"}},
	}

	t.Run("strict_family=true filters to matching family", func(t *testing.T) {
		target := service.MatchTarget{
			VCPU:         4,
			RAMGB:        16,
			Family:       "general",
			StrictFamily: true,
			Category:     "compute",
		}
		result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)
		if result == nil {
			t.Fatal("expected result")
		}
		if result.Observation.SkuID != "general-4-16" {
			t.Errorf("expected general-4-16, got %s", result.Observation.SkuID)
		}
	})

	t.Run("strict_family=false allows all families", func(t *testing.T) {
		target := service.MatchTarget{
			VCPU:         4,
			RAMGB:        16,
			Family:       "general",
			StrictFamily: false,
			Category:     "compute",
		}
		result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)
		if result == nil {
			t.Fatal("expected result")
		}
		// Both are exact matches, tie-break on SKU ID: "general-4-16" < "memory-4-16"
		if result.Observation.SkuID != "general-4-16" {
			t.Errorf("expected general-4-16 (tie-break), got %s", result.Observation.SkuID)
		}
	})

	t.Run("strict_family=true with no matching family returns nil", func(t *testing.T) {
		target := service.MatchTarget{
			VCPU:         4,
			RAMGB:        16,
			Family:       "compute_optimized",
			StrictFamily: true,
			Category:     "compute",
		}
		result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)
		if result != nil {
			t.Errorf("expected nil when no matching family, got %s", result.Observation.SkuID)
		}
	})
}

func TestMatchObservations_ApproximateTier(t *testing.T) {
	// Candidate with distance between close and approximate cutoffs
	obsList := []domain.PriceObservation{
		{SkuID: "approx", Attributes: domain.ComputeAttributes{VCPU: 5.5, RAMGB: 16}},
	}

	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	if result == nil {
		t.Fatal("expected result for approximate match")
	}
	// distance = |5.5-4|/4 + 0 = 0.375 → within approximate cutoff (0.50) but above close (0.10)
	if result.MatchQuality != "approximate" {
		t.Errorf("expected 'approximate', got %s", result.MatchQuality)
	}
}

func TestMatchObservations_CloseTier(t *testing.T) {
	obsList := []domain.PriceObservation{
		{SkuID: "close", Attributes: domain.ComputeAttributes{VCPU: 4.2, RAMGB: 16}},
	}

	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	result := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	if result == nil {
		t.Fatal("expected result for close match")
	}
	// distance = |4.2-4|/4 = 0.05 → within close cutoff (0.10)
	if result.MatchQuality != "close" {
		t.Errorf("expected 'close', got %s", result.MatchQuality)
	}
}

// --- Category-specific threshold tests ---

func TestMatchObservations_StorageTier(t *testing.T) {
	tests := []struct {
		name        string
		candClass   string
		targetClass string
		wantQuality string
	}{
		{"exact (same class)", "standard", "standard", "exact"},
		{"approximate (one tier apart)", "infrequent_access", "standard", "approximate"}, // 50% delta <= 60%
		{"none (two tiers apart)", "archive", "standard", ""},                            // 100% delta > 60% → nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obsList := []domain.PriceObservation{
				{SkuID: "s", StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: tt.candClass}},
			}
			target := service.MatchTarget{SizeGB: 100, StorageClass: tt.targetClass, Category: "storage"}
			result := service.MatchObservations(service.StorageScorer{}, obsList, target, service.StorageThresholds)

			if tt.wantQuality == "" {
				if result != nil {
					t.Errorf("expected nil for none tier, got quality %s", result.MatchQuality)
				}
			} else {
				if result == nil {
					t.Fatalf("expected result with quality %s, got nil", tt.wantQuality)
				}
				if result.MatchQuality != tt.wantQuality {
					t.Errorf("expected quality %s, got %s (delta_pct=%f)", tt.wantQuality, result.MatchQuality, result.MatchDeltaPct)
				}
			}
		})
	}
}

func TestMatchObservations_NetworkTier(t *testing.T) {
	tests := []struct {
		name        string
		candType    string
		targetType  string
		wantQuality string
	}{
		{"exact (same type)", "internet_egress", "internet_egress", "exact"},
		{"approximate (one tier apart)", "inter_region", "internet_egress", "approximate"}, // 50% delta <= 50%
		{"none (two tiers apart)", "intra_region", "internet_egress", ""},                  // 100% delta > 50% → nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obsList := []domain.PriceObservation{
				{SkuID: "n", NetworkAttributes: domain.NetworkAttributes{EgressGB: 1, TransferType: tt.candType}},
			}
			target := service.MatchTarget{EgressGB: 500, TransferType: tt.targetType, Category: "network"}
			result := service.MatchObservations(service.NetworkScorer{}, obsList, target, service.NetworkThresholds)

			if tt.wantQuality == "" {
				if result != nil {
					t.Errorf("expected nil for none tier, got quality %s", result.MatchQuality)
				}
			} else {
				if result == nil {
					t.Fatalf("expected result with quality %s, got nil", tt.wantQuality)
				}
				if result.MatchQuality != tt.wantQuality {
					t.Errorf("expected quality %s, got %s (delta_pct=%f)", tt.wantQuality, result.MatchQuality, result.MatchDeltaPct)
				}
			}
		})
	}
}

func TestMatchObservations_MissingOptionalDimension(t *testing.T) {
	// Target requests storage_class but candidate doesn't have it.
	// The candidate should still be eligible (passes guard on size_gb),
	// but storage_class should appear in missing_attributes.
	obsList := []domain.PriceObservation{
		{
			SkuID:             "no-class",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: ""},
		},
	}

	target := service.MatchTarget{
		SizeGB:       100,
		StorageClass: "standard",
		Category:     "storage",
	}

	result := service.MatchObservations(service.StorageScorer{}, obsList, target, service.StorageThresholds)
	if result == nil {
		t.Fatal("expected result — candidate should be eligible despite missing class")
	}
	if len(result.MissingAttributes) != 1 || result.MissingAttributes[0] != "storage_class" {
		t.Errorf("expected [storage_class] in missing, got %v", result.MissingAttributes)
	}
	if result.MatchQuality != "exact" {
		t.Errorf("expected exact (class excluded from distance), got %s", result.MatchQuality)
	}
}

// --- Compute regression test ---
// This test asserts that the new MatchObservations with ComputeScorer
// produces equivalent results to the legacy ScoreComputeObservations for
// the same fixture data.

func TestComputeRegressionMatchVsLegacy(t *testing.T) {
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

	// Legacy: ScoreComputeObservations with reqVCPU=4, reqRAMGB=16, strictFamily=false
	legacyResults := service.ScoreComputeObservations(4, 16, false, obsList)

	// New engine: MatchObservations picks the BEST candidate (lowest distance).
	target := service.MatchTarget{VCPU: 4, RAMGB: 16, Category: "compute"}
	newResult := service.MatchObservations(service.ComputeScorer{}, obsList, target, service.ComputeThresholds)

	// Legacy returns 2 results (exact-match and close-match; far-match filtered at >50%).
	if len(legacyResults) != 2 {
		t.Fatalf("legacy: expected 2, got %d", len(legacyResults))
	}

	// The new engine picks the best single match — which should be exact-match.
	if newResult == nil {
		t.Fatal("new engine: expected result, got nil")
	}
	if newResult.Observation.SkuID != "exact-match" {
		t.Errorf("new engine: expected exact-match, got %s", newResult.Observation.SkuID)
	}
	if newResult.MatchQuality != "exact" {
		t.Errorf("new engine: expected quality 'exact', got %s", newResult.MatchQuality)
	}

	// Verify the legacy first result (exact) matches the new engine result.
	if legacyResults[0].MatchQuality != newResult.MatchQuality {
		t.Errorf("regression: legacy quality=%s, new quality=%s",
			legacyResults[0].MatchQuality, newResult.MatchQuality)
	}
}

func TestStorageScorer_4NewProviders(t *testing.T) {
	scorer := service.StorageScorer{}
	target := service.MatchTarget{
		SizeGB:       500,
		StorageClass: "standard",
		Category:     "storage",
	}

	obsList := []domain.PriceObservation{
		{
			Provider:          "oracle",
			SkuID:             "SKU-OCI-B88206",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
		},
		{
			Provider:          "ibm",
			SkuID:             "SKU-IBM-STANDARD-STORAGE",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
		},
		{
			Provider:          "alibaba",
			SkuID:             "SKU-ALI-STORAGE-STANDARD",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
		},
		{
			Provider:          "digitalocean",
			SkuID:             "SKU-DO-STORAGE-SPACES",
			StorageAttributes: domain.StorageAttributes{SizeGB: 500, StorageClass: "standard"},
		},
	}

	for _, obs := range obsList {
		t.Run(obs.Provider, func(t *testing.T) {
			dist, missing, eligible := scorer.Score(obs, target)
			if !eligible {
				t.Fatalf("expected %s observation to be eligible", obs.Provider)
			}
			if dist != 0 {
				t.Errorf("expected %s distance 0, got %f", obs.Provider, dist)
			}
			if len(missing) != 0 {
				t.Errorf("expected no missing attributes for %s, got %v", obs.Provider, missing)
			}
		})
	}
}

func TestNetworkScorer_4NewProviders(t *testing.T) {
	scorer := service.NetworkScorer{}
	target := service.MatchTarget{
		EgressGB:     1000,
		TransferType: "internet_egress",
		Category:     "network",
	}

	obsList := []domain.PriceObservation{
		{
			Provider:          "oracle",
			SkuID:             "SKU-OCI-B88210",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000, TransferType: "internet_egress"},
		},
		{
			Provider:          "ibm",
			SkuID:             "SKU-IBM-PUBLIC-EGRESS",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000, TransferType: "internet_egress"},
		},
		{
			Provider:          "alibaba",
			SkuID:             "SKU-ALI-NETWORK-DATA-TRANSFER-OUT",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000, TransferType: "internet_egress"},
		},
		{
			Provider:          "digitalocean",
			SkuID:             "SKU-DO-NETWORK-BANDWIDTH",
			NetworkAttributes: domain.NetworkAttributes{EgressGB: 1000, TransferType: "internet_egress"},
		},
	}

	for _, obs := range obsList {
		t.Run(obs.Provider, func(t *testing.T) {
			dist, missing, eligible := scorer.Score(obs, target)
			if !eligible {
				t.Fatalf("expected %s observation to be eligible", obs.Provider)
			}
			if dist != 0 {
				t.Errorf("expected %s distance 0, got %f", obs.Provider, dist)
			}
			if len(missing) != 0 {
				t.Errorf("expected no missing attributes for %s, got %v", obs.Provider, missing)
			}
		})
	}
}
