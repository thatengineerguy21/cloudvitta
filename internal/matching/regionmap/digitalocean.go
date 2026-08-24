package regionmap

import (
	"fmt"
)

var digitalOceanRegionMap = map[string]string{
	// US East (New York)
	"nyc1":       "us-east",
	"nyc2":       "us-east",
	"nyc3":       "us-east",
	"New York 1": "us-east",
	"New York 2": "us-east",
	"New York 3": "us-east",

	// US West (San Francisco)
	"sfo1":            "us-west",
	"sfo2":            "us-west",
	"sfo3":            "us-west",
	"San Francisco 1": "us-west",
	"San Francisco 2": "us-west",
	"San Francisco 3": "us-west",

	// Europe Central (Frankfurt)
	"fra1":        "eu-central",
	"Frankfurt 1": "eu-central",

	// Europe West (Amsterdam)
	"ams2":        "eu-west",
	"ams3":        "eu-west",
	"Amsterdam 2": "eu-west",
	"Amsterdam 3": "eu-west",

	// United Kingdom (London)
	"lon1":     "uk-south",
	"London 1": "uk-south",

	// Asia Southeast (Singapore)
	"sgp1":        "ap-southeast",
	"Singapore 1": "ap-southeast",

	// Asia Southeast (Sydney)
	"syd1":     "ap-southeast",
	"Sydney 1": "ap-southeast",

	// Asia South (Bangalore)
	"blr1":        "ap-south",
	"Bangalore 1": "ap-south",

	// Canada Central (Toronto)
	"tor1":      "ca-central",
	"Toronto 1": "ca-central",

	// Global
	"global": "global",
	"Global": "global",
}

var digitalOceanGroupToNativeMap = map[string]string{
	"us-east":      "nyc3",
	"us-west":      "sfo3",
	"eu-central":   "fra1",
	"eu-west":      "ams3",
	"uk-south":     "lon1",
	"ap-southeast": "sgp1",
	"ap-south":     "blr1",
	"ca-central":   "tor1",
	"global":       "global",
}

// MapDigitalOceanRegion resolves a DigitalOcean region identifier or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapDigitalOceanRegion(region string) (string, error) {
	regionGroup, ok := digitalOceanRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveDigitalOceanNativeRegion resolves a normalized region group to the primary DigitalOcean native region identifier.
func ResolveDigitalOceanNativeRegion(regionGroup string) (string, error) {
	native, ok := digitalOceanGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}

// KnownDigitalOceanRegions returns a copy of known DigitalOcean region mapping pairs.
func KnownDigitalOceanRegions() map[string]string {
	m := make(map[string]string, len(digitalOceanRegionMap))
	for k, v := range digitalOceanRegionMap {
		m[k] = v
	}
	return m
}
