package regionmap

import (
	"fmt"
)

var azureRegionMap = map[string]string{
	"eastus":    "us-east",
	"US East":   "us-east",
	"eastus2":   "us-east",
	"US East 2": "us-east",
}

// MapAzureRegion resolves an Azure region code or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapAzureRegion(region string) (string, error) {
	regionGroup, ok := azureRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}
