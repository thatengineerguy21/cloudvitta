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
}

// MapGCPTier maps a GCP GKE SKU description, resource group, or tier to a canonical tier identifier.
func MapGCPTier(rawTier string) (domain.KubernetesTier, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := gcpTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: gcp tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
