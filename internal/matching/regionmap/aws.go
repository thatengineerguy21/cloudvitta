package regionmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedRegion is returned when an AWS region is not found in the region map.
var ErrUnmappedRegion = errors.New("regionmap: unmapped region")

var awsRegionMap = map[string]string{
	// US East
	"us-east-1":             "us-east",
	"US East (N. Virginia)": "us-east",
	"us-east-2":             "us-east",
	"US East (Ohio)":        "us-east",

	// US West
	"us-west-1":               "us-west",
	"US West (N. California)": "us-west",
	"us-west-2":               "us-west",
	"US West (Oregon)":        "us-west",

	// Canada
	"ca-central-1":          "ca-central",
	"Canada (Central)":      "ca-central",
	"ca-west-1":             "ca-central",
	"Canada West (Calgary)": "ca-central",

	// Europe
	"eu-west-1":          "eu-west",
	"Europe (Ireland)":   "eu-west",
	"eu-west-2":          "eu-west",
	"Europe (London)":    "eu-west",
	"eu-west-3":          "eu-west",
	"Europe (Paris)":     "eu-west",
	"eu-central-1":       "eu-central",
	"Europe (Frankfurt)": "eu-central",
	"eu-central-2":       "eu-central",
	"Europe (Zurich)":    "eu-central",
	"eu-north-1":         "eu-north",
	"Europe (Stockholm)": "eu-north",
	"eu-south-1":         "eu-south",
	"Europe (Milan)":     "eu-south",
	"eu-south-2":         "eu-south",
	"Europe (Spain)":     "eu-south",

	// Asia Pacific
	"ap-east-1":                "ap-east",
	"Asia Pacific (Hong Kong)": "ap-east",
	"ap-south-1":               "ap-south",
	"Asia Pacific (Mumbai)":    "ap-south",
	"ap-south-2":               "ap-south",
	"Asia Pacific (Hyderabad)": "ap-south",
	"ap-southeast-1":           "ap-southeast",
	"Asia Pacific (Singapore)": "ap-southeast",
	"ap-southeast-2":           "ap-southeast",
	"Asia Pacific (Sydney)":    "ap-southeast",
	"ap-southeast-3":           "ap-southeast",
	"Asia Pacific (Jakarta)":   "ap-southeast",
	"ap-southeast-4":           "ap-southeast",
	"Asia Pacific (Melbourne)": "ap-southeast",
	"ap-southeast-5":           "ap-southeast",
	"Asia Pacific (Malaysia)":  "ap-southeast",
	"ap-southeast-7":           "ap-southeast",
	"Asia Pacific (Thailand)":  "ap-southeast",
	"ap-northeast-1":           "ap-northeast",
	"Asia Pacific (Tokyo)":     "ap-northeast",
	"ap-northeast-2":           "ap-northeast",
	"Asia Pacific (Seoul)":     "ap-northeast",
	"ap-northeast-3":           "ap-northeast",
	"Asia Pacific (Osaka)":     "ap-northeast",

	// South America
	"sa-east-1":                 "sa-east",
	"South America (Sao Paulo)": "sa-east",

	// Middle East
	"me-south-1":            "me-south",
	"Middle East (Bahrain)": "me-south",
	"me-central-1":          "me-central",
	"Middle East (UAE)":     "me-central",
	"il-central-1":          "il-central",
	"Israel (Tel Aviv)":     "il-central",

	// Africa
	"af-south-1":        "af-south",
	"Africa (Cape Town)": "af-south",
	"Nigeria (Lagos)":    "af-south",

	// GovCloud
	"us-gov-west-1":           "us-west",
	"AWS GovCloud (US-West)":  "us-west",
	"us-gov-east-1":           "us-east",
	"AWS GovCloud (US-East)":  "us-east",

	// Global / Any
	"Global":   "global",
	"global":   "global",
	"Any":      "global",
	"External": "global",
}

var awsGroupToNativeMap = map[string]string{
	"us-east":               "us-east-1",
	"us-east-1":             "us-east-1",
	"US East (N. Virginia)": "us-east-1",
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
