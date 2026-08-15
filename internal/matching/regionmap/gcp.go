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

// MapGCPRegion resolves a GCP region code to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapGCPRegion(region string) (string, error) {
	regionGroup, ok := gcpRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}
