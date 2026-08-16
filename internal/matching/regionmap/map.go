package regionmap

import "fmt"

// MapRegion resolves a region code to a normalized region group for a given provider.
// It fails loudly with ErrUnmappedRegion if the provider or region is not mapped.
func MapRegion(provider, region string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSRegion(region)
	case "azure":
		return MapAzureRegion(region)
	case "gcp":
		return MapGCPRegion(region)
	default:
		return "", fmt.Errorf("regionmap: unmapped provider %q", provider)
	}
}

// ResolveNativeRegion resolves a normalized region group (e.g. "us-east") to the primary native provider region.
// If the input is already a valid native region or group, it resolves to the canonical ingested native region.
func ResolveNativeRegion(provider, regionGroup string) (string, error) {
	switch provider {
	case "aws":
		return ResolveAWSNativeRegion(regionGroup)
	case "azure":
		return ResolveAzureNativeRegion(regionGroup)
	case "gcp":
		return ResolveGCPNativeRegion(regionGroup)
	default:
		return "", fmt.Errorf("regionmap: unmapped provider %q", provider)
	}
}
