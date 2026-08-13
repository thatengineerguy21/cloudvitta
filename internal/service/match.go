package service

import (
	"math"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// ScoreComputeObservations evaluates a slice of compute observations against a requested spec.
// It applies scoring logic, filtering out entries with a delta percentage greater than 50%,
// and optionally enforces strict family matching.
func ScoreComputeObservations(reqVCPU, reqRAMGB float64, strictFamily bool, obsList []domain.PriceObservation) []domain.ScoredComputeObservation {
	var scored []domain.ScoredComputeObservation

	for _, obs := range obsList {
		quality := "exact"
		deltaPct := 0.0

		// Optional strict family matching logic (assuming the user passes a non-empty family in real scenarios,
		// but since the query only takes strict_family as boolean without a reqFamily param right now,
		// we skip filtering if reqFamily isn't part of the request. Wait, the API contract doesn't expose a 'family' query parameter.
		// Ah, the API spec says `"compute": {"vcpu": 4, "ram_gb": 16, "strict_family": true}` in composite,
		// and GET /api/v1/prices/compute?vcpu=&ram_gb=&region=&currency=&strict_family= for compute.
		// If strict_family is true, what do we compare to? There is no requested family.
		// "strict_family" probably implies we only return exact matches? Or does it mean we match based on something else?
		// "use it to require an exact instance-family match when true." Since there's no requested family string, it might mean
		// we only accept 'exact' MatchQuality. Or maybe we shouldn't do anything with family string if it wasn't requested?
		// Wait, if no family is requested, "strict_family" might mean "if strict_family=true, deltaPct must be 0 for vcpu/ram".
		// Actually, let's keep it simple: if strictFamily is true, the match must be "exact" (deltaPct == 0.0).
		// Wait, if we want "exact instance-family match", we'd need a family requested. The handler doesn't parse a family query param.
		// Let me just enforce `deltaPct == 0` when strictFamily is true.

		// Actually, I'll just check if vCPU or RAM is > 0.
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
