package regionmap

import (
	"fmt"
)

var alibabaRegionMap = map[string]string{
	// US East (Virginia)
	"us-east-1":     "us-east",
	"US (Virginia)": "us-east",

	// US West (Silicon Valley)
	"us-west-1":           "us-west",
	"US (Silicon Valley)": "us-west",

	// Europe Central (Frankfurt)
	"eu-central-1":        "eu-central",
	"Germany (Frankfurt)": "eu-central",

	// Europe West (London / UK)
	"eu-west-1":   "uk-south",
	"UK (London)": "uk-south",

	// Asia Southeast (Singapore)
	"ap-southeast-1": "ap-southeast",
	"Singapore":      "ap-southeast",

	// Asia Southeast (Sydney)
	"ap-southeast-2":     "ap-southeast",
	"Australia (Sydney)": "ap-southeast",

	// Asia Southeast (Kuala Lumpur)
	"ap-southeast-3":          "ap-southeast",
	"Malaysia (Kuala Lumpur)": "ap-southeast",

	// Asia Southeast (Jakarta)
	"ap-southeast-5":      "ap-southeast",
	"Indonesia (Jakarta)": "ap-southeast",

	// Asia Northeast (Tokyo)
	"ap-northeast-1": "ap-northeast",
	"Japan (Tokyo)":  "ap-northeast",

	// Asia Northeast (Seoul)
	"ap-northeast-2":      "ap-northeast",
	"South Korea (Seoul)": "ap-northeast",

	// Asia South (Mumbai)
	"ap-south-1":     "ap-south",
	"India (Mumbai)": "ap-south",

	// Middle East (Dubai)
	"me-east-1":   "me-central",
	"UAE (Dubai)": "me-central",

	// Middle East (Riyadh)
	"me-central-1":          "me-central",
	"Saudi Arabia (Riyadh)": "me-central",

	// China East (Hangzhou)
	"cn-hangzhou":      "cn-east",
	"China (Hangzhou)": "cn-east",

	// China East (Shanghai)
	"cn-shanghai":      "cn-east",
	"China (Shanghai)": "cn-east",

	// China North (Beijing)
	"cn-beijing":      "cn-north",
	"China (Beijing)": "cn-north",

	// China South (Shenzhen)
	"cn-shenzhen":      "cn-south",
	"China (Shenzhen)": "cn-south",

	// China (Hong Kong) / Asia East
	"cn-hongkong":       "ap-east",
	"China (Hong Kong)": "ap-east",

	// Global
	"global": "global",
	"Global": "global",
}

var alibabaGroupToNativeMap = map[string]string{
	"us-east":      "us-east-1",
	"us-west":      "us-west-1",
	"eu-central":   "eu-central-1",
	"uk-south":     "eu-west-1",
	"ap-southeast": "ap-southeast-1",
	"ap-northeast": "ap-northeast-1",
	"ap-south":     "ap-south-1",
	"me-central":   "me-east-1",
	"cn-east":      "cn-hangzhou",
	"cn-north":     "cn-beijing",
	"cn-south":     "cn-shenzhen",
	"ap-east":      "cn-hongkong",
	"global":       "global",
}

// MapAlibabaRegion resolves an Alibaba Cloud region identifier or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapAlibabaRegion(region string) (string, error) {
	regionGroup, ok := alibabaRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveAlibabaNativeRegion resolves a normalized region group to the primary Alibaba Cloud native region identifier.
func ResolveAlibabaNativeRegion(regionGroup string) (string, error) {
	native, ok := alibabaGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}

// KnownAlibabaRegions returns a copy of known Alibaba Cloud region mapping pairs.
func KnownAlibabaRegions() map[string]string {
	m := make(map[string]string, len(alibabaRegionMap))
	for k, v := range alibabaRegionMap {
		m[k] = v
	}
	return m
}
