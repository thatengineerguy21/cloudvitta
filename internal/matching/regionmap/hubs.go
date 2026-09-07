package regionmap

import "strings"

// TargetHub represents one of the eight strategic global infrastructure hubs.
type TargetHub struct {
	HubID       string `json:"hub_id"`
	DisplayName string `json:"display_name"`
	RegionGroup string `json:"region_group"`
	AWSRegion   string `json:"aws_region"`
	AzureRegion string `json:"azure_region"`
	GCPRegion   string `json:"gcp_region"`
}

var targetHubs = []TargetHub{
	{
		HubID:       "us-east-virginia",
		DisplayName: "US East (Virginia)",
		RegionGroup: "us-east",
		AWSRegion:   "us-east-1",
		AzureRegion: "eastus",
		GCPRegion:   "us-east4",
	},
	{
		HubID:       "us-west-oregon",
		DisplayName: "US West (Oregon)",
		RegionGroup: "us-west",
		AWSRegion:   "us-west-2",
		AzureRegion: "westus2",
		GCPRegion:   "us-west1",
	},
	{
		HubID:       "europe-frankfurt",
		DisplayName: "Europe (Frankfurt)",
		RegionGroup: "eu-central",
		AWSRegion:   "eu-central-1",
		AzureRegion: "germanywestcentral",
		GCPRegion:   "europe-west3",
	},
	{
		HubID:       "uk-london",
		DisplayName: "UK (London)",
		RegionGroup: "eu-west",
		AWSRegion:   "eu-west-2",
		AzureRegion: "uksouth",
		GCPRegion:   "europe-west2",
	},
	{
		HubID:       "asia-singapore",
		DisplayName: "Asia (Singapore)",
		RegionGroup: "ap-southeast",
		AWSRegion:   "ap-southeast-1",
		AzureRegion: "southeastasia",
		GCPRegion:   "asia-southeast1",
	},
	{
		HubID:       "asia-tokyo",
		DisplayName: "Asia (Tokyo)",
		RegionGroup: "ap-northeast",
		AWSRegion:   "ap-northeast-1",
		AzureRegion: "japaneast",
		GCPRegion:   "asia-northeast1",
	},
	{
		HubID:       "india-mumbai",
		DisplayName: "India (Mumbai)",
		RegionGroup: "ap-south",
		AWSRegion:   "ap-south-1",
		AzureRegion: "centralindia",
		GCPRegion:   "asia-south1",
	},
	{
		HubID:       "australia-sydney",
		DisplayName: "Australia (Sydney)",
		RegionGroup: "ap-southeast",
		AWSRegion:   "ap-southeast-2",
		AzureRegion: "australiaeast",
		GCPRegion:   "australia-southeast1",
	},
}

// TargetHubs returns a copy of the eight strategic global infrastructure hubs.
func TargetHubs() []TargetHub {
	out := make([]TargetHub, len(targetHubs))
	copy(out, targetHubs)
	return out
}

// TargetAWSRegions returns the native AWS region identifiers for all eight target hubs.
func TargetAWSRegions() []string {
	out := make([]string, len(targetHubs))
	for i, h := range targetHubs {
		out[i] = h.AWSRegion
	}
	return out
}

// TargetAzureRegions returns the native Azure region identifiers for all eight target hubs.
func TargetAzureRegions() []string {
	out := make([]string, len(targetHubs))
	for i, h := range targetHubs {
		out[i] = h.AzureRegion
	}
	return out
}

// TargetGCPRegions returns the native GCP region identifiers for all eight target hubs.
func TargetGCPRegions() []string {
	out := make([]string, len(targetHubs))
	for i, h := range targetHubs {
		out[i] = h.GCPRegion
	}
	return out
}

var (
	targetAWSSet = map[string]struct{}{
		"global": {},
	}
	targetAzureSet = map[string]struct{}{
		"global": {},
		"westus": {},
	}
	targetGCPSet = map[string]struct{}{
		"global":      {},
		"us-east1":    {},
		"us-central1": {},
	}
)

func init() {
	for _, h := range targetHubs {
		targetAWSSet[strings.ToLower(h.AWSRegion)] = struct{}{}
		targetAzureSet[strings.ToLower(h.AzureRegion)] = struct{}{}
		targetGCPSet[strings.ToLower(h.GCPRegion)] = struct{}{}

		// Register hub ID in provider region maps
		awsRegionMap[h.HubID] = h.RegionGroup
		azureRegionMap[h.HubID] = h.RegionGroup
		gcpRegionMap[h.HubID] = h.RegionGroup

		// Register hub ID in provider group-to-native maps
		awsGroupToNativeMap[h.HubID] = h.AWSRegion
		azureGroupToNativeMap[h.HubID] = h.AzureRegion
		gcpGroupToNativeMap[h.HubID] = h.GCPRegion
	}
	// Sydney native cross-provider aliases
	azureGroupToNativeMap["ap-southeast-2"] = "australiaeast"
	gcpGroupToNativeMap["ap-southeast-2"] = "australia-southeast1"
}

// IsTargetRegion reports whether a region belongs to the target hub scope for the provider.
func IsTargetRegion(provider, region string) bool {
	r := strings.ToLower(strings.TrimSpace(region))
	switch strings.ToLower(provider) {
	case "aws":
		_, ok := targetAWSSet[r]
		return ok
	case "azure":
		_, ok := targetAzureSet[r]
		return ok
	case "gcp":
		_, ok := targetGCPSet[r]
		return ok
	default:
		return false
	}
}
