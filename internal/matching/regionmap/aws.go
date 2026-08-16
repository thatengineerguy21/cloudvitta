package regionmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedRegion is returned when an AWS region is not found in the region map.
var ErrUnmappedRegion = errors.New("regionmap: unmapped region")

var awsRegionMap = map[string]string{
	"us-east-1":             "us-east",
	"US East (N. Virginia)": "us-east",
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
