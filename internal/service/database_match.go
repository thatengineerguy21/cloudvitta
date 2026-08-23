package service

import (
	"math"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// Weights for database RDBMS distance scoring formula (§2 key architecture decisions).
const (
	WeightVCPU           = 1.0
	WeightRAM            = 1.0
	WeightStorage        = 0.5
	WeightIOPS           = 0.5
	HighAvailPenaltyRate = 0.50
)

// DatabaseRDBMSScorer implements CategoryScorer for relational database resources.
// Distance formula:
//
//	Distance = w_vcpu * Δvcpu + w_ram * Δram + w_storage * Δstorage + w_iops * Δiops + Penalty_ha
//	where:
//	  Δvcpu    = |C.vcpu - T.vcpu| / T.vcpu
//	  Δram     = |C.ram - T.ram| / T.ram
//	  Δstorage = |C.storage - T.storage| / T.storage
//	  Δiops    = |C.iops - T.iops| / T.iops
//	  Penalty_ha = 0.50 (if C.multi_az != T.multi_az)
type DatabaseRDBMSScorer struct{}

// Score computes weighted distance between a joined database candidate and the target spec.
func (s DatabaseRDBMSScorer) Score(candidate domain.PriceObservation, target MatchTarget) (float64, []string, bool) {
	candAttrs := candidate.DatabaseRDBMSAttributes

	// Hard Engine Mismatch Pre-filter: non-matching engines are immediately ineligible
	if target.Engine != "" && candAttrs.Engine != "" {
		if !strings.EqualFold(candAttrs.Engine, target.Engine) {
			return 0, nil, false
		}
	}

	// Minimum comparable dimensions guard: compute instance components require nonzero vCPU/RAM if target requested
	if target.VCPU > 0 && candAttrs.VCPU <= 0 {
		return 0, nil, false
	}
	if target.RAMGB > 0 && candAttrs.RAMGB <= 0 {
		return 0, nil, false
	}

	var distance float64
	var missingAttrs []string

	// vCPU term (weight = 1.0)
	if target.VCPU > 0 && candAttrs.VCPU > 0 {
		distance += WeightVCPU * math.Abs(candAttrs.VCPU-target.VCPU) / target.VCPU
	}

	// RAM term (weight = 1.0)
	if target.RAMGB > 0 && candAttrs.RAMGB > 0 {
		distance += WeightRAM * math.Abs(candAttrs.RAMGB-target.RAMGB) / target.RAMGB
	}

	// Storage term (weight = 0.5)
	if target.DatabaseStorageGB > 0 && candAttrs.StorageGB > 0 {
		distance += WeightStorage * math.Abs(candAttrs.StorageGB-target.DatabaseStorageGB) / target.DatabaseStorageGB
	}

	// IOPS term (weight = 0.5)
	if target.DatabaseIOPS != nil && *target.DatabaseIOPS > 0 {
		if candAttrs.IOPS != nil && *candAttrs.IOPS > 0 {
			distance += WeightIOPS * math.Abs(float64(*candAttrs.IOPS-*target.DatabaseIOPS)) / float64(*target.DatabaseIOPS)
		} else {
			missingAttrs = append(missingAttrs, "iops")
		}
	}

	// High Availability penalty
	if candAttrs.MultiAZ != target.MultiAZ {
		distance += HighAvailPenaltyRate
	}

	return distance, missingAttrs, true
}

// MatchDatabaseObservations orchestrates dynamic query-time join between compute instance
// and storage observations, scores the composite candidates, and picks the best match.
func MatchDatabaseObservations(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) *MatchResult {
	if len(obsList) == 0 {
		return nil
	}

	var instances []domain.PriceObservation
	var storages []domain.PriceObservation

	for _, obs := range obsList {
		if obs.DatabaseRDBMSAttributes.ComponentType == "storage" {
			storages = append(storages, obs)
		} else {
			instances = append(instances, obs)
		}
	}

	if len(instances) == 0 {
		return nil
	}

	type scoredCandidate struct {
		obs          domain.PriceObservation
		distance     float64
		missingAttrs []string
	}

	var candidates []scoredCandidate
	scorer := DatabaseRDBMSScorer{}

	requestedStorageGB := target.DatabaseStorageGB
	if requestedStorageGB <= 0 {
		requestedStorageGB = 1
	}

	for _, inst := range instances {
		// Find best matching storage row for query-time join
		bestStorage := findMatchingStorage(inst, storages, target)

		joinedObs := inst
		joinedObs.DatabaseRDBMSAttributes.StorageGB = requestedStorageGB
		if target.DatabaseIOPS != nil {
			joinedObs.DatabaseRDBMSAttributes.IOPS = target.DatabaseIOPS
		}

		if bestStorage != nil {
			joinedObs.DatabaseRDBMSAttributes.StorageFamily = bestStorage.DatabaseRDBMSAttributes.StorageFamily
			storageHourly := CalculateStorageHourlyCost(bestStorage.PriceAmount, decimal.NewFromFloat(requestedStorageGB))
			joinedObs.PriceAmount = inst.PriceAmount.Add(storageHourly)
		}

		dist, missing, eligible := scorer.Score(joinedObs, target)
		if !eligible {
			continue
		}

		candidates = append(candidates, scoredCandidate{
			obs:          joinedObs,
			distance:     dist,
			missingAttrs: missing,
		})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort: lowest distance first, then lexicographic SKU ID for deterministic tie-breaking.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].obs.SkuID < candidates[j].obs.SkuID
	})

	best := candidates[0]
	quality := classifyTier(best.distance, thresholds)
	if quality == "none" {
		return nil
	}

	missing := best.missingAttrs
	if missing == nil {
		missing = []string{}
	}

	return &MatchResult{
		Observation:       best.obs,
		MatchQuality:      quality,
		MatchDeltaPct:     roundTo2(best.distance * 100),
		MissingAttributes: missing,
	}
}

// findMatchingStorage finds the most suitable storage observation to join with an instance candidate.
func findMatchingStorage(inst domain.PriceObservation, storages []domain.PriceObservation, target MatchTarget) *domain.PriceObservation {
	if len(storages) == 0 {
		return nil
	}

	var best *domain.PriceObservation
	var bestScore = -1

	for i := range storages {
		stor := &storages[i]

		// Region and provider must match
		if stor.Provider != inst.Provider || stor.Region != inst.Region {
			continue
		}

		// If storage specifies an engine, it must match instance engine
		if stor.DatabaseRDBMSAttributes.Engine != "" && stor.DatabaseRDBMSAttributes.Engine != "any" {
			if !strings.EqualFold(stor.DatabaseRDBMSAttributes.Engine, inst.DatabaseRDBMSAttributes.Engine) {
				continue
			}
		}

		score := 0

		// Prefer matching MultiAZ
		if stor.DatabaseRDBMSAttributes.MultiAZ == inst.DatabaseRDBMSAttributes.MultiAZ {
			score += 10
		}

		// Prefer matching StorageFamily
		if target.StorageFamily != "" && strings.EqualFold(stor.DatabaseRDBMSAttributes.StorageFamily, target.StorageFamily) {
			score += 20
		} else if strings.EqualFold(stor.DatabaseRDBMSAttributes.StorageFamily, "gp3") || strings.EqualFold(stor.DatabaseRDBMSAttributes.StorageFamily, "ssd") {
			score += 5
		}

		if score > bestScore {
			bestScore = score
			best = stor
		}
	}

	return best
}
