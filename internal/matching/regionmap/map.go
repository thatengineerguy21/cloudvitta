package regionmap

import "fmt"

// MapRegion resolves a region code to a normalized region group for a given provider.
// It fails loudly with ErrUnmappedRegion if the provider or region is not mapped.
func MapRegion(provider, region string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSRegion(region)
	default:
		return "", fmt.Errorf("regionmap: unmapped provider %q", provider)
	}
}
