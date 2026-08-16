package regionmap

import (
	"fmt"
)

var gcpRegionMap = map[string]string{
	"us-east1":    "us-east",
	"us-east4":    "us-east",
	"us-central1": "us-central",
	"us-west1":    "us-west",
	"us-west2":    "us-west",
	"us-west3":    "us-west",
	"us-west4":    "us-west",
}

var gcpGroupToNativeMap = map[string]string{
	"us-east":     "us-east4",
	"us-east4":    "us-east4",
	"us-east1":    "us-east1",
	"us-central":  "us-central1",
	"us-central1": "us-central1",
	"us-west":     "us-west1",
	"us-west1":    "us-west1",
	"us-west2":    "us-west2",
	"us-west3":    "us-west3",
	"us-west4":    "us-west4",
}

// MapGCPRegion resolves a GCP region code to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapGCPRegion(region string) (string, error) {
	regionGroup, ok := gcpRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveGCPNativeRegion resolves a normalized region group to the primary GCP native region code.
func ResolveGCPNativeRegion(regionGroup string) (string, error) {
	native, ok := gcpGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}
