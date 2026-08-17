package regionmap

import (
	"fmt"
)

var azureRegionMap = map[string]string{
	// US East
	"eastus":    "us-east",
	"US East":   "us-east",
	"eastus2":   "us-east",
	"US East 2": "us-east",

	// US West
	"westus":    "us-west",
	"US West":   "us-west",
	"westus2":   "us-west",
	"US West 2": "us-west",
	"westus3":   "us-west",
	"US West 3": "us-west",

	// US Central
	"centralus":        "us-central",
	"US Central":       "us-central",
	"northcentralus":   "us-central",
	"US North Central": "us-central",
	"southcentralus":   "us-central",
	"US South Central": "us-central",
	"westcentralus":    "us-central",
	"US West Central":  "us-central",

	// Canada
	"canadacentral": "ca-central",
	"Canada Central": "ca-central",
	"canadaeast":    "ca-central",
	"Canada East":   "ca-central",

	// Europe
	"northeurope":          "eu-north",
	"North Europe":         "eu-north",
	"westeurope":           "eu-west",
	"West Europe":          "eu-west",
	"uksouth":              "eu-west",
	"UK South":             "eu-west",
	"ukwest":               "eu-west",
	"UK West":              "eu-west",
	"francecentral":        "eu-west",
	"France Central":       "eu-west",
	"germanywestcentral":   "eu-central",
	"Germany West Central": "eu-central",
	"switzerlandnorth":     "eu-central",
	"Switzerland North":    "eu-central",
	"swedencentral":        "eu-north",
	"Sweden Central":       "eu-north",
	"norwayeast":           "eu-north",
	"Norway East":          "eu-north",
	"polandcentral":        "eu-central",
	"Poland Central":       "eu-central",
	"italynorth":           "eu-south",
	"Italy North":          "eu-south",
	"spaincentral":         "eu-south",
	"Spain Central":        "eu-south",

	// Asia Pacific
	"eastasia":           "ap-east",
	"East Asia":          "ap-east",
	"southeastasia":      "ap-southeast",
	"Southeast Asia":     "ap-southeast",
	"australiaeast":      "ap-southeast",
	"Australia East":     "ap-southeast",
	"australiasoutheast": "ap-southeast",
	"Australia Southeast": "ap-southeast",
	"centralindia":       "ap-south",
	"Central India":      "ap-south",
	"southindia":         "ap-south",
	"South India":        "ap-south",
	"westindia":          "ap-south",
	"West India":         "ap-south",
	"japaneast":          "ap-northeast",
	"Japan East":         "ap-northeast",
	"japanwest":          "ap-northeast",
	"Japan West":         "ap-northeast",
	"koreacentral":       "ap-northeast",
	"Korea Central":      "ap-northeast",
	"koreasouth":         "ap-northeast",
	"Korea South":        "ap-northeast",

	// South America
	"brazilsouth":  "sa-east",
	"Brazil South": "sa-east",

	// Africa
	"southafricanorth":   "af-south",
	"South Africa North": "af-south",

	// Middle East
	"uaenorth":       "me-central",
	"UAE North":      "me-central",
	"israelcentral":  "il-central",
	"Israel Central": "il-central",

	// Global
	"global": "global",
	"Global": "global",
}

var azureGroupToNativeMap = map[string]string{
	"us-east":   "eastus",
	"eastus":    "eastus",
	"US East":   "eastus",
	"eastus2":   "eastus2",
	"US East 2": "eastus2",
}

// MapAzureRegion resolves an Azure region code or location name to a normalized region group.
// It fails loudly with ErrUnmappedRegion if the region is not explicitly mapped.
func MapAzureRegion(region string) (string, error) {
	regionGroup, ok := azureRegionMap[region]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, region)
	}
	return regionGroup, nil
}

// ResolveAzureNativeRegion resolves a normalized region group to the primary Azure native region code.
func ResolveAzureNativeRegion(regionGroup string) (string, error) {
	native, ok := azureGroupToNativeMap[regionGroup]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedRegion, regionGroup)
	}
	return native, nil
}
