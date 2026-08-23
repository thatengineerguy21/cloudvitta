package kubernetestieremap

import (
	"fmt"
	"strings"
)

var awsTierMap = map[string]string{
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

// MapAWSTier maps an AWS EKS tier or usage type string to a canonical tier identifier.
func MapAWSTier(rawTier string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawTier))
	canonical, ok := awsTierMap[key]
	if !ok {
		return "", fmt.Errorf("%w: aws tier %q", ErrUnmappedTier, rawTier)
	}
	return canonical, nil
}
