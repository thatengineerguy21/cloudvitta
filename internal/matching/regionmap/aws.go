package regionmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedRegion is returned when an AWS region is not found in the region map.
var ErrUnmappedRegion = errors.New("regionmap: unmapped region")

var awsRegionMap = map[string]string{
	// US East
	"us-east-1":                         "us-east",
	"US East (N. Virginia)":             "us-east",
	"US East (New York City)":           "us-east",
	"US East (New York)":                "us-east",
	"US East (Boston)":                  "us-east",
	"US East (Philadelphia)":            "us-east",
	"US East (Atlanta)":                 "us-east",
	"US East (Miami)":                   "us-east",
	"US East (Chicago)":                 "us-east",
	"US East (Dallas)":                  "us-central",
	"US East (Houston)":                 "us-central",
	"US East (Kansas City)":             "us-central",
	"US East (Kansas City 2)":           "us-central",
	"US East (Lenexa)":                  "us-central",
	"US East (Verizon) - Atlanta":       "us-east",
	"US East (Verizon) - Boston":        "us-east",
	"US East (Verizon) - Charlotte":     "us-east",
	"US East (Verizon) - Chicago":       "us-east",
	"US East (Verizon) - Dallas":        "us-central",
	"US East (Verizon) - Detroit":       "us-east",
	"US East (Verizon) - Houston":       "us-central",
	"US East (Verizon) - Miami":         "us-east",
	"US East (Verizon) - Minneapolis":   "us-central",
	"US East (Verizon) - Nashville":     "us-east",
	"US East (Verizon) - New York":      "us-east",
	"US East (Verizon) - Tampa":         "us-east",
	"US East (Verizon) - Washington DC": "us-east",
	"us-east-2":                         "us-east",
	"US East (Ohio)":                    "us-east",

	// US West
	"us-west-1":                                  "us-west",
	"US West (N. California)":                    "us-west",
	"US West (Los Angeles)":                      "us-west",
	"US West (San Francisco)":                    "us-west",
	"US West (Seattle)":                          "us-west",
	"US West (Denver)":                           "us-west",
	"US West (Phoenix)":                          "us-west",
	"US West (Las Vegas)":                        "us-west",
	"US West (Portland)":                         "us-west",
	"US West (Honolulu)":                         "us-west",
	"us-west-2":                                  "us-west",
	"US West (Oregon)":                           "us-west",
	"US West (Verizon) - Denver":                 "us-west",
	"US West (Verizon) - Las Vegas":              "us-west",
	"US West (Verizon) - Los Angeles":            "us-west",
	"US West (Verizon) - Phoenix":                "us-west",
	"US West (Verizon) - San Francisco":          "us-west",
	"US West (Verizon) - San Francisco Bay Area": "us-west",
	"US West (Verizon) - Seattle":                "us-west",

	// Canada
	"ca-central-1":            "ca-central",
	"Canada (Central)":        "ca-central",
	"Canada (Toronto)":        "ca-central",
	"Canada (BELL) - Toronto": "ca-central",
	"Bell Canada - Toronto":   "ca-central",
	"ca-west-1":               "ca-central",
	"Canada West (Calgary)":   "ca-central",

	// Mexico
	"mx-central-1":       "us-central",
	"Mexico (Central)":   "us-central",
	"Mexico (Queretaro)": "us-central",

	// Europe
	"eu-west-1":                             "eu-west",
	"Europe (Ireland)":                      "eu-west",
	"EU (Ireland)":                          "eu-west",
	"eu-west-2":                             "eu-west",
	"Europe (London)":                       "eu-west",
	"EU (London)":                           "eu-west",
	"Vodafone - London":                     "eu-west",
	"Europe (Vodafone) - London":            "eu-west",
	"Vodafone - Manchester":                 "eu-west",
	"Europe (Vodafone) - Manchester":        "eu-west",
	"Europe (British Telecom) - Manchester": "eu-west",
	"eu-west-3":                             "eu-west",
	"Europe (Paris)":                        "eu-west",
	"EU (Paris)":                            "eu-west",
	"eu-central-1":                          "eu-central",
	"Europe (Frankfurt)":                    "eu-central",
	"EU (Frankfurt)":                        "eu-central",
	"Europe (Munich)":                       "eu-central",
	"Europe (Berlin)":                       "eu-central",
	"Germany (Hamburg)":                     "eu-central",
	"Poland (Warsaw)":                       "eu-central",
	"Vodafone - Munich":                     "eu-central",
	"Europe (Vodafone) - Munich":            "eu-central",
	"Vodafone - Berlin":                     "eu-central",
	"Europe (Vodafone) - Berlin":            "eu-central",
	"Vodafone - Dortmund":                   "eu-central",
	"Europe (Vodafone) - Dortmund":          "eu-central",
	"eu-central-2":                          "eu-central",
	"Europe (Zurich)":                       "eu-central",
	"EU (Zurich)":                           "eu-central",
	"eu-north-1":                            "eu-north",
	"Europe (Stockholm)":                    "eu-north",
	"EU (Stockholm)":                        "eu-north",
	"Europe (Copenhagen)":                   "eu-north",
	"Denmark (Copenhagen)":                  "eu-north",
	"Europe (Helsinki)":                     "eu-north",
	"Finland (Helsinki)":                    "eu-north",
	"eu-south-1":                            "eu-south",
	"Europe (Milan)":                        "eu-south",
	"EU (Milan)":                            "eu-south",
	"eu-south-2":                            "eu-south",
	"Europe (Spain)":                        "eu-south",
	"EU (Spain)":                            "eu-south",
	"Greece (Athens)":                       "eu-south",
	"Turkey (Istanbul)":                     "eu-south",

	// Asia Pacific
	"ap-east-1":                    "ap-east",
	"ap-east-2":                    "ap-east",
	"Asia Pacific (Hong Kong)":     "ap-east",
	"Asia Pacific (Taipei)":        "ap-east",
	"Taiwan (Taipei)":              "ap-east",
	"ap-south-1":                   "ap-south",
	"Asia Pacific (Mumbai)":        "ap-south",
	"Asia Pacific (Kolkata)":       "ap-south",
	"Asia Pacific (Delhi)":         "ap-south",
	"India (Delhi)":                "ap-south",
	"India (Kolkata)":              "ap-south",
	"ap-south-2":                   "ap-south",
	"Asia Pacific (Hyderabad)":     "ap-south",
	"ap-southeast-1":               "ap-southeast",
	"Asia Pacific (Singapore)":     "ap-southeast",
	"ap-southeast-2":               "ap-southeast",
	"Asia Pacific (Sydney)":        "ap-southeast",
	"Asia Pacific (Perth)":         "ap-southeast",
	"Asia Pacific (Brisbane)":      "ap-southeast",
	"Australia (Perth)":            "ap-southeast",
	"Australia (Brisbane)":         "ap-southeast",
	"ap-southeast-3":               "ap-southeast",
	"Asia Pacific (Jakarta)":       "ap-southeast",
	"Indonesia (Jakarta)":          "ap-southeast",
	"ap-southeast-4":               "ap-southeast",
	"Asia Pacific (Melbourne)":     "ap-southeast",
	"ap-southeast-5":               "ap-southeast",
	"Asia Pacific (Malaysia)":      "ap-southeast",
	"Malaysia (Kuala Lumpur)":      "ap-southeast",
	"ap-southeast-6":               "ap-southeast",
	"ap-southeast-7":               "ap-southeast",
	"Asia Pacific (Thailand)":      "ap-southeast",
	"Asia Pacific (Bangkok)":       "ap-southeast",
	"Thailand (Bangkok)":           "ap-southeast",
	"Asia Pacific (Philippines)":   "ap-southeast",
	"Philippines (Manila)":         "ap-southeast",
	"Vietnam (Hanoi)":              "ap-southeast",
	"Asia Pacific (New Zealand)":   "ap-southeast",
	"Asia Pacific (Auckland)":      "ap-southeast",
	"New Zealand (Auckland)":       "ap-southeast",
	"ap-northeast-1":               "ap-northeast",
	"Asia Pacific (Tokyo)":         "ap-northeast",
	"Asia Pacific (KDDI) - Tokyo":  "ap-northeast",
	"Asia Pacific (KDDI) - Osaka":  "ap-northeast",
	"KDDI - Tokyo":                 "ap-northeast",
	"KDDI - Osaka":                 "ap-northeast",
	"ap-northeast-2":               "ap-northeast",
	"Asia Pacific (Seoul)":         "ap-northeast",
	"Asia Pacific (SKT) - Seoul":   "ap-northeast",
	"Asia Pacific (SKT) - Daejeon": "ap-northeast",
	"SKT - Daejeon":                "ap-northeast",
	"SKT - Seoul":                  "ap-northeast",
	"ap-northeast-3":               "ap-northeast",
	"Asia Pacific (Osaka)":         "ap-northeast",
	"Asia Pacific (Osaka-Local)":   "ap-northeast",

	// China
	"cn-north-1":      "ap-northeast",
	"China (Beijing)": "ap-northeast",
	"cn-northwest-1":  "ap-northeast",
	"China (Ningxia)": "ap-northeast",

	// South America
	"sa-east-1":                    "sa-east",
	"South America (Sao Paulo)":    "sa-east",
	"South America (Buenos Aires)": "sa-east",
	"South America (Santiago)":     "sa-east",
	"Argentina (Buenos Aires)":     "sa-east",
	"Chile (Santiago)":             "sa-east",
	"Peru (Lima)":                  "sa-east",

	// Middle East
	"me-south-1":            "me-south",
	"Middle East (Bahrain)": "me-south",
	"me-central-1":          "me-central",
	"Middle East (UAE)":     "me-central",
	"Middle East (Muscat)":  "me-central",
	"Oman (Muscat)":         "me-central",
	"il-central-1":          "il-central",
	"Israel (Tel Aviv)":     "il-central",

	// Africa
	"af-south-1":           "af-south",
	"Africa (Cape Town)":   "af-south",
	"Nigeria (Lagos)":      "af-south",
	"Morocco (Casablanca)": "af-south",
	"Senegal (Dakar)":      "af-south",

	// GovCloud & Secret Regions
	"us-gov-west-1":          "us-west",
	"AWS GovCloud (US-West)": "us-west",
	"AWS GovCloud (US)":      "us-west",
	"us-gov-east-1":          "us-east",
	"AWS GovCloud (US-East)": "us-east",
	"us-iso-east-1":          "us-east",
	"us-isob-east-1":         "us-east",
	"us-iso-west-1":          "us-west",
	"eu-isoe-west-1":         "eu-west",

	// Location Aliases
	"Spain (Madrid)":        "eu-south",
	"Switzerland (Zurich)":  "eu-central",
	"India (Hyderabad)":     "ap-south",
	"Australia (Melbourne)": "ap-southeast",
	"UAE (Dubai)":           "me-central",
	"United Arab Emirates":  "me-central",
	"Tel Aviv":              "il-central",

	// Global / Any
	"Global":            "global",
	"global":            "global",
	"Any":               "global",
	"External":          "global",
	"Amazon CloudFront": "global",
}

var awsGroupToNativeMap = map[string]string{
	"us-east":               "us-east-1",
	"us-east-1":             "us-east-1",
	"US East (N. Virginia)": "us-east-1",
	"us-central":            "us-east-2",
	"us-west":               "us-west-2",
	"us-west-2":             "us-west-2",
	"ca-central":            "ca-central-1",
	"eu-west":               "eu-west-1",
	"eu-west-2":             "eu-west-2",
	"eu-central":            "eu-central-1",
	"eu-central-1":          "eu-central-1",
	"eu-north":              "eu-north-1",
	"eu-south":              "eu-south-1",
	"ap-east":               "ap-east-1",
	"ap-south":              "ap-south-1",
	"ap-south-1":            "ap-south-1",
	"ap-southeast":          "ap-southeast-1",
	"ap-southeast-1":        "ap-southeast-1",
	"ap-southeast-2":        "ap-southeast-2",
	"ap-northeast":          "ap-northeast-1",
	"ap-northeast-1":        "ap-northeast-1",
	"sa-east":               "sa-east-1",
	"me-central":            "me-central-1",
	"me-south":              "me-south-1",
	"il-central":            "il-central-1",
	"af-south":              "af-south-1",
	"global":                "global",
}

// MapAWSRegion resolves an AWS region code or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapAWSRegion(region string) (string, error) {
	regionGroup, ok := awsRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveAWSNativeRegion resolves a normalized region group to the primary AWS native region code.
func ResolveAWSNativeRegion(regionGroup string) (string, error) {
	native, ok := awsGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}

// KnownAWSRegions returns a copy of known AWS region mapping pairs.
func KnownAWSRegions() map[string]string {
	m := make(map[string]string, len(awsRegionMap))
	for k, v := range awsRegionMap {
		m[k] = v
	}
	return m
}
