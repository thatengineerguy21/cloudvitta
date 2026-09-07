package regionmap

import (
	"testing"
)

func TestTargetHubs_Count(t *testing.T) {
	hubs := TargetHubs()
	if len(hubs) != 8 {
		t.Fatalf("expected 8 target hubs, got %d", len(hubs))
	}
}

func TestTargetHubs_BidirectionalMapping(t *testing.T) {
	hubs := TargetHubs()
	for _, hub := range hubs {
		t.Run(hub.HubID, func(t *testing.T) {
			awsGroup, err := MapAWSRegion(hub.AWSRegion)
			if err != nil {
				t.Fatalf("AWS region %q failed to map: %v", hub.AWSRegion, err)
			}
			if awsGroup != hub.RegionGroup {
				t.Errorf("AWS region %q mapped to %q, expected %q", hub.AWSRegion, awsGroup, hub.RegionGroup)
			}

			azureGroup, err := MapAzureRegion(hub.AzureRegion)
			if err != nil {
				t.Fatalf("Azure region %q failed to map: %v", hub.AzureRegion, err)
			}
			if azureGroup != hub.RegionGroup {
				t.Errorf("Azure region %q mapped to %q, expected %q", hub.AzureRegion, azureGroup, hub.RegionGroup)
			}

			gcpGroup, err := MapGCPRegion(hub.GCPRegion)
			if err != nil {
				t.Fatalf("GCP region %q failed to map: %v", hub.GCPRegion, err)
			}
			if gcpGroup != hub.RegionGroup {
				t.Errorf("GCP region %q mapped to %q, expected %q", hub.GCPRegion, gcpGroup, hub.RegionGroup)
			}
		})
	}
}

func TestTargetRegions_Lists(t *testing.T) {
	aws := TargetAWSRegions()
	if len(aws) != 8 {
		t.Errorf("expected 8 AWS regions, got %d", len(aws))
	}

	azure := TargetAzureRegions()
	if len(azure) != 8 {
		t.Errorf("expected 8 Azure regions, got %d", len(azure))
	}

	gcp := TargetGCPRegions()
	if len(gcp) != 8 {
		t.Errorf("expected 8 GCP regions, got %d", len(gcp))
	}
}

func TestIsTargetRegion(t *testing.T) {
	for _, r := range TargetAWSRegions() {
		if !IsTargetRegion("aws", r) {
			t.Errorf("expected AWS region %q to be target region", r)
		}
	}
	if IsTargetRegion("aws", "us-nonexistent-1") {
		t.Errorf("did not expect us-nonexistent-1 to be AWS target region")
	}
	if !IsTargetRegion("aws", "global") {
		t.Errorf("expected global to be target region for aws")
	}

	for _, r := range TargetAzureRegions() {
		if !IsTargetRegion("azure", r) {
			t.Errorf("expected Azure region %q to be target region", r)
		}
	}
	if !IsTargetRegion("azure", "westus") {
		t.Errorf("expected westus alias to be target region for azure")
	}
	if IsTargetRegion("azure", "brazilsouth") {
		t.Errorf("did not expect brazilsouth to be Azure 8-hub target region")
	}
	if !IsTargetRegion("azure", "global") {
		t.Errorf("expected global to be target region for azure")
	}

	for _, r := range TargetGCPRegions() {
		if !IsTargetRegion("gcp", r) {
			t.Errorf("expected GCP region %q to be target region", r)
		}
	}
	if !IsTargetRegion("gcp", "us-east1") {
		t.Errorf("expected us-east1 alias to be target region for gcp")
	}
	if IsTargetRegion("gcp", "europe-north1") {
		t.Errorf("did not expect europe-north1 to be GCP 8-hub target region")
	}
	if !IsTargetRegion("gcp", "global") {
		t.Errorf("expected global to be target region for gcp")
	}

	if IsTargetRegion("unmapped_provider", "us-east-1") {
		t.Errorf("did not expect unmapped provider to return true")
	}
}

func TestTargetHubs_HubIDResolution(t *testing.T) {
	for _, hub := range TargetHubs() {
		// MapRegion by HubID
		for _, prov := range []string{"aws", "azure", "gcp"} {
			group, err := MapRegion(prov, hub.HubID)
			if err != nil {
				t.Fatalf("MapRegion(%s, %s) failed: %v", prov, hub.HubID, err)
			}
			if group != hub.RegionGroup {
				t.Errorf("MapRegion(%s, %s) = %q, expected %q", prov, hub.HubID, group, hub.RegionGroup)
			}
		}

		// ResolveNativeRegion by HubID
		awsNative, err := ResolveNativeRegion("aws", hub.HubID)
		if err != nil || awsNative != hub.AWSRegion {
			t.Errorf("ResolveNativeRegion(aws, %s) = %q, err=%v, expected %q", hub.HubID, awsNative, err, hub.AWSRegion)
		}
		azureNative, err := ResolveNativeRegion("azure", hub.HubID)
		if err != nil || azureNative != hub.AzureRegion {
			t.Errorf("ResolveNativeRegion(azure, %s) = %q, err=%v, expected %q", hub.HubID, azureNative, err, hub.AzureRegion)
		}
		gcpNative, err := ResolveNativeRegion("gcp", hub.HubID)
		if err != nil || gcpNative != hub.GCPRegion {
			t.Errorf("ResolveNativeRegion(gcp, %s) = %q, err=%v, expected %q", hub.HubID, gcpNative, err, hub.GCPRegion)
		}
	}
}
