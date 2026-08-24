package regionmap

import (
	"fmt"
)

var oracleRegionMap = map[string]string{
	// US East
	"us-ashburn-1":      "us-east",
	"US East (Ashburn)": "us-east",
	"us-ashburn":        "us-east",

	// US West
	"us-phoenix-1":       "us-west",
	"US West (Phoenix)":  "us-west",
	"us-phoenix":         "us-west",
	"us-sanjose-1":       "us-west",
	"US West (San Jose)": "us-west",

	// US Central
	"us-chicago-1":         "us-central",
	"US Midwest (Chicago)": "us-central",
	"us-chicago":           "us-central",

	// Canada
	"ca-toronto-1":                "ca-central",
	"Canada Southeast (Toronto)":  "ca-central",
	"ca-montreal-1":               "ca-central",
	"Canada Southeast (Montreal)": "ca-central",

	// Europe
	"eu-frankfurt-1":                    "eu-central",
	"Germany Central (Frankfurt)":       "eu-central",
	"eu-amsterdam-1":                    "eu-west",
	"Netherlands Northwest (Amsterdam)": "eu-west",
	"eu-milan-1":                        "eu-south",
	"Italy Northwest (Milan)":           "eu-south",
	"eu-madrid-1":                       "eu-south",
	"Spain Central (Madrid)":            "eu-south",
	"eu-paris-1":                        "eu-west",
	"France Central (Paris)":            "eu-west",
	"eu-marseille-1":                    "eu-west",
	"France South (Marseille)":          "eu-west",
	"eu-zurich-1":                       "eu-central",
	"Switzerland North (Zurich)":        "eu-central",
	"eu-stockholm-1":                    "eu-north",
	"Sweden Central (Stockholm)":        "eu-north",
	"uk-london-1":                       "uk-south",
	"UK South (London)":                 "uk-south",
	"uk-cardiff-1":                      "uk-south",
	"UK West (Newport)":                 "uk-south",

	// Asia Pacific
	"ap-tokyo-1":                      "ap-northeast",
	"Japan East (Tokyo)":              "ap-northeast",
	"ap-osaka-1":                      "ap-northeast",
	"Japan Central (Osaka)":           "ap-northeast",
	"ap-seoul-1":                      "ap-northeast",
	"South Korea Central (Seoul)":     "ap-northeast",
	"ap-chuncheon-1":                  "ap-northeast",
	"South Korea North (Chuncheon)":   "ap-northeast",
	"ap-singapore-1":                  "ap-southeast",
	"Singapore (Singapore)":           "ap-southeast",
	"ap-sydney-1":                     "ap-southeast",
	"Australia East (Sydney)":         "ap-southeast",
	"ap-melbourne-1":                  "ap-southeast",
	"Australia Southeast (Melbourne)": "ap-southeast",
	"ap-mumbai-1":                     "ap-south",
	"India West (Mumbai)":             "ap-south",
	"ap-hyderabad-1":                  "ap-south",
	"India South (Hyderabad)":         "ap-south",

	// South America
	"sa-saopaulo-1":              "sa-east",
	"Brazil East (Sao Paulo)":    "sa-east",
	"sa-vinhedo-1":               "sa-east",
	"Brazil Southeast (Vinhedo)": "sa-east",
	"sa-santiago-1":              "sa-east",
	"Chile Central (Santiago)":   "sa-east",
	"sa-bogota-1":                "sa-east",
	"Colombia Central (Bogota)":  "sa-east",
	"sa-valparaiso-1":            "sa-east",
	"Chile West (Valparaiso)":    "sa-east",

	// Middle East
	"me-dubai-1":                    "me-central",
	"UAE East (Dubai)":              "me-central",
	"me-abudhabi-1":                 "me-central",
	"UAE Central (Abu Dhabi)":       "me-central",
	"me-jeddah-1":                   "me-central",
	"Saudi Arabia West (Jeddah)":    "me-central",
	"me-riyadh-1":                   "me-central",
	"Saudi Arabia Central (Riyadh)": "me-central",

	// Israel & Africa
	"il-jerusalem-1":                      "il-central",
	"Israel Central (Jerusalem)":          "il-central",
	"af-johannesburg-1":                   "af-south",
	"South Africa Central (Johannesburg)": "af-south",

	// Global
	"global": "global",
	"Global": "global",
}

var oracleGroupToNativeMap = map[string]string{
	"us-east":      "us-ashburn-1",
	"us-ashburn-1": "us-ashburn-1",
	"us-west":      "us-phoenix-1",
	"us-phoenix-1": "us-phoenix-1",
	"us-central":   "us-chicago-1",
	"ca-central":   "ca-toronto-1",
	"eu-central":   "eu-frankfurt-1",
	"eu-west":      "eu-amsterdam-1",
	"eu-north":     "eu-stockholm-1",
	"eu-south":     "eu-milan-1",
	"uk-south":     "uk-london-1",
	"ap-northeast": "ap-tokyo-1",
	"ap-southeast": "ap-singapore-1",
	"ap-south":     "ap-mumbai-1",
	"sa-east":      "sa-saopaulo-1",
	"me-central":   "me-dubai-1",
	"il-central":   "il-jerusalem-1",
	"af-south":     "af-johannesburg-1",
	"global":       "global",
}

// MapOracleRegion resolves an Oracle OCI region identifier or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapOracleRegion(region string) (string, error) {
	regionGroup, ok := oracleRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveOracleNativeRegion resolves a normalized region group to the primary Oracle OCI native region identifier.
func ResolveOracleNativeRegion(regionGroup string) (string, error) {
	native, ok := oracleGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}

// KnownOracleRegions returns a copy of known Oracle region mapping pairs.
func KnownOracleRegions() map[string]string {
	m := make(map[string]string, len(oracleRegionMap))
	for k, v := range oracleRegionMap {
		m[k] = v
	}
	return m
}
