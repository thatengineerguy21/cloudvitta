package kubernetestieremap

import (
	"fmt"
	"strings"
)

var azureTierMap = map[string]string{
	"free":               TierFree,
	"free tier":          TierFree,
	"freetier":           TierFree,
	"standard":           TierStandard,
	"standard tier":      TierStandard,
	"uptime sla":         TierStandard,
	"cluster management": TierStandard,
	"extended_support":   TierExtendedSupport,
	"extended support":   TierExtendedSupport,
	"extended-support":   TierExtendedSupport,
	"extended":           TierExtendedSupport,
	"long term support":  TierExtendedSupport,
	"lts":                TierExtendedSupport,
	"premium":            TierExtendedSupport,
}

// MapAzureTier maps an Azure AKS tier, meter name, or SKU name to a canonical tier identifier.
func MapAzureTier(rawTier string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := azureTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: azure tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
