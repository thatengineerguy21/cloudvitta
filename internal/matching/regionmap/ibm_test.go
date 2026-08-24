package regionmap

import (
	"errors"
	"testing"
)

func TestMapIBMRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantGroup string
		wantErr   error
	}{
		{
			name:      "valid us-east",
			region:    "us-east",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid US East (Washington DC)",
			region:    "US East (Washington DC)",
			wantGroup: "us-east",
			wantErr:   nil,
		},
		{
			name:      "valid us-south",
			region:    "us-south",
			wantGroup: "us-central",
			wantErr:   nil,
		},
		{
			name:      "valid ca-tor",
			region:    "ca-tor",
			wantGroup: "ca-central",
			wantErr:   nil,
		},
		{
			name:      "valid eu-de",
			region:    "eu-de",
			wantGroup: "eu-central",
			wantErr:   nil,
		},
		{
			name:      "valid eu-gb",
			region:    "eu-gb",
			wantGroup: "uk-south",
			wantErr:   nil,
		},
		{
			name:      "valid eu-es",
			region:    "eu-es",
			wantGroup: "eu-south",
			wantErr:   nil,
		},
		{
			name:      "valid jp-tok",
			region:    "jp-tok",
			wantGroup: "ap-northeast",
			wantErr:   nil,
		},
		{
			name:      "valid jp-osa",
			region:    "jp-osa",
			wantGroup: "ap-northeast",
			wantErr:   nil,
		},
		{
			name:      "valid au-syd",
			region:    "au-syd",
			wantGroup: "ap-southeast",
			wantErr:   nil,
		},
		{
			name:      "valid in-che",
			region:    "in-che",
			wantGroup: "ap-south",
			wantErr:   nil,
		},
		{
			name:      "valid br-sao",
			region:    "br-sao",
			wantGroup: "sa-east",
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
			gotGroup, err := MapIBMRegion(tt.region)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MapIBMRegion(%q) expected error %v, got nil", tt.region, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("MapIBMRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("MapIBMRegion(%q) unexpected error: %v", tt.region, err)
				}
				if gotGroup != tt.wantGroup {
					t.Errorf("MapIBMRegion(%q) = %q, want %q", tt.region, gotGroup, tt.wantGroup)
				}
			}
		})
	}
}

func TestResolveIBMNativeRegion(t *testing.T) {
	tests := []struct {
		group   string
		want    string
		wantErr bool
	}{
		{"us-east", "us-east", false},
		{"us-central", "us-south", false},
		{"ca-central", "ca-tor", false},
		{"eu-central", "eu-de", false},
		{"uk-south", "eu-gb", false},
		{"eu-south", "eu-es", false},
		{"ap-northeast", "jp-tok", false},
		{"ap-southeast", "au-syd", false},
		{"ap-south", "in-che", false},
		{"sa-east", "br-sao", false},
		{"invalid-group", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			got, err := ResolveIBMNativeRegion(tt.group)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveIBMNativeRegion(%q) expected error, got nil", tt.group)
				}
			} else {
				if err != nil {
					t.Errorf("ResolveIBMNativeRegion(%q) unexpected error: %v", tt.group, err)
				}
				if got != tt.want {
					t.Errorf("ResolveIBMNativeRegion(%q) = %q, want %q", tt.group, got, tt.want)
				}
			}
		})
	}
}

func TestKnownIBMRegions(t *testing.T) {
	known := KnownIBMRegions()
	if len(known) == 0 {
		t.Fatal("KnownIBMRegions() returned empty map")
	}
	if known["us-east"] != "us-east" {
		t.Errorf("expected us-east -> us-east, got %q", known["us-east"])
	}
}
