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

var azureGroupToNativeMap = map[string]string{
	"us-east":   "eastus",
	"eastus":    "eastus",
	"US East":   "eastus",
	"eastus2":   "eastus2",
	"US East 2": "eastus2",
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

// ResolveAzureNativeRegion resolves a normalized region group to the primary Azure native region code.
func ResolveAzureNativeRegion(regionGroup string) (string, error) {
	native, ok := azureGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}
