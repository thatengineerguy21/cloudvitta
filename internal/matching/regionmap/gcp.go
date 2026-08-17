package regionmap

import (
	"fmt"
)

var gcpRegionMap = map[string]string{
	// US East
	"us-east1": "us-east",
	"us-east4": "us-east",
	"us-east5": "us-east",

	// US Central
	"us-central1": "us-central",
	"us-south1":   "us-central",

	// US West
	"us-west1": "us-west",
	"us-west2": "us-west",
	"us-west3": "us-west",
	"us-west4": "us-west",

	// Canada
	"northamerica-northeast1": "ca-central",
	"northamerica-northeast2": "ca-central",

	// South America
	"southamerica-east1": "sa-east",
	"southamerica-west1": "sa-east",

	// Europe
	"europe-west1":      "eu-west",
	"europe-west2":      "eu-west",
	"europe-west3":      "eu-central",
	"europe-west4":      "eu-west",
	"europe-west6":      "eu-central",
	"europe-west8":      "eu-south",
	"europe-west9":      "eu-west",
	"europe-west10":     "eu-central",
	"europe-west12":     "eu-south",
	"europe-north1":     "eu-north",
	"europe-north2":     "eu-north",
	"europe-central2":   "eu-central",
	"europe-southwest1": "eu-south",

	// Asia Pacific
	"asia-east1":           "ap-east",
	"asia-east2":           "ap-east",
	"asia-northeast1":      "ap-northeast",
	"asia-northeast2":      "ap-northeast",
	"asia-northeast3":      "ap-northeast",
	"asia-south1":          "ap-south",
	"asia-south2":          "ap-south",
	"asia-southeast1":      "ap-southeast",
	"asia-southeast2":      "ap-southeast",
	"australia-southeast1": "ap-southeast",
	"australia-southeast2": "ap-southeast",

	// Middle East
	"me-west1":    "il-central",
	"me-central1": "me-central",
	"me-central2": "me-central",

	// Africa
	"africa-south1": "af-south",

	// Global / Multi-region / Dual-region
	"global":       "global",
	"us":           "us-east",
	"eu":           "eu-west",
	"asia":         "ap-east",
	"asia1":        "ap-east",
	"asia2":        "ap-southeast",
	"nam4":         "us-east",
	"nam5":         "us-east",
	"nam6":         "us-west",
	"nam7":         "us-west",
	"nam8":         "us-west",
	"nam9":         "us-west",
	"nam10":        "us-central",
	"nam11":        "us-east",
	"nam12":        "us-east",
	"nam13":        "us-east",
	"nam-eur-asia1": "global",
	"eur4":         "eu-central",
	"eur5":         "eu-west",
	"eur6":         "eu-west",
	"eur7":         "eu-central",
	"eur8":         "eu-central",
	"us-central2":  "us-central",
	"us-east7":     "us-east",
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
