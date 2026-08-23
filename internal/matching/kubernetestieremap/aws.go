package kubernetestieremap

import (
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

var awsTierMap = map[string]domain.KubernetesTier{
	"standard":                        TierStandard,
	"amazoneks":                       TierStandard,
	"amazoneks-hours":                 TierStandard,
	"amazoneks-hours:percluster":      TierStandard,
	"percluster":                      TierStandard,
	"createcluster":                   TierStandard,
	"extended_support":                TierExtendedSupport,
	"extended-support":                TierExtendedSupport,
	"extendedsupport":                 TierExtendedSupport,
	"extended":                        TierExtendedSupport,
	"amazoneks-extendedsupport-hours": TierExtendedSupport,
	"amazoneks-extendedsupport-hours:percluster": TierExtendedSupport,
	"clustersupport": TierExtendedSupport,
}

// MapAWSTier maps an AWS EKS tier, usage type, or operation string to a canonical tier identifier.
func MapAWSTier(rawTier string) (domain.KubernetesTier, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := awsTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: aws tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
