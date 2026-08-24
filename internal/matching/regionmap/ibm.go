package regionmap

import (
	"fmt"
)

var ibmRegionMap = map[string]string{
	// US East (Washington DC)
	"us-east":                 "us-east",
	"wdc":                     "us-east",
	"washington":              "us-east",
	"US East (Washington DC)": "us-east",

	// US Central / South (Dallas)
	"us-south":          "us-central",
	"dal":               "us-central",
	"dallas":            "us-central",
	"US South (Dallas)": "us-central",

	// Canada (Toronto)
	"ca-tor":           "ca-central",
	"tor":              "ca-central",
	"toronto":          "ca-central",
	"Canada (Toronto)": "ca-central",

	// Europe Central (Frankfurt)
	"eu-de":               "eu-central",
	"fra":                 "eu-central",
	"frankfurt":           "eu-central",
	"Germany (Frankfurt)": "eu-central",

	// Europe West (London / UK)
	"eu-gb":                   "uk-south",
	"lon":                     "uk-south",
	"london":                  "uk-south",
	"United Kingdom (London)": "uk-south",

	// Europe South (Madrid)
	"eu-es":          "eu-south",
	"mad":            "eu-south",
	"madrid":         "eu-south",
	"Spain (Madrid)": "eu-south",

	// Asia Northeast (Tokyo)
	"jp-tok":        "ap-northeast",
	"tok":           "ap-northeast",
	"tokyo":         "ap-northeast",
	"Japan (Tokyo)": "ap-northeast",

	// Asia Northeast (Osaka)
	"jp-osa":        "ap-northeast",
	"osa":           "ap-northeast",
	"osaka":         "ap-northeast",
	"Japan (Osaka)": "ap-northeast",

	// Asia Southeast (Sydney)
	"au-syd":             "ap-southeast",
	"syd":                "ap-southeast",
	"sydney":             "ap-southeast",
	"Australia (Sydney)": "ap-southeast",

	// Asia South (Chennai)
	"in-che":          "ap-south",
	"che":             "ap-south",
	"chennai":         "ap-south",
	"India (Chennai)": "ap-south",

	// South America East (Sao Paulo)
	"br-sao":             "sa-east",
	"sao":                "sa-east",
	"saopaulo":           "sa-east",
	"sao-paulo":          "sa-east",
	"Brazil (Sao Paulo)": "sa-east",

	// Global
	"global": "global",
	"Global": "global",
}

var ibmGroupToNativeMap = map[string]string{
	"us-east":      "us-east",
	"us-central":   "us-south",
	"ca-central":   "ca-tor",
	"eu-central":   "eu-de",
	"uk-south":     "eu-gb",
	"eu-south":     "eu-es",
	"ap-northeast": "jp-tok",
	"ap-southeast": "au-syd",
	"ap-south":     "in-che",
	"sa-east":      "br-sao",
	"global":       "global",
}

// MapIBMRegion resolves an IBM Cloud region identifier or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapIBMRegion(region string) (string, error) {
	regionGroup, ok := ibmRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveIBMNativeRegion resolves a normalized region group to the primary IBM Cloud native region identifier.
func ResolveIBMNativeRegion(regionGroup string) (string, error) {
	native, ok := ibmGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}

// KnownIBMRegions returns a copy of known IBM Cloud region mapping pairs.
func KnownIBMRegions() map[string]string {
	m := make(map[string]string, len(ibmRegionMap))
	for k, v := range ibmRegionMap {
		m[k] = v
	}
	return m
}
