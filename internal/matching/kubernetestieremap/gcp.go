package kubernetestieremap

import (
	"fmt"
	"strings"
)

var gcpTierMap = map[string]string{
	"standard":               TierStandard,
	"standard tier":          TierStandard,
	"cluster management":     TierStandard,
	"cluster management fee": TierStandard,
	"gke":                    TierStandard,
	"kubernetes engine":      TierStandard,
}

// MapGCPTier maps a GCP GKE SKU description, resource group, or tier to a canonical tier identifier.
func MapGCPTier(rawTier string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := gcpTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: gcp tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
