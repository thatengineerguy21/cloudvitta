package regionmap

import (
	"errors"
	"testing"
)

func TestMapDigitalOceanRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "valid nyc1",
			region:    "nyc1",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid nyc3",
			region:    "nyc3",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid New York 3",
			region:    "New York 3",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid sfo2",
			region:    "sfo2",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid sfo3",
			region:    "sfo3",
			wantGroup: "us-west",
			wantErr:   nil,
		},
		{
			name:      "valid fra1",
			region:    "fra1",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid ams3",
			region:    "ams3",
			wantGroup: "eu-west",
			wantErr:   nil,
		},
		{
			name:      "valid lon1",
			region:    "lon1",
			wantGroup: "uk-south",
			wantErr:   nil,
		},
		{
			name:      "valid sgp1",
			region:    "sgp1",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid syd1",
			region:    "syd1",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid blr1",
			region:    "blr1",
			wantGroup: "ap-south",
			wantErr:   nil,
		},
		{
			name:      "valid tor1",
			region:    "tor1",
			wantGroup: "ca-central",
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
			gotGroup, err := MapDigitalOceanRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapDigitalOceanRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapDigitalOceanRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapDigitalOceanRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapDigitalOceanRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestResolveDigitalOceanNativeRegion(t *testing.T) {
	tests := []struct {
		group   string
		want    string
		wantErr bool
	}{
		{"us-east", "nyc3", false},
		{"us-west", "sfo3", false},
		{"eu-central", "fra1", false},
		{"eu-west", "ams3", false},
		{"uk-south", "lon1", false},
		{"ap-southeast", "sgp1", false},
		{"ap-south", "blr1", false},
		{"ca-central", "tor1", false},
		{"invalid-group", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			got, err := ResolveDigitalOceanNativeRegion(tt.group)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveDigitalOceanNativeRegion(%q) expected error, got nil", tt.group)
				}
			} else {
				if err != nil {
					t.Errorf("ResolveDigitalOceanNativeRegion(%q) unexpected error: %v", tt.group, err)
				}
				if got != tt.want {
					t.Errorf("ResolveDigitalOceanNativeRegion(%q) = %q, want %q", tt.group, got, tt.want)
				}
			}
		})
	}
}

func TestKnownDigitalOceanRegions(t *testing.T) {
	known := KnownDigitalOceanRegions()
	if len(known) == 0 {
		t.Fatal("KnownDigitalOceanRegions() returned empty map")
	}
	if known["nyc1"] != "us-east" {
		t.Errorf("expected nyc1 -> us-east, got %q", known["nyc1"])
	}
	if known["sfo3"] != "us-west" {
		t.Errorf("expected sfo3 -> us-west, got %q", known["sfo3"])
	}
}
