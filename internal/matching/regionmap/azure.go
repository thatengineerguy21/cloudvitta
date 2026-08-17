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
	"francesouth":          "eu-west",
	"France South":         "eu-west",
	"germanywestcentral":   "eu-central",
	"Germany West Central": "eu-central",
	"germanynorth":         "eu-central",
	"Germany North":        "eu-central",
	"switzerlandnorth":     "eu-central",
	"Switzerland North":    "eu-central",
	"switzerlandwest":      "eu-central",
	"Switzerland West":     "eu-central",
	"swedencentral":        "eu-north",
	"Sweden Central":       "eu-north",
	"norwayeast":           "eu-north",
	"Norway East":          "eu-north",
	"norwaywest":           "eu-north",
	"Norway West":          "eu-north",
	"polandcentral":        "eu-central",
	"Poland Central":       "eu-central",
	"italynorth":           "eu-south",
	"Italy North":          "eu-south",
	"spaincentral":         "eu-south",
	"Spain Central":        "eu-south",
	"austriaeast":          "eu-central",
	"Austria East":         "eu-central",

	// Asia Pacific
	"eastasia":            "ap-east",
	"East Asia":           "ap-east",
	"southeastasia":       "ap-southeast",
	"Southeast Asia":      "ap-southeast",
	"australiaeast":       "ap-southeast",
	"Australia East":      "ap-southeast",
	"australiasoutheast":  "ap-southeast",
	"Australia Southeast": "ap-southeast",
	"australiacentral":    "ap-southeast",
	"Australia Central":   "ap-southeast",
	"australiacentral2":   "ap-southeast",
	"Australia Central 2": "ap-southeast",
	"centralindia":        "ap-south",
	"Central India":       "ap-south",
	"southindia":          "ap-south",
	"South India":         "ap-south",
	"westindia":           "ap-south",
	"West India":          "ap-south",
	"jioindiacentral":     "ap-south",
	"Jio India Central":   "ap-south",
	"jioindiawest":        "ap-south",
	"Jio India West":      "ap-south",
	"japaneast":           "ap-northeast",
	"Japan East":          "ap-northeast",
	"japanwest":           "ap-northeast",
	"Japan West":          "ap-northeast",
	"koreacentral":        "ap-northeast",
	"Korea Central":       "ap-northeast",
	"koreasouth":          "ap-northeast",
	"Korea South":         "ap-northeast",
	"taiwannorth":         "ap-northeast",
	"Taiwan North":        "ap-northeast",
	"newzealandnorth":     "ap-southeast",
	"New Zealand North":   "ap-southeast",
	"indonesiacentral":    "ap-southeast",
	"Indonesia Central":   "ap-southeast",
	"malaysiawest":        "ap-southeast",
	"Malaysia West":       "ap-southeast",

	// South America
	"brazilsouth":      "sa-east",
	"Brazil South":     "sa-east",
	"brazilsoutheast":  "sa-east",
	"Brazil Southeast": "sa-east",
	"chilecentral":     "sa-east",
	"Chile Central":    "sa-east",

	// Africa
	"southafricanorth":   "af-south",
	"South Africa North": "af-south",
	"southafricawest":    "af-south",
	"South Africa West":  "af-south",

	// Middle East
	"uaenorth":       "me-central",
	"UAE North":      "me-central",
	"uaecentral":     "me-central",
	"UAE Central":    "me-central",
	"israelcentral":  "il-central",
	"Israel Central": "il-central",
	"qatarcentral":   "me-central",
	"Qatar Central":  "me-central",
	"saudicentral":   "me-central",
	"Saudi Central":  "me-central",

	// US Government & Security
	"usgovarizona":   "us-west",
	"USGov Arizona":  "us-west",
	"usgovtexas":     "us-central",
	"USGov Texas":    "us-central",
	"usgovvirginia":  "us-east",
	"USGov Virginia": "us-east",
	"usgoviowa":      "us-central",
	"USGov Iowa":     "us-central",
	"usdodeast":      "us-east",
	"USDoD East":     "us-east",
	"usdodcentral":   "us-central",
	"USDoD Central":  "us-central",
	"ussecwest":      "us-west",
	"USSec West":     "us-west",
	"usseceast":      "us-east",
	"USSec East":     "us-east",

	// Mexico & Others
	"mexicocentral":  "us-central",
	"Mexico Central": "us-central",

	// Edge Zones (AT&T)
	"attdetroit1":     "us-east",
	"ATT Detroit 1":   "us-east",
	"attdetroit":      "us-east",
	"attdallas1":      "us-central",
	"ATT Dallas 1":    "us-central",
	"attdallas":       "us-central",
	"attatlanta1":     "us-east",
	"ATT Atlanta 1":   "us-east",
	"attatlanta":      "us-east",
	"attlosangeles1":  "us-west",
	"ATT Los Angeles 1": "us-west",
	"attlosangeles":   "us-west",
	"attnewyork1":     "us-east",
	"ATT New York 1":  "us-east",
	"attnewyork":      "us-east",
	"attmiami1":       "us-east",
	"ATT Miami 1":     "us-east",
	"attmiami":        "us-east",
	"attchicago1":     "us-east",
	"ATT Chicago 1":   "us-east",
	"attchicago":      "us-east",
	"attphoenix1":     "us-west",
	"attseattle1":     "us-west",
	"attorlando1":     "us-east",

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
