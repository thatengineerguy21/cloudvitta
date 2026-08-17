package regionmap

import (
	"errors"
	"testing"
)

func TestMapGCPRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "region us-east1",
			region:    "us-east1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "region us-east4",
			region:    "us-east4",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "region us-central1",
			region:    "us-central1",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "region us-west1",
			region:    "us-west1",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid region europe-west1",
			region:    "europe-west1",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid region asia1",
			region:    "asia1",
			wantGroup: "ap-east",
			wantErr:   nil,
		},
		{
			name:      "valid region europe-north2",
			region:    "europe-north2",
			wantGroup: "eu-north",
			wantErr:   nil,
		},
		{
			name:      "valid region eur5",
			region:    "eur5",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid multi-region europe",
			region:    "europe",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid region northamerica-south1",
			region:    "northamerica-south1",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "unmapped region fails loudly",
			region:    "unknown-gcp-region-99",
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
			gotGroup, err := MapGCPRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapGCPRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapGCPRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapGCPRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapGCPRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestMapRegion_GCP(t *testing.T) {
	group, err := MapRegion("gcp", "us-east1")
	if err != nil {
		t.Fatalf("MapRegion(gcp, us-east1) unexpected error: %v", err)
	}
	if group != "us-east" {
		t.Errorf("MapRegion(gcp, us-east1) = %q, want %q", group, "us-east")
	}

	_, err = MapRegion("gcp", "unknown-gcp-region")
	if !errors.Is(err, ErrUnmappedRegion) {
		t.Errorf("MapRegion(gcp, unknown-gcp-region) expected ErrUnmappedRegion, got: %v", err)
	}
}
