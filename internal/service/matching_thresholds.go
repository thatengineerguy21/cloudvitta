package service

// CategoryThresholds defines per-category boundaries for match quality classification.
// The matching engine classifies candidates into tiers based on their weighted distance:
//   - exact:       distance == 0
//   - close:       distance <= CloseCutoff
//   - approximate: distance <= ApproximateCutoff
//   - none:        distance > ApproximateCutoff (provider omitted from results)
type CategoryThresholds struct {
	// CloseCutoff is the maximum distance for a "close" match (e.g. 0.10 = 10%).
	CloseCutoff float64

	// ApproximateCutoff is the maximum distance for an "approximate" match.
	// Beyond this, the candidate is classified as "none" and the provider is omitted.
	ApproximateCutoff float64
}

// Minimum comparable-dimensions guard:
// An optional dimension only participates in the guard if the target/query actually
// requested it. If the caller omits storage_class or transfer_type, the guard reduces
// to requiring just the required dimension (size_gb or egress_gb). It does not trivially
// fail, nor does it demand the optional dimension be present regardless.
// This rule applies uniformly to compute, storage, and network categories.

var (
	// ComputeThresholds defines match quality boundaries for compute category.
	// close: within 10% combined vCPU+RAM delta.
	// approximate: within 50% combined delta (same as prior behavior where >50% was dropped).
	ComputeThresholds = CategoryThresholds{
		CloseCutoff:       0.10,
		ApproximateCutoff: 0.50,
	}

	// StorageThresholds defines match quality boundaries for storage category.
	// Storage matching is more lenient since size is a continuous, scalable dimension.
	// close: within 15% combined size+class delta.
	// approximate: within 60% combined delta.
	StorageThresholds = CategoryThresholds{
		CloseCutoff:       0.15,
		ApproximateCutoff: 0.60,
	}

	// NetworkThresholds defines match quality boundaries for network category.
	// Network matching uses similar tolerance to compute.
	// close: within 10% combined egress+transfer_type delta.
	// approximate: within 50% combined delta.
	NetworkThresholds = CategoryThresholds{
		CloseCutoff:       0.10,
		ApproximateCutoff: 0.50,
	}

	// DatabaseRDBMSThresholds defines match quality boundaries for relational database category.
	// close: within 20% combined vCPU+RAM+storage+iops delta.
	// approximate: within 50% combined delta.
	DatabaseRDBMSThresholds = CategoryThresholds{
		CloseCutoff:       0.20,
		ApproximateCutoff: 0.50,
	}

	// DatabaseNoSQLThresholds defines match quality boundaries for NoSQL database category.
	// close: within 20% combined throughput+storage delta.
	// approximate: within 50% combined delta.
	DatabaseNoSQLThresholds = CategoryThresholds{
		CloseCutoff:       0.20,
		ApproximateCutoff: 0.50,
	}
)

// ThresholdsForCategory returns the per-category thresholds for the given category.
func ThresholdsForCategory(category string) CategoryThresholds {
	switch category {
	case "compute":
		return ComputeThresholds
	case "storage":
		return StorageThresholds
	case "network":
		return NetworkThresholds
	case "database_rdbms":
		return DatabaseRDBMSThresholds
	case "database_nosql":
		return DatabaseNoSQLThresholds
	default:
		// Fallback to the strictest thresholds for unknown categories.
		return ComputeThresholds
	}
}

// classifyTier classifies a weighted distance into one of the four match quality tiers.
func classifyTier(distance float64, th CategoryThresholds) string {
	switch {
	case distance == 0:
		return "exact"
	case distance <= th.CloseCutoff:
		return "close"
	case distance <= th.ApproximateCutoff:
		return "approximate"
	default:
		return "none"
	}
}
