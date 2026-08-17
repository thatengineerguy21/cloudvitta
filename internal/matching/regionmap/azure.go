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
	"eastus3":   "us-east",
	"US East 3": "us-east",

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
	"canadacentral":  "ca-central",
	"Canada Central": "ca-central",
	"canadaeast":     "ca-central",
	"Canada East":    "ca-central",

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
	"germanycentral":       "eu-central",
	"Germany Central":      "eu-central",
	"germanynortheast":     "eu-central",
	"Germany Northeast":    "eu-central",
	"switzerlandnorth":     "eu-central",
	"Switzerland North":    "eu-central",
	"switzerlandwest":      "eu-central",
	"Switzerland West":     "eu-central",
	"swedencentral":        "eu-north",
	"Sweden Central":       "eu-north",
	"swedensouth":          "eu-north",
	"Sweden South":         "eu-north",
	"denmarkeast":          "eu-north",
	"Denmark East":         "eu-north",
	"finlandcentral":       "eu-north",
	"Finland Central":      "eu-north",
	"norwayeast":           "eu-north",
	"Norway East":          "eu-north",
	"norwaywest":           "eu-north",
	"Norway West":          "eu-north",
	"polandcentral":        "eu-central",
	"Poland Central":       "eu-central",
	"italynorth":           "eu-south",
	"Italy North":          "eu-south",
	"italycentral":         "eu-south",
	"Italy Central":        "eu-south",
	"spaincentral":         "eu-south",
	"Spain Central":        "eu-south",
	"spainsouth":           "eu-south",
	"Spain South":          "eu-south",
	"greececentral":        "eu-south",
	"Greece Central":       "eu-south",
	"belgiumcentral":       "eu-west",
	"Belgium Central":      "eu-west",
	"austriaeast":          "eu-central",
	"Austria East":         "eu-central",

	// Asia Pacific
	"eastasia":             "ap-east",
	"East Asia":            "ap-east",
	"southeastasia":        "ap-southeast",
	"Southeast Asia":       "ap-southeast",
	"australiaeast":        "ap-southeast",
	"Australia East":       "ap-southeast",
	"australiasoutheast":   "ap-southeast",
	"Australia Southeast":  "ap-southeast",
	"australiacentral":     "ap-southeast",
	"Australia Central":    "ap-southeast",
	"australiacentral2":    "ap-southeast",
	"Australia Central 2":  "ap-southeast",
	"centralindia":         "ap-south",
	"Central India":        "ap-south",
	"southindia":           "ap-south",
	"South India":          "ap-south",
	"westindia":            "ap-south",
	"West India":           "ap-south",
	"indiasouthcentral":    "ap-south",
	"South Central India":  "ap-south",
	"India":                "ap-south",
	"jioindiacentral":      "ap-south",
	"Jio India Central":    "ap-south",
	"jioindiawest":         "ap-south",
	"Jio India West":       "ap-south",
	"japaneast":            "ap-northeast",
	"Japan East":           "ap-northeast",
	"japanwest":            "ap-northeast",
	"Japan West":           "ap-northeast",
	"koreacentral":         "ap-northeast",
	"Korea Central":        "ap-northeast",
	"koreasouth":           "ap-northeast",
	"Korea South":          "ap-northeast",
	"taiwannorth":          "ap-northeast",
	"Taiwan North":         "ap-northeast",
	"taiwannorthcentral":   "ap-northeast",
	"Taiwan North Central": "ap-northeast",
	"newzealandnorth":      "ap-southeast",
	"New Zealand North":    "ap-southeast",
	"indonesiacentral":     "ap-southeast",
	"Indonesia Central":    "ap-southeast",
	"malaysiawest":         "ap-southeast",
	"Malaysia West":        "ap-southeast",
	"malaysiasouth":        "ap-southeast",
	"Malaysia South":       "ap-southeast",

	// China Sovereign
	"chinaeast":     "ap-east",
	"China East":    "ap-east",
	"chinaeast2":    "ap-east",
	"China East 2":  "ap-east",
	"chinaeast3":    "ap-east",
	"China East 3":  "ap-east",
	"chinanorth":    "ap-east",
	"China North":   "ap-east",
	"chinanorth2":   "ap-east",
	"China North 2": "ap-east",
	"chinanorth3":   "ap-east",
	"China North 3": "ap-east",

	// South America
	"brazilsouth":      "sa-east",
	"Brazil South":     "sa-east",
	"brazilsoutheast":  "sa-east",
	"Brazil Southeast": "sa-east",
	"brazilne":         "sa-east",
	"brazilnortheast":  "sa-east",
	"Brazil Northeast": "sa-east",
	"chilecentral":     "sa-east",
	"Chile Central":    "sa-east",
	"chilesouth":       "sa-east",
	"Chile South":      "sa-east",

	// Africa
	"southafricanorth":   "af-south",
	"South Africa North": "af-south",
	"southafricawest":    "af-south",
	"South Africa West":  "af-south",

	// Middle East
	"uaenorth":             "me-central",
	"UAE North":            "me-central",
	"uaecentral":           "me-central",
	"UAE Central":          "me-central",
	"israelcentral":        "il-central",
	"Israel Central":       "il-central",
	"israelnorthwest":      "il-central",
	"Israel Northwest":     "il-central",
	"qatarcentral":         "me-central",
	"Qatar Central":        "me-central",
	"saudicentral":         "me-central",
	"Saudi Central":        "me-central",
	"saudiarabiacentral":   "me-central",
	"Saudi Arabia Central": "me-central",

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
	"mexicoeast":     "us-central",
	"Mexico East":    "us-central",

	// Edge Zones (AT&T / Singtel)
	"attdetroit1":       "us-east",
	"ATT Detroit 1":     "us-east",
	"attdetroit":        "us-east",
	"attdallas1":        "us-central",
	"ATT Dallas 1":      "us-central",
	"attdallas":         "us-central",
	"attatlanta1":       "us-east",
	"ATT Atlanta 1":     "us-east",
	"attatlanta":        "us-east",
	"attlosangeles1":    "us-west",
	"ATT Los Angeles 1": "us-west",
	"attlosangeles":     "us-west",
	"attnewyork1":       "us-east",
	"ATT New York 1":    "us-east",
	"attnewyork":        "us-east",
	"attmiami1":         "us-east",
	"ATT Miami 1":       "us-east",
	"attmiami":          "us-east",
	"attchicago1":       "us-east",
	"ATT Chicago 1":     "us-east",
	"attchicago":        "us-east",
	"attphoenix1":       "us-west",
	"attseattle1":       "us-west",
	"attorlando1":       "us-east",
	"sgxsingapore1":     "ap-southeast",
	"SGX Singapore 1":   "ap-southeast",
	"sgxsingapore":      "ap-southeast",

	// Macro Regions (Network Transfer / Egress)
	"Oceania":       "ap-southeast",
	"oceania":       "ap-southeast",
	"Asia":          "ap-east",
	"asia":          "ap-east",
	"Asia Pacific":  "ap-east",
	"Asia-Pacific":  "ap-east",
	"Europe":        "eu-west",
	"europe":        "eu-west",
	"North America":          "us-east",
	"South America":          "sa-east",
	"Middle East":            "me-central",
	"Middle East And Africa": "me-central",
	"Middle East and Africa": "me-central",
	"MEA":                    "me-central",
	"Africa":                 "af-south",
	"US":            "us-east",
	"United States": "us-east",
	"Canada":        "ca-central",
	"Latin America": "sa-east",
	"LATAM":         "sa-east",

	// Global & Transfer
	"global":           "global",
	"Global":           "global",
	"Any":              "global",
	"Inter-Region":     "global",
	"Intra-Region":     "global",
	"Intercontinental": "global",
	"intercontinental": "global",
}

var azureGroupToNativeMap = map[string]string{
	"us-east":      "eastus",
	"eastus":       "eastus",
	"US East":      "eastus",
	"eastus2":      "eastus2",
	"US East 2":    "eastus2",
	"us-central":   "centralus",
	"us-west":      "westus2",
	"ca-central":   "canadacentral",
	"eu-west":      "westeurope",
	"eu-central":   "germanywestcentral",
	"eu-north":     "northeurope",
	"eu-south":     "italynorth",
	"ap-east":      "eastasia",
	"ap-south":     "centralindia",
	"ap-southeast": "southeastasia",
	"ap-northeast": "japaneast",
	"sa-east":      "brazilsouth",
	"me-central":   "uaenorth",
	"il-central":   "israelcentral",
	"af-south":     "southafricanorth",
	"global":       "global",
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

// KnownAzureRegions returns a copy of known Azure region mapping pairs.
func KnownAzureRegions() map[string]string {
	m := make(map[string]string, len(azureRegionMap))
	for k, v := range azureRegionMap {
		m[k] = v
	}
	return m
}
