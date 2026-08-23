package kubernetestieremap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// Canonical Kubernetes cluster tier taxonomy constants.
const (
	TierFree            = domain.KubernetesTierFree
	TierStandard        = domain.KubernetesTierStandard
	TierExtendedSupport = domain.KubernetesTierExtendedSupport
)

// ErrUnmappedTier is returned when a raw tier value cannot be mapped to canonical taxonomy.
var ErrUnmappedTier = errors.New("kubernetestieremap: unmapped tier")

// SupportedCanonicalTiers returns the list of canonical Kubernetes tiers active in the current stage.
func SupportedCanonicalTiers() []domain.KubernetesTier {
	return []domain.KubernetesTier{
		TierFree,
		TierStandard,
		TierExtendedSupport,
	}
}

// IsSupportedStageTier returns true if the tier is active in the current stage.
func IsSupportedStageTier(tier domain.KubernetesTier) bool {
	switch domain.KubernetesTier(strings.ToLower(strings.TrimSpace(string(tier)))) {
	case TierFree, TierStandard, TierExtendedSupport:
		return true
	default:
		return false
	}
}

// ResolveCanonicalTier maps a user-supplied tier string to a canonical Kubernetes tier.
// It checks canonical constants first, then tries provider-specific mappings across AWS, Azure, and GCP.
// If rawTier cannot be mapped, ErrUnmappedTier is returned.
func ResolveCanonicalTier(rawTier string) (domain.KubernetesTier, error) {
	trimmed := strings.TrimSpace(rawTier)
	if trimmed == "" {
		return "", nil
	}

	lower := domain.KubernetesTier(strings.ToLower(trimmed))
	switch lower {
	case TierFree, TierStandard, TierExtendedSupport:
		return lower, nil
	}

	for _, prov := range []string{"aws", "azure", "gcp"} {
		if norm, err := NormalizeTier(prov, trimmed); err == nil {
			return norm, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrUnmappedTier, rawTier)
}

// NormalizeTier resolves a provider-specific raw tier name to a canonical Kubernetes tier.
func NormalizeTier(provider, rawTier string) (domain.KubernetesTier, error) {
	switch provider {
	case "aws":
		return MapAWSTier(rawTier)
	case "azure":
		return MapAzureTier(rawTier)
	case "gcp":
		return MapGCPTier(rawTier)
	default:
		return "", fmt.Errorf("kubernetestieremap: unmapped provider %q", provider)
	}
}
