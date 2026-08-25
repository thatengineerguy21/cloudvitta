package kubernetestieremap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var azureTierMap = map[string]domain.KubernetesTier{
	"free":                               TierFree,
	"free tier":                          TierFree,
	"freetier":                           TierFree,
	"standard":                           TierStandard,
	"standard tier":                      TierStandard,
	"uptime sla":                         TierStandard,
	"cluster management":                 TierStandard,
	"automatic":                          TierStandard,
	"automatic high performance compute": TierStandard,
	"automatic gpu accelerated":          TierStandard,
	"automatic general purpose":          TierStandard,
	"automatic memory optimized":         TierStandard,
	"automatic compute optimized":        TierStandard,
	"automatic storage optimized":        TierStandard,
	"automatic standard":                 TierStandard,
	"automatic premium":                  TierStandard,
	"extended_support":                   TierExtendedSupport,
	"extended support":                   TierExtendedSupport,
	"extended-support":                   TierExtendedSupport,
	"extended":                           TierExtendedSupport,
	"long term support":                  TierExtendedSupport,
	"lts":                                TierExtendedSupport,
	"premium":                            TierExtendedSupport,
}

// MapAzureTier maps an Azure AKS tier, meter name, or SKU name to a canonical tier identifier.
func MapAzureTier(rawTier string) (domain.KubernetesTier, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := azureTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: azure tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
