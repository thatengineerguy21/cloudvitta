package regionmap

import (
	"fmt"
)

var gcpRegionMap = map[string]string{
	// US East
	"us-east1": "us-east",
	"us-east2": "us-east",
	"us-east3": "us-east",
	"us-east4": "us-east",
	"us-east5": "us-east",
	"us-east6": "us-east",
	"us-east7": "us-east",

	// US Central
	"us-central1": "us-central",
	"us-central2": "us-central",
	"us-central3": "us-central",
	"us-south1":   "us-central",

	// US West
	"us-west1":  "us-west",
	"us-west2":  "us-west",
	"us-west3":  "us-west",
	"us-west4":  "us-west",
	"us-west5":  "us-west",
	"us-west6":  "us-west",
	"us-west7":  "us-west",
	"us-west8":  "us-west",
	"us-west9":  "us-west",
	"us-west10": "us-west",

	// Canada
	"northamerica-northeast1": "ca-central",
	"northamerica-northeast2": "ca-central",
	"northamerica-central1":   "ca-central",
	"northamerica":            "ca-central",

	// Mexico
	"northamerica-south1": "us-central",

	// South America
	"southamerica-east1":    "sa-east",
	"southamerica-west1":    "sa-east",
	"southamerica-central1": "sa-east",
	"southamerica-south1":   "sa-east",
	"southamerica":          "sa-east",

	// Europe
	"europe-west1":      "eu-west",
	"europe-west2":      "eu-west",
	"europe-west3":      "eu-central",
	"europe-west4":      "eu-west",
	"europe-west5":      "eu-central",
	"europe-west6":      "eu-central",
	"europe-west7":      "eu-west",
	"europe-west8":      "eu-south",
	"europe-west9":      "eu-west",
	"europe-west10":     "eu-central",
	"europe-west11":     "eu-central",
	"europe-west12":     "eu-south",
	"europe-north1":     "eu-north",
	"europe-north2":     "eu-north",
	"europe-north3":     "eu-north",
	"europe-central1":   "eu-central",
	"europe-central2":   "eu-central",
	"europe-south1":     "eu-south",
	"europe-south2":     "eu-south",
	"europe-southwest1": "eu-south",

	// Asia Pacific & Australia
	"asia-east1":           "ap-east",
	"asia-east2":           "ap-east",
	"asia-east3":           "ap-east",
	"asia-northeast1":      "ap-northeast",
	"asia-northeast2":      "ap-northeast",
	"asia-northeast3":      "ap-northeast",
	"asia-south1":          "ap-south",
	"asia-south2":          "ap-south",
	"asia-south3":          "ap-south",
	"asia-southeast1":      "ap-southeast",
	"asia-southeast2":      "ap-southeast",
	"asia-southeast3":      "ap-southeast",
	"australia-southeast1": "ap-southeast",
	"australia-southeast2": "ap-southeast",
	"australia-central1":   "ap-southeast",
	"australia-central2":   "ap-southeast",
	"australia-east1":      "ap-southeast",
	"australia":            "ap-southeast",
	"oceania":              "ap-southeast",
	"apac":                 "ap-east",

	// Middle East
	"me-west1":    "il-central",
	"me-west2":    "il-central",
	"me-central1": "me-central",
	"me-central2": "me-central",
	"me-central3": "me-central",
	"me-south1":   "me-central",
	"middleeast":  "me-central",

	// Africa
	"africa-south1": "af-south",
	"africa-south2": "af-south",
	"africa-north1": "af-south",
	"africa":        "af-south",

	// Global / Multi-region / Dual-region
	"global":        "global",
	"us":            "us-east",
	"eu":            "eu-west",
	"europe":        "eu-west",
	"eur":           "eu-west",
	"nam":           "us-east",
	"asia":          "ap-east",
	"asia1":         "ap-east",
	"asia2":         "ap-southeast",
	"asia3":         "ap-northeast",
	"asia4":         "ap-southeast",
	"nam1":          "us-east",
	"nam2":          "us-east",
	"nam3":          "us-east",
	"nam4":          "us-east",
	"nam5":          "us-east",
	"nam6":          "us-west",
	"nam7":          "us-west",
	"nam8":          "us-west",
	"nam9":          "us-west",
	"nam10":         "us-central",
	"nam11":         "us-east",
	"nam12":         "us-east",
	"nam13":         "us-east",
	"nam14":         "us-east",
	"nam-eur-asia1": "global",
	"eur1":          "eu-west",
	"eur2":          "eu-west",
	"eur3":          "eu-central",
	"eur4":          "eu-central",
	"eur5":          "eu-west",
	"eur6":          "eu-west",
	"eur7":          "eu-central",
	"eur8":          "eu-central",
	"eur9":          "eu-central",
	"eur10":         "eu-central",
}

var gcpGroupToNativeMap = map[string]string{
	"us-east":      "us-east4",
	"us-east4":     "us-east4",
	"us-east1":     "us-east1",
	"us-central":   "us-central1",
	"us-central1":  "us-central1",
	"us-west":      "us-west1",
	"us-west1":     "us-west1",
	"us-west2":     "us-west2",
	"us-west3":     "us-west3",
	"us-west4":     "us-west4",
	"ca-central":   "northamerica-northeast1",
	"eu-west":      "europe-west1",
	"eu-central":   "europe-west3",
	"eu-north":     "europe-north1",
	"eu-south":     "europe-west8",
	"ap-east":      "asia-east1",
	"ap-south":     "asia-south1",
	"ap-southeast": "asia-southeast1",
	"ap-northeast": "asia-northeast1",
	"sa-east":      "southamerica-east1",
	"me-central":   "me-central1",
	"il-central":   "me-west1",
	"af-south":     "africa-south1",
	"global":       "global",
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

// KnownGCPRegions returns a copy of known GCP region mapping pairs.
func KnownGCPRegions() map[string]string {
	m := make(map[string]string, len(gcpRegionMap))
	for k, v := range gcpRegionMap {
		m[k] = v
	}
	return m
}
