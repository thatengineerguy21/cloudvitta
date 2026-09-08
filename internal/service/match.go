package service

import (
	"math"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// --- Storage class ordinal map ---
// storageClassOrdinal assigns a graduated ordinal penalty to canonical storage classes.
// Used for distance scoring: the further apart two classes are, the higher the penalty.
// This is a categorical-ordinal penalty, NOT a strict_family filter.
var storageClassOrdinal = map[string]int{
	"standard":          0,
	"infrequent_access": 1,
	"archive":           2,
}

// maxStorageClassOrdinal is the highest ordinal value in storageClassOrdinal.
const maxStorageClassOrdinal = 2

// --- Transfer type ordinal map ---
// transferTypeOrdinal assigns a graduated ordinal penalty to canonical transfer types.
// Used for network distance scoring, same principle as storageClassOrdinal.
var transferTypeOrdinal = map[string]int{
	"intra_region":    0,
	"inter_region":    1,
	"internet_egress": 2,
}

// maxTransferTypeOrdinal is the highest ordinal value in transferTypeOrdinal.
const maxTransferTypeOrdinal = 2

// --- Compute scorer ---

// ComputeScorer implements CategoryScorer for compute resources.
// Distance formula:
//
//	distance = w_vcpu * |C.vcpu - T.vcpu| / T.vcpu + w_ram * |C.ram_gb - T.ram_gb| / T.ram_gb
//
// Weights default to 1.0. Both vcpu and ram_gb are required dimensions for compute.
type ComputeScorer struct{}

// Score computes weighted distance between a compute candidate and the target spec.
// Minimum dimensions guard: at least one of vcpu or ram_gb must be nonzero in the target,
// and the candidate must have matching nonzero values for the requested dimensions.
func (s ComputeScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	// If no compute dimensions are requested, all candidates are exact matches.
	if target.VCPU <= 0 && target.RAMGB <= 0 {
		return 0, []string{}, true
	}

	var distance float64
	var missingAttrs []string

	if target.VCPU > 0 {
		if candidate.Attributes.VCPU <= 0 {
			// Candidate lacks the required dimension — fails the guard.
			return 0, nil, false
		}
		distance += math.Abs(candidate.Attributes.VCPU-target.VCPU) / target.VCPU
	}

	if target.RAMGB > 0 {
		if candidate.Attributes.RAMGB <= 0 {
			return 0, nil, false
		}
		distance += math.Abs(candidate.Attributes.RAMGB-target.RAMGB) / target.RAMGB
	}

	return distance, missingAttrs, true
}

// --- Storage scorer ---

// StorageScorer implements CategoryScorer for storage resources.
// Storage observations represent unit rates ($/GB-month) with SizeGB = 1.
// Volume (size_gb) is a cost multiplier in pricing calculation rather than a
// distance term. The distance formula evaluates storage_class compatibility:
//
//	distance = w_class * |ordinal(C.storage_class) - ordinal(T.storage_class)| / max_ordinal
//
// storage_class is an optional dimension: if the target omits it, the class term is excluded
// entirely (not defaulted to "standard"), and the candidate's class is recorded in
// missing_attributes if the candidate has one. This diverges from compute's family logic,
// where family is a strict filter rather than a distance term.
type StorageScorer struct{}

// Score computes weighted distance between a storage candidate and the target spec.
func (s StorageScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	// Eligibility guard: candidate must have a valid unit-rate size.
	if candidate.StorageAttributes.SizeGB <= 0 {
		// Candidate lacks the required dimension.
		return 0, nil, false
	}

	var distance float64
	var missingAttrs []string

	// Volume dimension (size_gb) is NOT a matchable spec for storage.
	// Storage observations represent unit rates ($/GB-month) with SizeGB=1.
	// The requested volume is applied as a cost multiplier in pricing_calc.go,
	// not as a distance term. Matching evaluates storage_class compatibility only.

	// Storage class term (optional, weight=1.0).
	if target.StorageClass != "" {
		targetOrd, targetOK := storageClassOrdinal[target.StorageClass]
		candOrd, candOK := storageClassOrdinal[candidate.StorageAttributes.StorageClass]

		if !candOK || candidate.StorageAttributes.StorageClass == "" {
			// Candidate has no mapped class — record as missing, exclude from distance.
			missingAttrs = append(missingAttrs, "storage_class")
		} else if targetOK {
			// Both have valid ordinals — graduated penalty.
			distance += float64(abs(candOrd-targetOrd)) / float64(maxStorageClassOrdinal)
		}
	}
	// Note: If target omitted storage_class but candidate has one,
	// exclude from distance formula and do not record as missing.
	// This is intentional — the caller didn't ask for this dimension.

	return distance, missingAttrs, true
}

// --- Network scorer ---

// NetworkScorer implements CategoryScorer for network / data transfer resources.
// Network observations represent unit rates ($/GB) with EgressGB = 1.
// Volume (egress_gb) is a cost multiplier in pricing calculation rather than a
// distance term. The distance formula evaluates transfer_type compatibility:
//
//	distance = w_transfer * |ordinal(C.transfer_type) - ordinal(T.transfer_type)| / max_ordinal
//
// transfer_type is an optional dimension: same logic as storage_class above.
type NetworkScorer struct{}

// Score computes weighted distance between a network candidate and the target spec.
func (s NetworkScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	// Eligibility guard: candidate must have a valid unit-rate egress value.
	if candidate.NetworkAttributes.EgressGB <= 0 {
		return 0, nil, false
	}

	var distance float64
	var missingAttrs []string

	// Volume dimension (egress_gb) is NOT a matchable spec for network.
	// Network observations represent unit rates ($/GB) with EgressGB=1.
	// The requested volume is applied as a cost multiplier in pricing_calc.go,
	// not as a distance term. Matching evaluates transfer_type compatibility only.

	// Scoring guard: private/dedicated and VPN egress types are isolated from general cloud egress.
	// A request for direct_connect_egress or vpn_egress must strictly match candidates of the identical type,
	// and general/unspecified requests must never match dedicated or VPN candidates.
	isDedicatedOrVPN := func(tt string) bool {
		return tt == "direct_connect_egress" || tt == "vpn_egress"
	}
	if isDedicatedOrVPN(target.TransferType) || isDedicatedOrVPN(candidate.NetworkAttributes.TransferType) {
		if target.TransferType != candidate.NetworkAttributes.TransferType {
			return 0, nil, false
		}
		return 0, nil, true
	}

	// Transfer type term (optional, weight=1.0).
	if target.TransferType != "" {
		targetOrd, targetOK := transferTypeOrdinal[target.TransferType]
		candOrd, candOK := transferTypeOrdinal[candidate.NetworkAttributes.TransferType]

		if !candOK || candidate.NetworkAttributes.TransferType == "" {
			missingAttrs = append(missingAttrs, "transfer_type")
		} else if targetOK {
			distance += float64(abs(candOrd-targetOrd)) / float64(maxTransferTypeOrdinal)
		}
	}

	return distance, missingAttrs, true
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// --- Legacy compatibility wrapper ---

// ScoreComputeObservations is preserved for backward compatibility and regression testing.
// For standard query-time matching, use MatchObservations with ComputeScorer.
func ScoreComputeObservations(reqVCPU, reqRAMGB float64, strictFamily bool, obsList []domain.PriceObservation) []domain.ScoredComputeObservation {
	var scored []domain.ScoredComputeObservation

	for _, obs := range obsList {
		quality := "exact"
		deltaPct := 0.0

		if reqVCPU > 0 || reqRAMGB > 0 {
			var vcpuDelta float64
			if reqVCPU > 0 {
				vcpuDelta = math.Abs(obs.Attributes.VCPU-reqVCPU) / reqVCPU
			}

			var ramDelta float64
			if reqRAMGB > 0 {
				ramDelta = math.Abs(obs.Attributes.RAMGB-reqRAMGB) / reqRAMGB
			}

			deltaPct = (vcpuDelta + ramDelta) * 100.0
			if deltaPct == 0.0 {
				quality = "exact"
			} else if deltaPct <= 50.0 {
				quality = "close"
			} else {
				// Delta too high, skip candidate
				continue
			}
		}

		if strictFamily && quality != "exact" {
			// strict_family requires an exact match for now
			continue
		}

		scored = append(scored, domain.ScoredComputeObservation{
			Observation:       obs,
			MatchQuality:      quality,
			MatchDeltaPct:     math.Round(deltaPct*100) / 100,
			MissingAttributes: []string{}, // No optional attributes implemented yet
		})
	}

	return scored
}
