package regionmap

import (
	"errors"
	"testing"
)

func TestMapAzureRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "armRegionName eastus",
			region:    "eastus",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "location US East",
			region:    "US East",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "armRegionName eastus2",
			region:    "eastus2",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid region westeurope",
			region:    "westeurope",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid region uaecentral",
			region:    "uaecentral",
			wantGroup: "me-central",
			wantErr:   nil,
		},
		{
			name:      "valid location UAE Central",
			region:    "UAE Central",
			wantGroup: "me-central",
			wantErr:   nil,
		},
		{
			name:      "valid region usgovarizona",
			region:    "usgovarizona",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid location USGov Arizona",
			region:    "USGov Arizona",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid region norwaywest",
			region:    "norwaywest",
			wantGroup: "eu-north",
			wantErr:   nil,
		},
		{
			name:      "valid location Norway West",
			region:    "Norway West",
			wantGroup: "eu-north",
			wantErr:   nil,
		},
		{
			name:      "valid edge region attdetroit1",
			region:    "attdetroit1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid edge region sgxsingapore1",
			region:    "sgxsingapore1",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid macro region Oceania",
			region:    "Oceania",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "unmapped region fails loudly",
			region:    "unknown-azure-region-99",
			wantGroup: "",
			wantErr:   ErrUnmappedRegion,
		},
		{
			name:      "empty region fails loudly",
			region:    "",
			wantGroup: "",
			wantErr:   ErrUnmappedRegion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGroup, err := MapAzureRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapAzureRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapAzureRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapAzureRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapAzureRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestMapRegion_Azure(t *testing.T) {
	group, err := MapRegion("azure", "eastus")
	if err != nil {
		t.Fatalf("MapRegion(azure, eastus) unexpected error: %v", err)
	}
	if group != "us-east" {
		t.Errorf("MapRegion(azure, eastus) = %q, want %q", group, "us-east")
	}

	_, err = MapRegion("azure", "unknown-region")
	if !errors.Is(err, ErrUnmappedRegion) {
		t.Errorf("MapRegion(azure, unknown-region) expected ErrUnmappedRegion, got: %v", err)
	}
}
