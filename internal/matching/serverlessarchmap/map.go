package serverlessarchmap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// ErrUnmappedArchitecture is returned when a raw architecture value cannot be mapped to canonical taxonomy.
var ErrUnmappedArchitecture = errors.New("serverlessarchmap: unmapped architecture")

// SupportedCanonicalArchitectures returns the list of canonical architectures active in the current stage.
func SupportedCanonicalArchitectures() []string {
	return []string{
		domain.ArchitectureX86_64,
		domain.ArchitectureARM64,
	}
}

// IsSupportedStageArchitecture returns true if the architecture is active in the current stage.
func IsSupportedStageArchitecture(arch string) bool {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case domain.ArchitectureX86_64, domain.ArchitectureARM64:
		return true
	default:
		return false
	}
}

// ResolveCanonicalArchitecture maps a user-supplied architecture string to a canonical architecture.
// It checks canonical constants first, then tries provider-specific mappings across AWS, Azure, and GCP.
// If rawArch cannot be mapped, ErrUnmappedArchitecture is returned.
func ResolveCanonicalArchitecture(rawArch string) (string, error) {
	trimmed := strings.TrimSpace(rawArch)
	if trimmed == "" {
		return "", nil
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case domain.ArchitectureX86_64, "x86", "x86-64", "amd64", "intel":
		return domain.ArchitectureX86_64, nil
	case domain.ArchitectureARM64, "arm", "graviton", "graviton2":
		return domain.ArchitectureARM64, nil
	}

	for _, prov := range []string{"aws", "azure", "gcp"} {
		if norm, err := NormalizeArchitecture(prov, trimmed); err == nil {
			return norm, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrUnmappedArchitecture, rawArch)
}

// NormalizeArchitecture resolves a provider-specific raw architecture name or meter description to a canonical architecture.
func NormalizeArchitecture(provider, rawArch string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSArchitecture(rawArch)
	case "azure":
		return MapAzureArchitecture(rawArch)
	case "gcp":
		return MapGCPArchitecture(rawArch)
	default:
		return "", fmt.Errorf("serverlessarchmap: unmapped provider %q", provider)
	}
}
