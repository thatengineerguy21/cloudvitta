package kubernetestieremap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var gcpTierMap = map[string]domain.KubernetesTier{
	"standard":               TierStandard,
	"standard tier":          TierStandard,
	"cluster management":     TierStandard,
	"cluster management fee": TierStandard,
	"kubernetes engine cluster management fee": TierStandard,
	"gke cluster management fee":               TierStandard,
	"gke standard":                             TierStandard,
	"gke":                                      TierStandard,
	"kubernetes engine":                        TierStandard,
	"cluster":                                  TierStandard,
	"zonal kubernetes clusters":                TierStandard,
	"regional kubernetes clusters":             TierStandard,
	"extended period kubernetes clusters":      TierExtendedSupport,
	"extended-support":                         TierExtendedSupport,
	"extended support":                         TierExtendedSupport,
}

// MapGCPTier maps a GCP GKE SKU description, resource group, or tier to a canonical tier identifier.
func MapGCPTier(rawTier string) (domain.KubernetesTier, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	if canonical, ok := gcpTierMap[key]; ok {
		return canonical, nil
	}

	// Substring matching for known GKE cluster management fee line items
	if strings.Contains(key, "cluster management") || strings.Contains(key, "cluster fee") || strings.Contains(key, "gke standard") {
		return TierStandard, nil
	}

	return "", fmt.Errorf("%w: gcp tier %q", ErrUnmappedTier, rawTier)
}
