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
