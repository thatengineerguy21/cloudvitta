package service

import (
	"math"
	"sort"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// ServerlessWorkload holds caller-specified workload parameters for serverless compute matching and cost estimation.
type ServerlessWorkload struct {
	Architecture        string          // canonical: x86_64, arm64
	Tier                string          // canonical: consumption, flex_consumption, 1st_gen, 2nd_gen
	ExecutionDurationMS decimal.Decimal // average execution duration in milliseconds
	MemoryMB            decimal.Decimal // allocated memory in MB
	RequestsPerMonth    decimal.Decimal // invocation requests per month
}

// CategoryScorer is the Strategy interface for per-category distance scoring.
// Each category (compute, storage, network) implements its own scorer.
type CategoryScorer interface {
	// Score computes the weighted distance between a candidate observation and
	// the target spec. Lower distance means closer match.
	// It returns:
	//   - distance: the weighted-distance score (0 = exact)
	//   - missingAttrs: optional dimensions present in the target but absent in the candidate
	//   - eligible: false if the candidate fails the minimum comparable-dimensions guard
	Score(candidate domain.PriceObservation, target MatchTarget) (distance float64, missingAttrs []string, eligible bool)
}

// MatchTarget holds the caller's requested spec for matching.
// Fields are populated per-category; unused fields stay at zero-value.
type MatchTarget struct {
	// Compute dimensions
	VCPU   float64
	RAMGB  float64
	Family string

	// Storage dimensions
	SizeGB       float64
	StorageClass string // canonical: standard, infrequent_access, archive

	// Network dimensions
	EgressGB     float64
	TransferType string // canonical: intra_region, inter_region, internet_egress

	// Database RDBMS dimensions
	Engine            string  // canonical: postgresql, mysql, sqlserver, mariadb, oracle
	DatabaseStorageGB float64 // storage requested in GB
	DatabaseIOPS      *int    // provisioned IOPS
	MultiAZ           bool    // High Availability requested
	StorageFamily     string  // optional: gp3, gp2, io1, ssd

	// Database NoSQL dimensions
	DataModel        string  // canonical: document, key_value, wide_column, graph, multi_model
	PricingMode      string  // canonical: provisioned, on_demand, serverless
	ReadUnits        float64 // requested reads/sec or RCU
	WriteUnits       float64 // requested writes/sec or WCU
	NoSQLStorageGB   float64 // requested storage in GB
	NoSQLMultiRegion bool    // Multi-Region replication requested

	// Kubernetes dimensions
	KubernetesTier  domain.KubernetesTier  // canonical: free, standard, extended_support
	ClusterTopology domain.ClusterTopology // GCP-specific: zonal, regional, autopilot

	// Serverless dimensions
	ServerlessWorkload ServerlessWorkload

	// Cross-category controls
	StrictFamily bool   // default true — only match within same family tier
	Category     string // compute, storage, network, database_rdbms, database_nosql, kubernetes, serverless
}

// MatchResult holds the outcome of matching one provider's observations.
type MatchResult struct {
	Observation       domain.PriceObservation
	MatchQuality      string  // exact, close, approximate, none
	MatchDeltaPct     float64 // rounded to 2 decimal places
	MissingAttributes []string
}

// MatchObservations runs the matching engine for a single provider's observations.
// It filters, scores, picks the lowest-distance candidate, and classifies the match tier.
//
// The engine:
//  1. Filters by family if strict_family is true (compute only)
//  2. Checks minimum comparable-dimensions guard per candidate
//  3. Scores each eligible candidate using the category-specific scorer
//  4. Picks the lowest-distance candidate (tie-break: lexicographic SKU ID)
//  5. Classifies into exact/close/approximate/none using category-specific thresholds
//
// Returns nil if no eligible candidate exists (the caller should omit this provider
// from results rather than showing misleading data).
func MatchObservations(scorer CategoryScorer, obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) *MatchResult {
	if len(obsList) == 0 {
		return nil
	}

	type scored struct {
		obs          domain.PriceObservation
		distance     float64
		missingAttrs []string
	}

	var candidates []scored

	for _, obs := range obsList {
		// Strict family filter: for compute, skip candidates outside the requested family.
		if target.StrictFamily && target.Category == "compute" && target.Family != "" {
			if obs.Attributes.Family != target.Family {
				continue
			}
		}

		dist, missing, eligible := scorer.Score(obs, target)
		if !eligible {
			continue
		}

		candidates = append(candidates, scored{
			obs:          obs,
			distance:     dist,
			missingAttrs: missing,
		})
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort: lowest distance first, then lexicographic SKU ID for deterministic tie-breaking.
	// Never use price for tie-breaking.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].obs.SkuID < candidates[j].obs.SkuID
	})

	best := candidates[0]

	// Classify match quality tier using category-specific thresholds.
	quality := classifyTier(best.distance, thresholds)

	// If the best candidate falls in "none" tier, omit this provider entirely.
	if quality == "none" {
		return nil
	}

	// Round delta percentage to 2 decimal places.
	deltaPct := roundTo2(best.distance * 100)

	missing := best.missingAttrs
	if missing == nil {
		missing = []string{}
	}

	return &MatchResult{
		Observation:       best.obs,
		MatchQuality:      quality,
		MatchDeltaPct:     deltaPct,
		MissingAttributes: missing,
	}
}

// roundTo2 rounds a float64 to 2 decimal places.
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
