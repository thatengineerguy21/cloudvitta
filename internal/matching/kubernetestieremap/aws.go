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
	"createoperation":                 TierStandard,
	"extended_support":                TierExtendedSupport,
	"extended-support":                TierExtendedSupport,
	"extendedsupport":                 TierExtendedSupport,
	"extended":                        TierExtendedSupport,
	"amazoneks-extendedsupport-hours": TierExtendedSupport,
	"amazoneks-extendedsupport-hours:percluster": TierExtendedSupport,
	"clustersupport":               TierExtendedSupport,
	"provisionedcontrolplaneusage": TierStandard,
	"provisionedtier":              TierStandard,
}

// MapAWSTier maps an AWS EKS tier, usage type, or operation string to a canonical tier identifier.
func MapAWSTier(rawTier string) (domain.KubernetesTier, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	if canonical, ok := awsTierMap[key]; ok {
		return canonical, nil
	}

	// Try stripping regional short-code prefix (e.g. "use1-amazoneks-hours:percluster" -> "amazoneks-hours:percluster")
	if idx := strings.Index(key, "-"); idx != -1 {
		stripped := key[idx+1:]
		if canonical, ok := awsTierMap[stripped]; ok {
			return canonical, nil
		}
	}

	if strings.Contains(key, "provisionedtier") || strings.Contains(key, "provisionedcontrolplane") {
		return TierStandard, nil
	}

	return "", fmt.Errorf("%w: aws tier %q", ErrUnmappedTier, rawTier)
}
