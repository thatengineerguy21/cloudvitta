package regionmap

import (
	"errors"
	"testing"
)

func TestMapOracleRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "valid us-ashburn-1",
			region:    "us-ashburn-1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid US East (Ashburn)",
			region:    "US East (Ashburn)",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid us-phoenix-1",
			region:    "us-phoenix-1",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid us-chicago-1",
			region:    "us-chicago-1",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "valid eu-frankfurt-1",
			region:    "eu-frankfurt-1",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid eu-amsterdam-1",
			region:    "eu-amsterdam-1",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid uk-london-1",
			region:    "uk-london-1",
			wantGroup: "uk-south",
			wantErr:   nil,
		},
		{
			name:      "valid ap-tokyo-1",
			region:    "ap-tokyo-1",
			wantGroup: "ap-northeast",
			wantErr:   nil,
		},
		{
			name:      "valid ap-singapore-1",
			region:    "ap-singapore-1",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid sa-saopaulo-1",
			region:    "sa-saopaulo-1",
			wantGroup: "sa-east",
			wantErr:   nil,
		},
		{
			name:      "valid me-dubai-1",
			region:    "me-dubai-1",
			wantGroup: "me-central",
			wantErr:   nil,
		},
		{
			name:      "unmapped region fails loudly",
			region:    "moon-crater-1",
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
			gotGroup, err := MapOracleRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapOracleRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapOracleRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapOracleRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapOracleRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestResolveOracleNativeRegion(t *testing.T) {
	tests := []struct {
		group   string
		want    string
		wantErr bool
	}{
		{"us-east", "us-ashburn-1", false},
		{"us-west", "us-phoenix-1", false},
		{"us-central", "us-chicago-1", false},
		{"eu-central", "eu-frankfurt-1", false},
		{"invalid-group", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			got, err := ResolveOracleNativeRegion(tt.group)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveOracleNativeRegion(%q) expected error, got nil", tt.group)
				}
			} else {
				if err != nil {
					t.Errorf("ResolveOracleNativeRegion(%q) unexpected error: %v", tt.group, err)
				}
				if got != tt.want {
					t.Errorf("ResolveOracleNativeRegion(%q) = %q, want %q", tt.group, got, tt.want)
				}
			}
		})
	}
}

func TestKnownOracleRegions(t *testing.T) {
	known := KnownOracleRegions()
	if len(known) == 0 {
		t.Fatal("KnownOracleRegions() returned empty map")
	}
	if known["us-ashburn-1"] != "us-east" {
		t.Errorf("expected us-ashburn-1 -> us-east, got %q", known["us-ashburn-1"])
	}
}
